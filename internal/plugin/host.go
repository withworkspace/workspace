package plugin

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/withworkspace/workspace/internal/acp"
)

// How long a plugin has to answer initialize, and to go once it's told to.
const (
	startTimeout = 15 * time.Second
	stopGrace    = 5 * time.Second
)

// maxSocketPath is under the smallest sun_path limit ws runs on.
const maxSocketPath = 100

// socketPath is a unix socket for the workspace at root: at want if a
// socket can be that long, else a short one of its own in the temp folder.
func socketPath(root, want, name string) string {
	if len(want) <= maxSocketPath {
		return want
	}
	sum := sha1.Sum([]byte(root + "\x00" + name))
	return filepath.Join(os.TempDir(), "ws-"+hex.EncodeToString(sum[:8])+".sock")
}

// State is where a plugin stands.
type State string

const (
	Ready   State = "ready"   // running, and can be asked
	Setup   State = "setup"   // something it requires isn't set
	Failed  State = "failed"  // it didn't start, or keeps stopping
	Stopped State = "stopped" // not running: started when it's next needed
)

// Status is a plugin as ws sees it: what it adds, and where it stands.
type Status struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
	State   State  `json:"state"`
	// Error is why it failed, or the last error it logged.
	Error string `json:"error,omitempty"`
	// Missing are the keys it requires that aren't set.
	Missing []string `json:"missing,omitempty"`
	// Linked is the folder it's a link to (ws plugin add --link).
	Linked string `json:"linked,omitempty"`
	// Source is the repo it was installed from, and which release of it.
	Source       *Source      `json:"source,omitempty"`
	Registration Registration `json:"registration"`
	// Badges are the counts beside its views' entries, by the view's path.
	Badges map[string]int `json:"badges,omitempty"`
	// Changed counts the times it's said what its view shows is stale.
	Changed uint64 `json:"changed"`
	// Starts counts the times it's been started: a page showing its view
	// loads it again when it moves, as what's there is from a run before.
	Starts uint64               `json:"starts"`
	Syncs  map[string]SyncState `json:"syncs,omitempty"`
}

// SyncState is how a plugin's sync last went.
type SyncState struct {
	At     time.Time `json:"at"`
	TookMS int       `json:"took_ms,omitempty"`
	Error  string    `json:"error,omitempty"`
}

// SetupError is a plugin that can't be asked yet: config it requires
// isn't set.
type SetupError struct {
	Plugin  string
	Missing []ConfigKey
}

func (e *SetupError) Error() string {
	keys := make([]string, len(e.Missing))
	for i, k := range e.Missing {
		keys[i] = k.Key
	}
	return fmt.Sprintf("%s needs setting up (%s): ws plugin config %s", e.Plugin, strings.Join(keys, ", "), e.Plugin)
}

// Host runs a workspace's plugins: one process each, started when it's
// first needed and kept until Shutdown.
type Host struct {
	Root string
	// Version is ws's own, told to a plugin when it starts.
	Version string
	Log     *log.Logger
	// Executable is ws's own program, which a plugin runs for ws's
	// commands: "" is this process's.
	Executable string
	// Reserved says whether a word after ws is ws's own: a plugin can't
	// take it for its commands. nil reserves none.
	Reserved func(word string) bool
	// Hold keeps the daemon up while a plugin's command runs; nil for
	// nothing to hold.
	Hold func() (release func())

	mu      sync.Mutex
	procs   map[string]*proc
	starts  map[string]*sync.Mutex // one start of a plugin at a time
	runs    map[string]uint64      // how many times each has been started
	closing bool

	seq      atomic.Uint64 // counts changes to what Statuses says
	commands atomic.Uint64
}

// proc is one plugin's process, running or not.
type proc struct {
	name     string
	manifest Manifest
	cmd      *exec.Cmd
	stdin    io.WriteCloser
	conn     *acp.Conn
	viewSock string
	exited   chan struct{}
	started  time.Time
	// stopping is set once ws has told it to go: its going isn't its own
	// idea.
	stopping atomic.Bool

	mu      sync.Mutex
	reg     Registration
	outputs map[string]func(stream, data string)
	badges  map[string]int
	changed uint64
	syncs   map[string]SyncState
	lastErr string

	// A plugin that didn't start: why, how many times running, and when
	// it's worth trying again.
	err      error
	failures int
	retryAt  time.Time
}

