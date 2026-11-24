package daemon

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/withworkspace/workspace/internal/crew"
	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/job"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/termpane"
)

// frameInterval caps how often a deck gets frames for one crew.
const frameInterval = 16 * time.Millisecond

// Server owns crew for one workspace.
type Server struct {
	Root string
	// IdleTimeout, if set, is how long the server stays up with nothing
	// running, attached or held (busyLocked) before it shuts itself down.
	// Zero, the default, is none: the server stays up until Shutdown.
	IdleTimeout time.Duration
	Log         *log.Logger
	// OnCrewEnded, if set, runs after a crew's end is recorded (see
	// crew.EndHook).
	OnCrewEnded crew.EndHook
	// Jobs runs work for agents that outlives whoever asked for it (the
	// orchestrator's runs, conversations' answers), taken over the socket
	// ("job") or from inside: nil refuses them. The server stays up while
	// one runs.
	Jobs *job.Queue
	// WebPort is the port of the web UI running alongside, 0 for none. It's
	// set before Serve, so every hello says it.
	WebPort int
	// OnACPTurnEnded, if set, runs when a turn of crew over ACP ends, as a
	// terminal crew's stop hook does (the orchestrator may take a turn
	// that ended without the crew saying where it is).
	OnACPTurnEnded func(slug, crewID string)
	// ReadProcs reads the user's processes, for the process check at the
	// end of a turn of crew over ACP: nil is harness.ReadProcs.
	ReadProcs func() (harness.Procs, error)
	// Build is the ws this server is (its version), said in every hello.
	Build string
	// Exe is the ws binary this server runs, for a server that restarts
	// itself (restart.go): "" for one that doesn't.
	Exe string
	// CrewAtWork, if set, names the crew a restart would stop mid-turn: a
	// restart that isn't to be now waits for them.
	CrewAtWork func() []string
	// TrustFolders, if set, says whether to answer Claude Code's trust
	// dialog yes for crew (crew.WatchTrust): ws.local.toml's [crew]
	// trust_folders. Nil leaves it to you.
	TrustFolders func() bool
	// Silos, if set, is where each silo the daemon keeps stands, as JSON
	// (workspace.SiloState), for ws silo status and watch; ResyncSilo
	// gives a repo's silo its target's files in full ("" for every one).
	Silos      func() json.RawMessage
	ResyncSilo func(repo string) error

	store mission.Store
	ln    net.Listener

	mu     sync.Mutex
	crews  map[string]*crewProc
	agents map[string]*acpCrew // crew over ACP (acp.go)
	// starting holds crew's key from their launch's check until they're
	// in crews or agents, or failed: a second launch meanwhile is refused,
	// not run alongside (an agent's start can take a minute).
	starting map[string]bool
	conns    map[*conn]bool
	holds    int // Hold: something else keeping the server up (open web UIs)
	idle     *time.Timer
	closing  bool
	done     chan struct{}

	restartWanted bool   // to restart once nothing's at work
	restarting    bool   // Shutdown was to restart: whoever ran Serve starts ws again
	next          string // the ws found in place of Exe: its version

	watchers sync.WaitGroup // crew end recorders; Shutdown waits for them
}

type crewProc struct {
	spec Spec
	pane *termpane.Pane

	stopRequested atomic.Bool

	mu   sync.Mutex
	subs map[*conn]*sub
	// watchers are in-process watchers of the screen (the web UI's crew
	// pages): each is woken when it may have changed.
	watchers map[chan struct{}]bool
}

// sub is one deck attached to one crew.
type sub struct {
	c      *conn
	wake   chan struct{}
	offset int
	mirror bool
	quit   chan struct{}
	// copies is how many of the crew's clipboard copies this deck has been
	// sent (or missed by attaching later); only push touches it.
	copies uint64
}

type conn struct {
	nc  net.Conn
	wmu sync.Mutex
	enc *json.Encoder
}

func (c *conn) send(m message) error {
	c.wmu.Lock()
	defer c.wmu.Unlock()
	c.nc.SetWriteDeadline(time.Now().Add(5 * time.Second))
	return c.enc.Encode(m)
}

