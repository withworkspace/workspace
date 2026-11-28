package ops

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/withworkspace/workspace/internal/config"
	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/page"
	"github.com/withworkspace/workspace/internal/prompt"
	"github.com/withworkspace/workspace/internal/pulls"
	"github.com/withworkspace/workspace/internal/ref"
	"github.com/withworkspace/workspace/internal/review"
	"github.com/withworkspace/workspace/internal/workspace"
)

// Passes over a review: an agent reads the change a pull request makes
// and says what it finds, each finding on the lines it's about. A pass is
// one of the workspace's conversations, anchored to the pull request; run
// again, its agent is told what you settled. Taking a finding puts it in
// your draft review, as the agent's; nothing's posted until you submit.

// ReviewPassDefs are a workspace's passes over a review: the built-in
// ones, and its own ([reviews.passes.<name>], the team's and yours). A
// pass that's a skill the workspace hasn't got says so (Unavailable). cfg
// nil is no config: the built-in ones.
func ReviewPassDefs(cfg *config.Config, ws *workspace.Workspace) []page.PassDef {
	var own []page.PassDef
	if cfg != nil {
		for name, p := range cfg.Reviews.Passes {
			def := page.PassDef{Name: name, Title: p.Title, About: p.About, Looks: p.Looks, File: p.File, Skill: strings.TrimSpace(p.Skill), Args: strings.TrimSpace(p.Args),
				Kinds: p.Kinds, Blocks: p.Blocks, Cold: p.Cold, Repos: p.Repos, Auto: p.Auto}
			if def.Skill != "" {
				def.File, def.Unavailable = skillFile(ws, def.Skill)
			}
			own = append(own, def)
		}
	}
	return page.ReviewPasses(own)
}

// ReviewPasses are the passes a review in a repo has.
func (w *Workspace) ReviewPasses(repo string) []page.PassDef {
	return page.ForRepo(ReviewPassDefs(w.Config(), w.WS()), repo)
}

// GuideDef is the guide to a review, as the workspace wants it
// ([reviews.guide]).
func (w *Workspace) GuideDef() page.PassDef {
	if cfg := w.Config(); cfg != nil {
		return page.GuideDef(cfg.Reviews.Guide.Looks, cfg.Reviews.Guide.File, cfg.Reviews.Guide.Auto)
	}
	return page.GuideDef("", "", false)
}

// What a review's pass is over: a pull request, or a mission's capsule
// (ReviewOf, as a review is of either).

// reviewRepo is the repo what's reviewed is in.
func reviewRepo(of ReviewOf) string {
	if of.PR != 0 {
		return of.Repo
	}
	repo, _, _ := strings.Cut(of.Capsule, "/")
	return repo
}

// passOver is what a review's pass is over, as its conversation is
// anchored.
func passOver(of ReviewOf) conversation.Anchor {
	if of.PR != 0 {
		return conversation.Anchor{PR: pulls.Key(of.Repo, of.PR)}
	}
	return conversation.ReviewedCapsule(of.Mission, of.Capsule)
}

// reviewOfAnchor is what a pass's conversation is over, from its anchor.
func reviewOfAnchor(a conversation.Anchor) (ReviewOf, bool) {
	switch {
	case a.PR != "":
		repo, number, ok := ref.ParsePR(a.PR)
		return OfPR(repo, number), ok
	case a.Capsule != "" && a.Mission != "":
		return ReviewOf{Mission: a.Mission, Capsule: a.Capsule}, true
	}
	return ReviewOf{}, false
}

// ReviewPassesOf are the passes a review has: its repo's, and for a
// mission's capsule, without Follow-up, which is of comments on GitHub
// (crew answer a capsule's comments themselves).
func (w *Workspace) ReviewPassesOf(of ReviewOf) []page.PassDef {
	defs := w.ReviewPasses(reviewRepo(of))
	if of.PR != 0 {
		return defs
	}
	return slices.DeleteFunc(slices.Clone(defs), func(d page.PassDef) bool { return d.Name == page.FollowUp })
}

