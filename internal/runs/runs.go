// Package runs is what ws starts for someone and ends for them: a dev
// server, a compose stack, a preview. A run is given a free port, and
// kept with who it's for (its owner) and how long it may last. Once
// either has gone, ws ends it, so nobody has to remember to.
//
// It's `just try`'s teardown (scripts/try.sh) for everyone's runs. The
// record is kept in the run's own folder, .ws/runs/<repo>/<worktree>/<name>/,
// and nowhere else: removing the folder removes the run's record. The
// daemon sweeps the folders every SweepEvery and as it starts, so a
// restart or an upgrade loses nothing and leaves nothing running. A run
// is never the daemon's child: it leads a session of its own, and outlives
// whichever ws started it.
//
// ws ends only what it started: a run's process group, as long as its
// leader is the process the record says (by its start time, so a pid
// given out again isn't taken for it).
package runs

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Version is the record's version (run.json's "version").
const Version = 1

const (
	// DefaultTTL is how long a run lasts when it's not told.
	DefaultTTL = 4 * time.Hour
	// SweepEvery is how often the daemon looks for runs to end.
	SweepEvery = 15 * time.Second
	// KeepEnded is how long a run that ended by itself is kept, its log
	// with it, before a sweep removes it.
	KeepEnded = time.Hour
	// LookLinger is how long a run owned by looking lasts with nobody
	// looking.
	LookLinger = 15 * time.Minute
	// OwnerAge is how long a process has to have been running to be a
	// run's owner when none is named: the shells, just and ws between it
	// and the run were started for this command, and end with it.
	OwnerAge = time.Minute
)

// Kind is what a run's owner is.
type Kind string

const (
	// ByPID is a process: the terminal's shell, or the agent, that
	// started it. It ends when that process exits.
	ByPID Kind = "pid"
	// ByCapsule ends when the capsule it runs in is burned or landed.
	ByCapsule Kind = "capsule"
	// ByMission ends when the mission ends or is archived.
	ByMission Kind = "mission"
	// ByStep ends when a step of a mission's plan passes or is skipped:
	// what crew start is for the step they're on.
	ByStep Kind = "step"
	// ByCrew ends when crew's session closes.
	ByCrew Kind = "crew"
	// ByConversation ends when a conversation is resolved.
	ByConversation Kind = "conversation"
	// ByLooking ends once nobody has had the page it was started from
	// open for LookLinger: "Run it" in the web UI.
	ByLooking Kind = "looking"
	// ByNone ends only when its time is up, or when it's stopped.
	ByNone Kind = "none"
	// ByRun is the runs that need it: one ws started because another
	// needed it, which ends once nothing that needs it is running.
	ByRun Kind = "run"
)

// Kinds are the owners `ws run --for` takes.
var Kinds = []Kind{ByPID, ByCapsule, ByMission, ByStep, ByCrew, ByConversation, ByNone}

// Owner is who a run is for.
type Owner struct {
	Kind Kind `json:"kind"`
	// PID, Began and Command are the owning process, for ByPID: Began is
	// its start as the kernel has it, and Command its name, to say.
	PID     int       `json:"pid,omitempty"`
	Began   time.Time `json:"began,omitzero"`
	Command string    `json:"command,omitempty"`
	// Mission is the mission's slug, for ByMission, ByStep, ByCrew, and
	// ByConversation for one of a mission's conversations.
	Mission string `json:"mission,omitempty"`
	// Step is the step's number when the run started, and StepText its
	// words, which find it again after the plan's renumbered (ByStep).
	Step     int    `json:"step,omitempty"`
	StepText string `json:"step_text,omitempty"`
	// Crew is the crew's id (ByCrew), and Conversation the conversation's
	// (ByConversation).
	Crew         string `json:"crew,omitempty"`
	Conversation string `json:"conversation,omitempty"`
	// App is the workspace recipe whose app page is looked at, for a run
	// started by opening it (ByLooking with no Mission).
	App string `json:"app,omitempty"`
}