// Listen binds the workspace socket. It fails if another server is live.
func (s *Server) Listen() error {
	path := SocketPath(s.Root)
	if c, err := net.DialTimeout("unix", path, time.Second); err == nil {
		c.Close()
		return fmt.Errorf("a daemon is already listening on %s", path)
	}
	os.Remove(path) // stale socket from a server that died
	os.MkdirAll(dirOf(path), 0o755)
	ln, err := net.Listen("unix", path)
	if err != nil {
		return err
	}
	os.Chmod(path, 0o600)
	s.ln = ln
	s.store = mission.Store{Root: s.Root}
	s.crews = map[string]*crewProc{}
	s.agents = map[string]*acpCrew{}
	s.starting = map[string]bool{}
	s.conns = map[*conn]bool{}
	s.done = make(chan struct{})
	if s.Log == nil {
		s.Log = log.New(io.Discard, "", 0)
	}
	s.mu.Lock()
	s.checkIdleLocked()
	s.mu.Unlock()
	if s.Exe != "" {
		// Looked at here, not once it's watching: what's installed a moment
		// after this is another ws than the one that's running.
		if was, err := os.Stat(s.Exe); err != nil {
			s.Log.Printf("not watching %s for another ws in its place: %v", s.Exe, err)
		} else {
			go s.watchExe(was, exeTick)
		}
	}
	return nil
}

func dirOf(p string) string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' {
			return p[:i]
		}
	}
	return "."
}

// Serve accepts decks until Shutdown, or until it idles out if it has an
// IdleTimeout.
func (s *Server) Serve() error {
	s.Log.Printf("daemon for %s listening on %s (pid %d)", s.Root, SocketPath(s.Root), os.Getpid())
	for {
		nc, err := s.ln.Accept()
		if err != nil {
			s.mu.Lock()
			closing := s.closing
			s.mu.Unlock()
			if closing && errors.Is(err, net.ErrClosed) {
				<-s.done // Shutdown has removed the socket and finished
				return nil
			}
			return err
		}
		c := &conn{nc: nc, enc: json.NewEncoder(nc)}
		s.mu.Lock()
		s.conns[c] = true
		s.checkIdleLocked()
		s.mu.Unlock()
		go s.handle(c)
	}
}

// Shutdown stops every crew (recording their ends) and closes the socket.
func (s *Server) Shutdown(reason string) {
	s.mu.Lock()
	if s.closing {
		s.mu.Unlock()
		return
	}
	s.closing = true
	crews := make([]*crewProc, 0, len(s.crews))
	for _, cr := range s.crews {
		crews = append(crews, cr)
	}
	agents := make([]*acpCrew, 0, len(s.agents))
	for _, cr := range s.agents {
		agents = append(agents, cr)
	}
	s.mu.Unlock()
	// Closed, not stopped: nobody asked them to stop, so their ends are
	// recorded as interrupted, for the next daemon to resume.
	for _, cr := range agents {
		go cr.agent.Close(3 * time.Second) //nolint:errcheck // its end is recorded by watchACP
	}

	var wg sync.WaitGroup
	for _, cr := range crews {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cr.pane.Close(3 * time.Second)
		}()
	}
	wg.Wait()
	s.watchers.Wait() // every crew's end is on its mission before we exit
	s.Log.Printf("shutting down: %s", reason)
	s.ln.Close()
	os.Remove(SocketPath(s.Root))
	close(s.done) // last: whoever waits on Done finds the socket gone
}

// Done is closed once the server has shut down and removed its socket.
func (s *Server) Done() <-chan struct{} { return s.done }

// Hold keeps the server from idling out until release is called, as an
// attached deck does. The web UI holds it for each open page.
func (s *Server) Hold() (release func()) {
	s.mu.Lock()
	s.holds++
	s.checkIdleLocked()
	s.mu.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			s.mu.Lock()
			s.holds--
			s.checkIdleLocked()
			s.mu.Unlock()
		})
	}
}

func (s *Server) busyLocked() bool {
	return len(s.crews) > 0 || len(s.agents) > 0 || len(s.conns) > 0 || s.holds > 0
}

