package ops

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/doc"
	"github.com/withworkspace/workspace/internal/job"
	"github.com/withworkspace/workspace/internal/pulls"
	"github.com/withworkspace/workspace/internal/ref"
	"github.com/withworkspace/workspace/internal/review"
	"github.com/withworkspace/workspace/internal/workspace"
)

// Pull requests outside a mission: the Reviews page's service, asking an
// agent about one (docked first, for it to read), Open here, tidying what
// was docked for a question, and accepting a comment an agent proposed.

// ReviewsService is the Reviews page's service for the workspace: GitHub
// asked through its client, a PR's commits fetched into its repo's clone.
// The daemon keeps one (Options.Reviews); reviews uses it, or makes one.
func (w *Workspace) ReviewsService() *pulls.Service {
	root := w.Root()
	return &pulls.Service{
		Root: root, Remote: func(repo string) ref.Remote { return w.WS().RemoteOf(repo) }, GitHub: w.GitHub(),
		// While a Reviews page is open, GitHub is asked as often as the
		// missions' PRs are watched.
		Fresh: WatchPRsEvery,
		Repos: func() []string {
			ws := w.WS()
			var out []string
			for _, name := range ws.ListedRepoNames() {
				if info, err := os.Stat(ws.BareDir(name)); err == nil && info.IsDir() {
					out = append(out, name)
				}
			}
			return out
		},
		BareDir: func(repo string) string { return filepath.Join(root, "repos", repo, ".bare") },
		Local:   w.reviewsLocal,
	}
}

// reviewsLocal is what the workspace knows of branches that GitHub
// doesn't: which are a mission's capsule's, and which are checked out.
// Each capsule's branch is read from its HEAD (branchOf): the list of
// pull requests asks this each time it's read, of every capsule there is.
func (w *Workspace) reviewsLocal() pulls.Local {
	local := pulls.Local{Missions: map[string]pulls.MissionRef{}, Capsules: map[string]string{}}
	for _, repo := range w.WS().RepoNames {
		capsules, _ := workspace.ListWorktrees(w.WS().RepoDir(repo))
		for _, c := range capsules {
			if b := branchOf(filepath.Join(w.WS().RepoDir(repo), c)); b != "" {
				local.Capsules[repo+"/"+b] = c
			}
		}
	}
	missions, _ := w.Missions.InPlay()
	for _, m := range missions {
		for _, c := range m.Capsules {
			branch := branchOf(filepath.Join(w.WS().RepoDir(c.Repo), c.Name))
			if branch == "" {
				branch = c.Branch // burned: the branch it was on
			}
			if branch != "" {
				local.Missions[c.Repo+"/"+branch] = pulls.MissionRef{Slug: m.Slug, Title: m.Title}
			}
		}
	}
	return local
}

// CapsuleOnBranch is the capsule a PR's branch is checked out in here, or "".
func (w *Workspace) CapsuleOnBranch(repo, branch string) string {
	capsules, _ := workspace.ListWorktrees(w.WS().RepoDir(repo))
	for _, c := range capsules {
		if branchOf(filepath.Join(w.WS().RepoDir(repo), c)) == branch {
			return c
		}
	}
	return ""
}

// DockedTop is the PR to dock so an agent can read pr: itself, or its
// stack's top layer, whose branch contains every layer below.
func DockedTop(pr *pulls.PR, st *pulls.Stack) *pulls.PR {
	if st != nil && len(st.Layers) > 0 {
		return &st.Layers[len(st.Layers)-1]
	}
	return pr
}

// dockQuietly checks a PR's branch out for an agent to read: as ws dock
// does, but ws's own to tidy up. It isn't boarded (so it stays out of
// your editor) and the repo's after_create hook isn't run: that would run
// what a teammate's branch says to. An existing capsule on the branch
// will do.
func (w *Workspace) dockQuietly(pr *pulls.PR) (string, error) {
	if pr.Fork {
		return "", fmt.Errorf("%s is from a fork: ws dock looks for its branch on origin, so it can't be docked", pr.Ref())
	}
	if c := w.CapsuleOnBranch(pr.Repo, pr.Branch); c != "" {
		return c, nil
	}
	c, err := w.Dock(pr.Repo, pr.Branch, CapsuleOptions{Bare: true, By: AsWS})
	if err != nil {
		return "", fmt.Errorf("docking %s: %w", pr.Ref(), err)
	}
	capsule := c.Name
	pulls.Update(w.WS().Root, pr.Repo, pr.Number, func(r *pulls.Record) {
		r.Title, r.Branch = pr.Title, pr.Branch
		r.Docked = &pulls.Docked{Capsule: capsule, Branch: pr.Branch, By: pulls.ByWS, At: time.Now()}
	})
	return capsule, nil
}