// Owners says whether owners ws keeps records of (missions, their steps
// and crew, conversations) have ended. ws's records are the caller's:
// runs knows processes and folders.
type Owners interface {
	// Ended is why o has ended, or "" while it hasn't, or when that can't
	// be told.
	Ended(o Owner) string
}

// OwnersFunc is a func as Owners.
type OwnersFunc func(o Owner) string

// Ended is f(o).
func (f OwnersFunc) Ended(o Owner) string { return f(o) }

// Record is a run as run.json keeps it.
type Record struct {
	Version int    `json:"version"`
	Name    string `json:"name"`
	Repo    string `json:"repo"`
	// Worktree is the folder in the repo it runs in: a capsule's name,
	// .ground or .silo.
	Worktree string `json:"worktree"`
	// Dir is where its command runs.
	Dir     string `json:"dir"`
	Command string `json:"command"`
	// Recipe is the repo's [run.<name>] it was started from, if it was.
	// Ready and Stop are that recipe's (config.RunRecipe).
	Recipe string `json:"recipe,omitempty"`
	Ready  string `json:"ready,omitempty"`
	Stop   string `json:"stop,omitempty"`
	Port   int    `json:"port"`
	// PID leads the run's session and process group; Began is its start,
	// as the kernel has it.
	PID   int       `json:"pid"`
	Began time.Time `json:"began"`
	Owner Owner     `json:"owner"`
	// Also are who else it's for: those who asked for it while it was
	// running already, as a recipe runs once in a worktree. It's ended
	// for its owner's going only once every one of them has gone.
	Also []Owner `json:"also,omitempty"`
	// Needs are what it was started needing, each met by another run or
	// by an address outside ws. While it runs, the runs that meet them
	// aren't ended.
	Needs []Need `json:"needs,omitempty"`
	// Hold is until when a run owned by what needs it (ByRun) isn't ended
	// for nothing needing it: it's up before the run it's for is started.
	Hold    time.Time `json:"hold,omitzero"`
	Created time.Time `json:"created"`
	// Expires is when its time is up: zero for never.
	Expires time.Time `json:"expires,omitzero"`
	// By is who started it: "you", or crew's id.
	By string `json:"by,omitempty"`
	// Looked is when someone last had open the page it was started
	// from, for a run owned by looking.
	Looked time.Time `json:"looked,omitzero"`
}

// Need is something a run was started needing: another run, found or
// started for it, an address outside ws, or a repo's folder.
type Need struct {
	// Name is what the run calls it (api).
	Name string `json:"name"`
	// Run is the run that meets it, by its ref (server.ground.backend),
	// which is Recipe in Repo's Worktree. With no run, an address outside
	// ws meets it, and nothing of ws's is behind it.
	Run      string `json:"run,omitempty"`
	Repo     string `json:"repo,omitempty"`
	Worktree string `json:"worktree,omitempty"`
	Recipe   string `json:"recipe,omitempty"`
	// Addr is the address the run was started with: the need's run's, or
	// the one outside ws.
	Addr string `json:"addr"`
	// Env is the variable Addr was handed over in.
	Env string `json:"env,omitempty"`
	// Mount is the path of the run's own name it answers at, through the
	// router (internal/router); Strip takes that path off what it's asked.
	Mount string `json:"mount,omitempty"`
	Strip bool   `json:"strip,omitempty"`
	// Dir is the folder that meets it, for a need of a repo's folder (one
	// of Repo's worktrees, Worktree) rather than a run: Run and Addr are
	// then "". Once it's gone, so is what the run was started on.
	Dir string `json:"dir,omitempty"`
}

// IsFolder is whether a folder meets the need, not a run or an address.
func (n Need) IsFolder() bool { return n.Dir != "" }

// Owners is everyone the run is for: its owner, then those who asked for
// it since.
func (r Record) Owners() []Owner {
	return append([]Owner{r.Owner}, r.Also...)
}

// State is where a run is.
type State string

const (
	Running State = "running"
	// Exited is a run whose command ended by itself, with its status.
	Exited State = "exited"
	// Gone is a run with nothing running and no status: it was killed
	// outside ws, or the machine restarted under it.
	Gone State = "gone"
)

