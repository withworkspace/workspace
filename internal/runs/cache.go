package runs

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
)

// The cache is where a cached task's passes are kept, each in a folder of
// its key, .ws/cache/runs/<key>/: what it printed, its reports and its
// outputs, as they were when it passed. It's the workspace's, so a pass
// in one capsule is reused in another with the same inputs.
//
// A pass is kept by the run's own wrapper, as its command ends and before
// it says it has: its outputs as the command left them, with nothing else
// in between. The wrapper calls back into ws for it (Start.Keep), which
// runs KeepPass.

// CacheMax is how much the cache keeps before the passes least lately
// reused (or kept) are dropped.
var CacheMax int64 = 5 << 30

const (
	entryFile    = "entry.json"
	outputsDir   = "outputs"
	keepFile     = "keep"     // the script the wrapper runs to keep a pass
	keepLogFile  = "keep.log" // what keeping it said, where it couldn't
	keptFile     = "kept"     // the key it was kept under, once it was
	afterFile    = "after"    // the script the wrapper runs to run its After first
	afterRanFile = "afterran" // the runs of its After it ran after, one to a line
	// undeclaredFile is the files a cached task changed as it ran that its
	// outputs don't name, one to a line: a reuse doesn't put them back.
	undeclaredFile = "undeclared"
)

// Reused is the pass a run is, where it wasn't run.
type Reused struct {
	// From is the run that passed, by its number (test.ground.api#12),
	// Ran when it started and Took how long it took.
	From string        `json:"from"`
	Ran  time.Time     `json:"ran"`
	Took time.Duration `json:"took"`
}

// Entry is a pass in the cache.
type Entry struct {
	Key string `json:"key"`
	// From is the run that passed, Ran when it started and Took how long
	// it took.
	From string        `json:"from"`
	Ran  time.Time     `json:"ran"`
	Took time.Duration `json:"took"`
	// Recipe is its recipe's name, Repo and Worktree where it ran, and
	// Command what it ran.
	Recipe   string `json:"recipe"`
	Repo     string `json:"repo,omitempty"`
	Worktree string `json:"worktree,omitempty"`
	Command  string `json:"command"`
	// Outputs are the patterns its outputs were kept by, and Files and
	// Bytes what they came to.
	Outputs []string `json:"outputs,omitempty"`
	Files   int      `json:"files"`
	Bytes   int64    `json:"bytes"`

	// Path is its folder, and Used when it was kept or last reused.
	Path string    `json:"-"`
	Used time.Time `json:"-"`
}

// StoreOf is the store a run's folder is in: the workspace its .ws is.
func StoreOf(dir string) (Store, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return Store{}, err
	}
	for at := abs; at != filepath.Dir(at); at = filepath.Dir(at) {
		if filepath.Base(at) == ".ws" {
			return Store{Root: filepath.Dir(at)}, nil
		}
	}
	return Store{}, fmt.Errorf("%s is not a run's folder", dir)
}

// CacheDir is where passes are kept.
func (s Store) CacheDir() string { return filepath.Join(s.Root, ".ws", "cache", "runs") }

// Lookup is the pass kept under key.
func (s Store) Lookup(key string) (Entry, bool) {
	if key == "" {
		return Entry{}, false
	}
	e, err := loadEntry(filepath.Join(s.CacheDir(), key))
	return e, err == nil
}

func loadEntry(dir string) (Entry, error) {
	b, err := os.ReadFile(filepath.Join(dir, entryFile))
	if err != nil {
		return Entry{}, err
	}
	var e Entry
	if err := json.Unmarshal(b, &e); err != nil {
		return Entry{}, fmt.Errorf("%s: %w", dir, err)
	}
	e.Path = dir
	if fi, err := os.Stat(filepath.Join(dir, entryFile)); err == nil {
		e.Used = fi.ModTime()
	}
	return e, nil
}

// Cached is every pass in the cache, the latest used first.
func (s Store) Cached() []Entry {
	dirs, _ := os.ReadDir(s.CacheDir())
	var out []Entry
	for _, d := range dirs {
		if !d.IsDir() || strings.HasPrefix(d.Name(), ".") {
			continue
		}
		if e, err := loadEntry(filepath.Join(s.CacheDir(), d.Name())); err == nil {
			out = append(out, e)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Used.After(out[j].Used) })
	return out
}

