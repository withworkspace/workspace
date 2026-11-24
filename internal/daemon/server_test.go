package daemon

import (
	"errors"
	"fmt"
	"github.com/withworkspace/workspace/internal/store/storetest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/withworkspace/workspace/internal/crew"
	"github.com/withworkspace/workspace/internal/mission"
)

func startServer(t *testing.T, idle time.Duration) (*Server, string) {
	t.Helper()
	root := t.TempDir()
	storetest.Own(t, root)
	s := &Server{Root: root, IdleTimeout: idle}
	if err := s.Listen(); err != nil {
		t.Fatal(err)
	}
	go s.Serve()
	t.Cleanup(func() { s.Shutdown("test over") })
	return s, root
}

// crewOn creates a mission with one crew record and a launch spec for it.
func crewOn(t *testing.T, root string, script string) Spec {
	t.Helper()
	store := mission.Store{Root: root}
	m, err := store.Create("Served")
	if err != nil {
		t.Fatal(err)
	}
	c, _ := store.StartCrew(m.Slug, "shell", "")
	return Spec{
		Slug: m.Slug, CrewID: c.ID, Harness: "shell", Capsule: "fe/x",
		Args: []string{"sh", "-c", script}, Dir: root,
		Env:   append(os.Environ(), "PS1=$ ", "ENV="),
		Width: 40, Height: 6,
	}
}