func (p *proc) alive() bool {
	if p == nil || p.conn == nil {
		return false
	}
	// Its connection closing is the first sign it's gone: its process is
	// reaped a moment after.
	select {
	case <-p.conn.Done():
		return false
	case <-p.exited:
		return false
	default:
		return true
	}
}

func (h *Host) logf(format string, args ...any) {
	if h.Log != nil {
		h.Log.Printf("plugin: "+format, args...)
	}
}

// Stamp changes whenever what Statuses says may have: a plugin started or
// stopped, its badge, its view going stale.
func (h *Host) Stamp() string { return strconv.FormatUint(h.seq.Load(), 10) }

func (h *Host) startLock(name string) *sync.Mutex {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.starts == nil {
		h.starts = map[string]*sync.Mutex{}
	}
	if h.starts[name] == nil {
		h.starts[name] = &sync.Mutex{}
	}
	return h.starts[name]
}

// ensure is the plugin's process, started if it isn't running.
func (h *Host) ensure(name string) (*proc, error) {
	if !ValidName(name) {
		return nil, fmt.Errorf("%q isn't a plugin's name", name)
	}
	lock := h.startLock(name)
	lock.Lock()
	defer lock.Unlock()

	h.mu.Lock()
	if h.closing {
		h.mu.Unlock()
		return nil, errors.New("the daemon is closing")
	}
	prev := h.procs[name]
	h.mu.Unlock()
	if prev.alive() {
		return prev, nil
	}
	failures := 0
	if prev != nil {
		failures = prev.failures
		if prev.err != nil && time.Now().Before(prev.retryAt) {
			return nil, prev.err
		}
	}
	p, err := h.start(name)
	if err != nil {
		failures++
		// Tried again when it's next needed, but not at once: a plugin that
		// can't start shouldn't be started on every keystroke.
		wait := min(time.Second<<min(failures, 6), time.Minute)
		p = &proc{name: name, err: err, failures: failures, retryAt: time.Now().Add(wait)}
		h.logf("%s didn't start: %v", name, err)
	}
	h.mu.Lock()
	if h.procs == nil {
		h.procs = map[string]*proc{}
	}
	h.procs[name] = p
	h.mu.Unlock()
	h.seq.Add(1)
	if err != nil {
		return nil, err
	}
	return p, nil
}