// reviewPassNamed is a review's pass by name: one of its passes, or the
// guide.
func (w *Workspace) reviewPassNamed(of ReviewOf, name string) (page.PassDef, bool) {
	if name == page.Guide {
		return w.GuideDef(), true
	}
	return page.PassNamed(w.ReviewPassesOf(of), name)
}

// ReviewPass is a pass over a review, by name (the guide is one): its
// definition and its conversation (nil if it's never been run).
func (w *Workspace) ReviewPass(of ReviewOf, name string) (page.PassDef, *conversation.Conversation, error) {
	def, ok := w.reviewPassNamed(of, name)
	if !ok {
		return def, nil, errors.Join(conversation.ErrNotFound, fmt.Errorf("reviews in %s have no pass named %s", reviewRepo(of), name))
	}
	return def, w.cells().PassOver(passOver(of), def.Name), nil
}

// change is what a review's pass reads: the commits a review is of, where
// they are, and what's said of them.
type change struct {
	of ReviewOf
	// ref names it (api#89; api/tokens), title is what it's called (the
	// pull request's; the mission's), and body what it says it does (the
	// pull request's description; the mission's brief).
	ref, title, body string
	// dir is where its commits are: the repo's clone, or the capsule.
	dir                 string
	baseRef, base, head string
	// A pull request's: itself, its stack, and how it's moved on since
	// you reviewed it.
	pr       *pulls.PR
	stack    *pulls.Stack
	since    string
	reviewed string
}

// changeOf reads what's reviewed: a pull request's commits, fetched, or a
// capsule's, since it left its base.
func (w *Workspace) changeOf(of ReviewOf) (*change, error) {
	if of.PR != 0 {
		svc, err := w.reviews()
		if err != nil {
			return nil, err
		}
		d, err := svc.Detail(of.Repo, of.PR)
		if err != nil {
			return nil, err
		}
		ch := &change{of: of, ref: d.PR.Ref(), title: d.PR.Title, body: d.Body, dir: svc.BareDir(of.Repo), baseRef: d.BaseRef, base: d.Base, head: d.Head, pr: &d.PR, stack: d.Stack, since: d.Since}
		if d.Reviewed != nil {
			ch.reviewed = d.Reviewed.Head
		}
		return ch, nil
	}
	repo, name, _ := strings.Cut(of.Capsule, "/")
	dir, baseRef, err := w.CapsuleDir(of.Mission, repo, name)
	if err != nil {
		return nil, err
	}
	sum, err := review.Summarize(dir, baseRef)
	if err != nil {
		return nil, err
	}
	head := review.CommitOf(dir, "HEAD")
	if sum.Base == "" || head == "" {
		return nil, fmt.Errorf("%s has no committed changes since %s", of.Capsule, baseRef)
	}
	ch := &change{of: of, ref: of.Capsule, dir: dir, baseRef: baseRef, base: sum.Base, head: head}
	if m, err := w.Missions.Load(of.Mission); err == nil {
		ch.title = m.Title
	}
	ch.body, _ = w.Missions.ReadBrief(of.Mission)
	return ch, nil
}

// RunPRPass runs a pass over a pull request's review (RunReviewPass).
func (w *Workspace) RunPRPass(repo string, number int, name string) (page.PassDef, *conversation.Conversation, error) {
	return w.RunReviewPass(OfPR(repo, number), name)
}

