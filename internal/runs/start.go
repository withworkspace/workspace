package runs

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Ports runs are given come from PortBase to PortBase+PortRange-1, next to
// the web UI's (47000 up).
const (
	PortBase  = 48000
	PortRange = 1000
)

// PortWord is replaced with the run's port in its command.
const PortWord = "{port}"

// Start is what a run is started with.
type Start struct {
	// Name is what it's called; "" is NameFor(Command). It's taken as
	// the next free one if a running run has it already.
	Name string
	// Type is Task for one that ends by itself: it's given no port, and
	// kept once it has ended. "" is a service.
	Type     Type
	Repo     string
	Worktree string
	Dir      string
	Command  string
	// Recipe, Ready and Stop are the recipe it's started from, if it is.
	Recipe, Ready, Stop string
	Owner               Owner
	// TTL is how long it may last: 0 for no limit.
	TTL time.Duration
	By  string
	// Env is the environment it runs in, PORT added: nil is this
	// process's.
	Env []string
	// Needs are what it's started needing, each met already (Record.Needs).
	Needs []Need
	// Hold is until when it isn't ended for nothing needing it, for one
	// owned by what needs it (Record.Hold).
	Hold time.Time
	// Commit and Dirty are where its worktree stands, Reports the files a
	// task's command writes that are kept with it, and Check the check it's
	// run for (Record's).
	Commit  string
	Dirty   bool
	Reports []Report
	Check   *CheckOf
}

// Start starts a run: claims its name, picks its port, starts its command
// in a session of its own with its output to its log, and records it.
func (s Store) Start(o Start, now time.Time) (Run, error) {
	r, _, err := s.start(o, now, false)
	return r, err
}

// Ensure is the run of o's recipe in its worktree: the one running there
// already, which is then o's owner's as well (Join), or one started now
// (started). A recipe runs once in a worktree, whoever asks for it; a
// command that's no recipe's is started as Start starts it.
func (s Store) Ensure(o Start, now time.Time) (r Run, started bool, err error) {
	if o.Recipe == "" {
		r, err = s.Start(o, now)
		return r, err == nil, err
	}
	if list, err := s.List(); err == nil {
		for _, old := range list {
			if old.State == Running && old.Repo == o.Repo && old.Worktree == o.Worktree && old.Recipe == o.Recipe {
				r, err = s.Join(old, o.Owner, o.TTL, now)
				return r, false, err
			}
		}
	}
	return s.start(o, now, true)
}

// Join makes o one more of who a running run is for, and its time the
// later of what it had and ttl from now (0: no limit). It's then ended
// for its owners' going only once all of them have. An owner it has
// already isn't added again; nor is ByRun, which is whatever needs it.
// A task's time is its timeout, from when it started: asking for it
// again gives it no more.
func (s Store) Join(r Run, o Owner, ttl time.Duration, now time.Time) (Run, error) {
	if o.Kind == "" || o.Kind == ByRun {
		return r, nil
	}
	r = s.again(r)
	changed := false
	if !slices.ContainsFunc(r.Owners(), func(has Owner) bool { return sameOwner(has, o) }) {
		r.Also = append(r.Also, o)
		changed = true
	}
	if !r.Expires.IsZero() && !r.IsTask() {
		switch until := now.Add(ttl).UTC().Truncate(time.Second); {
		case ttl <= 0:
			r.Expires, changed = time.Time{}, true
		case until.After(r.Expires):
			r.Expires, changed = until, true
		}
	}
	if !changed {
		return r, nil
	}
	return r, save(r.Record, r.Path)
}

// sameOwner is whether a and b are one owner: the same process, capsule,
// mission, step, crew or conversation.
func sameOwner(a, b Owner) bool {
	if a.Kind != b.Kind {
		return false
	}
	switch a.Kind {
	case ByPID:
		return a.PID == b.PID && sameStart(a.Began, b.Began)
	case ByStep:
		return a.Mission == b.Mission && a.StepText == b.StepText
	case ByCrew:
		return a.Mission == b.Mission && a.Crew == b.Crew
	case ByConversation:
		return a.Mission == b.Mission && a.Conversation == b.Conversation
	}
	return a.Mission == b.Mission && a.App == b.App
}

// HoldUntil keeps a running run from being ended for nothing needing it
// until then (the zero time: no longer), while the run it's for is being
// started.
func (s Store) HoldUntil(r Run, until time.Time) (Run, error) {
	if !until.IsZero() {
		until = until.UTC().Truncate(time.Second)
	}
	r = s.again(r)
	if r.Hold.Equal(until) {
		return r, nil
	}
	r.Hold = until
	return r, save(r.Record, r.Path)
}