// askPR starts an ask about a pull request, and about lines of its diff
// when the question is: it docks the PR first (a stack at its top layer),
// so the agent reads the code as files, and has it answer. It's an ask as
// one about a document is (conversation/ask.go): ephemeral until you keep
// it, but its agent may propose review comments from the start, since
// they go to your draft review, which is yours to keep or not.
func (w *Workspace) askPR(svc *pulls.Service, repo string, number int, q PRQuestion) (*conversation.Conversation, error) {
	pr, st, err := svc.Open(repo, number)
	if err != nil {
		return nil, err
	}
	if _, _, err := Roles(w.Config()).ForConversation(); err != nil {
		return nil, fmt.Errorf("no agent can answer here: %w", err)
	}
	if _, err := w.dockQuietly(DockedTop(pr, st)); err != nil {
		return nil, err
	}
	anchor := conversation.Anchor{PR: pr.Ref()}
	if q.Path != "" {
		end := q.EndLine
		if end < q.Line {
			end = q.Line
		}
		lines, err := review.Blob(svc.BareDir(repo), q.Commit, q.Path)
		if err != nil {
			return nil, err
		}
		if end > len(lines) {
			return nil, fmt.Errorf("%s has %d lines there, not %d", q.Path, len(lines), end)
		}
		anchor.Path, anchor.Side, anchor.Line, anchor.EndLine, anchor.Commit = q.Path, q.Side, q.Line, end, q.Commit
		anchor.Excerpt = lines[q.Line-1 : end]
	}
	c, err := conversation.ForWorkspace(w.WS().Root).OpenAsk(anchor, conversation.You, q.Body, time.Now())
	if err != nil {
		return nil, &Invalid{Reason: err.Error()}
	}
	w.answer(job.Answer, doc.Scope{}, c.ID)
	svc.Stale() // it's docked now: the list says where
	return c, nil
}

// DockForReview docks a PR's branch as ws dock does (boarded, the repo's
// hook run), as you; tests replace it.
var DockForReview = func(w *Workspace, repo, branch string) error {
	c, err := w.Dock(repo, branch, CapsuleOptions{By: AsYou})
	if err == nil && c.HookErr != nil {
		err = fmt.Errorf("docked %s, but its after_create hook failed: %w", c.Path, c.HookErr)
	}
	return err
}

// openHere checks a PR out here as ws dock does, boarded, and then it's
// yours. A capsule ws docked for a question becomes yours: boarded, and
// the repo's after_create hook run in it, as docking it would have.
func (w *Workspace) openHere(svc *pulls.Service, repo string, number int) (string, error) {
	pr, _, err := svc.Open(repo, number)
	if err != nil {
		return "", err
	}
	if pr.Fork {
		return "", fmt.Errorf("%s is from a fork: ws dock looks for its branch on origin, so it can't be opened here", pr.Ref())
	}
	rec, err := pulls.Load(w.WS().Root, repo, number)
	if err != nil {
		return "", err
	}
	mine := func(capsule string) {
		pulls.Update(w.WS().Root, repo, number, func(r *pulls.Record) {
			r.Title, r.Branch = pr.Title, pr.Branch
			r.Docked = &pulls.Docked{Capsule: capsule, Branch: pr.Branch, By: pulls.ByYou, At: time.Now()}
		})
		svc.Stale()
	}
	if capsule := w.CapsuleOnBranch(repo, pr.Branch); capsule != "" {
		path := filepath.Join(w.WS().RepoDir(repo), capsule)
		if rec.Docked == nil || rec.Docked.By != pulls.ByWS || rec.Docked.Capsule != capsule {
			return fmt.Sprintf("It's here already: %s.", path), nil
		}
		said := fmt.Sprintf("It's yours now: %s.", path)
		if c := w.Adopt(repo, capsule); c.HookErr != nil {
			said += " Its after_create hook failed: " + c.HookErr.Error() + "."
		}
		mine(capsule)
		return said, nil
	}
	if err := DockForReview(w, repo, pr.Branch); err != nil {
		return "", fmt.Errorf("docking %s: %w", pr.Ref(), err)
	}
	capsule := w.CapsuleOnBranch(repo, pr.Branch)
	if capsule == "" {
		return "", fmt.Errorf("ws dock said it docked %s, but no capsule is on %s", pr.Ref(), pr.Branch)
	}
	mine(capsule)
	return fmt.Sprintf("Docked: %s.", filepath.Join(w.WS().RepoDir(repo), capsule)), nil
}