// RunReviewPass runs a pass over a review, the first time or again. Over
// a pull request, one that reads the code has it docked first, as asking
// about it does (a stack at its top layer); a cold one is given its
// changes, and nothing's docked. A capsule is there already. One that's
// running isn't started twice. It answers the pass's conversation as it is
// then.
func (w *Workspace) RunReviewPass(of ReviewOf, name string) (page.PassDef, *conversation.Conversation, error) {
	def, c, err := w.ReviewPass(of, name)
	if err != nil {
		return def, nil, err
	}
	if def.Unavailable != "" {
		return def, nil, errors.New(def.Title + " cannot run: " + def.Unavailable)
	}
	if _, _, err := Roles(w.Config()).ForConversation(); err != nil {
		return def, nil, fmt.Errorf("no agent can run a pass here: %w", err)
	}
	if c != nil && c.Answering {
		return def, c, nil
	}
	if of.PR != 0 {
		svc, err := w.reviews()
		if err != nil {
			return def, nil, err
		}
		pr, st, err := svc.Open(of.Repo, of.PR)
		if err != nil {
			return def, nil, err
		}
		if def.Name == page.FollowUp {
			threads, err := openThreadsOfYours(svc, of.Repo, of.PR)
			if err != nil {
				return def, nil, err
			}
			if len(threads) == 0 {
				return def, nil, &Invalid{Reason: "Nothing to follow up: no open comments of yours on " + pr.Ref() + "."}
			}
		}
		if needsDocking(def) {
			if pr.Fork {
				return def, nil, fmt.Errorf("%s is from a fork and cannot be docked: only a cold pass can run on it", pr.Ref())
			}
			// In the daemon, docked by its answer (reviewPassReady): a
			// fetch and a worktree, which the page needn't wait on.
			if w.opts.Outbox == nil {
				if _, err := w.dockQuietly(DockedTop(pr, st)); err != nil {
					return def, nil, err
				}
				svc.Stale() // it's docked now: the list says where
			}
		}
	} else if _, err := w.changeOf(of); err != nil {
		return def, nil, err
	}
	switch {
	case c == nil:
		c, err = w.cells().OpenPassOver(passOver(of), def.Name, w.now())
	case !c.NeedsAnswer(): // asked for already, and not yet begun: it's asked for once
		c, err = w.cells().Say(c.ID, conversation.Turn{By: conversation.You, At: w.now(), Body: "Run again."})
	}
	if err != nil {
		return def, nil, err
	}
	return def, w.FillCell(c), nil
}

// needsDocking is whether a pass's agent reads the code as files: a cold
// one is given the change, and a follow-up reads the repo's clone, where
// the commits before and after are.
func needsDocking(def page.PassDef) bool { return !def.Cold && def.Name != page.FollowUp }

// ReviewedPR is whether you've reviewed a pull request: here, or on
// GitHub. One you haven't has nothing to follow up.
func (w *Workspace) ReviewedPR(repo string, number int) bool {
	svc := w.PRService()
	if svc == nil {
		return false
	}
	if rec, err := pulls.Load(w.Root(), repo, number); err == nil && rec != nil && rec.Reviewed != nil {
		return true
	}
	if list := svc.Peek(); list != nil {
		if pr, _ := list.Find(repo, number); pr != nil && pr.Reviewed != nil {
			return true
		}
	}
	return false
}

// openThreadsOfYours are the comments you've made on a pull request's
// lines that aren't resolved, as GitHub has them.
func openThreadsOfYours(svc *pulls.Service, repo string, number int) ([]review.Note, error) {
	t, err := svc.Threads(repo, number)
	if err != nil {
		return nil, err
	}
	var out []review.Note
	for _, n := range t.Comments {
		if n.By == "" && n.Status != review.Resolved {
			out = append(out, n)
		}
	}
	return out, nil
}

// followedUp settles what the follow-up said of a thread once you've
// resolved it (and opens it again if you reopen it): it's dealt with.
func (w *Workspace) followedUp(repo string, number int, thread string, resolved bool) {
	c := w.cells().ReviewPassOn(pulls.Key(repo, number), page.FollowUp)
	if c == nil || c.Written() == nil {
		return
	}
	state := conversation.FindingOpen
	if resolved {
		state = conversation.FindingDone
	}
	for _, f := range c.Written().Findings {
		if f.Thread == thread && c.FindingState(f.Key) != state {
			logFailed(w.cells().SettleFinding(c.ID, f.Key, state, "", w.now()))
		}
	}
}

// SettleReviewFinding is you dismissing a pass's finding (and why), or
// opening it again.
func (w *Workspace) SettleReviewFinding(of ReviewOf, name, key, state, why string) (page.PassDef, *conversation.Conversation, error) {
	def, c, err := w.ReviewPass(of, name)
	if err == nil && c == nil {
		err = errors.Join(conversation.ErrNotFound, errors.New(def.Name+" has not run on this review"))
	}
	if err != nil {
		return def, nil, err
	}
	c, err = w.cells().SettleFinding(c.ID, key, state, why, w.now())
	return def, c, err
}