// again is r as its record says now: another ws may have written it
// since r was read. What List found of it (who needs it) is kept.
func (s Store) again(r Run) Run {
	now, err := load(r.Path, s.snapshot())
	if err != nil {
		return r
	}
	now.NeededBy = r.NeededBy
	return now
}

// start starts o. With single, o is a recipe's, and where a run of the
// recipe turns out to be running in its worktree (another ws started it
// this moment), that one is returned, joined, and nothing's started.
func (s Store) start(o Start, now time.Time, single bool) (Run, bool, error) {
	if strings.TrimSpace(o.Command) == "" {
		return Run{}, false, errors.New("a run needs a command")
	}
	want := Slug(o.Name)
	if want == "" {
		want = NameFor(o.Command)
	}
	recipe := ""
	if single {
		recipe = o.Recipe
	}
	task := o.Type == Task
	for _, rp := range o.Reports {
		if err := rp.valid(); err != nil {
			return Run{}, false, err
		}
	}
	name, dir, n, running, err := s.claim(o.Repo, o.Worktree, want, recipe, task)
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
	// A task listens on nothing: it has no port.
	port, command := 0, o.Command
	if !task {
		taken := map[int]bool{}
		if runs, err := s.List(); err == nil {
			for _, r := range runs {
				if r.State == Running {
					taken[r.Port] = true
				}
			}
		}
		if port, err = pickPort(o.Repo+"/"+o.Worktree+"/"+name, taken); err != nil {
			return Run{}, false, err
		}
		command = strings.ReplaceAll(o.Command, PortWord, strconv.Itoa(port))
	}

	log, err := os.OpenFile(filepath.Join(dir, logFile), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return Run{}, false, err
	}
	defer log.Close()
	null, err := os.Open(os.DevNull)
	if err != nil {
		return Run{}, false, err
	}
	defer null.Close()
	// The command runs under a shell that writes its status when it ends,
	// so a run that ended by itself says how. A task's reports are kept
	// first, by the same shell: nothing of ws's is waiting for it to end.
	args := []string{"-c", wrapper, "ws-run", command, dir}
	for _, rp := range o.Reports {
		args = append(args, rp.Name, rp.Path)
	}
	cmd := exec.Command("/bin/sh", args...)
	cmd.Dir = o.Dir
	env := o.Env
	if env == nil {
		env = os.Environ()
	}
	cmd.Env = withoutKey(env, "PORT")
	if !task {
		cmd.Env = append(cmd.Env, "PORT="+strconv.Itoa(port))
	}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = null, log, log
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return Run{}, false, fmt.Errorf("starting %s: %w", name, err)
	}
	pid := cmd.Process.Pid
	began := now
	if at, ok := s.snapshot().started(pid); ok {
		began = at
	}
	rec := Record{
		Version: Version, Name: name, Repo: o.Repo, Worktree: o.Worktree, Dir: o.Dir,
		Command: command, Recipe: o.Recipe, Ready: o.Ready, Stop: o.Stop,
		Port: port, PID: pid, Began: began, Owner: o.Owner, Needs: o.Needs,
		Created: now.UTC().Truncate(time.Second), By: o.By,
		Commit: o.Commit, Dirty: o.Dirty, Check: o.Check,
	}
	if task {
		rec.Type, rec.N, rec.Reports = Task, n, o.Reports
	}
	if !o.Hold.IsZero() {
		rec.Hold = o.Hold.UTC().Truncate(time.Second)
	}
	if o.Owner.Kind == ByLooking {
		rec.Looked = rec.Created
	}
	if o.TTL > 0 {
		rec.Expires = now.Add(o.TTL).UTC().Truncate(time.Second)
	}
	if err := save(rec, dir); err != nil {
		syscall.Kill(-pid, syscall.SIGKILL)
		cmd.Wait()
		return Run{}, false, err
	}
	keep = true
	// It's nobody's child to wait for: the sweep sees it end.
	go cmd.Wait()
	return Run{Record: rec, Path: dir, State: Running}, true, nil
}

