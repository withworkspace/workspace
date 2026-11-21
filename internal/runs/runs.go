// Package runs is what ws starts for someone and ends for them: a dev
// server, a compose stack, a preview. A run is given a free port, and
// kept with who it's for (its owner) and how long it may last. Once
// either has gone, ws ends it, so nobody has to remember to.
//
// That's a service, one type of run. The other is a task: tests, lint, a
// build, which ends by itself, and whose worth is what it came to. A task
// has no port. Once it has ended it's kept: how it exited, how long it
// took, all it printed, the commit it ran at and the reports it wrote,
// with the last KeepPast runs of the same task behind it (.past).
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
	// TaskTimeout is how long a task may take when neither its recipe nor
	// whoever asks says: a task ends by itself, and one still running then
	// is stopped, as something that was meant to be kept up is a service.
	TaskTimeout = time.Hour
	// SweepEvery is how often the daemon looks for runs to end.
	SweepEvery = 15 * time.Second
	// KeepEnded is how long a service that ended by itself is kept, its
	// log with it, before a sweep removes it.
	KeepEnded = time.Hour
	// KeepResults is how long a task's result is kept after it ended,
	// while where it ran is there; and KeepPast how many earlier runs of
	// the task are kept behind its latest.
	KeepResults = 30 * 24 * time.Hour
	KeepPast    = 10
	// LookLinger is how long a run owned by looking lasts with nobody
	// looking.
	LookLinger = 15 * time.Minute
	// OwnerAge is how long a process has to have been running to be a
	// run's owner when none is named: the shells, just and ws between it
	// and the run were started for this command, and end with it.
	OwnerAge = time.Minute
)

// Type is which sort of run one is.
type Type string

const (
	// Service is a run that's kept up until it's ended: a dev server. It
	// has a port, and it's ended when its owner goes or its time is up.
	Service Type = "service"
	// Task is a run that ends by itself: tests, lint, a build. It has no
	// port, and what it came to is kept once it has ended.
	Task Type = "task"
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
	// open for LookLinger: "Run" in the web UI, on a mission's page (its
	// Mission) or on the Files page (no Mission: its own worktree).
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
	// ByConversation for one of a mission's conversations. For ByLooking
	// it's the mission whose page it was started from, or "" for the Files
	// page, where it's looked at while its own ground or capsule is open.
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
	// Type is Task for one that ends by itself; "" is a service.
	Type Type   `json:"type,omitempty"`
	Repo string `json:"repo"`
	// Worktree is the folder in the repo it runs in: a capsule's name,
	// .ground or .silo.
	Worktree string `json:"worktree"`
	// Dir is where its command runs.
	Dir     string `json:"dir"`
	Command string `json:"command"`
	// Recipe is the recipe ([run.service.<name>], [run.task.<name>]) it was
	// started from, if it was.
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
	// N is which run of the task this is, from 1, and Commit the commit
	// its worktree was at when it started, Dirty with changes not
	// committed: what a task's result is a result of.
	N      int    `json:"n,omitempty"`
	Commit string `json:"commit,omitempty"`
	Dirty  bool   `json:"dirty,omitempty"`
	// Reports are the files a task's command writes that are kept with
	// its result, as its recipe names them.
	Reports []Report `json:"reports,omitempty"`
	// Check is the check of a mission's step it was run for, if it was:
	// the mission's slug, the step's number and the check's, from 1.
	Check *CheckOf `json:"check,omitempty"`
}

// Report is a file, or the files a pattern matches, that a task's command
// writes: kept with the run once it has ended, for whatever reads its
// format to show.
type Report struct {
	// Name is what the recipe calls it (coverage).
	Name string `json:"name"`
	// Path is where the command writes it, from where it runs: a file, or
	// a pattern (junit/*.xml).
	Path string `json:"path"`
	// Format is what it's written in (lcov, gocover, junit…): a plugin
	// that reads the format shows it.
	Format string `json:"format,omitempty"`
}

// CheckOf is the check a task was run for.
type CheckOf struct {
	Mission string `json:"mission"`
	Step    int    `json:"step"`
	N       int    `json:"n"`
}

// IsTask is whether the run is a task: one that ends by itself, and is
// kept for what it came to.
func (r Record) IsTask() bool { return r.Type == Task }

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
	// Why is why a task was stopped before it ended by itself, when ws
	// stopped it.
	Why string
	// Past is how many earlier runs of the task are kept behind it.
	Past int
}