// Run is a run's record, where it's kept and what it's doing.
type Run struct {
	Record
	// Path is its folder.
	Path  string
	State State
	// Status is the command's exit status, and Ended when it ended, for
	// an Exited run (Ended is when it was found gone, for a Gone one).
	Status int
	Ended  time.Time
	// NeededBy are the running runs that need it, by their refs, as List
	// finds them: while there are any, it isn't ended.
	NeededBy []string
}

// Label is how a worktree is said in a run's name: ground, silo, or the
// capsule's name.
func Label(worktree string) string {
	switch worktree {
	case ".ground":
		return "ground"
	case ".silo":
		return "silo"
	}
	return worktree
}

// Ref is how a run is named everywhere: <name>.<worktree>.<repo>, as its
// local address will have it. A workspace's own run (Repo "") is its name
// alone.
func (r Run) Ref() string {
	if r.Repo == "" {
		return r.Name
	}
	return r.Name + "." + Label(r.Worktree) + "." + r.Repo
}

// OfWorkspace is whether the run is the workspace's own (a recipe of
// ws.toml's, run at its root), rather than a repo's.
func (r Run) OfWorkspace() bool { return r.Repo == "" }

// workspacePlace is the folders a workspace's own runs are kept in, below
// the store's, in place of a repo's and a worktree's: no repo can be
// called @workspace, as GitHub has no @ in a name.
var workspacePlace = [2]string{"@workspace", "@"}

// place is the folder below the store's that runs of repo's worktree are
// kept in.
func (s Store) place(repo, worktree string) string {
	if repo == "" {
		return filepath.Join(s.Dir(), workspacePlace[0], workspacePlace[1])
	}
	return filepath.Join(s.Dir(), repo, worktree)
}

// Link is where the run is reached.
func (r Run) Link() string { return fmt.Sprintf("http://127.0.0.1:%d", r.Port) }

// LogPath is the run's output, its standard output and error.
func (r Run) LogPath() string { return filepath.Join(r.Path, logFile) }

const (
	recordFile = "run.json"
	logFile    = "out.log"
	// exitFile is written by the run's wrapper as its command ends: the
	// command's status.
	exitFile = "exit"
)

// Store is a workspace's runs.
type Store struct {
	Root string
	// Procs reads the user's processes: nil is the system's.
	Procs func() (Procs, error)
}

// Dir is where runs are kept.
func (s Store) Dir() string { return filepath.Join(s.Root, ".ws", "runs") }

func (s Store) snapshot() Procs {
	read := s.Procs
	if read == nil {
		read = ReadProcs
	}
	p, err := read()
	if err != nil {
		return Procs{}
	}
	return p
}

// List is every run, by repo, worktree and name.
func (s Store) List() ([]Run, error) {
	paths, err := filepath.Glob(filepath.Join(s.Dir(), "*", "*", "*", recordFile))
	if err != nil {
		return nil, err
	}
	procs := s.snapshot()
	var out []Run
	for _, p := range paths {
		r, err := load(filepath.Dir(p), procs)
		if err != nil {
			continue // one half-written, or not ours: a sweep leaves it be
		}
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Repo != b.Repo {
			return a.Repo < b.Repo
		}
		if a.Worktree != b.Worktree {
			return a.Worktree < b.Worktree
		}
		return a.Name < b.Name
	})
	needed := map[string][]string{}
	for _, r := range out {
		if r.State != Running {
			continue
		}
		for _, n := range r.Needs {
			if n.Run != "" {
				needed[n.Run] = append(needed[n.Run], r.Ref())
			}
		}
	}
	for i, r := range out {
		if r.State == Running {
			out[i].NeededBy = needed[r.Ref()]
		}
	}
	return out, nil
}

