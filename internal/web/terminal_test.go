package web

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/ops"
)

type fakeTerminals struct {
	ops.NoCrew
	mu      sync.Mutex
	text    string
	changes chan struct{}
	ended   chan struct{}
	typed   []string
}

func (f *fakeTerminals) Screen(slug, crew string) (ops.Screen, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if slug != "m" || crew != "c1" {
		return ops.Screen{}, false
	}
	return ops.Screen{Text: f.text, Width: 40, Height: 6, CursorX: 2, CursorVisible: true}, true
}

func (f *fakeTerminals) Watch(slug, crew string) (<-chan struct{}, <-chan struct{}, func(), bool) {
	if slug != "m" || crew != "c1" {
		return nil, nil, nil, false
	}
	return f.changes, f.ended, func() {}, true
}

func (f *fakeTerminals) Key(slug, crew string, k ops.TermKey) error {
	if crew != "c1" {
		return errors.New("not running")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.typed = append(f.typed, k.Key)
	return nil
}

func (f *fakeTerminals) Paste(slug, crew, text string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.typed = append(f.typed, "paste:"+text)
	return nil
}

// A page streams a running crew's screen, each change and its end, and
// types and pastes into it, signed in and from its own origin only.
func TestServer_CrewTerminal(t *testing.T) {
	s := startTest(t, t.TempDir(), &fakeWorkspace{}, nil)
	ft := &fakeTerminals{text: "$ ", changes: make(chan struct{}, 1), ended: make(chan struct{})}
	fake(s).crew = ft

	res := do(t, s, "GET", "/api/missions/m/crew/c1/screen", true, nil)
	if res.StatusCode != 200 || res.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatalf("stream: %d %s", res.StatusCode, res.Header.Get("Content-Type"))
	}
	events := bufio.NewReader(res.Body)
	next := func() (string, Screen) {
		t.Helper()
		var name string
		var sc Screen
		for {
			line, err := events.ReadString('\n')
			if err != nil {
				t.Fatalf("stream ended: %v", err)
			}
			line = strings.TrimRight(line, "\n")
			switch {
			case strings.HasPrefix(line, "event: "):
				name = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "data: "):
				json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &sc)
			case line == "" && name != "":
				return name, sc
			}
		}
	}
	if name, sc := next(); name != "screen" || sc.Text != "$ " || sc.Width != 40 || !sc.CursorVisible {
		t.Errorf("first: %s %+v", name, sc)
	}
	ft.mu.Lock()
	ft.text = "$ echo hi\nhi"
	ft.mu.Unlock()
	ft.changes <- struct{}{}
	if name, sc := next(); name != "screen" || !strings.Contains(sc.Text, "hi\nhi") {
		t.Errorf("change: %s %+v", name, sc)
	}
	close(ft.ended)
	if name, _ := next(); name != "exit" {
		t.Errorf("end: %s", name)
	}

	input := func(in TermInput, signedIn bool, origin string) *http.Response {
		data, _ := json.Marshal(in)
		return do(t, s, "POST", "/api/missions/m/crew/c1/input", signedIn, func(r *http.Request) {
			r.Body = io.NopCloser(bytes.NewReader(data))
			r.ContentLength = int64(len(data))
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("Origin", origin)
		})
	}
	if r := input(TermInput{Key: &TermKey{Key: "Enter"}}, true, s.Info().Origin()); r.StatusCode != 200 {
		t.Errorf("key: %d", r.StatusCode)
	}
	if r := input(TermInput{Paste: "ls\r"}, true, s.Info().Origin()); r.StatusCode != 200 {
		t.Errorf("paste: %d", r.StatusCode)
	}
	if r := input(TermInput{Key: &TermKey{Key: "x"}}, true, "https://evil.example"); r.StatusCode != http.StatusForbidden {
		t.Errorf("another origin typed: %d", r.StatusCode)
	}
	if r := input(TermInput{Key: &TermKey{Key: "x"}}, false, s.Info().Origin()); r.StatusCode != http.StatusUnauthorized {
		t.Errorf("typed without signing in: %d", r.StatusCode)
	}
	if strings.Join(ft.typed, " ") != "Enter paste:ls\r" {
		t.Errorf("typed %q", ft.typed)
	}
	if r := do(t, s, "GET", "/api/missions/m/crew/c9/screen", true, nil); r.StatusCode != http.StatusNotFound {
		t.Errorf("crew not running: %d", r.StatusCode)
	}
	if r := do(t, s, "GET", "/api/missions/m/crew/c1/screen", false, nil); r.StatusCode != http.StatusUnauthorized {
		t.Errorf("watched without signing in: %d", r.StatusCode)
	}
}

// A page follows a running crew's screen over its socket: the screen as
// it is, as soon as it's followed, then each change, then its end. Crew
// that aren't running are refused.
func TestSocket_FollowsACrewsScreen(t *testing.T) {
	s := startTest(t, t.TempDir(), &fakeWorkspace{}, nil)
	ft := &fakeTerminals{text: "$ ", changes: make(chan struct{}, 1), ended: make(chan struct{})}
	fake(s).crew = ft
	conn, _, err := dialSocket(t, s, s.Info().Origin())
	if err != nil {
		t.Fatal(err)
	}
	st := talkTo(t, conn)
	st.ask(SocketAsk{Op: SocketFollow, Topic: "screen:m/c2"})
	if got := st.next(3 * time.Second); got != "refused screen:m/c2: c2 isn't running" {
		t.Errorf("crew not running: %q", got)
	}
	st.ask(SocketAsk{Op: SocketFollow, Topic: "screen:m/c1"})
	if got := st.next(3 * time.Second); got != "following screen:m/c1" {
		t.Fatalf("following: %q", got)
	}
	if got := st.next(3 * time.Second); got != "changed screen:m/c1" || st.last.Screen == nil || st.last.Screen.Screen == nil || st.last.Screen.Screen.Text != "$ " {
		t.Fatalf("its screen: %q, %+v", got, st.last.Screen)
	}
	ft.mu.Lock()
	ft.text = "$ ls"
	ft.mu.Unlock()
	ft.changes <- struct{}{}
	if got := st.next(3 * time.Second); got != "changed screen:m/c1" || st.last.Screen.Screen == nil || st.last.Screen.Screen.Text != "$ ls" {
		t.Fatalf("a change: %q, %+v", got, st.last.Screen)
	}
	close(ft.ended)
	if got := st.next(3 * time.Second); got != "changed screen:m/c1" || !st.last.Screen.Exit {
		t.Errorf("its end: %q, %+v", got, st.last.Screen)
	}
}
