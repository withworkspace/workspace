package ops

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/withworkspace/workspace/internal/config"
	"github.com/withworkspace/workspace/internal/events"
	"github.com/withworkspace/workspace/internal/git"
	"github.com/withworkspace/workspace/internal/ide"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/review"
	"github.com/withworkspace/workspace/internal/workspace"
)

// The steps of making a capsule, as they're shown.
const (
	StepAlign = "Aligning ground"
	StepMake  = "Making capsule"
	StepCopy  = "Copying files"
)

// CapsuleOptions are how a capsule's made.
type CapsuleOptions struct {
	// Mission is the mission it joins; "" for none.
	Mission string
	By      Actor
	// Bare is a capsule only for reading or for one change: fetched and
	// made, and nothing else. The ground isn't brought up to date, nothing
	// is copied into it, the repo's after_create hook isn't run (it would
	// run what a teammate's branch says to), and it isn't boarded (it stays
	// out of your editor).
	Bare bool
	// Aside is a capsule that isn't boarded, though it's made in full: an
	// agent's own, to try things in, which stays out of your editor.
	Aside bool
	// Show runs the steps of making it, by name, through run, stopping at
	// the first that fails, and then the repo's after_create hook (when
	// hook isn't "") in the capsule, dir(): for someone to watch. nil does
	// it quietly. It returns how the hook went, and the step that failed.
	Show func(steps []string, run func(step string) error, hook string, dir func() string) (hookErr, err error)
}

// Capsule is a capsule made: where, and what didn't go to plan making it.
type Capsule struct {
	Repo, Name, Path string
	// CopySkipped are the files copy_from_ground names that the ground
	// hasn't.
	CopySkipped []string
	HookErr     error  // the after_create hook failed
	IDEErr      error  // the editor's workspace files couldn't be written
	Mission     string // the mission it was to join
	AttachErr   error  // why it didn't join it
	// Joined is one that was there already, which a conversation's agent
	// took on rather than made (joinForConversation).
	Joined bool
}

// Lift makes a capsule for new work: branch, from base ("" for the
// default branch, as fetched).
func (w *Workspace) Lift(repo, branch, base string, o CapsuleOptions) (Capsule, error) {
	if base == "" {
		base = "origin/" + w.WS().DefaultBranch
	}
	return w.makeCapsule(repo, branch, o, func() (string, error) {
		if o.Bare {
			return w.WS().LiftWorktree(repo, branch, base)
		}
		return w.WS().CreateLiftWorktree(repo, branch, base)
	})
}

// Dock makes a capsule of a branch that's on the remote already: a
// teammate's, a pull request's.
func (w *Workspace) Dock(repo, branch string, o CapsuleOptions) (Capsule, error) {
	return w.makeCapsule(repo, branch, o, func() (string, error) {
		if o.Bare {
			return w.WS().DockWorktree(repo, branch)
		}
		return w.WS().CreateDockWorktree(repo, branch)
	})
}

func (w *Workspace) makeCapsule(repo, branch string, o CapsuleOptions, create func() (string, error)) (Capsule, error) {
	out := Capsule{Repo: repo}
	if !o.Bare {
		name := workspace.CapsuleName(branch)
		if _, err := os.Stat(filepath.Join(w.WS().RepoDir(repo), name)); err == nil {
			return out, refused("capsule-exists", "capsule %q already exists for %s", name, repo)
		}
	}
	rc, err := config.ParseRepoConfig(filepath.Join(w.WS().MainWorktree(repo), config.RepoFileName))
	if err != nil {
		return out, err
	}
	steps := []string{StepMake}
	hook := ""
	if !o.Bare {
		steps = []string{StepAlign, StepMake}
		if rc != nil && len(rc.Capsule.CopyFromGround) > 0 {
			steps = append(steps, StepCopy)
		}
		hook = w.AfterCreateHook(repo)
	}
	run := func(step string) error {
		switch step {
		case StepAlign:
			if err := workspace.GitFetch(w.WS().BareDir(repo)); err != nil {
				return err
			}
			return workspace.GitFFMerge(w.WS().MainWorktree(repo), "origin/"+w.WS().DefaultBranch)
		case StepMake:
			name, err := create()
			if err != nil {
				return err
			}
			out.Name, out.Path = name, filepath.Join(w.WS().RepoDir(repo), name)
		case StepCopy:
			skipped, err := workspace.CopyFromGround(w.WS().MainWorktree(repo), out.Path, rc.Capsule.CopyFromGround)
			out.CopySkipped = skipped
			return err
		}
		return nil
	}
	show := o.Show
	if show == nil {
		show = quietly
	}
	hookErr, err := show(steps, run, hook, func() string { return out.Path })
	if out.Name == "" {
		if err == nil {
			err = errors.New("failed to create capsule")
		}
		return out, err
	}
	out.HookErr = hookErr
	if !o.Bare && !o.Aside {
		out.IDEErr = w.board(repo, out.Name)
	}
	if out.Mission = o.Mission; o.Mission != "" {
		_, out.AttachErr = w.Attach(o.Mission, repo, out.Name, false, o.By)
	}
	return out, nil
}