// wrapper is the shell a run's command runs under: $1 the command, $2 the
// run's folder, then a name and a path for each report it's to keep. A
// report is kept only if it isn't older than the command's start (it's
// this run's), under reports/<name>/. Its path is left unquoted, so a
// pattern is every file it matches.
const wrapper = `d="$2"; : > "$d/` + beganFile + `"
/bin/sh -c "$1"; s=$?
shift 2
while [ $# -ge 2 ]; do
	for f in $2; do
		if [ -e "$f" ] && ! [ "$d/` + beganFile + `" -nt "$f" ]; then
			mkdir -p "$d/` + reportsDir + `/$1" && cp -R "$f" "$d/` + reportsDir + `/$1/"
		fi
	done
	shift 2
done
echo "$s" > "$d/` + exitFile + `.new" && mv -f "$d/` + exitFile + `.new" "$d/` + exitFile + `"`

var reportName = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)

// valid is whether the report can be kept: a name that's a folder's, and
// a path below where the command runs, with nothing the shell would read
// as more than a pattern.
func (r Report) valid() error {
	if !reportName.MatchString(r.Name) {
		return fmt.Errorf("a report's name is lower case letters, numbers, - and _, not %q", r.Name)
	}
	p := strings.TrimSpace(r.Path)
	switch {
	case p == "":
		return fmt.Errorf("report %s has no path: the file the command writes", r.Name)
	case filepath.IsAbs(p) || slices.Contains(strings.Split(filepath.ToSlash(p), "/"), ".."):
		return fmt.Errorf("report %s: %s isn't below where the command runs", r.Name, p)
	case strings.ContainsAny(p, " \t\n;&|<>$`\"'(){}!~\\"):
		return fmt.Errorf("report %s: %s: a report's path is a file's, or a pattern of * and ?, with no spaces", r.Name, p)
	}
	return nil
}

func withoutKey(env []string, key string) []string {
	out := make([]string, 0, len(env))
	for _, kv := range env {
		if k, _, _ := strings.Cut(kv, "="); k != key {
			out = append(out, kv)
		}
	}
	return out
}

// pickPort is a free port for the run key names: the one its key hashes
// to if that's free, so a run started again gets the port it had, else
// the next free one after it. Free is not another run's, and nothing
// listening on it at all, on either loopback or every interface.
func pickPort(key string, taken map[int]bool) (int, error) {
	h := fnv.New32a()
	h.Write([]byte(key))
	first := int(h.Sum32() % PortRange)
	for i := range PortRange {
		p := PortBase + (first+i)%PortRange
		if !taken[p] && portFree(p) {
			return p, nil
		}
	}
	return 0, fmt.Errorf("no free port from %d to %d", PortBase, PortBase+PortRange-1)
}

// portFree is whether nothing listens on port. On macOS a socket on every
// interface can be bound beside one on 127.0.0.1, so each is tried.
func portFree(port int) bool {
	for _, host := range []string{"127.0.0.1", "", "::1"} {
		l, err := net.Listen("tcp", net.JoinHostPort(host, strconv.Itoa(port)))
		if err != nil {
			if host == "::1" && !errors.Is(err, syscall.EADDRINUSE) {
				continue // no IPv6 loopback here
			}
			return false
		}
		l.Close()
	}
	return true
}

// Up is whether the run is ready: its Ready path answers below 500, or
// with none, something listens on its port.
func (r Run) Up() bool {
	if r.Ready == "" {
		return r.Listening()
	}
	path := r.Ready
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	client := http.Client{Timeout: time.Second, Transport: Loopback}
	resp, err := client.Get(r.Link() + path)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode < 500
}

// loopbacks are where a run may listen. A dev server that binds localhost
// gets whichever the name gives first, which for Node can be ::1 alone.
var loopbacks = []string{"127.0.0.1", "::1"}

// Loopback is how a run is asked over HTTP: at its port on 127.0.0.1, and
// with nothing listening there, on ::1. It's what Listening counts as up,
// so a run that's up is one that answers.
var Loopback http.RoundTripper = func() *http.Transport {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.Proxy = nil
	t.DialContext = dialLoopback
	return t
}()

// dialLoopback dials addr, and where that's 127.0.0.1 with nothing
// listening, the same port on ::1. The first refusal is what's returned
// when neither answers.
func dialLoopback(ctx context.Context, network, addr string) (net.Conn, error) {
	d := net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}
	c, err := d.DialContext(ctx, network, addr)
	if err == nil || !errors.Is(err, syscall.ECONNREFUSED) {
		return c, err
	}
	host, port, splitErr := net.SplitHostPort(addr)
	if splitErr != nil || host != loopbacks[0] {
		return nil, err
	}
	for _, other := range loopbacks[1:] {
		if c, otherErr := d.DialContext(ctx, network, net.JoinHostPort(other, port)); otherErr == nil {
			return c, nil
		}
	}
	return nil, err
}