// TakeReviewFinding puts a pass's finding in your draft review, as a
// comment marked as the agent's, yours to edit or delete like any other:
// nothing reaches GitHub (or crew) until you submit. The finding is then
// done. One on lines is a comment on those lines, where they are now; one
// on none is a comment on the change as a whole, which goes in the
// review's body.
func (w *Workspace) TakeReviewFinding(of ReviewOf, name, key string) (page.PassDef, *conversation.Conversation, error) {
	def, c, err := w.ReviewPass(of, name)
	if err == nil && c == nil {
		err = errors.Join(conversation.ErrNotFound, errors.New(def.Name+" has not run on this review"))
	}
	if err != nil {
		return def, nil, err
	}
	var f *conversation.Finding
	if wr := c.Written(); wr != nil {
		for i := range wr.Findings {
			if wr.Findings[i].Key == key {
				f = &wr.Findings[i]
			}
		}
	}
	if f == nil {
		return def, nil, errors.Join(conversation.ErrNotFound, fmt.Errorf("%s has no finding %s", def.Name, key))
	}
	if f.Thread != "" {
		return def, nil, &Invalid{Reason: "About a comment you already made: resolve or reply in that thread."}
	}
	if c.FindingState(key) == conversation.FindingDone {
		return def, c, nil
	}
	ch, err := w.changeOf(of)
	if err != nil {
		return def, nil, err
	}
	n := review.NewNote{Kind: noteKindOf(def, f.Kind), Body: f.Text, By: conversation.Agent}
	if f.Suggest != "" && f.Placed() {
		s := f.Suggest
		n.Suggestion = &s
	}
	if f.Placed() {
		commit := ch.head
		if f.Side == "old" {
			commit = ch.base
		}
		line, err := linesNow(ch.dir, commit, *f)
		if err != nil {
			return def, nil, err
		}
		n.Path, n.Side, n.Line, n.EndLine = f.Path, f.Side, line, line+(f.EndLine-f.Line)
	}
	store, at := w.Reviews(of)
	if _, err := store.AddNote(at, review.Diffed{Dir: ch.dir, BaseRef: ch.baseRef, Base: ch.base, Head: ch.head}, n, w.now()); err != nil {
		return def, nil, err
	}
	c, err = w.cells().SettleFinding(c.ID, key, conversation.FindingDone, "", w.now())
	return def, c, err
}

// noteKindOf is the kind of comment a finding becomes: its own, where
// it's one a comment can be; else blocking if it blocks, a suggestion if
// not. It's yours to change in the draft.
func noteKindOf(def page.PassDef, kind string) string {
	if slices.Contains(review.Kinds, kind) {
		return kind
	}
	if def.Blocking(kind) {
		return review.Kinds[0]
	}
	return "suggestion"
}