// quietly makes a capsule with nobody watching: each step in turn, then
// the hook, its output discarded.
func quietly(steps []string, run func(string) error, hook string, dir func() string) (error, error) {
	for _, s := range steps {
		if err := run(s); err != nil {
			return nil, err
		}
	}
	if hook != "" {
		return workspace.RunHook(dir(), hook, io.Discard, io.Discard), nil
	}
	return nil, nil
}

// AfterCreateHook is the repo's hook for a new capsule: ws.toml's, else
// the repo's own, read from its default branch.
func (w *Workspace) AfterCreateHook(repo string) string {
	if hook := w.WS().AfterCreateHooks[repo]; hook != "" {
		return hook
	}
	if rc, err := config.ParseRepoConfig(filepath.Join(w.WS().MainWorktree(repo), config.RepoFileName)); err == nil && rc != nil {
		return rc.Capsule.AfterCreate
	}
	return ""
}

// Adopt makes a bare capsule (one ws made for reading) yours, as one you'd
// made would be: the repo's hook run in it, and boarded.
func (w *Workspace) Adopt(repo, name string) Capsule {
	out := Capsule{Repo: repo, Name: name, Path: filepath.Join(w.WS().RepoDir(repo), name)}
	if hook := w.AfterCreateHook(repo); hook != "" {
		out.HookErr = workspace.RunHook(out.Path, hook, io.Discard, io.Discard)
	}
	out.IDEErr = w.board(repo, name)
	return out
}

// board puts a capsule in your editor's workspace, and says why the
// editor's files couldn't be written, if they couldn't.
func (w *Workspace) board(repo, name string) error {
	if err := w.WS().Board(repo, name); err != nil {
		return nil // not there to board
	}
	if err := config.SaveBoarded(w.Root(), w.WS().Boarded); err != nil {
		return err
	}
	return ide.Regenerate(w.Root(), w.WS().Boarded, w.WS().DisplayNames, w.WS().RemoteOf)
}

// What becomes of a silo pointed at a capsule that's burned.
const (
	SiloToGround = iota // it's pointed at the ground, which is synced into it
	SiloDrop            // it's pointed at nothing
)

// BurnOptions are how a capsule's burned.
type BurnOptions struct {
	Force      bool  // even with changes nobody's committed
	DropBranch bool  // its branch is deleted too: ws made it, and nothing's on it
	Silo       int   // SiloToGround or SiloDrop
	By         Actor // who burns it: you, unless said
	// quiet keeps no event of it: a capsule ws lifted for itself, to save
	// a document in, whose saving is what's kept (doc.added).
	quiet bool
}

// Burned is a capsule burned.
type Burned struct {
	Repo, Name string
	Mission    string // the mission it belonged to, which logs it
	// IDEErr is why the editor's workspace files couldn't be written.
	IDEErr  error
	SiloErr error // the silo couldn't be synced with the ground
	Silo    bool  // a silo was pointed at it
}

