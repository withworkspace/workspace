package ops

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/artifact"
	"github.com/withworkspace/workspace/internal/config"
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
	if !o.Bare {
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
	config.SaveBoarded(w.Root(), w.WS().Boarded)
	return ide.Regenerate(w.Root(), w.WS().Boarded, w.WS().DisplayNames, w.WS().Org)
}

// What becomes of a silo pointed at a capsule that's burned.
const (
	SiloToGround = iota // it's pointed at the ground, which is synced into it
	SiloDrop            // it's pointed at nothing
)

// BurnOptions are how a capsule's burned.
type BurnOptions struct {
	Force      bool // even with changes nobody's committed
	DropBranch bool // its branch is deleted too: ws made it, and nothing's on it
	Silo       int  // SiloToGround or SiloDrop
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
		w.WS().Unboard(repo, name)
		config.SaveBoarded(w.Root(), w.WS().Boarded)
		out.IDEErr = ide.Regenerate(w.Root(), w.WS().Boarded, w.WS().DisplayNames, w.WS().Org)
	}
	branch := workspace.GitCurrentBranch(filepath.Join(w.WS().RepoDir(repo), name))
	owner, owned := w.Missions.FindByCapsule(repo, name)
	if owned && branch != "" {
		w.Missions.Update(owner.Slug, func(m *mission.Mission) error {
			m.AddCapsuleBranch(repo, name, branch)
			return nil
		})
	}
	if err := w.WS().RemoveWorktree(repo, name, o.Force); err != nil {
		return out, err
	}
	if owned {
		out.Mission = owner.Slug
		w.Missions.AppendLog(owner.Slug, mission.Event{Kind: mission.EventCapsuleBurned, Repo: repo, Capsule: name})
	}
	if o.DropBranch && branch != "" {
		git.Run(w.WS().BareDir(repo), "branch", "-q", "-D", branch)
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
		config.SaveSilo(w.Root(), w.WS().Silo)
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
// landing is public the moment it's made. An error is a landing that
// stopped (a *workspace.LandError), with nothing pushed. Once it's pushed
// there's no error to give: what went wrong after is in what comes back.
func (w *Workspace) Land(repo, name string, by Actor, o workspace.LandOptions) (Landed, error) {
	if err := MayLand(by); err != nil {
		return Landed{}, err
	}
	if owner, ok := w.Missions.FindByCapsule(repo, name); ok && w.WS().Direct(repo) {
		if err := w.holdReview(GuaranteeReviewBeforeLand, owner.Slug, repo, name, "landing", reviewThen(by, "land it")); err != nil {
			return Landed{}, err
		}
	}
	if c := w.ArtifactsConfig(); c.Repo == repo {
		// Two people can add v3 to the same artifact, or make folders by
		// one name: the second to land takes the next.
		settler := artifact.Settler{Dir: c.Dir, Name: c.Name}
		if o.Settle == nil {
			o.Settle = settler.Settle
		}
		if o.Check == nil {
			o.Check = settler.Check
		}
	}
	landed, err := w.WS().Land(repo, name, o)
	if err != nil {
		return Landed{}, err
	}
	out := Landed{Landed: landed}
	if owner, ok := w.Missions.FindByCapsule(repo, name); ok {
		out.Mission = owner.Slug
		at := time.Now()
		record := func() error {
			_, err := w.Missions.Update(owner.Slug, func(m *mission.Mission) error {
				for i := range m.Capsules {
					if c := &m.Capsules[i]; c.Repo == repo && c.Name == name {
						c.Landed = &mission.Landing{At: at, Head: landed.Head}
					}
				}
				return nil
			})
			return err
		}
		// The commits are public now, and the record is all that says so
		// once the capsule's gone: a mission that's busy gets a second go.
		if err := record(); err != nil {
			if err = record(); err != nil {
				out.RecordErr = err
				return out, nil
			}
		}
		w.Missions.AppendLog(owner.Slug, mission.Event{Kind: mission.EventLanded, Repo: repo, Capsule: name, Crew: "you",
			Text: fmt.Sprintf("%s on %s (%s)", commitsWord(len(landed.Commits)), landed.Branch, short(landed.Head))})
		w.youActed(owner.Slug, by)
		// A Land step waiting on it is done.
		w.passOnLand(owner.Slug, repo, name)
	}
	if _, err := w.Burn(repo, name, BurnOptions{}); err != nil {
		out.BurnErr = err
	}
	return out, nil
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
			if c.Repo == repo && c.Name == name && c.Landed != nil && c.Landed.Head == head {
				l.Landed = &LandedOn{Branch: l.Branch, Head: c.Landed.Head, Mission: m.Slug}
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
	var stopped *workspace.LandError
	if errors.As(err, &stopped) {
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
	return fmt.Sprintf("Mission %s couldn't record the landing (%v). The capsule is kept, so the mission still has it: its commits are on %s",
		l.Mission, l.RecordErr, l.Landed.Branch)
}