// start runs the plugin and has it say what it adds.
func (h *Host) start(name string) (*proc, error) {
	app, err := filepath.EvalSymlinks(appDir(h.Root, name))
	if err != nil {
		return nil, fmt.Errorf("there's no plugin called %s", name)
	}
	m, err := ReadManifest(app)
	if err != nil {
		return nil, err
	}
	if m.Name != name {
		return nil, fmt.Errorf("the plugin in %s calls itself %s, not %s", app, m.Name, name)
	}
	data := dataDir(h.Root, name)
	if err := os.MkdirAll(data, 0o755); err != nil {
		return nil, err
	}
	logFile, err := os.OpenFile(LogPath(h.Root, name), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	defer logFile.Close() // the plugin has its own handle on it once it's started

	cmd := exec.Command(m.Command[0], m.Command[1:]...)
	cmd.Dir = app
	cmd.Stderr = logFile
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting %s: %w", strings.Join(m.Command, " "), err)
	}
	p := &proc{
		name: name, manifest: m, cmd: cmd, stdin: stdin, exited: make(chan struct{}),
		viewSock: socketPath(h.Root, filepath.Join(data, "view.sock"), "view:"+name),
		outputs:  map[string]func(string, string){}, badges: map[string]int{}, syncs: map[string]SyncState{},
		started: time.Now(),
	}
	p.conn = acp.NewConn(stdout, stdin, func(ctx context.Context, method string, params json.RawMessage) (any, error) {
		return h.handle(p, method, params)
	})
	go func() {
		<-p.conn.Done()   // its stdout's been read to the end
		err := cmd.Wait() // how it went is in its log
		close(p.exited)
		h.seq.Add(1)
		h.logf("%s stopped", name)
		// A plugin that goes of its own accord, cleanly, is asking to be
		// started afresh: what it registers follows config that changed.
		// One that does so as soon as it's started isn't asked again until
		// it's next needed.
		if err == nil && !p.stopping.Load() && time.Since(p.started) > 2*time.Second {
			if _, err := h.ensure(name); err != nil {
				h.logf("%s asked to be started afresh, and wasn't: %v", name, err)
			}
		}
	}()

	values, err := Values(h.Root, name)
	if err != nil {
		h.kill(p)
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), startTimeout)
	defer cancel()
	var answer struct {
		Protocol     int          `json:"protocol"`
		Registration Registration `json:"registration"`
	}
	err = p.conn.Call(ctx, "initialize", map[string]any{
		"protocol":  ProtocolVersion,
		"ws":        map[string]string{"version": h.Version, "executable": h.executable()},
		"plugin":    map[string]string{"name": name, "version": m.Version, "dir": app, "data": data},
		"workspace": map[string]string{"root": h.Root},
		"config":    values,
		"view":      map[string]string{"listen": p.viewSock, "base": ViewBase(name)},
	}, &answer)
	if err != nil {
		h.kill(p)
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, fmt.Errorf("%s didn't answer in %s of starting: see %s", name, startTimeout, LogPath(h.Root, name))
		}
		if errors.Is(err, acp.ErrClosed) {
			return nil, fmt.Errorf("%s stopped as it started: see %s", name, LogPath(h.Root, name))
		}
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	if answer.Protocol != ProtocolVersion {
		h.kill(p)
		return nil, fmt.Errorf("%s speaks version %d of the plugin protocol, and this ws speaks %d", name, answer.Protocol, ProtocolVersion)
	}
	if err := h.acceptable(name, answer.Registration); err != nil {
		h.kill(p)
		return nil, err
	}
	p.reg = answer.Registration
	if err := writeRegistration(h.Root, name, p.reg); err != nil {
		h.logf("%s: keeping what it registered: %v", name, err)
	}
	h.mu.Lock()
	if h.runs == nil {
		h.runs = map[string]uint64{}
	}
	h.runs[name]++
	h.mu.Unlock()
	h.logf("%s %s started", name, m.Version)
	return p, nil
}

// executable is the program a plugin's told is ws.
func (h *Host) executable() string {
	if h.Executable != "" {
		return h.Executable
	}
	self, err := os.Executable()
	if err != nil {
		return ""
	}
	return self
}

// ViewBase is the path a plugin's view is served under.
func ViewBase(name string) string { return "/plugins/" + name }

// acceptable refuses a registration that takes a word after ws that's
// ws's own, or another plugin's.
func (h *Host) acceptable(name string, r Registration) error {
	word := r.CommandName(name)
	if word == "" {
		return nil
	}
	if !ValidName(word) {
		return fmt.Errorf("%s: its command %q must be lower-case letters, digits and hyphens", name, word)
	}
	if h.Reserved != nil && h.Reserved(word) {
		return fmt.Errorf("%s: ws %s is ws's own command, so the plugin can't have it", name, word)
	}
	for _, other := range Installed(h.Root) {
		if other == name {
			continue
		}
		if reg, ok := ReadRegistration(h.Root, other); ok && reg.CommandName(other) == word {
			return fmt.Errorf("%s: ws %s is the plugin %s's command already", name, word, other)
		}
	}
	return nil
}