// Passed is whether a task ended by itself with status 0.
func (r Run) Passed() bool { return r.State == Exited && r.Status == 0 }

// Took is how long the run ran, once it has ended.
func (r Run) Took() time.Duration {
	if r.State == Running || r.Ended.IsZero() || r.Began.IsZero() || r.Ended.Before(r.Began) {
		return 0
	}
	return r.Ended.Sub(r.Began)
}

// Result is what a task came to, in a word or two: running, passed,
// failed (exit 2), or stopped, for one that didn't end by itself.
func (r Run) Result() string {
	switch {
	case r.State == Running:
		return "running"
	case r.State == Exited && r.Why == "" && r.Status == 0:
		return "passed"
	case r.State == Exited && r.Why == "":
		return fmt.Sprintf("failed (exit %d)", r.Status)
	}
	return "stopped"
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
// alone. A task's ref is its latest run's.
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

// NRef is one run of a task, by its number: test.my-capsule.workspace#3.
func (r Run) NRef() string {
	if r.N == 0 {
		return r.Ref()
	}
	return r.Ref() + "#" + strconv.Itoa(r.N)
}

// ReportsDir is where a task's reports are kept, each in a folder of its
// name.
func (r Run) ReportsDir() string { return filepath.Join(r.Path, reportsDir) }

// ReportFiles is the files kept for the report name, by path.
func (r Run) ReportFiles(name string) []string {
	var out []string
	root := filepath.Join(r.ReportsDir(), name)
	filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			out = append(out, p)
		}
		return nil
	})
	sort.Strings(out)
	return out
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
	// stoppedFile says why ws stopped a task before it ended by itself.
	stoppedFile = "stopped"
	// beganFile is made as a task's command starts: a report older than
	// it is another run's, and isn't kept.
	beganFile = "began"
	// reportsDir holds a task's reports, and pastDir, in the store's
	// folder, the earlier runs of each task.
	reportsDir = "reports"
	pastDir    = ".past"
)

// Store is a workspace's runs.
type Store struct {
	Root string
	// Procs reads the user's processes: nil is the system's.
	Procs func() (Procs, error)
}

// Dir is where runs are kept.
func (s Store) Dir() string { return filepath.Join(s.Root, ".ws", "runs") }

// pastOf is where the earlier runs of the task kept in dir are, each in a
// folder of its number.
func (s Store) pastOf(dir string) string {
	rel, err := filepath.Rel(s.Dir(), dir)
	if err != nil {
		rel = filepath.Base(dir)
	}
	return filepath.Join(s.Dir(), pastDir, rel)
}

// Past is the earlier runs of a task, the latest first.
func (s Store) Past(r Run) []Run {
	entries, err := os.ReadDir(s.pastOf(r.Path))
	if err != nil {
		return nil
	}
	var out []Run
	for _, e := range entries {
		if p, err := load(filepath.Join(s.pastOf(r.Path), e.Name()), Procs{}); err == nil {
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].N > out[j].N })
	return out
}

