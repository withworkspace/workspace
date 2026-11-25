package web

import (
	"encoding/json"
	"errors"
	"hash/fnv"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/pulls"
	"github.com/withworkspace/workspace/internal/review"
)

// The Reviews page: the open pull requests across the workspace's repos,
// and reviewing one (or a stack of them) with the review a capsule gets,
// read from commits fetched into the repo's clone. Drafts, viewed files
// and comments go through the same handlers as a capsule's (review.go),
// under /api/prs/{repo}/{number}; what differs is here: the list, a PR in
// full, its diff by scope, submitting (which posts to GitHub), GitHub's
// threads, asking an agent, and docking.

// ReviewsCount is what the sidebar says of the Reviews page: how many PRs
// need you, as GitHub last said. Ready is how many of your own are ready
// to merge (its Ready to merge section, a stack one), for the foot.
type ReviewsCount struct {
	Count int       `json:"count"`
	Ready int       `json:"ready,omitempty"`
	At    time.Time `json:"at"`
}

// ReviewsList is GET /api/reviews.
type ReviewsList struct {
	*pulls.List
	// Stale: GitHub hasn't been asked lately (the page's socket asks,
	// and says when the list has changed). Error is why it couldn't be.
	Stale bool   `json:"stale,omitempty"`
	Error string `json:"error,omitempty"`
	// Passes are how each pass that's been run over a pull request stands
	// there, by pull request (api#89): what a row says before it's opened.
	Passes map[string][]ops.PassMark `json:"passes,omitempty"`
}

// listHeads are each listed pull request's head, by its ref.
func listHeads(l *pulls.List) map[string]string {
	heads := map[string]string{}
	if l == nil {
		return heads
	}
	for _, sec := range l.Sections {
		for _, row := range sec.Rows {
			if row.PR != nil {
				heads[row.PR.Ref()] = row.PR.Head
			}
			if row.Stack != nil {
				for _, layer := range row.Stack.Layers {
					heads[layer.Ref()] = layer.Head
				}
			}
		}
	}
	return heads
}

// PRDetail is GET /api/prs/{repo}/{number}.
type PRDetail struct {
	*pulls.Detail
	// Talks are the conversations about it, newest first.
	Talks []Talk `json:"talks"`
	// NoAsk is why an agent can't be asked about it (a fork's can't be
	// docked), when it can't.
	NoAsk string `json:"noAsk,omitempty"`
	// Unsent is what was said from here that GitHub didn't take (a reply,
	// a comment, resolving a thread): to send again, or let go of.
	Unsent []pulls.Unsent `json:"unsent,omitempty"`
}

// PRQuestion is what's asked about a PR: the question, and the lines of
// its diff it's about, if any.
type PRQuestion struct {
	Body    string `json:"body"`
	Path    string `json:"path"`
	Side    string `json:"side"`
	Line    int    `json:"line"`
	EndLine int    `json:"endLine"`
	Commit  string `json:"commit"`
}

// reviewsFresh is how long a list answers before it's called stale.
const reviewsFresh = 2 * time.Minute

func (s *Server) noReviews(w http.ResponseWriter) bool {
	if s.Reviews == nil {
		jsonError(w, http.StatusNotFound, "there's no GitHub to ask here")
		return true
	}
	return false
}

func (s *Server) reviewsList(w http.ResponseWriter, r *http.Request) {
	if s.noReviews(w) {
		return
	}
	out := ReviewsList{}
	var err error
	if r.URL.Query().Get("refresh") != "" {
		out.List, err = s.Reviews.List(true)
	} else if out.List = s.Reviews.Peek(); out.List == nil {
		out.List, err = s.Reviews.List(false)
	}
	if out.List == nil {
		msg := "the list couldn't be read"
		if err != nil {
			msg = err.Error()
		}
		jsonError(w, http.StatusBadGateway, msg)
		return
	}
	if err != nil {
		out.Error = err.Error()
	}
	out.Stale = s.now().Sub(out.List.At) > reviewsFresh
	out.Passes = s.App.PassMarks(listHeads(out.List))
	writeJSON(w, http.StatusOK, out)
}

// reviewsFingerprint says whether the list has changed. Taking it asks
// GitHub, if it hasn't been asked lately: an open Reviews page is what
// keeps the list fresh.
//
// It's also when passes set to run by themselves are started, one at a
// time: a pull request that needs you has them run while a Reviews page
// is open, and how they stand is part of what's changed.
func (s *Server) reviewsFingerprint() string {
	l, _ := s.Reviews.List(false)
	s.App.AutoPasses()
	return listFingerprint(l, s.App.PassMarks(listHeads(l)))
}

func listFingerprint(l *pulls.List, marks map[string][]ops.PassMark) string {
	if l == nil {
		return ""
	}
	data, _ := json.Marshal(struct {
		S []pulls.Section
		P []string
		M map[string][]ops.PassMark
	}{l.Sections, l.Problems, marks})
	h := fnv.New64a()
	h.Write(data)
	return strconv.FormatUint(h.Sum64(), 16)
}