// handle is what a plugin tells ws unasked.
func (h *Host) handle(p *proc, method string, params json.RawMessage) (any, error) {
	switch method {
	case "command/output":
		var o struct {
			ID     string `json:"id"`
			Stream string `json:"stream"`
			Data   string `json:"data"`
		}
		if json.Unmarshal(params, &o) != nil {
			return nil, nil
		}
		p.mu.Lock()
		out := p.outputs[o.ID]
		p.mu.Unlock()
		if out != nil {
			out(o.Stream, o.Data)
		}
	case "log":
		var l struct {
			Level   string `json:"level"`
			Message string `json:"message"`
		}
		if json.Unmarshal(params, &l) != nil {
			return nil, nil
		}
		if l.Level == "error" {
			p.mu.Lock()
			p.lastErr = l.Message
			p.mu.Unlock()
			h.seq.Add(1)
		}
		if l.Level == "error" || l.Level == "warn" {
			h.logf("%s: %s: %s", p.name, l.Level, l.Message)
		}
	case "view/changed":
		p.mu.Lock()
		p.changed++
		p.mu.Unlock()
		h.seq.Add(1)
	case "view/badge":
		var b struct {
			Path  string `json:"path"`
			Count int    `json:"count"`
		}
		if json.Unmarshal(params, &b) != nil {
			return nil, nil
		}
		p.mu.Lock()
		if b.Count > 0 {
			p.badges[b.Path] = b.Count
		} else {
			delete(p.badges, b.Path)
		}
		p.mu.Unlock()
		h.seq.Add(1)
	case "sync/state":
		var s struct {
			Name string `json:"name"`
			SyncState
		}
		if json.Unmarshal(params, &s) != nil || s.Name == "" {
			return nil, nil
		}
		p.mu.Lock()
		p.syncs[s.Name] = s.SyncState
		if s.Error == "" && p.lastErr != "" {
			p.lastErr = "" // it's well again
		}
		p.mu.Unlock()
		h.seq.Add(1)
	default:
		return nil, &acp.RPCError{Code: acp.CodeMethodNotFound, Message: "ws has no method " + method}
	}
	return nil, nil
}

// kill ends a plugin's process without asking.
func (h *Host) kill(p *proc) {
	p.stopping.Store(true)
	p.stdin.Close()
	p.cmd.Process.Kill() //nolint:errcheck // gone already is as good
	<-p.exited
}

// stop asks a plugin to finish, and ends it if it hasn't soon.
func (h *Host) stop(p *proc) {
	p.stopping.Store(true)
	if !p.alive() {
		return
	}
	// A plugin waits a little for its commands before it answers.
	ctx, cancel := context.WithTimeout(context.Background(), stopGrace)
	p.conn.Call(ctx, "shutdown", nil, nil) //nolint:errcheck // it's told by its stdin closing too
	cancel()
	p.stdin.Close()
	select {
	case <-p.exited:
	case <-time.After(stopGrace):
		p.cmd.Process.Kill() //nolint:errcheck // gone already is as good
		<-p.exited
	}
}

// ready is a plugin that can be asked: running, with what it requires
// set.
func (h *Host) ready(name string) (*proc, error) {
	p, err := h.ensure(name)
	if err != nil {
		return nil, err
	}
	values, err := Values(h.Root, name)
	if err != nil {
		return nil, err
	}
	p.mu.Lock()
	missing := Missing(p.reg, values)
	p.mu.Unlock()
	if len(missing) > 0 {
		return nil, &SetupError{Plugin: name, Missing: missing}
	}
	return p, nil
}

// Terminal is what a command's output is going to.
type Terminal struct {
	TTY   bool `json:"tty"`
	Width int  `json:"width,omitempty"`
	Color bool `json:"color"`
}

// Run runs one of a plugin's commands: args are what followed its word,
// dir is where it was run from, and out is given its output as it comes
// (stream is "stdout" or "stderr"). It's the command's exit code. ctx
// ending is Ctrl-C.
func (h *Host) Run(ctx context.Context, name string, args []string, dir string, term Terminal, out func(stream, data string)) (int, error) {
	p, err := h.ready(name)
	if err != nil {
		return 1, err
	}
	if h.Hold != nil {
		defer h.Hold()()
	}
	id := "c" + strconv.FormatUint(h.commands.Add(1), 10)
	p.mu.Lock()
	p.outputs[id] = out
	p.mu.Unlock()
	defer func() {
		p.mu.Lock()
		delete(p.outputs, id)
		p.mu.Unlock()
	}()
	if args == nil {
		args = []string{}
	}
	var answer struct {
		Exit int `json:"exit"`
	}
	err = p.conn.Call(ctx, "command/run", map[string]any{"id": id, "args": args, "dir": dir, "terminal": term}, &answer)
	switch {
	case errors.Is(err, acp.ErrClosed):
		return 1, fmt.Errorf("%s stopped while the command ran: see %s", name, LogPath(h.Root, name))
	case errors.Is(err, context.Canceled):
		return 130, nil
	case err != nil:
		return 1, err
	}
	return answer.Exit, nil
}