// ClearCache drops every pass: each task is run again the next time.
func (s Store) ClearCache() error { return os.RemoveAll(s.CacheDir()) }

// keepScript is what the wrapper runs to keep a pass: cmd, with the run's
// folder after it.
func keepScript(cmd []string) string {
	quoted := make([]string, len(cmd))
	for i, a := range cmd {
		quoted[i] = "'" + strings.ReplaceAll(a, "'", `'\''`) + "'"
	}
	return "exec " + strings.Join(quoted, " ") + ` "$1"` + "\n"
}

// KeepPass keeps the task in dir, whose command has just passed, in the
// cache under its key: what it printed, its reports, and its outputs as
// they are now. It also notes the files it changed that its outputs don't
// name. The run's wrapper calls it, before it says the command has ended.
func (s Store) KeepPass(dir string, now time.Time) error {
	r, err := load(dir, Procs{})
	if err != nil {
		return err
	}
	if r.Key == "" {
		return nil
	}
	began := r.Began
	if fi, err := os.Stat(filepath.Join(dir, beganFile)); err == nil {
		began = fi.ModTime()
	}
	if err := os.MkdirAll(s.CacheDir(), 0o755); err != nil {
		return err
	}
	tmp, err := os.MkdirTemp(s.CacheDir(), ".new-"+r.Key+"-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	files, err := Files(r.Dir, r.Outputs)
	if err != nil {
		return fmt.Errorf("reading its outputs: %w", err)
	}
	var size int64
	for _, f := range files {
		n, err := copyPath(filepath.Join(r.Dir, filepath.FromSlash(f)), filepath.Join(tmp, outputsDir, filepath.FromSlash(f)))
		if err != nil {
			return fmt.Errorf("keeping %s: %w", f, err)
		}
		size += n
	}
	if _, err := copyPath(filepath.Join(dir, logFile), filepath.Join(tmp, logFile)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err := copyTree(filepath.Join(dir, reportsDir), filepath.Join(tmp, reportsDir)); err != nil {
		return err
	}
	e := Entry{
		Key: r.Key, From: r.NRef(), Ran: began, Took: now.Sub(began).Round(time.Millisecond),
		Recipe: r.Recipe, Repo: r.Repo, Worktree: r.Worktree, Command: r.Command,
		Outputs: r.Outputs, Files: len(files), Bytes: size,
	}
	b, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(tmp, entryFile), append(b, '\n'), 0o644); err != nil {
		return err
	}
	if err := putInPlace(tmp, filepath.Join(s.CacheDir(), r.Key)); err != nil {
		return err
	}
	theirs := slices.Clone(r.Outputs)
	for _, a := range r.After {
		theirs = append(theirs, a.Outputs...)
	}
	if undeclared := changedOutside(r.Dir, began, theirs); len(undeclared) > 0 {
		_ = os.WriteFile(filepath.Join(dir, undeclaredFile), []byte(strings.Join(undeclared, "\n")+"\n"), 0o644) // a note: the pass is kept all the same
	}
	if err := os.WriteFile(filepath.Join(dir, keptFile), []byte(r.Key+"\n"), 0o644); err != nil {
		return err
	}
	s.evict(r.Key)
	return nil
}

// putInPlace moves the pass kept in tmp to to, in place of one there: a
// run with the same inputs passed too, maybe this moment, and this is the
// latest.
func putInPlace(tmp, to string) error {
	var err error
	for i := range 5 {
		if err = os.Rename(tmp, to); err == nil {
			return nil
		}
		old := fmt.Sprintf("%s.old-%d", tmp, i)
		if os.Rename(to, old) == nil {
			defer os.RemoveAll(old)
		}
	}
	return err
}

// undeclaredMax is how many files outside its outputs a run notes.
const undeclaredMax = 20

// changedOutside is the files below dir (slashed from it) changed since
// began that outs don't name, as git finds new, changed and ignored files,
// past what's skipped. Nothing, where dir isn't in git.
func changedOutside(dir string, began time.Time, outs []string) []string {
	said, err := gitIn(dir, "rev-parse", "--show-prefix")
	if err != nil {
		return nil
	}
	prefix := strings.TrimSpace(string(said))
	status, err := gitIn(dir, append([]string{"status", "-z", "--porcelain=v1", "--untracked-files=all", "--ignored=traditional", "--no-renames", "--"}, pathspecs([]string{"**"})...)...)
	if err != nil {
		return nil
	}
	var out []string
	for _, entry := range strings.Split(string(status), "\x00") {
		if len(entry) < 4 || !strings.HasPrefix(entry[3:], prefix) {
			continue
		}
		name := strings.TrimPrefix(entry[3:], prefix)
		if MatchesAny(outs, name) {
			continue
		}
		fi, err := os.Lstat(filepath.Join(dir, filepath.FromSlash(name)))
		if err != nil || fi.ModTime().Before(began) {
			continue
		}
		out = append(out, name)
		if len(out) == undeclaredMax {
			break
		}
	}
	return out
}

// evict drops the passes least lately used while the cache holds more
// than CacheMax, all but keep (the one just kept).
func (s Store) evict(keep string) {
	all := s.Cached()
	var total int64
	for _, e := range all {
		total += e.Bytes
	}
	for i := len(all) - 1; i >= 0 && total > CacheMax; i-- {
		if all[i].Key == keep {
			continue
		}
		if os.RemoveAll(all[i].Path) == nil {
			total -= all[i].Bytes
		}
	}
}

// Reuse is a run of o made from the pass e, rather than run: its outputs
// are put back where it runs (what o's outputs name there now is removed
// first), and it's kept as a run that passed, with what the pass printed
// and its reports, saying which run it was. A run of o's recipe that's
// running already is returned instead, as Ensure would (started false).
func (s Store) Reuse(o Start, e Entry, now time.Time) (r Run, started bool, err error) {
	want := Slug(o.Name)
	if want == "" {
		want = NameFor(o.Command)
	}
	name, dir, n, running, err := s.claim(o.Repo, o.Worktree, want, o.Recipe, true)
	if err != nil {
		return Run{}, false, err
	}
	if running != nil {
		r, err := s.Join(*running, o.Owner, o.TTL, now)
		return r, false, err
	}
	keep := false
	defer func() {
		if !keep {
			os.RemoveAll(dir)
		}
	}()
	if mine := uncommitted(o.Dir, o.Outputs); len(mine) > 0 {
		more := ""
		if len(mine) > 3 {
			mine, more = mine[:3], fmt.Sprintf(" and %d more", len(mine)-3)
		}
		return Run{}, false, fmt.Errorf("its outputs name %s%s, which have changes not committed: putting the pass back would lose them", strings.Join(mine, ", "), more)
	}
	if err := restore(o.Dir, o.Outputs, filepath.Join(e.Path, outputsDir)); err != nil {
		return Run{}, false, fmt.Errorf("putting back %s's outputs: %w", e.From, err)
	}
	if _, err := copyPath(filepath.Join(e.Path, logFile), filepath.Join(dir, logFile)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return Run{}, false, err
	}
	if err := copyTree(filepath.Join(e.Path, reportsDir), filepath.Join(dir, reportsDir)); err != nil {
		return Run{}, false, err
	}
	if err := os.WriteFile(filepath.Join(dir, beganFile), nil, 0o644); err != nil {
		return Run{}, false, err
	}
	at := now.UTC().Truncate(time.Second)
	rec := Record{
		Version: Version, Type: Task, Name: name, N: n, Repo: o.Repo, Worktree: o.Worktree, Dir: o.Dir,
		Command: o.Command, Recipe: o.Recipe, Stop: o.Stop, Began: now, Owner: o.Owner,
		Created: at, By: o.By, Commit: o.Commit, Dirty: o.Dirty, Reports: o.Reports,
		Check: o.Check, Chain: o.Chain, Key: e.Key, Outputs: o.Outputs, After: o.After,
		Reused: &Reused{From: e.From, Ran: e.Ran, Took: e.Took},
	}
	if err := save(rec, dir); err != nil {
		return Run{}, false, err
	}
	if err := os.WriteFile(filepath.Join(dir, exitFile), []byte("0\n"), 0o644); err != nil {
		return Run{}, false, err
	}
	keep = true
	t := time.Now()
	_ = os.Chtimes(filepath.Join(e.Path, entryFile), t, t) // used: the last to be dropped
	r, err = load(dir, Procs{})
	return r, true, err
}

// uncommitted is the files below dir (slashed from it) that outs name and
// git has changes to that aren't committed: tracked files changed, and new
// files that aren't ignored. A reuse removes what outs name, so these are
// someone's work it would lose. What's ignored (dist) is what outputs are.
func uncommitted(dir string, outs []string) []string {
	if len(outs) == 0 {
		return nil
	}
	said, err := gitIn(dir, "rev-parse", "--show-prefix")
	if err != nil {
		return nil
	}
	prefix := strings.TrimSpace(string(said))
	status, err := gitIn(dir, append([]string{"status", "-z", "--porcelain=v1", "--untracked-files=all", "--no-renames", "--"}, pathspecs(outs)...)...)
	if err != nil {
		return nil
	}
	var out []string
	for _, entry := range strings.Split(string(status), "\x00") {
		if len(entry) < 4 || !strings.HasPrefix(entry[3:], prefix) {
			continue
		}
		out = append(out, strings.TrimPrefix(entry[3:], prefix))
	}
	return out
}

// restore puts the outputs kept in from back in dir, once what outs name
// there now is gone.
func restore(dir string, outs []string, from string) error {
	now, err := Files(dir, outs)
	if err != nil {
		return err
	}
	for _, f := range now {
		if err := os.Remove(filepath.Join(dir, filepath.FromSlash(f))); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	return copyTree(from, dir)
}

// copyTree copies every file below from to the same place below to.
// Nothing at from is nothing to copy.
func copyTree(from, to string) error {
	err := filepath.WalkDir(from, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(from, p)
		if err != nil {
			return err
		}
		_, err = copyPath(p, filepath.Join(to, rel))
		return err
	})
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}

// copyPath copies the file, or link, at from to to, its folder made, and
// says how many bytes it was.
func copyPath(from, to string) (int64, error) {
	fi, err := os.Lstat(from)
	if err != nil {
		return 0, err
	}
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		return 0, err
	}
	if err := os.Remove(to); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return 0, err
	}
	if fi.Mode()&fs.ModeSymlink != 0 {
		target, err := os.Readlink(from)
		if err != nil {
			return 0, err
		}
		return 0, os.Symlink(target, to)
	}
	in, err := os.Open(from)
	if err != nil {
		return 0, err
	}
	defer in.Close()
	out, err := os.OpenFile(to, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, fi.Mode().Perm())
	if err != nil {
		return 0, err
	}
	n, err := io.Copy(out, in)
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	return n, err
}