// repoName is what a repo's name may be: it names folders.
var repoName = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// prOf reads the request's pull request: its repo and number.
func (s *Server) prOf(w http.ResponseWriter, r *http.Request) (repo string, number int, ok bool) {
	if s.noReviews(w) {
		return "", 0, false
	}
	repo = r.PathValue("repo")
	if !repoName.MatchString(repo) || strings.Trim(repo, ".") == "" {
		jsonError(w, http.StatusNotFound, "the workspace has no such repo")
		return "", 0, false
	}
	number, err := strconv.Atoi(r.PathValue("number"))
	if err != nil || number < 1 {
		jsonError(w, http.StatusNotFound, "pull requests are numbered: "+r.PathValue("number")+" isn't one")
		return "", 0, false
	}
	return repo, number, true
}

// prError answers for something a PR couldn't do: 404 for one that isn't
// open, else what was wrong.
func prError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, pulls.ErrNotOpen), errors.Is(err, review.ErrNotFound), errors.Is(err, conversation.ErrNotFound), errors.Is(err, ErrNotFound):
		jsonError(w, http.StatusNotFound, err.Error())
	default:
		jsonError(w, http.StatusUnprocessableEntity, err.Error())
	}
}

// prDir is where a PR's commits are read from: its repo's clone, and the
// branch it's for.
func (s *Server) prDir(w http.ResponseWriter, r *http.Request) (dir, baseRef string, ok bool) {
	repo, number, ok := s.prOf(w, r)
	if !ok {
		return "", "", false
	}
	pr, _, err := s.Reviews.Known(repo, number)
	if err != nil {
		prError(w, err)
		return "", "", false
	}
	return s.Reviews.BareDir(repo), pulls.BaseRef(pr.Base), true
}

func (s *Server) prDetail(w http.ResponseWriter, r *http.Request) {
	repo, number, ok := s.prOf(w, r)
	if !ok {
		return
	}
	d, err := s.Reviews.KeptDetail(repo, number)
	if err != nil {
		prError(w, err)
		return
	}
	out := PRDetail{Detail: d, Talks: PRTalks(s.Root, pulls.Key(repo, number)), Unsent: s.Reviews.Unsent(repo, number)}
	switch {
	case d.PR.Fork:
		out.NoAsk = "It's from a fork, which ws can't dock, so an agent can't read it here."
	case !s.App.AsksPRs():
		out.NoAsk = "Conversations aren't available here."
	}
	writeJSON(w, http.StatusOK, out)
}

// PRTalks are the workspace's conversations about a pull request, newest
// first.
func PRTalks(root, ref string) []Talk {
	store := conversation.ForWorkspace(root)
	return talksOf(store, store.AboutPR(ref))
}

func (s *Server) prDiff(w http.ResponseWriter, r *http.Request) {
	repo, number, ok := s.prOf(w, r)
	if !ok {
		return
	}
	scope := r.URL.Query().Get("scope")
	switch scope {
	case "":
		scope = pulls.ScopeLayer
	case pulls.ScopeLayer, pulls.ScopeSince, pulls.ScopeWhole:
	default:
		jsonError(w, http.StatusBadRequest, "scope is layer, since or whole")
		return
	}
	d, err := s.Reviews.KeptDiff(repo, number, scope)
	if err != nil {
		prError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, d)
}

type prSubmitBody struct {
	Head    string `json:"head"`
	Verdict string `json:"verdict"`
	Summary string `json:"summary"`
}

func (b prSubmitBody) ok(w http.ResponseWriter) bool {
	if b.Head != "" && !fullSHA.MatchString(b.Head) {
		jsonError(w, http.StatusBadRequest, "head is the full commit id you reviewed")
		return false
	}
	return true
}