// archive puts the ended task kept in dir behind the next run of it, as
// its run number n, and drops the oldest past KeepPast. The folder's then
// gone: whoever moved it has the name to claim.
func (s Store) archive(dir string, n int) error {
	past := s.pastOf(dir)
	if err := os.MkdirAll(past, 0o755); err != nil {
		return err
	}
	to := filepath.Join(past, strconv.Itoa(n))
	os.RemoveAll(to)
	if err := os.Rename(dir, to); err != nil {
		return err
	}
	entries, _ := os.ReadDir(past)
	var ns []int
	for _, e := range entries {
		if k, err := strconv.Atoi(e.Name()); err == nil {
			ns = append(ns, k)
		}
	}
	sort.Ints(ns)
	for len(ns) > KeepPast {
		os.RemoveAll(filepath.Join(past, strconv.Itoa(ns[0])))
		ns = ns[1:]
	}
	return nil
}

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
		if r.IsTask() {
			if past, err := os.ReadDir(s.pastOf(r.Path)); err == nil {
				out[i].Past = len(past)
			}
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
	if r.State != Running {
		if b, err := os.ReadFile(filepath.Join(dir, stoppedFile)); err == nil {
			r.Why = strings.TrimSpace(string(b))
			// It went when ws stopped it, which is when that was written:
			// nearer than its last words, which may be its first.
			if fi, err := os.Stat(filepath.Join(dir, stoppedFile)); err == nil && r.State == Gone {
				r.Ended = fi.ModTime()
			}
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
	if at, n, ok := strings.Cut(ref, "#"); ok {
		// One run of a task, by its number: the latest, or one behind it.
		k, err := strconv.Atoi(n)
		if err != nil || k < 1 {
			return Run{}, fmt.Errorf("%s: after # is which run of a task, a number from 1", ref)
		}
		r, err := s.Find(at, repo, worktree)
		if err != nil {
			return Run{}, err
		}
		if r.N == k {
			return r, nil
		}
		for _, p := range s.Past(r) {
			if p.N == k {
				return p, nil
			}
		}
		return Run{}, fmt.Errorf("no run %d of %s is kept (its latest is %d, with %d behind it)", k, r.Ref(), r.N, r.Past)
	}
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

// Looking is the pages someone has open that runs owned by looking are
// started from: missions' pages, by slug, and grounds and capsules on the
// Files page, by repo/worktree (.ground for a ground).
type Looking struct {
	Missions  map[string]bool
	Worktrees map[string]bool
}

// at is whether o, one of r's owners, is someone looking at a page open
// in l.
func (l Looking) at(r Run, o Owner) bool {
	if o.Kind != ByLooking {
		return false
	}
	if o.Mission != "" {
		return l.Missions[o.Mission]
	}
	// A run of the workspace's own is in no worktree: its app's page keeps
	// it (LookedRun).
	return r.Repo != "" && l.Worktrees[r.Repo+"/"+r.Worktree]
}

// Looked marks the runs owned by looking at the pages open in l as looked
// at now.
func (s Store) Looked(l Looking, now time.Time) error {
	runs, err := s.List()
	if err != nil {
		return err
	}
	var errs []error
	for _, r := range runs {
		looking := slices.ContainsFunc(r.Owners(), func(o Owner) bool { return l.at(r, o) })
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
//
// With task said, a task of the name that has ended isn't lost to the new
// one: it's put behind it (archive), and n is the new one's number.
func (s Store) claim(repo, worktree, want, recipe string, task bool) (name, dir string, n int, running *Run, err error) {
	parent := s.place(repo, worktree)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return "", "", 0, nil, err
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
			return name, dir, s.nextN(dir, task), nil, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return "", "", 0, nil, err
		}
		old, lerr := load(dir, procs)
		if lerr != nil {
			if _, serr := os.Stat(filepath.Join(dir, recordFile)); errors.Is(serr, os.ErrNotExist) {
				// A folder with no record long after it was made is a start
				// that was killed before it recorded anything: the name's free.
				if fi, ferr := os.Stat(dir); ferr == nil && time.Since(fi.ModTime()) > 2*claimWait {
					if os.RemoveAll(dir) == nil && os.Mkdir(dir, 0o755) == nil {
						return name, dir, s.nextN(dir, task), nil, nil
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
			return "", "", 0, &old, nil
		}
		if lerr == nil && old.State != Running {
			if task && old.IsTask() {
				if s.archive(dir, max(old.N, 1)) == nil && os.Mkdir(dir, 0o755) == nil {
					return name, dir, s.nextN(dir, task), nil, nil
				}
				continue
			}
			if os.RemoveAll(dir) == nil && os.Mkdir(dir, 0o755) == nil {
				os.RemoveAll(s.pastOf(dir)) // a service has the name now
				return name, dir, s.nextN(dir, task), nil, nil
			}
		}
	}
	if repo == "" {
		return "", "", 0, nil, fmt.Errorf("no free name for %s in the workspace", want)
	}
	return "", "", 0, nil, fmt.Errorf("no free name for %s in %s/%s", want, repo, worktree)
}

// nextN is the number of the task being started in dir: one more than the
// latest behind it. A service has none.
func (s Store) nextN(dir string, task bool) int {
	if !task {
		return 0
	}
	n := 0
	entries, _ := os.ReadDir(s.pastOf(dir))
	for _, e := range entries {
		if k, err := strconv.Atoi(e.Name()); err == nil && k > n {
			n = k
		}
	}
	return n + 1
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
