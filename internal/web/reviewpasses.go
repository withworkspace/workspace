package web

import (
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/ops"
	pages "github.com/withworkspace/workspace/internal/page"
	"github.com/withworkspace/workspace/internal/pulls"
)

// Passes over a review (a pull request's, or a mission's capsule's): each
// pass as it stands (go or no go, what it said and found), running one,
// and settling what it found: taken into your draft, dismissed, or opened
// again.

// ReviewPass is a pass over a review, as the overview shows it: a page's
// pass, with how it stands and the commit it last ran at.
type ReviewPass struct {
	PagePass
	Standing pages.Standing `json:"standing"`
	// Ran is the commit it last read the change at; Moved (a page's pass's
	// word for it) is that the change has been pushed to since.
	Ran string `json:"ran,omitempty"`
}

// reviewPass is a pass as it stands over a review whose head is head.
func reviewPass(store conversation.Store, def pages.PassDef, c *conversation.Conversation, head string) ReviewPass {
	p := ReviewPass{PagePass: pagePass(store, def, c, nil), Standing: pages.ReviewStanding(def, c)}
	p.Moved = false
	if c != nil {
		if w := c.Written(); w != nil {
			p.Ran = w.Around
			p.Moved = w.Around != "" && head != "" && w.Around != head
		}
	}
	for i := range p.Findings {
		p.Findings[i].Blocks = def.Blocking(p.Findings[i].Kind) || p.Findings[i].Finding.Blocks
	}
	return p
}

// prHead is the pull request's head as ws last fetched it, or "".
func (s *Server) prHead(repo string, number int) string {
	if rec, err := pulls.Load(s.Root, repo, number); err == nil && rec != nil {
		return rec.Head
	}
	return ""
}

// passesOf reads what the request's passes are over (a pull request, or a
// mission's capsule), and its head as it is now, to say what's moved on.
func (s *Server) passesOf(w http.ResponseWriter, r *http.Request) (of ops.ReviewOf, head string, ok bool) {
	if r.PathValue("number") != "" {
		repo, number, ok := s.prOf(w, r)
		if !ok {
			return of, "", false
		}
		return ops.OfPR(repo, number), s.prHead(repo, number), true
	}
	dir, _, ok := s.capsuleDir(w, r)
	if !ok {
		return of, "", false
	}
	return ops.OfCapsule(r.PathValue("slug"), r.PathValue("repo"), r.PathValue("name")), ops.HeadOf(dir), true
}

// reviewPasses is every pass a review has, as it stands over this one.
func (s *Server) reviewPasses(w http.ResponseWriter, r *http.Request) {
	of, head, ok := s.passesOf(w, r)
	if !ok {
		return
	}
	store := conversation.ForWorkspace(s.Root)
	out := []ReviewPass{}
	for _, def := range s.App.ReviewPassesOf(of) {
		_, c, _ := s.App.ReviewPass(of, def.Name)
		// There's nothing to follow up on one you haven't reviewed.
		if def.Name == pages.FollowUp && c == nil && !s.App.ReviewedPR(of.Repo, of.PR) {
			continue
		}
		out = append(out, reviewPass(store, def, c, head))
	}
	writeJSON(w, http.StatusOK, map[string][]ReviewPass{"passes": out})
}

// runReviewPass runs a pass over the review, the first time or again.
func (s *Server) runReviewPass(w http.ResponseWriter, r *http.Request) {
	of, head, ok := s.passesOf(w, r)
	var b struct{}
	if !ok || !readBody(w, r, &b) {
		return
	}
	def, c, err := s.App.RunReviewPass(of, r.PathValue("pass"))
	if err != nil {
		prError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, reviewPass(conversation.ForWorkspace(s.Root), def, c, head))
}