// prPreview is what submitting would post to GitHub, and where, for the
// dialog to show before it does.
func (s *Server) prPreview(w http.ResponseWriter, r *http.Request) {
	repo, number, ok := s.prOf(w, r)
	var b prSubmitBody
	if !ok || !readBody(w, r, &b) || !b.ok(w) {
		return
	}
	p, err := s.Reviews.Posting(repo, number, b.Head, b.Verdict, b.Summary)
	if err != nil {
		prError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

// prSubmit posts the PR's draft to GitHub, as you.
func (s *Server) prSubmit(w http.ResponseWriter, r *http.Request) {
	repo, number, ok := s.prOf(w, r)
	var b prSubmitBody
	if !ok || !readBody(w, r, &b) || !b.ok(w) {
		return
	}
	sub, err := s.App.SubmitPR(repo, number, b.Head, b.Verdict, b.Summary)
	if err != nil {
		prError(w, err)
		return
	}
	st, err := s.reviewState(s.Reviews.Store(repo, number), pulls.Key(repo, number))
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	st.URL = sub.URL
	writeJSON(w, http.StatusOK, st)
}

// prThreads is everyone's comments on the PR's lines, from GitHub, as a
// review's comments: as kept, and read again behind when they're stale.
func (s *Server) prThreads(w http.ResponseWriter, r *http.Request) {
	repo, number, ok := s.prOf(w, r)
	if !ok {
		return
	}
	t, err := s.Reviews.KeptThreads(repo, number)
	if err != nil {
		prError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

// prRespond is you on one of GitHub's threads: a reply, or resolving or
// reopening it. It answers with the threads as they now show: GitHub's
// told behind.
func (s *Server) prRespond(w http.ResponseWriter, r *http.Request) {
	repo, number, ok := s.prOf(w, r)
	var b struct {
		What string `json:"what"` // reply, resolve, reopen
		Body string `json:"body"`
	}
	if !ok || !readBody(w, r, &b) {
		return
	}
	what := review.Response(b.What)
	if what != review.Answer && what != review.Resolve && what != review.Reopen {
		jsonError(w, http.StatusBadRequest, "what is reply, resolve or reopen")
		return
	}
	t, err := s.App.RespondPR(repo, number, r.PathValue("id"), what, b.Body)
	if err != nil {
		prError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

// prDiscussion is what's been said on the PR itself, on GitHub: reviews
// and comments, and where each reviewer stands. As kept, and read again
// behind when it's stale.
func (s *Server) prDiscussion(w http.ResponseWriter, r *http.Request) {
	repo, number, ok := s.prOf(w, r)
	if !ok {
		return
	}
	d, err := s.Reviews.KeptDiscussion(repo, number)
	if err != nil {
		prError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, d)
}

// prComment is you saying something on the PR itself. It answers with
// what's been said as it now shows: GitHub's told behind.
func (s *Server) prComment(w http.ResponseWriter, r *http.Request) {
	repo, number, ok := s.prOf(w, r)
	var b struct {
		Body string `json:"body"`
	}
	if !ok || !readBody(w, r, &b) {
		return
	}
	d, err := s.App.CommentPR(repo, number, b.Body)
	if err != nil {
		prError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, d)
}

// prUnsent is something said from the PR's page that GitHub didn't take:
// sent again (POST …/resend), or let go of (DELETE).
func (s *Server) prUnsent(w http.ResponseWriter, r *http.Request) {
	repo, number, ok := s.prOf(w, r)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, http.StatusNotFound, "nothing's waiting to be sent by that name")
		return
	}
	if r.Method == http.MethodDelete {
		err = s.App.DiscardPR(repo, number, id)
	} else {
		err = s.App.ResendPR(repo, number, id)
	}
	if err != nil {
		prError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, struct{}{})
}

// prDecline takes your name off the PR's reviewers on GitHub: it isn't
// yours to review. What needs you no longer lists it, from now.
func (s *Server) prDecline(w http.ResponseWriter, r *http.Request) {
	repo, number, ok := s.prOf(w, r)
	if !ok {
		return
	}
	if err := s.App.DeclinePR(repo, number); err != nil {
		prError(w, err)
		return
	}
	s.topics[topicWorkspace].nudge()
	writeJSON(w, http.StatusOK, struct{}{})
}

// prAsk starts an ask about the PR (and lines of it, if given), docking
// it first so the agent can read it: the review's ask bar. It answers the
// ask, as POST /api/asks does, to be followed and said to again as any
// conversation is, and kept with .../keep.
func (s *Server) prAsk(w http.ResponseWriter, r *http.Request) {
	repo, number, ok := s.prOf(w, r)
	var q PRQuestion
	if !ok || !readBody(w, r, &q) {
		return
	}
	if !s.App.AsksPRs() {
		jsonError(w, http.StatusNotFound, "conversations aren't available here: no agent here can answer")
		return
	}
	if strings.TrimSpace(q.Body) == "" {
		jsonError(w, http.StatusUnprocessableEntity, "say what you want to know")
		return
	}
	if q.Path != "" && (!validPath(q.Path) || q.Line < 1 || (q.Side != "new" && q.Side != "old") || !fullSHA.MatchString(q.Commit)) {
		jsonError(w, http.StatusBadRequest, "lines are a path, a side (new or old), a line and the commit they're at")
		return
	}
	c, err := s.App.AskPR(repo, number, ops.PRQuestion(q))
	if err != nil {
		prError(w, err)
		return
	}
	s.answerTalk(w, conversation.ForWorkspace(s.Root), c.ID, nil)
}

// prDock checks the PR out here as ws dock does: Open here.
func (s *Server) prDock(w http.ResponseWriter, r *http.Request) {
	repo, number, ok := s.prOf(w, r)
	var b struct{}
	if !ok || !readBody(w, r, &b) {
		return
	}
	said, err := s.App.DockPR(repo, number)
	if err != nil {
		prError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": said})
}

// prNoRespond answers for the route a capsule's review has that a PR's
// doesn't: its comments, once submitted, are GitHub's threads.
func (s *Server) prNoRespond(w http.ResponseWriter, r *http.Request) {
	jsonError(w, http.StatusNotFound, "a submitted comment on a pull request is on GitHub: answer its thread")
}
