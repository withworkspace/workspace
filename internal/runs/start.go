package runs

import (
	"errors"
	"fmt"
	"hash/fnv"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
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
	Name     string
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
}

// Start starts a run: claims its name, picks its port, starts its command
// in a session of its own with its output to its log, and records it.
func (s Store) Start(o Start, now time.Time) (Run, error) {
	if strings.TrimSpace(o.Command) == "" {
		return Run{}, errors.New("a run needs a command")
	}
	want := Slug(o.Name)
	if want == "" {
		want = NameFor(o.Command)
	}
	name, dir, err := s.claim(o.Repo, o.Worktree, want)
	if err != nil {
		return Run{}, err
	}
	keep := false
	defer func() {
		if !keep {
			os.RemoveAll(dir)
		}
	}()
	taken := map[int]bool{}
	if runs, err := s.List(); err == nil {
		for _, r := range runs {
			if r.State == Running {
				taken[r.Port] = true
			}
		}
	}
	port, err := pickPort(o.Repo+"/"+o.Worktree+"/"+name, taken)
	if err != nil {
		return Run{}, err
	}
	command := strings.ReplaceAll(o.Command, PortWord, strconv.Itoa(port))

	log, err := os.OpenFile(filepath.Join(dir, logFile), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return Run{}, err
	}
	defer log.Close()
	null, err := os.Open(os.DevNull)
	if err != nil {
		return Run{}, err
	}
	defer null.Close()
	// The command runs under a shell that writes its status when it ends,
	// so a run that ended by itself says how.
	cmd := exec.Command("/bin/sh", "-c", `/bin/sh -c "$1"; s=$?; echo "$s" > "$2.new" && mv -f "$2.new" "$2"`,
		"ws-run", command, filepath.Join(dir, exitFile))
	cmd.Dir = o.Dir
	env := o.Env
	if env == nil {
		env = os.Environ()
	}
	cmd.Env = append(withoutKey(env, "PORT"), "PORT="+strconv.Itoa(port))
	cmd.Stdin, cmd.Stdout, cmd.Stderr = null, log, log
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return Run{}, fmt.Errorf("starting %s: %w", name, err)
	}
	pid := cmd.Process.Pid
	began := now
	if at, ok := s.snapshot().started(pid); ok {
		began = at
	}
	rec := Record{
		Version: Version, Name: name, Repo: o.Repo, Worktree: o.Worktree, Dir: o.Dir,
		Command: command, Recipe: o.Recipe, Ready: o.Ready, Stop: o.Stop,
		Port: port, PID: pid, Began: began, Owner: o.Owner,
		Created: now.UTC().Truncate(time.Second), By: o.By,
	}
	if o.TTL > 0 {
		rec.Expires = now.Add(o.TTL).UTC().Truncate(time.Second)
	}
	if err := save(rec, dir); err != nil {
		syscall.Kill(-pid, syscall.SIGKILL)
		cmd.Wait()
		return Run{}, err
	}
	keep = true
	// It's nobody's child to wait for: the sweep sees it end.
	go cmd.Wait()
	return Run{Record: rec, Path: dir, State: Running}, nil
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
	for i := 0; i < PortRange; i++ {
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
	client := http.Client{Timeout: time.Second}
	resp, err := client.Get(r.Link() + path)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode < 500
}

// Listening is whether something answers on the run's port.
func (r Run) Listening() bool {
	for _, host := range []string{"127.0.0.1", "::1"} {
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

// EndedError is a run that ended before it listened.
type EndedError struct{ Run Run }

func (e *EndedError) Error() string {
	if e.Run.State == Exited {
		return fmt.Sprintf("%s exited (%d) before it was up on %d", e.Run.Ref(), e.Run.Status, e.Run.Port)
	}
	return fmt.Sprintf("%s ended before it was up on %d", e.Run.Ref(), e.Run.Port)
}

// Tail is the last n lines of the run's log.
func (r Run) Tail(n int) string {
	b, err := os.ReadFile(r.LogPath())
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