// Burn removes a capsule: off your editor's workspace, its branch kept by
// the mission that owns it (so its pull request can still be found), the
// burn logged there, and a silo pointed at it moved off it.
func (w *Workspace) Burn(repo, name string, o BurnOptions) (Burned, error) {
	out := Burned{Repo: repo, Name: name}
	if w.WS().IsBoarded(repo, name) {
		logFailed(w.WS().Unboard(repo, name))
		logFailed(config.SaveBoarded(w.Root(), w.WS().Boarded))
		out.IDEErr = ide.Regenerate(w.Root(), w.WS().Boarded, w.WS().DisplayNames, w.WS().RemoteOf)
	}
	branch := workspace.GitCurrentBranch(filepath.Join(w.WS().RepoDir(repo), name))
	owner, owned := w.Missions.FindByCapsule(repo, name)
	if owned && branch != "" {
		logFailed(w.Missions.Update(owner.Slug, func(m *mission.Mission) error {
			m.AddCapsuleBranch(repo, name, branch)
			return nil
		}))
	}
	if err := w.WS().RemoveWorktree(repo, name, o.Force); err != nil {
		return out, err
	}
	if owned {
		out.Mission = owner.Slug
		w.Missions.AppendLogAfter(owner.Slug, mission.Event{Kind: mission.EventCapsuleBurned, Repo: repo, Capsule: name, By: o.By.By()})
	} else if !o.quiet {
		w.emit(events.Event{Kind: events.CapsuleBurned, Repo: repo, Capsule: name}, o.By)
	}
	if o.DropBranch && branch != "" {
		logFailed(git.Run(w.WS().BareDir(repo), "branch", "-q", "-D", branch))
	}
	if target, ok := w.WS().Silo[repo]; ok && target == name {
		out.Silo = true
		if o.Silo == SiloDrop {
			delete(w.WS().Silo, repo)
		} else {
			w.WS().Silo[repo] = workspace.GroundDir
			if _, err := workspace.FullSync(w.WS().MainWorktree(repo), w.WS().SiloWorktree(repo)); err != nil {
				out.SiloErr = err
			}
		}
		logFailed(config.SaveSilo(w.Root(), w.WS().Silo))
	}
	return out, nil
}

// Landed is a capsule landed: its commits on the default branch, the
// mission that owns it told, and the capsule burned.
type Landed struct {
	*workspace.Landed
	Mission string // the mission that owns the capsule, if one does
	// RecordErr is why the mission hasn't the landing, if it hasn't. The
	// capsule is kept then: burned, the mission would have neither it nor
	// what became of it.
	RecordErr error
	BurnErr   error // why the capsule's still there, if it is
}

// Land lands a capsule of a repo that takes commits straight to its
// default branch, for you: never for crew or the orchestrator, as a
// landing is public the moment it's made. A confirmed push is returned
// with any errors from recording the landing or cleaning up the capsule.
func (w *Workspace) Land(repo, name string, by Actor, o workspace.LandOptions) (Landed, error) {
	return w.land(repo, name, by, o, false)
}

// land is Land; quiet keeps no event of a capsule of no mission landed,
// nor of its burning (BurnOptions.quiet).
func (w *Workspace) land(repo, name string, by Actor, o workspace.LandOptions, quiet bool) (Landed, error) {
	if err := MayLand(by); err != nil {
		return Landed{}, err
	}
	ws := w.WS()
	lock, err := os.OpenFile(filepath.Join(ws.BareDir(repo), "ws-land.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return Landed{}, fmt.Errorf("open landing lock: %w", err)
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return Landed{}, refused("landing", "%s already has a landing in progress", repo)
		}
		return Landed{}, fmt.Errorf("lock landing: %w", err)
	}

	missions, err := w.Missions.List()
	if err != nil {
		return Landed{}, fmt.Errorf("find the mission before landing: %w", err)
	}
	dir := filepath.Join(ws.RepoDir(repo), name)
	var owner *mission.Mission
	for _, m := range missions {
		for _, c := range m.Capsules {
			if c.Repo != repo || c.Name != name {
				continue
			}
			owner = m
			if p := c.PendingLanding; p != nil {
				if p.Branch != ws.DefaultBranch {
					return Landed{}, refused("landing", "%s/%s has a pending landing on %s. Restore that default branch and retry the landing first", repo, name, p.Branch)
				}
				o.Pending = &workspace.Landed{Repo: repo, Capsule: name, Branch: p.Branch, Head: p.Head, Commits: p.Commits, Settled: p.Settled}
			}
			break
		}
		if owner != nil {
			break
		}
	}
	if owner != nil {
		beforeRebase := o.BeforeRebase
		o.BeforeRebase = func() error {
			if beforeRebase != nil {
				if err := beforeRebase(); err != nil {
					return err
				}
			}
			return w.holdReview(GuaranteeReviewBeforeLand, owner.Slug, repo, name, "landing", reviewThen(by, "land it"))
		}
		beforePush := o.BeforePush
		o.BeforePush = func(candidate workspace.LandCandidate) error {
			if beforePush != nil {
				if err := beforePush(candidate); err != nil {
					return err
				}
			}
			if err := w.holdLandingReview(owner.Slug, dir, repo+"/"+name, by, candidate); err != nil {
				return err
			}
			err := w.Missions.PrepareLanding(owner.Slug, repo, name, mission.LandingAttempt{
				At: time.Now(), Head: candidate.Head,
				Branch: candidate.Branch, Commits: candidate.Commits, Settled: candidate.Settled,
			})
			if err != nil {
				return fmt.Errorf("couldn't record the landing before pushing: %w", err)
			}
			return nil
		}
	}
	landed, err := ws.Land(repo, name, o)
	if err != nil {
		return Landed{}, err
	}
	out := Landed{Landed: landed}
	if owner == nil && !quiet {
		// A mission's landing is kept as it records it (RecordLanding).
		w.emit(events.Event{Kind: events.CapsuleLanded, Repo: repo, Capsule: name,
			Text: fmt.Sprintf("%s on %s (%s)", commitsWord(len(landed.Commits)), landed.Branch, short(landed.Head))}, by)
	}
	if owner != nil {
		out.Mission = owner.Slug
		if workspace.GitRevParse(dir, "HEAD") != landed.Head {
			out.RecordErr = fmt.Errorf("%s/%s moved during landing", repo, name)
			return out, nil
		}
		text := fmt.Sprintf("%s on %s (%s)", commitsWord(len(landed.Commits)), landed.Branch, short(landed.Head))
		if err := w.Missions.RecordLanding(owner.Slug, repo, name, landed.Head, text); err != nil {
			out.RecordErr = err
			return out, nil
		}
		w.youActed(owner.Slug, by)
		w.passOnLand(owner.Slug, repo, name)
	}
	if workspace.GitRevParse(dir, "HEAD") != landed.Head {
		out.BurnErr = fmt.Errorf("%s/%s moved during landing", repo, name)
	} else if _, err := w.Burn(repo, name, BurnOptions{By: by, quiet: quiet}); err != nil {
		out.BurnErr = err
	}
	return out, nil
}