func dialT(t *testing.T, root string) *Client {
	t.Helper()
	c, err := Dial(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

func waitText(t *testing.T, term *Term, want string) {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for !strings.Contains(term.Text(), want) {
		select {
		case <-term.Updates():
		case <-time.After(20 * time.Millisecond):
		case <-deadline:
			t.Fatalf("never saw %q; screen:\n%s", want, term.Text())
		}
	}
}

func typeKeys(term *Term, s string) {
	for _, r := range s {
		term.SendKey(uv.KeyPressEvent{Code: r, Text: string(r)})
	}
	term.SendKey(uv.KeyPressEvent{Code: uv.KeyEnter})
}

func TestServer_LaunchTypeAndOrder(t *testing.T) {
	_, root := startServer(t, time.Minute)
	spec := crewOn(t, root, `exec sh -i`)
	c := dialT(t, root)

	term, err := c.Launch(spec)
	if err != nil {
		t.Fatal(err)
	}
	if term.Pid() == 0 {
		t.Error("no pid")
	}
	waitText(t, term, "$") // type once the shell is ready, as a person would
	// Many quick keystrokes must arrive in order.
	typeKeys(term, "echo abcdefghijklmnopqrstuvwxyz0123456789")
	waitText(t, term, "\nabcdefghijklmnopqrstuvwxyz0123456789")

	list, err := c.List()
	if err != nil || len(list) != 1 || list[0].Spec.Key() != spec.Key() || list[0].Pid != term.Pid() {
		t.Errorf("list = %+v, %v", list, err)
	}
}

func TestServer_ReattachAfterDeckLeaves(t *testing.T) {
	_, root := startServer(t, time.Minute)
	spec := crewOn(t, root, `exec sh -i`)

	first, _ := Dial(root)
	term, err := first.Launch(spec)
	if err != nil {
		t.Fatal(err)
	}
	waitText(t, term, "$")
	typeKeys(term, "echo before-detach")
	waitText(t, term, "\nbefore-detach")
	first.Close() // the deck quits, detaching

	second := dialT(t, root)
	list, _ := second.List()
	if len(list) != 1 {
		t.Fatalf("crew didn't survive the deck leaving: %+v", list)
	}
	again, err := second.Attach(list[0])
	if err != nil {
		t.Fatal(err)
	}
	waitText(t, again, "before-detach") // screen state kept
	typeKeys(again, "echo after-reattach")
	waitText(t, again, "\nafter-reattach")
}

func TestServer_StopRecordsEndOnMission(t *testing.T) {
	_, root := startServer(t, time.Minute)
	spec := crewOn(t, root, `while :; do sleep 0.05; done`)
	c := dialT(t, root)
	term, _ := c.Launch(spec)

	term.Close(2 * time.Second)
	if !term.Exited() || term.ExitSummary() != "hangup" {
		t.Fatalf("exited=%v summary=%q", term.Exited(), term.ExitSummary())
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		m, _ := mission.Store{Root: root}.Load(spec.Slug)
		if m.Crew[0].Ended != nil {
			if m.Crew[0].Exit != "hangup" {
				t.Errorf("recorded exit = %q", m.Crew[0].Exit)
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Error("server didn't record the crew's end")
}

func TestServer_ScrollbackOverTheWire(t *testing.T) {
	_, root := startServer(t, time.Minute)
	spec := crewOn(t, root, `i=1; while [ $i -le 40 ]; do printf 'row%02d\r\n' $i; i=$((i+1)); done; sleep 10`)
	c := dialT(t, root)
	term, _ := c.Launch(spec)
	waitText(t, term, "row40")
	if term.ScrollbackLen() < 30 {
		t.Fatalf("scrollback = %d", term.ScrollbackLen())
	}
	term.RenderScrolled(1000) // ask for the top
	deadline := time.Now().Add(3 * time.Second)
	for !strings.Contains(term.RenderScrolled(1000), "row01") {
		if time.Now().After(deadline) {
			t.Fatalf("top of history never arrived:\n%s", term.RenderScrolled(1000))
		}
		time.Sleep(20 * time.Millisecond)
	}
}

type lockedBuf struct {
	mu sync.Mutex
	b  strings.Builder
}

func (l *lockedBuf) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *lockedBuf) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

func TestServer_MirrorAndRaw(t *testing.T) {
	_, root := startServer(t, time.Minute)
	spec := crewOn(t, root, `stty raw -echo; printf ready; c=$(dd bs=1 count=3 2>/dev/null); stty sane; printf '\r\nraw:%s\033[1m!' "$c"; sleep 10`)
	c := dialT(t, root)
	term, _ := c.Launch(spec)
	waitText(t, term, "ready")

	var buf lockedBuf
	term.SetMirror(&buf)
	term.WriteRaw([]byte("xyz"))
	deadline := time.Now().Add(3 * time.Second)
	for !strings.Contains(buf.String(), "raw:xyz\x1b[1m!") {
		if time.Now().After(deadline) {
			t.Fatalf("mirror got %q", buf.String())
		}
		time.Sleep(20 * time.Millisecond)
	}
	term.SetMirror(nil)
}

func TestServer_IdleShutdown(t *testing.T) {
	s, root := startServer(t, 200*time.Millisecond)
	select {
	case <-s.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("idle server didn't shut down")
	}
	if _, err := os.Stat(SocketPath(root)); !os.IsNotExist(err) {
		t.Errorf("socket left behind: %v", err)
	}
}

// A server with no IdleTimeout, the default, never idles out: with no
// crew, connection or hold, and after a connection and a hold have come
// and gone, it arms no idle timer and is still up well past the opt-in's
// wait (TestServer_IdleShutdown, 200ms). It goes when it's closed, as ws
// daemon stop closes it.
func TestServer_NoIdleTimeoutStaysUpUntilClosed(t *testing.T) {
	s, root := startServer(t, 0)
	c, err := Dial(root)
	if err != nil {
		t.Fatal(err)
	}
	c.Close()
	s.Hold()()
	for deadline := time.Now().Add(3 * time.Second); ; time.Sleep(10 * time.Millisecond) {
		s.mu.Lock()
		busy, armed := s.busyLocked(), s.idle != nil
		s.mu.Unlock()
		if armed {
			t.Fatal("idle timer armed with no idle timeout")
		}
		if !busy {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the connection never went")
		}
	}
	select {
	case <-s.Done():
		t.Fatal("server with no idle timeout shut down idle")
	case <-time.After(time.Second):
	}

	if err := dialT(t, root).Shutdown(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-s.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("server didn't shut down when closed")
	}
	if _, err := os.Stat(SocketPath(root)); !os.IsNotExist(err) {
		t.Errorf("socket left behind: %v", err)
	}
}

// A daemon says its idle timeout when a client connects, 0 for none, so
// ws daemon status can say when it exits.
func TestServer_HelloSaysIdleTimeout(t *testing.T) {
	for _, idle := range []time.Duration{0, time.Minute} {
		_, root := startServer(t, idle)
		if got, ok := dialT(t, root).IdleTimeout(); !ok || got != idle {
			t.Errorf("server with IdleTimeout %v says %v (said: %v)", idle, got, ok)
		}
	}
}

// A daemon says where its web UI is when a client connects; one
// without (an older ws's) says nothing, which reads as 0.
func TestServer_HelloSaysWebPort(t *testing.T) {
	s, root := startServer(t, time.Minute)
	if c := dialT(t, root); c.WebPort() != 0 {
		t.Errorf("no web UI: port %d", c.WebPort())
	}
	s.WebPort = 47123 // set before Serve in real use
	if c := dialT(t, root); c.WebPort() != 47123 {
		t.Errorf("web port %d", c.WebPort())
	}
}

// An open web UI holds the server up like an attached deck; releasing the
// hold (twice is harmless) lets it idle out.
func TestServer_StaysUpWhileHeld(t *testing.T) {
	s, _ := startServer(t, 200*time.Millisecond)
	release := s.Hold()
	select {
	case <-s.Done():
		t.Fatal("server shut down while held")
	case <-time.After(600 * time.Millisecond):
	}
	release()
	release()
	select {
	case <-s.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("released server didn't idle out")
	}
}

func TestServer_StaysUpWhileCrewRun(t *testing.T) {
	s, root := startServer(t, 200*time.Millisecond)
	spec := crewOn(t, root, `sleep 10`)
	c, _ := Dial(root)
	if _, err := c.Launch(spec); err != nil {
		t.Fatal(err)
	}
	c.Close()
	select {
	case <-s.Done():
		t.Fatal("server shut down with crew running")
	case <-time.After(600 * time.Millisecond):
	}
}

func TestServer_RefusesSecondListener(t *testing.T) {
	_, root := startServer(t, time.Minute)
	other := &Server{Root: root}
	if err := other.Listen(); err == nil || !strings.Contains(err.Error(), "already listening") {
		t.Errorf("err = %v", err)
	}
}

func TestServer_DuplicateCrewRefused(t *testing.T) {
	_, root := startServer(t, time.Minute)
	spec := crewOn(t, root, `sleep 10`)
	c := dialT(t, root)
	if _, err := c.Launch(spec); err != nil {
		t.Fatal(err)
	}
	// Another deck launching the same crew is refused by the server (this
	// client refuses before asking: see TestClient_FollowsCrewOnce).
	if _, err := dialT(t, root).Launch(spec); err == nil || !strings.Contains(err.Error(), "already running") {
		t.Errorf("err = %v", err)
	}
}

func TestSocketPath_LongRootFallsBack(t *testing.T) {
	short := SocketPath("/w")
	if short != filepath.Join("/w", ".ws", "daemon.sock") {
		t.Errorf("short = %s", short)
	}
	long := SocketPath("/" + strings.Repeat("x", 200))
	if len(long) > maxSocketPath || !strings.HasPrefix(filepath.Base(long), "ws-daemon-") {
		t.Errorf("long = %s", long)
	}
	if SocketPath("/"+strings.Repeat("x", 200)) != long {
		t.Error("fallback isn't stable")
	}
}

func TestServer_CrewEndHookAndTranscript(t *testing.T) {
	s, root := startServer(t, time.Minute)
	type end struct {
		exit    string
		stopped bool
	}
	ends := make(chan end, 4)
	s.OnCrewEnded = func(spec Spec, exit string, stopped bool) { ends <- end{exit, stopped} }

	spec := crewOn(t, root, `printf 'hello-transcript\r\n'; sleep 0.3; exit 2`)
	c := dialT(t, root)
	term, _ := c.Launch(spec)
	waitText(t, term, "hello-transcript")
	if tr, err := c.Transcript(spec.Key()); err != nil || !strings.Contains(tr, "hello-transcript") {
		t.Errorf("live transcript = %q, %v", tr, err)
	}
	select {
	case e := <-ends:
		if e.exit != "exit 2" || e.stopped {
			t.Errorf("end = %+v", e)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no end hook")
	}
	data, _ := os.ReadFile(mission.Store{Root: root}.TranscriptPath(spec.Slug, spec.CrewID))
	if !strings.Contains(string(data), "hello-transcript") {
		t.Errorf("saved transcript = %q", data)
	}

	spec2 := crewOn2(t, root, `sleep 30`)
	term2, _ := c.Launch(spec2)
	term2.Close(2 * time.Second)
	select {
	case e := <-ends:
		if !e.stopped {
			t.Errorf("a stopped crew reported stopped=false: %+v", e)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no end hook for stopped crew")
	}
}

// Two launches of the same crew at once start one: the other is told
// they're running, though the first hasn't finished starting them.
func TestServer_LaunchingTwiceStartsOne(t *testing.T) {
	s, root := startServer(t, 0)
	spec := crewOn(t, root, `while :; do sleep 0.05; done`)
	errs := make(chan error, 8)
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := s.Launch(spec); errs <- err }()
	}
	wg.Wait()
	close(errs)
	started := 0
	for err := range errs {
		switch {
		case err == nil:
			started++
		case !errors.Is(err, crew.ErrRunning):
			t.Errorf("a launch failed otherwise: %v", err)
		}
	}
	if started != 1 {
		t.Errorf("%d started", started)
	}
	s.mu.Lock()
	n := len(s.starting)
	s.mu.Unlock()
	if n != 0 {
		t.Errorf("%d reservations left", n)
	}
}

// Crew the daemon shuts down under are recorded as interrupted, for the
// next daemon to resume; crew someone asked to stop aren't, even when the
// daemon closes before they've gone.
func TestServer_ShutdownRecordsCrewInterrupted(t *testing.T) {
	s, root := startServer(t, 0)
	working := crewOn(t, root, `while :; do sleep 0.05; done`)
	stopped := crewOn2(t, root, `trap '' HUP; while :; do sleep 0.05; done`)
	for _, spec := range []Spec{working, stopped} {
		if _, err := s.Launch(spec); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Stop(stopped.Key()); err != nil {
		t.Fatal(err)
	}
	s.Shutdown("restart")

	m, err := mission.Store{Root: root}.Load(working.Slug)
	if err != nil {
		t.Fatal(err)
	}
	w, _ := m.CrewByID(working.CrewID)
	if w.Ended == nil || !w.Stopped || !w.Interrupted || !strings.HasPrefix(w.Exit, "stopped: ") {
		t.Errorf("crew the daemon shut down under: ended %v, stopped %v, interrupted %v, exit %q", w.Ended != nil, w.Stopped, w.Interrupted, w.Exit)
	}
	st, _ := m.CrewByID(stopped.CrewID)
	if st.Ended == nil || !st.Stopped || st.Interrupted {
		t.Errorf("crew asked to stop: ended %v, stopped %v, interrupted %v", st.Ended != nil, st.Stopped, st.Interrupted)
	}
}

func crewOn2(t *testing.T, root, script string) Spec {
	t.Helper()
	store := mission.Store{Root: root}
	c, _ := store.StartCrew("served", "shell", "")
	return Spec{Slug: "served", CrewID: c.ID, Harness: "shell", Args: []string{"sh", "-c", script}, Dir: root, Env: os.Environ(), Width: 40, Height: 6}
}

func TestClient_FollowsCrewOnce(t *testing.T) {
	_, root := startServer(t, time.Minute)
	spec := crewOn(t, root, `sleep 10`)
	c := dialT(t, root)
	term, err := c.Launch(spec)
	if err != nil {
		t.Fatal(err)
	}
	// A second Term for the same crew (an adoption racing the launch) is
	// refused, and the first keeps getting the crew's frames.
	list, _ := c.List()
	if _, err := c.Attach(list[0]); !errors.Is(err, ErrFollowing) {
		t.Fatalf("second attach: %v", err)
	}
	if _, err := c.Launch(spec); !errors.Is(err, ErrFollowing) {
		t.Fatalf("second launch: %v", err)
	}
	term.Close(2 * time.Second)
	if !term.Exited() {
		t.Error("the first Term lost the crew's exit")
	}
	// Another client can still follow it.
	other := dialT(t, root)
	spec2 := crewOn2(t, root, `sleep 10`)
	c.Launch(spec2)
	list, _ = other.List()
	if len(list) != 1 {
		t.Fatalf("list = %+v", list)
	}
	if _, err := other.Attach(list[0]); err != nil {
		t.Errorf("another client attaching: %v", err)
	}
}

// copyText has the crew's shell copy text to the clipboard (OSC 52), then
// print marker.
func copyText(term *Term, base64Text, marker string) {
	typeKeys(term, `printf '\033]52;c;`+base64Text+`\a'; echo `+marker[:2]+"''"+marker[2:])
}

func TestServer_ClipboardCopiesReachAttachedDecks(t *testing.T) {
	_, root := startServer(t, time.Minute)
	spec := crewOn(t, root, `exec sh -i`)
	first := dialT(t, root)
	term, err := first.Launch(spec)
	if err != nil {
		t.Fatal(err)
	}
	waitText(t, term, "$")
	copyText(term, "aGVsbG8=", "copied-one")
	waitText(t, term, "\ncopied-one")
	if text, ok := term.TakeCopied(); !ok || text != "hello" {
		t.Errorf("copy = %q, %v; want hello", text, ok)
	}
	if _, ok := term.TakeCopied(); ok {
		t.Error("the same copy was taken twice")
	}

	// A deck attaching later doesn't get copies made before it came.
	second := dialT(t, root)
	list, _ := second.List()
	again, err := second.Attach(list[0])
	if err != nil {
		t.Fatal(err)
	}
	waitText(t, again, "copied-one")
	if text, ok := again.TakeCopied(); ok {
		t.Errorf("a later deck got an earlier copy: %q", text)
	}

	// Both get the next one.
	copyText(term, "d29ybGQ=", "copied-two")
	waitText(t, term, "\ncopied-two")
	waitText(t, again, "\ncopied-two")
	for name, tm := range map[string]*Term{"first": term, "second": again} {
		if text, ok := tm.TakeCopied(); !ok || text != "world" {
			t.Errorf("%s deck: copy = %q, %v; want world", name, text, ok)
		}
	}
}

// The web UI, in the server's own process, reads a crew's screen, hears
// when it changes, and types and pastes into it.
func TestServer_InProcessScreen(t *testing.T) {
	s, root := startServer(t, time.Minute)
	spec := crewOn(t, root, `exec sh -i`)
	if _, err := dialT(t, root).Launch(spec); err != nil {
		t.Fatal(err)
	}
	key := spec.Key()
	changes, ended, cancel, ok := s.WatchScreen(key)
	if !ok {
		t.Fatal("can't watch a running crew")
	}
	defer cancel()
	waitFor := func(what string, seen func(screen string) bool) *Frame {
		t.Helper()
		deadline := time.After(5 * time.Second)
		for {
			f, ok := s.Screen(key)
			if ok && seen(f.Screen) {
				return f
			}
			select {
			case <-changes:
			case <-time.After(50 * time.Millisecond):
			case <-deadline:
				t.Fatalf("never saw %s; screen:\n%s", what, f.Screen)
			}
		}
	}
	waitScreen := func(want string) *Frame {
		t.Helper()
		return waitFor(fmt.Sprintf("%q", want), func(screen string) bool { return strings.Contains(screen, want) })
	}
	// waitPrompts waits for the shell's nth prompt: it's read what came
	// before, and what's sent next is echoed after the prompt, not before.
	waitPrompts := func(n int) {
		t.Helper()
		waitFor(fmt.Sprintf("%d prompts", n), func(screen string) bool { return strings.Count(screen, "$") >= n })
	}
	f := waitScreen("$")
	if f.Width != 40 || f.Height != 6 {
		t.Errorf("frame %dx%d", f.Width, f.Height)
	}
	for _, r := range "echo typed" {
		s.SendKey(key, uv.KeyPressEvent{Code: r, Text: string(r)})
	}
	s.SendKey(key, uv.KeyPressEvent{Code: uv.KeyEnter})
	waitScreen("\ntyped")
	waitPrompts(2)
	s.Paste(key, "echo pasted\r")
	waitScreen("\npasted")

	if _, ok := s.Screen("nope/c9"); ok {
		t.Error("a screen for crew that isn't running")
	}
	if err := s.SendKey("nope/c9", uv.KeyPressEvent{Code: 'x'}); err == nil {
		t.Error("typed into crew that isn't running")
	}
	s.SendKey(key, uv.KeyPressEvent{Code: 'd', Mod: uv.ModCtrl})
	select {
	case <-ended:
	case <-time.After(5 * time.Second):
		t.Error("ended didn't close when the crew exited")
	}
}

// A test binary never starts itself as a daemon: started so, it would
// run its tests, and a test reaching here again would start another.
func TestConnect_NeverSpawnsFromATestBinary(t *testing.T) {
	if _, err := Connect(t.TempDir()); err == nil || !strings.Contains(err.Error(), "a test binary doesn't start daemons") {
		t.Errorf("connect with no server: %v", err)
	}
}
