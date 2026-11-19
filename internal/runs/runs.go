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
	// ByNone ends only when its time is up, or when it's stopped.
	ByNone Kind = "none"
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
	PID     int       `json:"pid"`
	Began   time.Time `json:"began"`
	Owner   Owner     `json:"owner"`
	Created time.Time `json:"created"`
	// Expires is when its time is up: zero for never.
	Expires time.Time `json:"expires,omitzero"`
	// By is who started it: "you", or crew's id.
	By string `json:"by,omitempty"`
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
// local address will have it.
func (r Run) Ref() string { return r.Name + "." + Label(r.Worktree) + "." + r.Repo }

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
func (s Store) claim(repo, worktree, want string) (name, dir string, err error) {
	parent := filepath.Join(s.Dir(), repo, worktree)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return "", "", err
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
			return name, dir, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return "", "", err
		}
		old, lerr := load(dir, procs)
		if lerr != nil {
			if _, serr := os.Stat(filepath.Join(dir, recordFile)); errors.Is(serr, os.ErrNotExist) {
				continue // another ws claiming it this moment, or not ours
			}
		}
		if lerr == nil && old.State != Running {
			if os.RemoveAll(dir) == nil && os.Mkdir(dir, 0o755) == nil {
				return name, dir, nil
			}
		}
	}
	return "", "", fmt.Errorf("no free name for %s in %s/%s", want, repo, worktree)
}