// settleReviewFinding is you settling what a pass found: taken (it goes
// in your draft review, as the agent's), dismissed (and why), or open
// again.
func (s *Server) settleReviewFinding(w http.ResponseWriter, r *http.Request) {
	of, head, ok := s.passesOf(w, r)
	var b struct {
		State string `json:"state"` // taken, dismissed, open
		Why   string `json:"why"`
	}
	if !ok || !readBody(w, r, &b) {
		return
	}
	var (
		def pages.PassDef
		c   *conversation.Conversation
		err error
	)
	switch b.State {
	case "taken":
		def, c, err = s.App.TakeReviewFinding(of, r.PathValue("pass"), r.PathValue("key"))
	case conversation.FindingDismissed, conversation.FindingOpen:
		def, c, err = s.App.SettleReviewFinding(of, r.PathValue("pass"), r.PathValue("key"), b.State, b.Why)
	default:
		jsonError(w, http.StatusBadRequest, "a finding is taken, dismissed or open")
		return
	}
	if err != nil {
		prError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, reviewPass(conversation.ForWorkspace(s.Root), def, c, head))
}

// ReviewGuide is a review's guide, as the overview shows it: what the
// change does and the order to read it in, once it's been written, and
// whether it's being written now.
type ReviewGuide struct {
	ID      string     `json:"id,omitempty"` // its conversation, once asked for
	Running bool       `json:"running,omitempty"`
	Failed  string     `json:"failed,omitempty"`
	At      *time.Time `json:"at,omitempty"`
	// Ran is the commit it read the change at; Moved, that it's been
	// pushed to since.
	Ran   string                    `json:"ran,omitempty"`
	Moved bool                      `json:"moved,omitempty"`
	Guide *conversation.ReviewGuide `json:"guide,omitempty"`
	// While it's being written: since when (none while it waits to start),
	// what its agent was given, whether it's thinking before it writes, and
	// what it's written so far, read as the guide will be (Draft), where its
	// harness says as it goes.
	Since    *time.Time                `json:"since,omitempty"`
	Given    *conversation.GuideGiven  `json:"given,omitempty"`
	Thinking bool                      `json:"thinking,omitempty"`
	Draft    *conversation.ReviewGuide `json:"draft,omitempty"`
}

// reviewGuide is the guide as it stands over a review whose head is head.
func reviewGuide(store conversation.Store, c *conversation.Conversation, head string) ReviewGuide {
	g := ReviewGuide{}
	if c == nil {
		return g
	}
	g.ID, g.Running = c.ID, c.Answering || (c.NeedsAnswer() && c.Written() == nil)
	if w := c.Written(); w != nil && w.Guide != nil {
		at := w.At
		g.At, g.Guide, g.Ran = &at, w.Guide, w.Around
		g.Moved = w.Around != "" && head != "" && w.Around != head
	}
	if last := c.Last(); last.By == conversation.WS && !c.Answering {
		g.Failed = last.Body
	}
	if c.Answering {
		g.Since, g.Given = c.AnsweringSince, store.GuideGiven(c.ID)
		if data, err := os.ReadFile(store.RunPath(c.ID)); err == nil {
			rep := harness.ReadRun(string(data))
			g.Thinking, g.Draft = rep.Thinking, guideSoFar(rep.SoFar(), g.Given)
		}
	}
	return g
}

// guideSoFar is what the guide's agent has written so far (text), read as
// its reply will be: the files it may place are those it was given. Nil
// before it's written anything.
func guideSoFar(text string, given *conversation.GuideGiven) *conversation.ReviewGuide {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	in := func(string) bool { return true }
	if given != nil {
		files := map[string]bool{}
		for _, f := range given.Files {
			files[f] = true
		}
		in = func(p string) bool { return files[p] }
	}
	return conversation.ParseGuideSoFar(text, in)
}

// reviewGuideGet is the guide to the review.
func (s *Server) reviewGuideGet(w http.ResponseWriter, r *http.Request) {
	of, head, ok := s.passesOf(w, r)
	if !ok {
		return
	}
	_, c, _ := s.App.ReviewPass(of, pages.Guide)
	writeJSON(w, http.StatusOK, reviewGuide(conversation.ForWorkspace(s.Root), c, head))
}

// runReviewGuide has the guide written, the first time or again.
func (s *Server) runReviewGuide(w http.ResponseWriter, r *http.Request) {
	of, head, ok := s.passesOf(w, r)
	var b struct{}
	if !ok || !readBody(w, r, &b) {
		return
	}
	_, c, err := s.App.RunReviewPass(of, pages.Guide)
	if err != nil {
		prError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, reviewGuide(conversation.ForWorkspace(s.Root), c, head))
}