// checkIdleLocked arms the idle timer, if the server has an IdleTimeout,
// when nothing is running, attached or held. A job running when it goes
// off puts it off again: nothing else says when a job's done.
func (s *Server) checkIdleLocked() {
	if s.idle != nil {
		s.idle.Stop()
		s.idle = nil
	}
	if s.IdleTimeout <= 0 || s.busyLocked() || s.closing {
		return
	}
	var wait func()
	wait = func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		switch {
		case s.busyLocked() || s.closing:
		case s.Jobs != nil && s.Jobs.Busy():
			s.idle = time.AfterFunc(s.IdleTimeout, wait)
		default:
			go s.Shutdown("idle")
		}
	}
	s.idle = time.AfterFunc(s.IdleTimeout, wait)
}

func (s *Server) handle(c *conn) {
	defer func() {
		c.nc.Close()
		s.mu.Lock()
		delete(s.conns, c)
		for _, cr := range s.crews {
			cr.detach(c)
		}
		s.checkIdleLocked()
		s.mu.Unlock()
	}()
	sc := bufio.NewScanner(c.nc)
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for sc.Scan() {
		var req request
		if err := json.Unmarshal(sc.Bytes(), &req); err != nil {
			continue
		}
		reply := s.do(c, req)
		reply.ID = req.ID
		if err := c.send(reply); err != nil {
			return
		}
	}
}

func (s *Server) crew(key string) (*crewProc, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cr, ok := s.crews[key]
	if !ok {
		return nil, fmt.Errorf("no running crew %s", key)
	}
	return cr, nil
}

func (s *Server) do(c *conn, req request) message {
	fail := func(err error) message { return message{Error: err.Error()} }
	switch req.Op {
	case "hello":
		idle := s.IdleTimeout
		s.mu.Lock()
		defer s.mu.Unlock()
		return message{Ver: ProtocolVersion, Pid: os.Getpid(), Web: s.WebPort, Idle: &idle, Build: s.Build, Exe: s.Exe, Wanted: s.restartWanted, Next: s.next}

	case "list":
		s.mu.Lock()
		defer s.mu.Unlock()
		var out []CrewInfo
		for _, cr := range s.crews {
			out = append(out, CrewInfo{Spec: cr.spec, Pid: cr.pane.Pid()})
		}
		return message{Crews: out}

	case "launch":
		if req.Spec == nil {
			return fail(errors.New("launch needs a spec"))
		}
		pid, err := s.Launch(*req.Spec)
		if err != nil {
			return fail(err)
		}
		return message{Pid: pid}

	case "shutdown":
		go s.Shutdown("requested")
		return message{}

	case "atwork":
		return message{AtWork: s.AtWork()}

	case "restart":
		if s.Exe == "" {
			// As a server from a ws before the op answers: whoever asked
			// stops it and starts another.
			return fail(fmt.Errorf("unknown op %q", req.Op))
		}
		waits, err := s.Restart(req.Now)
		if err != nil {
			return fail(err)
		}
		return message{AtWork: waits}

	case "job":
		if req.Job == nil || s.Jobs == nil {
			return fail(errors.New("this server runs no jobs"))
		}
		s.Jobs.Start(*req.Job)
		return message{}

	case "say":
		said, err := s.Say(req.Crew, req.Text)
		if err != nil {
			return fail(err)
		}
		return message{Text: said}

	case "silos":
		if s.Silos == nil {
			return fail(errors.New("this server keeps no silos"))
		}
		return message{Silos: s.Silos()}

	case "silo-resync":
		if s.ResyncSilo == nil {
			return fail(errors.New("this server keeps no silos"))
		}
		if err := s.ResyncSilo(req.Text); err != nil {
			return fail(err)
		}
		return message{}
	}

	if a, ok := s.acpCrew(req.Crew); ok {
		// No terminal: what it's said is its record's, and it stops.
		switch req.Op {
		case "transcript":
			data, _ := os.ReadFile(a.record)
			return message{Text: harness.ACPTranscriptText(data)}
		case "stop":
			grace := time.Duration(req.Grace) * time.Millisecond
			if grace <= 0 {
				grace = 3 * time.Second
			}
			s.Log.Printf("stop requested for %s", req.Crew)
			a.stop(grace)
			return message{}
		}
		return fail(fmt.Errorf("%s runs over ACP, with no terminal: see it in the web UI", req.Crew))
	}
	cr, err := s.crew(req.Crew)
	if err != nil {
		return fail(err)
	}
	switch req.Op {
	case "transcript":
		return message{Text: cr.pane.Transcript()}
	case "attach":
		cr.attach(c)
	case "key":
		if req.Key != nil {
			cr.pane.SendKey(*req.Key)
		}
	case "mouse":
		if req.Mouse != nil {
			cr.pane.SendMouse(uvWheel(*req.Mouse))
		}
	case "paste":
		cr.pane.Paste(req.Text)
	case "raw":
		cr.pane.WriteRaw(req.Data)
	case "resize":
		cr.pane.Resize(req.W, req.H)
	case "scroll":
		cr.setOffset(c, req.Offset)
	case "mirror":
		cr.setMirror(c, req.On)
	case "stop":
		s.Log.Printf("stop requested for %s", req.Crew)
		grace := time.Duration(req.Grace) * time.Millisecond
		if grace <= 0 {
			grace = 3 * time.Second
		}
		// Hang up in the background: this connection may carry input for
		// other crew. The deck learns of the end from the exit event.
		cr.stopRequested.Store(true)
		go cr.pane.Close(grace)
	default:
		return fail(fmt.Errorf("unknown op %q", req.Op))
	}
	return message{}
}