// Listening is whether something answers on the run's port.
func (r Run) Listening() bool {
	for _, host := range loopbacks {
		c, err := net.DialTimeout("tcp", net.JoinHostPort(host, strconv.Itoa(r.Port)), 200*time.Millisecond)
		if err == nil {
			c.Close()
			return true
		}
	}
	return false
}

// Wait waits up to d for the run to be up (Up). It's ready when it is;
// an error when it ends first; and neither (false, nil) when the time
// passes with it running and not up.
func (s Store) Wait(r Run, d time.Duration) (bool, error) {
	deadline := time.Now().Add(d)
	for {
		if r.Up() {
			return true, nil
		}
		now, err := load(r.Path, s.snapshot())
		if err != nil {
			return false, err
		}
		if now.State != Running {
			return false, &EndedError{Run: now}
		}
		if time.Now().After(deadline) {
			return false, nil
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// WaitEnd waits up to d (0: without limit) for the run to end, and
// returns it as it then stands: ended is false when the time passed with
// it still running. line, if it isn't nil, is called now and then with
// what it has printed since.
func (s Store) WaitEnd(r Run, d time.Duration, more func(string)) (now Run, ended bool, err error) {
	deadline := time.Now().Add(d)
	var read int64
	say := func() {
		if more == nil {
			return
		}
		f, err := os.Open(r.LogPath())
		if err != nil {
			return
		}
		defer f.Close()
		if _, err := f.Seek(read, io.SeekStart); err != nil {
			return
		}
		b, _ := io.ReadAll(f)
		read += int64(len(b))
		if len(b) > 0 {
			more(string(b))
		}
	}
	for {
		now, err = load(r.Path, s.snapshot())
		if err != nil {
			return r, false, err
		}
		say()
		if now.State != Running {
			say() // what it printed as it went
			return now, true, nil
		}
		if d > 0 && time.Now().After(deadline) {
			return now, false, nil
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// EndedError is a run that ended before it listened.
type EndedError struct{ Run Run }

func (e *EndedError) Error() string {
	if e.Run.State == Exited {
		return fmt.Sprintf("%s exited (%d) before it was up on %d", e.Run.Ref(), e.Run.Status, e.Run.Port)
	}
	return fmt.Sprintf("%s ended before it was up on %d", e.Run.Ref(), e.Run.Port)
}

// tailBytes is as much of a log's end as Tail reads: a dev server's log
// can run to megabytes.
const tailBytes = 256 << 10

// Tail is the last n lines of the run's log (of its last tailBytes).
func (r Run) Tail(n int) string {
	text, _ := r.TailSized(n)
	return strings.TrimSuffix(text, "\n")
}

// TailSized is Tail as the log has it (its last line's end kept, so what's
// written after follows on), with how much of the log it's as of: its
// size when it was read, from which what's written after is read
// (ReadFrom).
func (r Run) TailSized(n int) (string, int64) {
	f, err := os.Open(r.LogPath())
	if err != nil {
		return "", 0
	}
	defer f.Close()
	cut := false
	if st, err := f.Stat(); err == nil && st.Size() > tailBytes {
		if _, err := f.Seek(-tailBytes, io.SeekEnd); err != nil {
			return "", 0
		}
		cut = true
	}
	b, err := io.ReadAll(f)
	if err != nil {
		return "", 0
	}
	end, _ := f.Seek(0, io.SeekCurrent)
	if i := bytes.IndexByte(b, '\n'); cut && i >= 0 {
		b = b[i+1:] // the line it began in the middle of
	}
	ended := bytes.HasSuffix(b, []byte("\n"))
	lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	text := strings.Join(lines, "\n")
	if ended && text != "" {
		text += "\n" // as the log has it: what's written after follows on
	}
	return text, end
}

// ReadFrom is what's been written to the log at path from offset from on,
// and its size now: at most max bytes of it (more is false when there's
// more past them). A log shorter than from was begun again: gone is true.
func ReadFrom(path string, from, max int64) (text string, size int64, more, gone bool, err error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, false, false, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return "", 0, false, false, err
	}
	if st.Size() < from {
		return "", st.Size(), false, true, nil
	}
	if st.Size()-from > max {
		return "", st.Size(), true, false, nil
	}
	if _, err := f.Seek(from, io.SeekStart); err != nil {
		return "", 0, false, false, err
	}
	b, err := io.ReadAll(io.LimitReader(f, max))
	if err != nil {
		return "", 0, false, false, err
	}
	return string(b), from + int64(len(b)), false, false, nil
}