// load reads the run kept in dir.
func load(dir string, procs Procs) (Run, error) {
	b, err := os.ReadFile(filepath.Join(dir, recordFile))
	if err != nil {
		return Run{}, err
	}
	var r Run
	if err := json.Unmarshal(b, &r.Record); err != nil {
		return Run{}, fmt.Errorf("%s: %w", dir, err)
	}
	if r.Version > Version {
		return Run{}, fmt.Errorf("%s: a run recorded by a newer ws (version %d)", dir, r.Version)
	}
	r.Path = dir
	r.State = Running
	if b, err := os.ReadFile(filepath.Join(dir, exitFile)); err == nil {
		r.State = Exited
		r.Status, _ = strconv.Atoi(strings.TrimSpace(string(b)))
		if fi, err := os.Stat(filepath.Join(dir, exitFile)); err == nil {
			r.Ended = fi.ModTime()
		}
	} else if len(procs.group(r.PID, r.Began)) == 0 {
		r.State = Gone
		r.Ended = time.Now()
		if fi, err := os.Stat(filepath.Join(dir, logFile)); err == nil {
			r.Ended = fi.ModTime() // its last words, the nearest to when it went
		}
	}
	return r, nil
}

// save writes the run's record, whole, in place of what was there.
func save(r Record, dir string) error {
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	tmp := filepath.Join(dir, recordFile+".new")
	if err := os.WriteFile(tmp, append(b, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(dir, recordFile))
}

// Find is the run ref names: <name>.<worktree>.<repo>, or a name alone for
// a run in here (repo and worktree, where the caller stands). A worktree
// may be said as ground or silo.
func (s Store) Find(ref, repo, worktree string) (Run, error) {
	runs, err := s.List()
	if err != nil {
		return Run{}, err
	}
	var match []Run
	for _, r := range runs {
		if r.Ref() == ref || (r.Name == ref && r.Repo == repo && r.Worktree == worktree) {
			match = append(match, r)
		}
	}
	if len(match) == 0 {
		for _, r := range runs {
			if r.Name == ref {
				match = append(match, r)
			}
		}
	}
	switch len(match) {
	case 1:
		return match[0], nil
	case 0:
		return Run{}, fmt.Errorf("no run %s (ws run ls lists them)", ref)
	}
	var refs []string
	for _, r := range match {
		refs = append(refs, r.Ref())
	}
	return Run{}, fmt.Errorf("there are runs named %s in several places: say which (%s)", ref, strings.Join(refs, ", "))
}

// Looked marks the runs owned by looking at the missions in slugs as
// looked at now.
func (s Store) Looked(slugs map[string]bool, now time.Time) error {
	runs, err := s.List()
	if err != nil {
		return err
	}
	var errs []error
	for _, r := range runs {
		looking := slices.ContainsFunc(r.Owners(), func(o Owner) bool { return o.Kind == ByLooking && o.App == "" && slugs[o.Mission] })
		if r.State == Running && looking {
			r.Looked = now.UTC().Truncate(time.Second)
			if err := save(r.Record, r.Path); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}

// LookedRun marks r as looked at when: a run started again keeps when it
// was last looked at.
func (s Store) LookedRun(r Run, when time.Time) error {
	r.Looked = when.UTC().Truncate(time.Second)
	return save(r.Record, r.Path)
}

// LookedApps marks the runs owned by looking at the apps in names (the
// workspace's recipes with a page) as looked at now.
func (s Store) LookedApps(names map[string]bool, now time.Time) error {
	runs, err := s.List()
	if err != nil {
		return err
	}
	var errs []error
	for _, r := range runs {
		looking := slices.ContainsFunc(r.Owners(), func(o Owner) bool { return o.Kind == ByLooking && o.App != "" && names[o.App] })
		if r.State == Running && looking {
			r.Looked = now.UTC().Truncate(time.Second)
			if err := save(r.Record, r.Path); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}

// LookedAt marks the runs of repo's worktree (the workspace's own, with no
// repo) that are owned by looking at an app or a recipe from its page as
// looked at now: someone has the page open.
func (s Store) LookedAt(repo, worktree string, now time.Time) error {
	runs, err := s.List()
	if err != nil {
		return err
	}
	var errs []error
	for _, r := range runs {
		if r.State != Running || r.Repo != repo || r.Worktree != worktree {
			continue
		}
		if slices.ContainsFunc(r.Owners(), func(o Owner) bool { return o.Kind == ByLooking && o.App != "" }) {
			r.Looked = now.UTC().Truncate(time.Second)
			if err := save(r.Record, r.Path); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}

// Keep sets how long a run has left from now: d of 0 is no limit.
func (s Store) Keep(r Run, d time.Duration, now time.Time) (Run, error) {
	r.Expires = time.Time{}
	if d > 0 {
		r.Expires = now.Add(d).UTC().Truncate(time.Second)
	}
	if err := save(r.Record, r.Path); err != nil {
		return r, err
	}
	return r, nil
}

var notSlug = regexp.MustCompile(`[^a-z0-9]+`)

// Slug is s as a name: lower case, letters, numbers and single dashes.
func Slug(s string) string {
	return strings.Trim(notSlug.ReplaceAllString(strings.ToLower(s), "-"), "-")
}

// NameFor is the name a command's run gets when it's given none: its
// first word's, as a slug.
func NameFor(command string) string {
	fields := strings.Fields(command)
	for _, f := range fields {
		if strings.Contains(f, "=") && !strings.HasPrefix(f, "=") {
			continue // FOO=bar before the command
		}
		if n := Slug(filepath.Base(f)); n != "" {
			return n
		}
	}
	return "run"
}

// claim takes a name for a run in repo's worktree: want, or the first of
// want-2, want-3… not taken by a run that's running. A run of that name
// that has ended gives up its name (and its folder) to the new one.
//
// With recipe said, the run is that recipe's, which runs once in a
// worktree: where a run of it holds the name already, or is being started
// under it this moment by another ws, that run is returned (running) and
// nothing is claimed.
func (s Store) claim(repo, worktree, want, recipe string) (name, dir string, running *Run, err error) {
	parent := s.place(repo, worktree)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return "", "", nil, err
	}
	procs := s.snapshot()
	for i := 1; i < 1000; i++ {
		name = want
		if i > 1 {
			name = fmt.Sprintf("%s-%d", want, i)
		}
		dir = filepath.Join(parent, name)
		err := os.Mkdir(dir, 0o755)
		if err == nil {
			return name, dir, nil, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return "", "", nil, err
		}
		old, lerr := load(dir, procs)
		if lerr != nil {
			if _, serr := os.Stat(filepath.Join(dir, recordFile)); errors.Is(serr, os.ErrNotExist) {
				// A folder with no record long after it was made is a start
				// that was killed before it recorded anything: the name's free.
				if fi, ferr := os.Stat(dir); ferr == nil && time.Since(fi.ModTime()) > 2*claimWait {
					if os.RemoveAll(dir) == nil && os.Mkdir(dir, 0o755) == nil {
						return name, dir, nil, nil
					}
				}
				if recipe == "" {
					continue // another ws claiming it this moment, or not ours
				}
				// Another ws is starting something under the name now: if
				// it's this recipe, it's the one run of it.
				if old, lerr = s.claimed(dir); lerr != nil {
					continue
				}
			}
		}
		if lerr == nil && old.State == Running && recipe != "" && old.Recipe == recipe {
			return "", "", &old, nil
		}
		if lerr == nil && old.State != Running {
			if os.RemoveAll(dir) == nil && os.Mkdir(dir, 0o755) == nil {
				return name, dir, nil, nil
			}
		}
	}
	if repo == "" {
		return "", "", nil, fmt.Errorf("no free name for %s in the workspace", want)
	}
	return "", "", nil, fmt.Errorf("no free name for %s in %s/%s", want, repo, worktree)
}

// claimWait is how long a recipe's start waits for another ws, claiming
// the recipe's name at the same moment, to record what it started.
var claimWait = 5 * time.Second

// claimed is the run another ws is recording in dir, once it has.
func (s Store) claimed(dir string) (Run, error) {
	var err error
	for deadline := time.Now().Add(claimWait); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
		var r Run
		if r, err = load(dir, s.snapshot()); err == nil {
			return r, nil
		}
		if _, serr := os.Stat(dir); errors.Is(serr, os.ErrNotExist) {
			return Run{}, serr // its start failed, and its claim went with it
		}
	}
	return Run{}, err
}