// ViewSocket is the unix socket a plugin's view is served on, the plugin
// started if it isn't running.
func (h *Host) ViewSocket(name string) (string, error) {
	p, err := h.ready(name)
	if err != nil {
		return "", err
	}
	return p.viewSock, nil
}

// ConfigChanged tells a running plugin its config, after a change.
func (h *Host) ConfigChanged(name string) error {
	h.mu.Lock()
	p := h.procs[name]
	h.mu.Unlock()
	h.seq.Add(1)
	if !p.alive() {
		return nil
	}
	values, err := Values(h.Root, name)
	if err != nil {
		return err
	}
	return p.conn.Notify("config/changed", map[string]any{"config": values})
}

// Stop stops a plugin, if it's running. It's started again when it's next
// needed.
func (h *Host) Stop(name string) {
	h.mu.Lock()
	p := h.procs[name]
	delete(h.procs, name)
	h.mu.Unlock()
	if p != nil {
		h.stop(p)
	}
	h.seq.Add(1)
}

// Refresh starts a plugin afresh, so what it registers is what it is now:
// after it's installed, or replaced.
func (h *Host) Refresh(name string) (Status, error) {
	h.Stop(name)
	if _, err := h.ensure(name); err != nil {
		return h.Status(name), err
	}
	return h.Status(name), nil
}

// Shutdown stops every plugin, and starts no more.
func (h *Host) Shutdown() {
	h.mu.Lock()
	h.closing = true
	procs := h.procs
	h.procs = nil
	h.mu.Unlock()
	var wg sync.WaitGroup
	for _, p := range procs {
		wg.Go(func() { h.stop(p) })
	}
	wg.Wait()
}

// Status is where one plugin stands, running or not.
func (h *Host) Status(name string) Status {
	st := Status{Name: name, State: Stopped}
	if m, err := ReadManifest(appDir(h.Root, name)); err == nil {
		st.Version = m.Version
	} else {
		st.State, st.Error = Failed, err.Error()
	}
	if target, ok := Linked(h.Root, name); ok {
		st.Linked = target
	}
	if src, ok := ReadSource(h.Root, name); ok {
		st.Source = &src
	}
	st.Registration, _ = ReadRegistration(h.Root, name)

	h.mu.Lock()
	p := h.procs[name]
	st.Starts = h.runs[name]
	h.mu.Unlock()
	if p != nil {
		p.mu.Lock()
		switch {
		case p.err != nil:
			st.State, st.Error = Failed, p.err.Error()
		case p.alive():
			st.State, st.Registration = Ready, p.reg
		}
		if p.err == nil {
			st.Error = p.lastErr
			st.Changed = p.changed
			if len(p.badges) > 0 {
				st.Badges = map[string]int{}
				for k, v := range p.badges {
					st.Badges[k] = v
				}
			}
			if len(p.syncs) > 0 {
				st.Syncs = map[string]SyncState{}
				for k, v := range p.syncs {
					st.Syncs[k] = v
				}
			}
		}
		p.mu.Unlock()
	}
	if st.State != Failed {
		if values, err := Values(h.Root, name); err == nil {
			for _, k := range Missing(st.Registration, values) {
				st.Missing = append(st.Missing, k.Key)
			}
			if len(st.Missing) > 0 {
				st.State = Setup
			}
		}
	}
	return st
}

// Statuses are the workspace's plugins, in order.
func (h *Host) Statuses() []Status {
	out := []Status{}
	for _, name := range Installed(h.Root) {
		out = append(out, h.Status(name))
	}
	return out
}