var (
	_ crew.Host = (*Server)(nil)
	_ crew.Runs = (*Server)(nil)
)

// Run is crew the server's running, however it runs: in a terminal, or as
// an agent over ACP.
func (s *Server) Run(key string) (crew.Run, bool) {
	if a, ok := s.acpCrew(key); ok {
		return crew.AgentRun{S: a.spec, Agent: a.agent, Root: s.Root}, true
	}
	cr, err := s.crew(key)
	if err != nil || cr.pane.Exited() {
		return nil, false
	}
	return crew.TermRun{S: cr.spec, T: cr.pane, Root: s.Root}, true
}

// Say gives crew the server's running a message (crew.Run.Say).
func (s *Server) Say(key, text string) (string, error) {
	r, ok := s.Run(key)
	if !ok {
		return "", crew.ErrNotRunning
	}
	return r.Say(text)
}

// Launch starts crew in this server, as crew.Host: a terminal, its opening
// prompt typed in once its screen settles, or (spec.ACP) an agent held over
// ACP, its opening message sent. Inside the server, this is the host: a
// launch from the web UI needs no socket.
func (s *Server) Launch(spec Spec) (int, error) {
	switch {
	case spec.ACP != nil:
		return s.launchACP(spec)
	case len(spec.Args) == 0:
		return 0, errors.New("launch needs a spec with args, or an ACP run")
	}
	return s.launch(spec)
}

func (s *Server) launch(spec Spec) (int, error) {
	if err := s.reserve(spec.Key()); err != nil {
		return 0, err
	}
	cmd := exec.Command(spec.Args[0], spec.Args[1:]...)
	cmd.Dir = spec.Dir
	cmd.Env = spec.Env
	pane, err := termpane.Start(cmd, spec.Width, spec.Height)
	if err != nil {
		s.release(spec.Key())
		return 0, err
	}
	cr := &crewProc{spec: spec, pane: pane, subs: map[*conn]*sub{}, watchers: map[chan struct{}]bool{}}

	s.mu.Lock()
	delete(s.starting, spec.Key())
	s.crews[spec.Key()] = cr
	s.checkIdleLocked()
	s.mu.Unlock()
	s.Log.Printf("launched %s (%s) pid %d in %s: %s", spec.Key(), spec.Harness, pane.Pid(), spec.Dir, argsForLog(spec.Args))

	go cr.fanOut()
	s.watchers.Add(1)
	go func() {
		defer s.watchers.Done()
		s.watch(cr)
	}()
	if spec.Prompt != "" {
		go crew.TypePrompt(s.Root, spec.Key(), crew.PaneTerm(pane), spec.Prompt)
	}
	go crew.WatchTrust(s.store, s.Root, spec, crew.PaneTerm(pane), s.TrustFolders)
	return pane.Pid(), nil
}