// linesNow is where a finding's lines are in its file at commit: where
// the pass found them, if they still read the same there, else wherever
// they've moved to, the nearest if they're in more than one place. Lines
// that no longer read as they did can't be commented on.
func linesNow(dir, commit string, f conversation.Finding) (int, error) {
	lines, err := review.Blob(dir, commit, f.Path)
	if err != nil {
		return 0, fmt.Errorf("%s no longer exists: run the pass again", f.Path)
	}
	n := len(f.Excerpt)
	same := func(at int) bool {
		if at < 1 || at+n-1 > len(lines) {
			return false
		}
		for i, l := range f.Excerpt {
			if strings.TrimSpace(lines[at-1+i]) != strings.TrimSpace(l) {
				return false
			}
		}
		return true
	}
	if n == 0 || same(f.Line) {
		if f.EndLine > len(lines) {
			return 0, fmt.Errorf("%s changed since the pass ran: run again", f.Path)
		}
		return f.Line, nil
	}
	best := 0
	for at := 1; at+n-1 <= len(lines); at++ {
		if same(at) && (best == 0 || abs(at-f.Line) < abs(best-f.Line)) {
			best = at
		}
	}
	if best == 0 {
		return 0, fmt.Errorf("the lines changed since the pass ran: run again")
	}
	return best, nil
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// Answering a review's pass.

// coldDiffRoom is how much of a change a cold pass is given.
const coldDiffRoom = 160 << 10

// reviewPassReady finds the pass and the change as its agent starts: the
// commits it's between, where it's checked out, and, for a cold pass, the
// change itself.
func reviewPassReady(a *answering) (bool, error) {
	gone := func(why string) (bool, error) {
		_, err := a.store.Say(a.c.ID, conversation.Turn{By: conversation.WS, At: a.w.now(), Body: why})
		return true, err
	}
	of, ok := reviewOfAnchor(a.c.Anchor)
	if !ok {
		return gone("Only runs on a pull request or a mission's capsule.")
	}
	def, ok := a.w.reviewPassNamed(of, a.c.Goal)
	if !ok {
		return gone("No pass named " + a.c.Goal + " here any more.")
	}
	if of.PR != 0 && needsDocking(def) {
		// Checked out for its agent to read, if it isn't yet: asking for
		// the pass didn't wait on it.
		if err := a.w.dockPRQuietly(of.Repo, of.PR); err != nil {
			return gone(fmt.Sprintf("Could not check out %s#%d here: %v", of.Repo, of.PR, err))
		}
	}
	ch, err := a.w.changeOf(of)
	if err != nil {
		return gone("Could not read the change: " + err.Error())
	}
	rp := prompt.ReviewPass{Def: def, Title: ch.title, Base: ch.base, Head: ch.head, Clone: ch.dir, GuidePath: GuidePath(a.w.Root(), def.File)}
	if of.PR != 0 {
		rp.PR = ch.ref
	} else {
		// A capsule's work: its mission says what it's meant to do, and it's
		// checked out where its commits are.
		rp.Over = fmt.Sprintf("the work in the capsule %s, of the mission %q (%s)", ch.ref, ch.title, of.Mission)
		rp.Start = fmt.Sprintf("Start with `ws show -f llm -m %s`: the mission, its brief (what the work is meant to do), its capsules, and the reviews there have been, which aren't to be said again. ", of.Mission)
		rp.BodyIs = "What the work is meant to do (the mission's brief)"
		if !def.Cold {
			rp.Capsule = ch.dir
		}
	}
	if def.Cold {
		diff, err := review.CompareCommits(ch.dir, ch.baseRef, ch.base, ch.head)
		if err != nil {
			return gone("Could not read the changes: " + err.Error())
		}
		rp.Body = ch.body
		text, left := review.Numbered(diff.Files, coldDiffRoom)
		rp.Diff = text
		for _, f := range diff.Files {
			rp.Files = append(rp.Files, f.Path)
		}
		given := conversation.GuideGiven{Files: rp.Files}
		for _, l := range left {
			rp.Left = append(rp.Left, l.String())
			given.Left = append(given.Left, conversation.GuideLeft{Path: l.Path, Why: l.Why})
		}
		// While the guide's written, its view says what it was given, and
		// reads what it's written so far against the files it may place.
		if def.Name == page.Guide {
			logFailed(a.store.SetGuideGiven(a.c.ID, given))
		}
		if of.PR != 0 {
			if svc, err := a.w.reviews(); err == nil {
				rp.Said = saidSoFar(svc, of.Repo, of.PR)
			}
		}
		if def.Skill == "" && rp.GuidePath != "" {
			if data, err := os.ReadFile(rp.GuidePath); err == nil {
				rp.GuideText = string(data)
			}
		}
	} else if needsDocking(def) && ch.pr != nil {
		top := DockedTop(ch.pr, ch.stack)
		if capsule := a.w.CapsuleOnBranch(of.Repo, top.Branch); capsule != "" {
			rp.Capsule, rp.Top = filepath.Join(a.w.WS().RepoDir(of.Repo), capsule), top.Ref()
		}
	}
	if !def.Cold {
		// It reads the change itself: pointed past what's generated.
		rp.Generated = review.GeneratedIn(ch.dir, ch.base, ch.head)
	}
	if def.Name == page.FollowUp {
		svc, err := a.w.reviews()
		if err != nil || of.PR == 0 {
			return gone("Follow-up only runs on your comments on a pull request.")
		}
		threads, err := openThreadsOfYours(svc, of.Repo, of.PR)
		if err != nil {
			return gone("Could not read your comments: " + err.Error())
		}
		if len(threads) == 0 {
			return gone("No open comments of yours: nothing to follow up.")
		}
		rp.Since, rp.Reviewed = ch.since, ch.reviewed
		for i, n := range threads {
			t := prompt.FollowThread{Label: fmt.Sprintf("T%d", i+1), ID: n.ID, Path: n.Path, Side: n.Side, Line: n.Line, EndLine: n.EndLine, Commit: n.Commit, Kind: n.Kind, Body: n.Body, Excerpt: n.Excerpt}
			for _, r := range n.Thread {
				by := r.By
				if by == "" || by == conversation.You {
					by = "the user"
				}
				t.Replies = append(t.Replies, by+": "+strings.Join(strings.Fields(r.Body), " "))
			}
			rp.Threads = append(rp.Threads, t)
		}
	}
	a.rpass, a.around = rp, ch.head
	return false, nil
}

// saidSoFar is what's been said on a pull request, on itself and on its
// lines, a line each: for a pass that can't ask.
func saidSoFar(svc *pulls.Service, repo string, number int) string {
	var b strings.Builder
	if d, err := svc.Discussion(repo, number); err == nil {
		for _, it := range d.Items {
			if it.Bot || it.Body == "" {
				continue
			}
			fmt.Fprintf(&b, "- %s (%s): %s\n", it.By, it.Kind, strings.Join(strings.Fields(it.Body), " "))
		}
	}
	if t, err := svc.Threads(repo, number); err == nil {
		for _, n := range t.Comments {
			by := n.By
			if by == "" {
				by = "the user"
			}
			fmt.Fprintf(&b, "- %s, on %s:%d: %s\n", by, n.Path, n.Line, strings.Join(strings.Fields(n.Body), " "))
		}
	}
	return b.String()
}

// reviewPassRead reads what a review's pass's agent said into findings,
// each on its lines as they read at the commit it ran at, and what it said
// of the change as a whole.
func reviewPassRead(a *answering, rep harness.Report, err error) reply {
	if err != nil {
		return reply{turns: []conversation.Turn{a.couldnt(err, rep)}, err: err}
	}
	rp := a.rpass
	if rp.Def.Name == page.Guide {
		return guideRead(a, rep)
	}
	body, findings := conversation.ParseReviewFindings(rep.Output)
	if rp.Def.Name == page.FollowUp {
		findings = followedFindings(rp, findings)
	}
	for i := range findings {
		if findings[i].Thread != "" {
			continue // it's where its comment is
		}
		f := &findings[i]
		f.Kind = heldKind(rp.Def, f.Kind)
		if !f.Placed() {
			continue
		}
		commit := rp.Head
		if f.Side == "old" {
			commit = rp.Base
		}
		lines, berr := review.Blob(rp.Clone, commit, f.Path)
		if berr != nil || f.EndLine > len(lines) {
			// It named lines that aren't there: what it says still stands,
			// about the change as a whole.
			f.Text = fmt.Sprintf("(On %s:%d, not in the change.) %s", f.Path, f.Line, f.Text)
			f.Path, f.Side, f.Line, f.EndLine, f.Suggest = "", "", 0, 0, ""
			continue
		}
		f.Commit, f.Excerpt = commit, lines[f.Line-1:f.EndLine]
	}
	conversation.KeyFindings(findings)
	if body == "" {
		body = "No comments on the change as a whole."
		if len(findings) == 0 {
			body = rep.Output
		}
	}
	t := conversation.Turn{By: conversation.Agent, At: a.w.now(), Body: body, Calls: turnCalls(rep.Calls), Around: a.around, Findings: findings}
	return reply{turns: []conversation.Turn{t}}
}

// heldKind holds a finding to its pass's kinds: one of a kind the pass
// hasn't (an agent naming what it found its own way) is of its first kind
// that doesn't block, so a slip of the tongue isn't a no go.
func heldKind(def page.PassDef, kind string) string {
	if len(def.Kinds) == 0 || slices.Contains(def.Kinds, kind) {
		return kind
	}
	for _, k := range def.Kinds {
		if !def.Blocking(k) {
			return k
		}
	}
	return def.Kinds[0]
}

// reviewPassFirst and reviewPassNext are what a review's pass's agent is
// told: a follow-up's is its own.
func reviewPassFirst(a *answering) string {
	switch a.rpass.Def.Name {
	case page.FollowUp:
		return prompt.FollowUpFirst(a.here, a.c, a.rpass)
	case page.Guide:
		return prompt.GuideFirst(a.here, a.c, a.rpass)
	}
	return prompt.ReviewPassFirst(a.here, a.c, a.rpass)
}

func reviewPassNext(a *answering) string {
	switch a.rpass.Def.Name {
	case page.FollowUp:
		return prompt.FollowUpNext(a.here, a.c, a.rpass)
	case page.Guide:
		return prompt.GuideNext(a.here, a.c, a.rpass)
	}
	return prompt.ReviewPassNext(a.here, a.c, a.rpass)
}

// followedFindings are a follow-up's findings, each on the comment it's
// about, where that comment is: what's said of anything else is left out.
// A blocking comment that isn't dealt with blocks.
func followedFindings(rp prompt.ReviewPass, findings []conversation.Finding) []conversation.Finding {
	by := map[string]prompt.FollowThread{}
	for _, t := range rp.Threads {
		by[t.Label] = t
	}
	out := []conversation.Finding{}
	seen := map[string]bool{}
	for _, f := range findings {
		t, ok := by[f.Thread]
		if !ok || seen[t.ID] {
			continue
		}
		seen[t.ID] = true
		switch f.Kind {
		case page.FollowedAddressed, page.FollowedPartly, page.FollowedNot:
		case "not", "unaddressed", "not-dealt-with":
			f.Kind = page.FollowedNot
		default:
			f.Kind = page.FollowedPartly
		}
		f.Thread, f.Path, f.Side, f.Line, f.EndLine, f.Commit, f.Excerpt = t.ID, t.Path, t.Side, t.Line, t.EndLine, t.Commit, t.Excerpt
		f.Suggest = ""
		f.Blocks = t.Kind == review.Kinds[0] && f.Kind != page.FollowedAddressed
		out = append(out, f)
	}
	return out
}

// guideRead reads what the guide's agent said into a guide: what the
// change does, where it differs from its description, and its files in
// groups. A reply with no groups in it isn't a guide.
func guideRead(a *answering, rep harness.Report) reply {
	in := map[string]bool{}
	for _, f := range a.rpass.Files {
		in[f] = true
	}
	g := conversation.ParseGuide(rep.Output, func(p string) bool { return in[p] })
	if len(g.Groups) == 0 {
		err := errors.New("the answer gave no reading order")
		return reply{turns: []conversation.Turn{a.couldnt(err, rep)}, err: err}
	}
	t := conversation.Turn{By: conversation.Agent, At: a.w.now(), Body: g.Does, Calls: turnCalls(rep.Calls), Around: a.around, Guide: g}
	return reply{turns: []conversation.Turn{t}}
}

// Running by themselves.

// AutoPasses runs the next pass that's set to run by itself (auto) and
// hasn't been, over the pull requests that need you: those in Needs your
// review and Back with you, as the list last had them. One at a time: with
// one running, nothing's started. It's run again when the pull request
// has been pushed to since, and not again after it's failed. Never over a
// bot's pull request or a draft; over a fork's, only a pass that's given
// the change (ws can't dock it). It says what it started, or "".
func (w *Workspace) AutoPasses() string {
	svc := w.PRService()
	if svc == nil {
		return ""
	}
	l := svc.Peek()
	if l == nil {
		return ""
	}
	if _, _, err := Roles(w.Config()).ForConversation(); err != nil {
		return ""
	}
	store := w.cells()
	all, err := store.PassesOverPRs()
	if err != nil {
		return ""
	}
	latest := map[string]*conversation.Conversation{} // by pull request and pass
	for _, c := range all {
		if c.Kind != conversation.Pass || c.Anchor.PR == "" {
			continue
		}
		// One's running, or about to. (One asked for long ago that never
		// began is stuck, and doesn't hold the rest up.)
		if c.Answering || (c.NeedsAnswer() && w.now().Sub(c.Last().At) < 2*TurnFor) {
			return ""
		}
		latest[c.Anchor.PR+" "+c.Goal] = c
	}
	autos := map[string][]page.PassDef{} // by repo
	defsFor := func(repo string) []page.PassDef {
		if defs, ok := autos[repo]; ok {
			return defs
		}
		var defs []page.PassDef
		if g := w.GuideDef(); g.Auto {
			defs = append(defs, g)
		}
		for _, d := range w.ReviewPasses(repo) {
			if d.Auto && d.Unavailable == "" {
				defs = append(defs, d)
			}
		}
		autos[repo] = defs
		return defs
	}
	try := func(pr *pulls.PR) string {
		if pr.Bot || pr.Draft || pr.Mine || (pr.State != pulls.NeedsReview && pr.State != pulls.BackWithYou) {
			return ""
		}
		for _, def := range defsFor(pr.Repo) {
			if pr.Fork && needsDocking(def) {
				continue
			}
			if c := latest[pr.Ref()+" "+def.Name]; c != nil {
				wr := c.Written()
				// Never said anything (it failed), failed since, or has read
				// it as it is now: it's had its go.
				if wr == nil || c.Last().By == conversation.WS || wr.Around == "" || pr.Head == "" || wr.Around == pr.Head {
					continue
				}
			}
			if _, _, err := w.RunPRPass(pr.Repo, pr.Number, def.Name); err != nil {
				continue
			}
			return def.Name + " over " + pr.Ref()
		}
		return ""
	}
	for _, sec := range l.Sections {
		if sec.Key != pulls.NeedsReview && sec.Key != pulls.BackWithYou {
			continue
		}
		for _, row := range sec.Rows {
			if row.PR != nil {
				if said := try(row.PR); said != "" {
					return said
				}
			}
			if row.Stack != nil {
				for i := range row.Stack.Layers {
					if said := try(&row.Stack.Layers[i]); said != "" {
						return said
					}
				}
			}
		}
	}
	return ""
}

// PassMark is how a pass stands over a pull request, in brief: for the
// Reviews list's rows.
type PassMark struct {
	Name     string        `json:"name"`
	Title    string        `json:"title"`
	Standing page.Standing `json:"standing"`
	// Moved: the pull request's been pushed to since it ran.
	Moved bool `json:"moved,omitempty"`
}

// PassMarks are how each pass that's been run stands, over every pull
// request it's been run over, by pull request (api#89). heads are the
// pull requests' heads as GitHub last gave them, to say which have moved.
func (w *Workspace) PassMarks(heads map[string]string) map[string][]PassMark {
	out := map[string][]PassMark{}
	all, _ := w.cells().PassesOverPRs()
	latest := map[string]*conversation.Conversation{}
	var order []string
	for _, c := range all {
		if c.Kind != conversation.Pass || c.Anchor.PR == "" || c.Goal == page.Guide {
			continue
		}
		key := c.Anchor.PR + " " + c.Goal
		if _, ok := latest[key]; !ok {
			order = append(order, key)
		}
		latest[key] = c
	}
	for _, key := range order {
		c := latest[key]
		// (It's anchored by its repo's own name: there's no alias to read.)
		repo, _, ok := ref.ParsePR(c.Anchor.PR)
		if !ok {
			continue
		}
		def, ok := page.PassNamed(w.ReviewPasses(repo), c.Goal)
		if !ok {
			continue
		}
		m := PassMark{Name: def.Name, Title: def.Title, Standing: page.ReviewStanding(def, c)}
		if wr := c.Written(); wr != nil {
			head := heads[c.Anchor.PR]
			m.Moved = wr.Around != "" && head != "" && wr.Around != head
		}
		out[c.Anchor.PR] = append(out[c.Anchor.PR], m)
	}
	return out
}
