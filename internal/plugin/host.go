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
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/withworkspace/plugin/protocol"
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
	// Refused is what it registered that ws left out, and why: a ref
	// that's wrong in itself, a prefix that's another plugin's already.
	Refused []string `json:"refused,omitempty"`
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
	// Answer answers what a plugin asks ws (a request: protocol's
	// mission/list and the rest), given which plugin asked. Its errors are
	// NoMethod and NotFound, or anything else, as an internal error. nil
	// answers nothing: every request is NoMethod.
	Answer func(ctx context.Context, plugin, method string, params json.RawMessage) (any, error)
	// LookLinger is how long nobody has to have been looking before a
	// plugin is told so (Look): 0 is five seconds.
	LookLinger time.Duration
	// Keep keeps how a plugin's things are named (the refs it registered)
	// in the workspace's records, when it starts, and says what it left
	// out: KeepRefs, in the daemon. nil keeps nothing, as a host with no
	// records to keep them in has it (ws plugin check): nothing it runs
	// names anything.
	Keep func(plugin string, refs []protocol.Ref) (refused []string, err error)

	mu        sync.Mutex
	procs     map[string]*proc
	starts    map[string]*sync.Mutex // one start of a plugin at a time
	runs      map[string]uint64      // how many times each has been started
	closing   bool
	pages     int                  // pages of the web UI that are showing
	apps      map[string]int       // those showing a plugin's app, by plugin
	pagesGone time.Time            // when the last page stopped showing
	appsGone  map[string]time.Time // when the last page showing a plugin's app did
	lookMu    sync.Mutex           // one telling of who's looking at a time

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
	// ready is closed once it's started: what it registered is kept, and
	// what it asks ws can be answered.
	ready   chan struct{}
	started time.Time
	// stopping is set once ws has told it to go: its going isn't its own
	// idea.
	stopping atomic.Bool
	// told is who it was last told is looking (Host.lookMu): nobody, for
	// one that's just started.
	told protocol.ViewLookingParams

	mu      sync.Mutex
	reg     Registration
	outputs map[string]func(stream, data string)
	inputs  map[string]*Input // a running command's standard input, by its id
	badges  map[string]int
	changed uint64
	syncs   map[string]SyncState
	lastErr string
	refused []string // what it registered that ws left out

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
	// Whoever's looking as it starts, it's told of.
	go h.tellLooking()
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
		name: name, manifest: m, cmd: cmd, stdin: stdin, exited: make(chan struct{}), ready: make(chan struct{}),
		viewSock: socketPath(h.Root, filepath.Join(data, "view.sock"), "view:"+name),
		outputs:  map[string]func(string, string){}, inputs: map[string]*Input{}, badges: map[string]int{}, syncs: map[string]SyncState{},
		started: time.Now(),
	}
	p.conn = acp.NewConn(stdout, stdin, func(ctx context.Context, method string, params json.RawMessage) (any, error) {
		return h.handle(ctx, p, method, params)
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

	// Its config is shaped by what it declared when it last ran: its
	// tables as rows, in order. One that's never run is given the files'
	// own shape, and its config again once it's said what it declares.
	known, _ := ReadRegistration(h.Root, name)
	values, err := h.wireValues(name, known.Config)
	if err != nil {
		h.kill(p)
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), startTimeout)
	defer cancel()
	var answer protocol.InitializeResult
	err = p.conn.Call(ctx, protocol.MethodInitialize, protocol.InitializeParams{
		Protocol:  slices.Max(Protocols),
		Protocols: Protocols,
		WS:        protocol.WSInfo{Version: h.Version, Executable: h.executable()},
		Plugin:    protocol.PluginInfo{Name: name, Version: m.Version, Dir: app, Data: data},
		Workspace: protocol.Workspace{Root: h.Root},
		Config:    values,
		View:      protocol.ViewInfo{Listen: p.viewSock, Base: ViewBase(name)},
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
	if err := agreed(name, answer); err != nil {
		h.kill(p)
		return nil, err
	}
	if err := h.acceptable(name, answer.Registration); err != nil {
		h.kill(p)
		return nil, err
	}
	p.reg = answer.Registration
	if err := writeRegistration(h.Root, name, p.reg); err != nil {
		h.logf("%s: keeping what it registered: %v", name, err)
	}
	// What it declares of its tables isn't what ws shaped its config by:
	// it's given its config again, as it now reads it.
	if HasTables(p.reg.Config) && !sameKeys(known.Config, p.reg.Config) {
		if values, err := h.wireValues(name, p.reg.Config); err != nil {
			h.logf("%s: its config, as it declares it: %v", name, err)
		} else if err := p.conn.Notify(protocol.MethodConfigChanged, protocol.ConfigChangedParams{Config: values}); err != nil {
			h.logf("%s: telling it its config: %v", name, err)
		}
	}
	if h.Keep != nil {
		refused, err := h.Keep(name, p.reg.Refs)
		if err != nil {
			h.logf("%s: keeping how its things are named: %v", name, err)
		}
		for _, r := range refused {
			h.logf("%s: %s", name, r)
		}
		p.refused = refused
	}
	h.mu.Lock()
	if h.runs == nil {
		h.runs = map[string]uint64{}
	}
	h.runs[name]++
	h.mu.Unlock()
	close(p.ready)
	h.logf("%s %s started", name, m.Version)
	return p, nil
}

// Protocols are the versions of the plugin protocol ws speaks, oldest
// first. It's ws's own list, not the protocol package's: a version is
// spoken once ws handles it, which a release of the package that adds its
// types doesn't do. Each is one the package has (a test says so), and
// once spoken, one ws goes on speaking.
var Protocols = []int{1}

// NoMethod is the answer to a request ws doesn't have.
func NoMethod(method string) error {
	return &acp.RPCError{Code: acp.CodeMethodNotFound, Message: "ws has no method " + method}
}

// NotFound is the answer to a request for something by name that ws
// hasn't: what says so.
func NotFound(what string) error {
	return &acp.RPCError{Code: protocol.CodeNotFound, Message: what}
}

// InvalidParams is the answer to a request whose params aren't its own.
func InvalidParams(err error) error {
	return &acp.RPCError{Code: acp.CodeInvalidParams, Message: err.Error()}
}

// agreed refuses a plugin whose answer to initialize isn't in a version
// of the protocol ws speaks, and says which side is behind.
func agreed(name string, answer protocol.InitializeResult) error {
	if slices.Contains(Protocols, answer.Protocol) {
		return nil
	}
	speaks := answer.Protocols
	if len(speaks) == 0 && answer.Protocol > 0 {
		speaks = []int{answer.Protocol} // from before versions were chosen
	}
	if len(speaks) == 0 {
		return fmt.Errorf("%s doesn't say which version of the plugin protocol it speaks, and this ws speaks %s. Update the plugin", name, versions(Protocols))
	}
	update := "Update the plugin"
	if slices.Max(speaks) > slices.Max(Protocols) {
		update = "Update ws"
	}
	return fmt.Errorf("%s speaks the plugin protocol's %s, and this ws speaks %s. %s", name, versions(speaks), versions(Protocols), update)
}

// versions says a list of the protocol's versions: "version 1",
// "versions 1 and 2".
func versions(vs []int) string {
	if len(vs) == 1 {
		return "version " + strconv.Itoa(vs[0])
	}
	words := make([]string, len(vs))
	for i, v := range vs {
		words[i] = strconv.Itoa(v)
	}
	return "versions " + strings.Join(words[:len(words)-1], ", ") + " and " + words[len(words)-1]
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

// ViewBase is the path a plugin's own HTTP is served under: its app's
// files, and what its app asks it. (The page its app runs in is ws's, at
// /plugin-frame/<name>/….)
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

// handle is what a plugin tells ws unasked (notifications), and what it
// asks (requests, which Answer answers).
func (h *Host) handle(ctx context.Context, p *proc, method string, params json.RawMessage) (any, error) {
	switch method {
	case protocol.MethodCommandOutput:
		var o protocol.CommandOutputParams
		if json.Unmarshal(params, &o) != nil {
			return nil, nil
		}
		p.mu.Lock()
		out := p.outputs[o.ID]
		p.mu.Unlock()
		if out != nil {
			out(o.Stream, o.Data)
		}
	case protocol.MethodCommandRead:
		var r protocol.CommandReadParams
		if err := json.Unmarshal(params, &r); err != nil {
			return nil, &acp.RPCError{Code: acp.CodeInvalidParams, Message: err.Error()}
		}
		p.mu.Lock()
		in := p.inputs[r.ID]
		p.mu.Unlock()
		if in == nil {
			return nil, &acp.RPCError{Code: protocol.CodeNotFound, Message: "no running command " + r.ID + " with input to read"}
		}
		return in.read(r.Max)
	case protocol.MethodLog:
		var l protocol.LogParams
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
	case protocol.MethodViewChanged:
		p.mu.Lock()
		p.changed++
		p.mu.Unlock()
		h.seq.Add(1)
	case protocol.MethodViewBadge:
		var b protocol.ViewBadgeParams
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
	case protocol.MethodSyncState:
		var s protocol.SyncStateParams
		if json.Unmarshal(params, &s) != nil || s.Name == "" {
			return nil, nil
		}
		at, err := time.Parse(time.RFC3339, s.At)
		if err != nil {
			return nil, nil
		}
		p.mu.Lock()
		p.syncs[s.Name] = SyncState{At: at, TookMS: int(s.TookMS), Error: s.Error}
		if s.Error == "" && p.lastErr != "" {
			p.lastErr = "" // it's well again
		}
		p.mu.Unlock()
		h.seq.Add(1)
	default:
		if h.Answer == nil {
			return nil, NoMethod(method)
		}
		// It may ask as soon as it's answered initialize, which is before
		// ws has taken what it registered: what it asks waits for that,
		// since what it tells ws of its things is checked against it.
		select {
		case <-p.ready:
		case <-p.exited:
			return nil, errors.New("the plugin stopped before it was started")
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		return h.Answer(ctx, p.name, method, params)
	}
	return nil, nil
}

// read is what's next of a command's input: at least a byte, or the end.
func (in *Input) read(max int) (protocol.CommandReadResult, error) {
	if max <= 0 || max > protocol.ReadAtOnce {
		max = protocol.ReadAtOnce
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	b := make([]byte, max)
	for {
		n, err := in.R.Read(b)
		switch {
		case n > 0:
			// What came with an error is given, and the error's met again
			// on the next read.
			return protocol.CommandReadResult{Data: b[:n]}, nil
		case errors.Is(err, io.EOF):
			return protocol.CommandReadResult{EOF: true}, nil
		case err != nil:
			return protocol.CommandReadResult{}, fmt.Errorf("reading the command's input: %w", err)
		}
	}
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
	p.conn.Call(ctx, protocol.MethodShutdown, nil, nil) //nolint:errcheck // it's told by its stdin closing too
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
type Terminal = protocol.Terminal

// Input is a command's standard input, which the plugin reads from ws as
// the command reads it (command/read): nothing's read from R that the
// command doesn't ask for, so one that never reads never waits on it.
type Input struct {
	// Kind is protocol.InputPiped or protocol.InputTerminal.
	Kind string
	R    io.Reader

	mu sync.Mutex // one read at a time
}

// Run runs one of a plugin's commands: args are what followed its word,
// dir is where it was run from, in is its standard input (nil for none),
// and out is given its output as it comes (stream is "stdout" or
// "stderr"). It's the command's exit code. ctx ending is Ctrl-C.
func (h *Host) Run(ctx context.Context, name string, args []string, dir string, term Terminal, in *Input, out func(stream, data string)) (int, error) {
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
	if in != nil {
		p.inputs[id] = in
	}
	p.mu.Unlock()
	defer func() {
		p.mu.Lock()
		delete(p.outputs, id)
		delete(p.inputs, id)
		p.mu.Unlock()
	}()
	if args == nil {
		args = []string{}
	}
	run := protocol.CommandRunParams{ID: id, Args: args, Dir: dir, Terminal: term}
	if in != nil {
		run.Input = in.Kind
	}
	var answer protocol.CommandRunResult
	err = p.conn.Call(ctx, protocol.MethodCommandRun, run, &answer)
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

// showTimeout is how long a plugin has to say what one of its things is.
const showTimeout = 20 * time.Second

// ErrNoThing is a plugin having nothing by a name: one that isn't a name
// it registered, or one it says isn't there. What's returned says which,
// and is this (errors.Is).
var ErrNoThing = errors.New("the plugin has nothing by that name")

type noThing struct{ why string }

func (e *noThing) Error() string        { return e.why }
func (e *noThing) Is(target error) bool { return target == ErrNoThing }

// ErrNoShow is a plugin having no more to say of its things than it's
// told ws: its kind has nothing that answers for one thing, now. What ws
// was told is all there is to show.
var ErrNoShow = errors.New("the plugin says no more of its things than it's told ws")

type noShow struct{ why string }

func (e *noShow) Error() string        { return e.why }
func (e *noShow) Is(target error) bool { return target == ErrNoShow }

// Shown is one of a plugin's things, as the plugin has it now: a value
// for each of its kind's fields.
type Shown = protocol.RefShowResult

// Show asks a plugin what one of its things is, now: kind is the ref's,
// as it registered it, and ref the thing's name as it was written
// (RAM-123). The plugin's started if it isn't running. It's asked only
// about a name that's one of that kind's, as it registered them:
// ErrNoThing otherwise, and when the plugin says it has none by it. A
// plugin with nothing that answers for one thing is ErrNoShow.
func (h *Host) Show(ctx context.Context, name, kind, ref string, full bool) (Shown, error) {
	var out Shown
	p, err := h.ready(name)
	if err != nil {
		return out, err
	}
	p.mu.Lock()
	i := slices.IndexFunc(p.reg.Refs, func(r protocol.Ref) bool { return r.Kind == kind })
	var registered protocol.Ref
	if i >= 0 {
		registered = p.reg.Refs[i]
	}
	p.mu.Unlock()
	if i < 0 || registered.Problem() != "" || !named(name, registered).Names(ref) {
		return out, &noThing{fmt.Sprintf("%s isn't a name %s registered for its %s", ref, name, kind)}
	}
	if h.Hold != nil {
		defer h.Hold()()
	}
	ctx, cancel := context.WithTimeout(ctx, showTimeout)
	defer cancel()
	err = p.conn.Call(ctx, protocol.MethodRefShow, protocol.RefShowParams{Kind: kind, Ref: ref, Full: full}, &out)
	var re *acp.RPCError
	switch {
	case errors.As(err, &re) && re.Code == protocol.CodeNotFound:
		return out, &noThing{fmt.Sprintf("%s has no %s called %s", name, kind, ref)}
	case errors.As(err, &re) && re.Code == protocol.CodeMethodNotFound:
		return out, &noShow{fmt.Sprintf("%s says no more of a %s than it's told ws", name, kind)}
	case errors.As(err, &re):
		return out, fmt.Errorf("%s: %s", name, re.Message)
	case errors.Is(err, acp.ErrClosed):
		return out, fmt.Errorf("%s stopped while it was asked about %s: see %s", name, ref, LogPath(h.Root, name))
	case errors.Is(err, context.DeadlineExceeded):
		return out, fmt.Errorf("%s didn't say what %s is in %s", name, ref, showTimeout)
	}
	return out, err
}

// actTimeout is how long a plugin has to run an action, or to ask what it
// wants answered first.
const actTimeout = 2 * time.Minute

// ErrNoAction is a plugin having no action by an id: one it didn't
// register, or one of a kind's asked of a name that isn't that kind's.
var ErrNoAction = errors.New("the plugin has no such action")

type noAction struct{ why string }

func (e *noAction) Error() string        { return e.why }
func (e *noAction) Is(target error) bool { return target == ErrNoAction }

// Act runs one of a plugin's actions, as it's chosen in Find: its own
// (in.Kind empty), or one of a kind's on one of its things. The plugin's
// started if it isn't running. It's run only if the plugin registered
// it, and for a thing, only by a name that's one of the kind's:
// ErrNoAction otherwise. What comes back may be the action asking for
// something (Ask): it's run again with the answer.
func (h *Host) Act(ctx context.Context, name string, in protocol.ActionRunParams) (protocol.ActionRunResult, error) {
	var out protocol.ActionRunResult
	p, err := h.ready(name)
	if err != nil {
		return out, err
	}
	p.mu.Lock()
	actions, named := p.reg.Actions, true
	if in.Kind != "" {
		actions, named = nil, false
		if i := slices.IndexFunc(p.reg.Refs, func(r protocol.Ref) bool { return r.Kind == in.Kind }); i >= 0 && p.reg.Refs[i].Problem() == "" {
			actions, named = p.reg.Refs[i].Actions, p.reg.Refs[i].Names(in.Ref)
		}
	}
	p.mu.Unlock()
	if !named || !slices.ContainsFunc(actions, func(a protocol.Action) bool { return a.ID == in.ID }) {
		return out, &noAction{fmt.Sprintf("%s has no action %s for that", name, in.ID)}
	}
	if h.Hold != nil {
		defer h.Hold()()
	}
	if in.Answers == nil {
		in.Answers = []string{}
	}
	ctx, cancel := context.WithTimeout(ctx, actTimeout)
	defer cancel()
	err = p.conn.Call(ctx, protocol.MethodActionRun, in, &out)
	var re *acp.RPCError
	switch {
	case errors.As(err, &re):
		return out, fmt.Errorf("%s: %s", name, re.Message)
	case errors.Is(err, acp.ErrClosed):
		return out, fmt.Errorf("%s stopped while it was doing it: see %s", name, LogPath(h.Root, name))
	case errors.Is(err, context.DeadlineExceeded):
		return out, fmt.Errorf("%s hadn't done it in %s", name, actTimeout)
	}
	return out, err
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
	values, err := h.wireValues(name, p.reg.Config)
	if err != nil {
		return err
	}
	return p.conn.Notify(protocol.MethodConfigChanged, protocol.ConfigChangedParams{Config: values})
}

// wireValues is a plugin's config as it's sent: each value as JSON, a
// table as its rows that are given, in the order they were written. What's
// wrong in it is in ws's log, and is what ws plugin config says.
func (h *Host) wireValues(name string, keys []ConfigKey) (map[string]json.RawMessage, error) {
	shaped, problems, err := Shaped(h.Root, name, keys)
	if err != nil {
		return nil, err
	}
	for _, p := range problems {
		h.logf("%s's config: %s (ws plugin config %s)", name, p, name)
	}
	values := Wire(shaped)
	out := make(map[string]json.RawMessage, len(values))
	for k, v := range values {
		data, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("%s's config %s: %w", name, k, err)
		}
		out[k] = data
	}
	return out, nil
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
			st.Refused = slices.Clone(p.refused)
			if len(p.badges) > 0 {
				st.Badges = map[string]int{}
				maps.Copy(st.Badges, p.badges)
			}
			if len(p.syncs) > 0 {
				st.Syncs = map[string]SyncState{}
				maps.Copy(st.Syncs, p.syncs)
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

// sameKeys is whether two registrations declare the same config.
func sameKeys(a, b []ConfigKey) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return string(x) == string(y)
}