// watch records the crew's end on its mission and tells attached decks.
func (s *Server) watch(cr *crewProc) {
	<-cr.pane.Done()
	exit := cr.pane.ExitSummary()
	s.mu.Lock()
	closing := s.closing
	s.mu.Unlock()
	if closing {
		exit = "stopped: " + exit
	}
	s.Log.Printf("%s ended: %s", cr.spec.Key(), exit)
	if err := crew.Finish(s.store, cr.spec, exit, cr.pane.Transcript(), crew.HowEnded(cr.stopRequested.Load(), closing), s.OnCrewEnded); err != nil {
		s.Log.Printf("recording end of %s: %v", cr.spec.Key(), err)
	}

	cr.mu.Lock()
	for c, sb := range cr.subs {
		close(sb.quit)
		c.send(message{Ev: "exit", Crew: cr.spec.Key(), Exit: exit})
	}
	cr.subs = map[*conn]*sub{}
	cr.mu.Unlock()

	s.mu.Lock()
	if s.crews[cr.spec.Key()] == cr { // not crew of the same key started since
		delete(s.crews, cr.spec.Key())
	}
	s.checkIdleLocked()
	s.mu.Unlock()
}

// fanOut turns the pane's single update signal into one per attached deck.
func (cr *crewProc) fanOut() {
	for {
		select {
		case <-cr.pane.Updates():
		case <-cr.pane.Done():
			return
		}
		cr.mu.Lock()
		for _, sb := range cr.subs {
			select {
			case sb.wake <- struct{}{}:
			default:
			}
		}
		for w := range cr.watchers {
			select {
			case w <- struct{}{}:
			default:
			}
		}
		cr.mu.Unlock()
	}
}

// Transcript is what running crew of key show now: a terminal's screen
// and what scrolled off it, or an agent's turns; "" for crew this server
// isn't running.
func (s *Server) Transcript(key string) string {
	s.mu.Lock()
	cr, term := s.crews[key]
	a, agent := s.agents[key]
	s.mu.Unlock()
	switch {
	case term:
		return cr.pane.Transcript()
	case agent:
		data, _ := os.ReadFile(a.record)
		return harness.ACPTranscriptText(data)
	}
	return ""
}

// frame renders the crew's screen, scrolled back offset lines.
func (cr *crewProc) frame(offset int) *Frame {
	x, y, visible := cr.pane.Cursor()
	w, h := cr.pane.Size()
	return &Frame{
		Screen:        cr.pane.RenderScrolled(offset),
		Offset:        offset,
		ScrollbackLen: cr.pane.ScrollbackLen(),
		AltScreen:     cr.pane.AltScreen(),
		CursorX:       x, CursorY: y, CursorVisible: visible,
		Title: cr.pane.Title(),
		Width: w, Height: h,
		Changed: cr.pane.LastChange().UnixMilli(),
	}
}

func (cr *crewProc) attach(c *conn) {
	cr.mu.Lock()
	defer cr.mu.Unlock()
	if _, ok := cr.subs[c]; ok {
		return
	}
	// A deck gets what crew copy while it's attached, not earlier copies.
	copies, _ := cr.pane.Copied()
	sb := &sub{c: c, wake: make(chan struct{}, 1), quit: make(chan struct{}), copies: copies}
	cr.subs[c] = sb
	sb.wake <- struct{}{} // first frame straight away
	go cr.push(sb)
}

func (cr *crewProc) detach(c *conn) {
	cr.mu.Lock()
	sb, ok := cr.subs[c]
	if ok {
		delete(cr.subs, c)
		close(sb.quit)
	}
	cr.mu.Unlock()
	if ok && sb.mirror {
		cr.refreshMirror()
	}
}