// At is the run kept in dir.
func At(dir string) (Run, error) { return load(dir, Store{}.snapshot()) }

// AfterRan notes the runs of its After the task in dir ran after, by their
// numbers.
func AfterRan(dir string, refs []string) error {
	return os.WriteFile(filepath.Join(dir, afterRanFile), []byte(strings.Join(refs, "\n")+"\n"), 0o644)
}

func afterRanOf(dir string) []string {
	b, err := os.ReadFile(filepath.Join(dir, afterRanFile))
	if err != nil {
		return nil
	}
	var out []string
	for _, l := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		if l != "" {
			out = append(out, l)
		}
	}
	return out
}

// keptOf is what keeping the run in dir came to: the key it's kept under,
// or why it couldn't be, and the files it changed outside its outputs.
func keptOf(dir string) (kept, failed string, undeclared []string) {
	if b, err := os.ReadFile(filepath.Join(dir, keptFile)); err == nil {
		kept = strings.TrimSpace(string(b))
	} else if b, err := os.ReadFile(filepath.Join(dir, keepLogFile)); err == nil {
		failed = strings.TrimSpace(string(b))
	}
	if b, err := os.ReadFile(filepath.Join(dir, undeclaredFile)); err == nil {
		for _, l := range strings.Split(strings.TrimSpace(string(b)), "\n") {
			if l != "" {
				undeclared = append(undeclared, l)
			}
		}
	}
	return kept, failed, undeclared
}

// SizeSaid is n bytes in words: 3.2 MB.
func SizeSaid(n int64) string {
	switch {
	case n < 1<<10:
		return strconv.FormatInt(n, 10) + " B"
	case n < 1<<20:
		return fmt.Sprintf("%.1f KB", float64(n)/(1<<10))
	case n < 1<<30:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	}
	return fmt.Sprintf("%.1f GB", float64(n)/(1<<30))
}