// TidyDocked removes the capsules ws docked for a question once their PR
// has merged or closed, unless something's been changed in one: then it's
// yours. GitHub is asked only about repos with such a capsule.
func (w *Workspace) TidyDocked() {
	if w.GitHub() == nil {
		return
	}
	open := map[string]map[int]bool{}
	docked, err := pulls.DockedBy(w.WS().Root, pulls.ByWS)
	if err != nil {
		return // can't tell: leave them
	}
	for _, k := range docked {
		d := k.Record.Docked
		forget := func(by string) {
			pulls.Update(w.WS().Root, k.Repo, k.Number, func(r *pulls.Record) {
				if by == "" {
					r.Docked = nil
				} else if r.Docked != nil {
					r.Docked.By = by
				}
			})
		}
		path := filepath.Join(w.WS().RepoDir(k.Repo), d.Capsule)
		if info, err := os.Stat(path); err != nil || !info.IsDir() {
			forget("") // gone already
			continue
		}
		if _, asked := open[k.Repo]; !asked {
			prs, err := w.GitHub().OpenPRs(w.WS().RemoteOf(k.Repo).Owner, w.WS().RemoteOf(k.Repo).Name)
			if err != nil {
				continue // can't tell: leave it
			}
			open[k.Repo] = map[int]bool{}
			for _, pr := range prs {
				open[k.Repo][pr.Number] = true
			}
		}
		if open[k.Repo][k.Number] {
			continue
		}
		ahead, _ := workspace.GitAheadBehind(path)
		if workspace.GitIsDirty(path) || ahead > 0 || w.WS().IsBoarded(k.Repo, d.Capsule) {
			forget(pulls.ByYou) // you've changed something in it: it's yours
			continue
		}
		// The branch ws made to check it out goes too: nothing of yours is on it.
		if _, err := w.Burn(k.Repo, d.Capsule, BurnOptions{DropBranch: true}); err != nil {
			continue
		}
		forget("")
	}
}

// ParsePRRef reads repo#number, the repo by its name or an alias.
func (w *Workspace) ParsePRRef(name string) (repo string, number int, err error) {
	given, number, ok := ref.ParsePR(name)
	if !ok {
		return "", 0, fmt.Errorf("%q isn't a pull request: they're named <repo>#<number> from 1, such as api#89", name)
	}
	repo, ok = w.WS().ResolveAlias(given)
	if !ok {
		return "", 0, fmt.Errorf("%s isn't a repo of this workspace", given)
	}
	return repo, number, nil
}

// acceptComment adds a comment an agent proposed to your draft review of
// its pull request, marked as the agent's: yours to edit or delete like
// any other, and nothing reaches GitHub until you submit.
func (w *Workspace) acceptComment(store conversation.Store, c *conversation.Conversation, p conversation.Proposal) (string, error) {
	repo, number, err := w.ParsePRRef(p.PR)
	if err != nil {
		return "", err
	}
	svc, err := w.reviews()
	if err != nil {
		return "", err
	}
	d, err := svc.Detail(repo, number)
	if err != nil {
		return "", err
	}
	r, err := svc.Store(repo, number).AddNote(pulls.Key(repo, number),
		review.Diffed{Dir: svc.BareDir(repo), BaseRef: d.BaseRef, Base: d.Base, Head: d.Head},
		review.NewNote{Path: p.Path, Side: firstNonEmpty(p.Side, "new"), Line: p.Line, EndLine: p.EndLine, Kind: p.NoteKind, Body: p.Text, Suggestion: p.Suggestion, By: conversation.Agent},
		time.Now())
	if err != nil {
		return "", err
	}
	id := r.Comments[len(r.Comments)-1].ID
	if _, err := store.SettleProposal(c.ID, p.N, conversation.ProposalAccepted, id, conversation.You, time.Now()); err != nil {
		return "", err
	}
	return fmt.Sprintf("Added to your draft review of %s: it's posted when you submit.", p.PR), nil
}

// PRService is the Reviews page's service: one for the workspace's
// operations, made when it's first wanted (Options.Reviews, if a test
// gives one); nil where there's no GitHub to ask.
func (w *Workspace) PRService() *pulls.Service {
	if w.opts.Reviews != nil {
		return w.opts.Reviews
	}
	w.prOnce.Do(func() {
		if w.GitHub() != nil {
			w.prSvc = w.ReviewsService()
		}
	})
	return w.prSvc
}

// errNoReviews is the Reviews page's work where there's no GitHub.
var errNoReviews = fmt.Errorf("%w: there's no GitHub to ask here", ErrNotFound)

// reviews is PRService, or why there's none.
func (w *Workspace) reviews() (*pulls.Service, error) {
	if svc := w.PRService(); svc != nil {
		return svc, nil
	}
	return nil, errNoReviews
}

// WatchPRsEvery is how often the daemon looks at the missions' pull
// requests, and asks GitHub of the Reviews page's while one's open.
const WatchPRsEvery = 2 * time.Minute