func (w *Workspace) holdLandingReview(slug, dir, capsule string, by Actor, c workspace.LandCandidate) error {
	g := GuaranteeReviewBeforeLand
	if !w.holding(g) {
		return nil
	}
	refuse := func(why string) error {
		return refused("guarantee", "ws holds %s before landing: %s. %s", g.Key, why, reviewThen(by, "land it"))
	}
	r, err := review.For(w.Root(), slug).Latest(capsule)
	if err != nil {
		return refuse(fmt.Sprintf("the latest review couldn't be read (%v)", err))
	}
	if r == nil || r.Verdict != review.Approve || (r.Head != c.OriginalHead && r.Head != c.Head) {
		return refuse("the latest review doesn't approve " + capsule + " at the commit being landed")
	}
	if r.Head == c.Head {
		return nil
	}
	// Compare the tree about to be pushed with a clean replay of the
	// approved change. Rebase or settlement can change the submitted diff.
	ctx := context.Background()
	base, err := git.ReadLine(ctx, dir, "merge-base", c.Base, r.Head)
	if err == nil && base == r.Base {
		expected, err := git.ReadLine(ctx, dir, "merge-tree", "--write-tree", "--no-messages", c.Base, r.Head)
		if err == nil {
			actual, err := git.ReadLine(ctx, dir, "rev-parse", c.Head+"^{tree}")
			if err == nil && actual == expected {
				return nil
			}
		}
	}
	return refuse("the rebased changes differ from what was approved")
}

// MayLand refuses anyone but you: a landing is public the moment it's
// made.
func MayLand(by Actor) error {
	who := ""
	switch by.Kind {
	case Crew:
		who = "crew (" + by.ID + ")"
	case Orchestrator:
		who = "the orchestrator"
	}
	if who != "" {
		return refused("not-yours", "landing is the user's to do, not %s: commit your work, say it's done (ws mission done), and they'll review it and land it", who)
	}
	return nil
}

func commitsWord(n int) string {
	if n == 1 {
		return "1 commit"
	}
	return fmt.Sprintf("%d commits", n)
}