// push sends frames to one deck, at most one per frameInterval.
func (cr *crewProc) push(sb *sub) {
	for {
		select {
		case <-sb.quit:
			return
		case <-sb.wake:
		}
		cr.mu.Lock()
		offset := sb.offset
		cr.mu.Unlock()
		f := cr.frame(offset)
		if n, text := cr.pane.Copied(); n > sb.copies {
			sb.copies, f.Copied = n, text
		}
		if err := sb.c.send(message{Ev: "frame", Crew: cr.spec.Key(), Frame: f}); err != nil {
			return
		}
		time.Sleep(frameInterval)
	}
}

func (cr *crewProc) setOffset(c *conn, offset int) {
	cr.mu.Lock()
	defer cr.mu.Unlock()
	if sb, ok := cr.subs[c]; ok {
		sb.offset = max(offset, 0)
		select {
		case sb.wake <- struct{}{}:
		default:
		}
	}
}

func (cr *crewProc) setMirror(c *conn, on bool) {
	cr.mu.Lock()
	if sb, ok := cr.subs[c]; ok {
		sb.mirror = on
	}
	cr.mu.Unlock()
	cr.refreshMirror()
}

// refreshMirror points the pane's raw mirror at every deck that wants it.
func (cr *crewProc) refreshMirror() {
	cr.mu.Lock()
	var want []*conn
	for c, sb := range cr.subs {
		if sb.mirror {
			want = append(want, c)
		}
	}
	cr.mu.Unlock()
	if len(want) == 0 {
		cr.pane.SetMirror(nil)
		return
	}
	key := cr.spec.Key()
	cr.pane.SetMirror(writerFunc(func(p []byte) (int, error) {
		data := append([]byte(nil), p...)
		for _, c := range want {
			c.send(message{Ev: "raw", Crew: key, Data: data})
		}
		return len(p), nil
	}))
}

type writerFunc func([]byte) (int, error)

func (f writerFunc) Write(p []byte) (int, error) { return f(p) }

// argsForLog shows a command line with long arguments shortened.
func argsForLog(args []string) string {
	out := make([]string, len(args))
	for i, a := range args {
		a = strings.ReplaceAll(a, "\n", "⏎")
		if len(a) > 60 {
			a = a[:57] + "…"
		}
		out[i] = a
	}
	return strings.Join(out, " ")
}

// The web UI runs in the server's own process: these are its way to a
// crew's terminal, as the protocol is a deck's.

// Screen is a running crew's screen now (key is slug/crew).
func (s *Server) Screen(key string) (*Frame, bool) {
	cr, err := s.crew(key)
	if err != nil {
		return nil, false
	}
	return cr.frame(0), true
}

// WatchScreen wakes changes whenever a running crew's screen may have
// changed, until cancel; ended closes when the crew ends.
func (s *Server) WatchScreen(key string) (changes <-chan struct{}, ended <-chan struct{}, cancel func(), ok bool) {
	cr, err := s.crew(key)
	if err != nil {
		return nil, nil, nil, false
	}
	w := make(chan struct{}, 1)
	cr.mu.Lock()
	cr.watchers[w] = true
	cr.mu.Unlock()
	return w, cr.pane.Done(), func() {
		cr.mu.Lock()
		delete(cr.watchers, w)
		cr.mu.Unlock()
	}, true
}

// SendKey types a key into a running crew, encoded for the modes it has
// on, as a deck's keys are.
func (s *Server) SendKey(key string, k uv.KeyPressEvent) error {
	cr, err := s.crew(key)
	if err != nil {
		return err
	}
	cr.pane.SendKey(k)
	return nil
}

// Paste pastes text into a running crew (bracketed, if it asked).
func (s *Server) Paste(key, text string) error {
	cr, err := s.crew(key)
	if err != nil {
		return err
	}
	cr.pane.Paste(text)
	return nil
}

// Stop hangs a running crew up, as a deck's stop does: it's recorded as
// stopped, not as ending by itself.
func (s *Server) Stop(key string) error {
	if a, ok := s.acpCrew(key); ok {
		s.Log.Printf("stop requested for %s", key)
		a.stop(3 * time.Second)
		return nil
	}
	cr, err := s.crew(key)
	if err != nil {
		return err
	}
	s.Log.Printf("stop requested for %s", key)
	cr.stopRequested.Store(true)
	go cr.pane.Close(3 * time.Second)
	return nil
}