func short(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

// Landing is where a capsule stands for landing: whether its repo takes
// commits straight to its default branch (Branch), and for one that does,
// the commits it would land, whether it has changes not committed, what
// must pass first, the mission it's of and whether your latest review of
// it approves it as it is, and its landing, if it's landed as it is.
type Landing struct {
	Direct   bool
	Branch   string
	Commits  []string
	Dirty    bool
	Verify   string
	Mission  string
	Approved bool
	Pending  bool
	Landed   *LandedOn
}

// LandedOn is what a landing put on a branch, and anything that didn't go
// as it should after it (Warnings).
type LandedOn struct {
	Branch, Head string
	Commits      []string
	Mission      string
	Warnings     []string
}

// LandStop is a landing that stopped, with nothing pushed: the step it
// stopped at, why, and what was said.
type LandStop struct{ Step, Message, Output string }

// LandOutcome is what landing a capsule did: landed, or stopped.
type LandOutcome struct {
	Landed  *LandedOn
	Stopped *LandStop
}

// capsuleOf checks repo has a capsule name: ErrNotFound if not.
func (w *Workspace) capsuleOf(repo, name string) error {
	ws := w.WS()
	capsules, _ := workspace.ListWorktrees(ws.RepoDir(repo))
	if !slices.Contains(ws.RepoNames, repo) || !slices.Contains(capsules, name) {
		return ErrNotFound
	}
	return nil
}

// Landing is where a capsule stands for landing.
func (w *Workspace) Landing(repo, name string) (*Landing, error) {
	if err := w.capsuleOf(repo, name); err != nil {
		return nil, err
	}
	ws := w.WS()
	l := &Landing{Direct: ws.Direct(repo), Branch: ws.DefaultBranch, Commits: []string{}}
	if !l.Direct {
		return l, nil
	}
	dir := filepath.Join(ws.RepoDir(repo), name)
	l.Verify = ws.VerifyCommand(repo)
	l.Dirty = workspace.GitIsDirty(dir)
	if out := git.Lines(dir, "log", "--reverse", "--format=%s", "origin/"+ws.DefaultBranch+"..HEAD"); len(out) > 0 {
		l.Commits = out
	}
	head := workspace.GitRevParse(dir, "HEAD")
	if m, ok := w.Missions.FindByCapsule(repo, name); ok {
		l.Mission = m.Slug
		r, _ := review.For(w.Root(), m.Slug).Latest(repo + "/" + name)
		l.Approved = r != nil && r.Verdict == review.Approve && r.Head == head
		for _, c := range m.Capsules {
			if c.Repo == repo && c.Name == name {
				l.Pending = c.PendingLanding != nil
				if c.Landed != nil && c.Landed.Head == head {
					l.Landed = &LandedOn{Branch: l.Branch, Head: c.Landed.Head, Mission: m.Slug}
				}
			}
		}
	}
	return l, nil
}

// LandCapsule lands a capsule for you, as ws land does: a landing that
// stops, with nothing pushed, is an outcome, not an error.
func (w *Workspace) LandCapsule(repo, name string) (*LandOutcome, error) {
	if err := w.capsuleOf(repo, name); err != nil {
		return nil, err
	}
	out, err := w.Land(repo, name, AsYou, workspace.LandOptions{Quiet: true})
	if stopped, ok := errors.AsType[*workspace.LandError](err); ok {
		// A sentence, unless it starts with the capsule's name.
		msg := stopped.Msg
		if !strings.HasPrefix(msg, repo) && msg != "" {
			msg = strings.ToUpper(msg[:1]) + msg[1:]
		}
		return &LandOutcome{Stopped: &LandStop{Step: stopped.Step, Message: msg, Output: stopped.Output}}, nil
	}
	if err != nil {
		return nil, err
	}
	l := out.Landed
	res := &LandedOn{Branch: l.Branch, Head: l.Head, Commits: l.Commits, Mission: out.Mission}
	for _, said := range l.Settled {
		// An artifact's version that landed under another number.
		res.Warnings = append(res.Warnings, "Artifacts: "+said)
	}
	if l.Ground != nil {
		res.Warnings = append(res.Warnings, "The ground wasn't brought up to it: "+l.Ground.Error())
	}
	switch {
	case out.RecordErr != nil:
		res.Warnings = append(res.Warnings, out.Unrecorded())
	case out.BurnErr != nil:
		res.Warnings = append(res.Warnings, "The capsule is still there: "+out.BurnErr.Error())
	}
	return &LandOutcome{Landed: res}, nil
}

// Unrecorded says a landing went out but its mission couldn't be told.
func (l Landed) Unrecorded() string {
	return fmt.Sprintf("Mission %s couldn't record the landing (%v). The capsule is kept, so the mission still has it: its commits are on %s. Retry the landing to finish recording it",
		l.Mission, l.RecordErr, l.Landed.Branch)
}
