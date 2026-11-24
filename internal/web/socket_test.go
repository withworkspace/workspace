package web

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/store/storetest"
)

// dialSocket opens a page's socket on s, as a page of its own origin
// would (or of origin, given), signed in.
func dialSocket(t *testing.T, s *Server, origin string) (*websocket.Conn, *http.Response, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	h := http.Header{}
	h.Set("Origin", origin)
	h.Set("Cookie", s.cookieName()+"="+s.Info().Token)
	url := "ws" + strings.TrimPrefix(s.Info().Origin(), "http") + "/api/socket"
	return websocket.Dial(ctx, url, &websocket.DialOptions{HTTPHeader: h})
}

// socketTalk is a page's side of a socket: what it asks, and what it's
// told next (or "(nothing)" within the time given).
type socketTalk struct {
	t    *testing.T
	c    *websocket.Conn
	said chan SocketSaid
}

func talkTo(t *testing.T, c *websocket.Conn) *socketTalk {
	st := &socketTalk{t: t, c: c, said: make(chan SocketSaid, 32)}
	go func() {
		for {
			_, data, err := c.Read(context.Background())
			if err != nil {
				close(st.said)
				return
			}
			var m SocketSaid
			json.Unmarshal(data, &m)
			st.said <- m
		}
	}()
	return st
}

func (st *socketTalk) ask(a SocketAsk) {
	st.t.Helper()
	data, _ := json.Marshal(a)
	if err := st.c.Write(context.Background(), websocket.MessageText, data); err != nil {
		st.t.Fatal(err)
	}
}

func (st *socketTalk) next(within time.Duration) string {
	select {
	case m, ok := <-st.said:
		if !ok {
			return "(closed)"
		}
		s := string(m.Kind) + " " + m.Topic
		if m.Error != "" {
			s += ": " + m.Error
		}
		return s
	case <-time.After(within):
		return "(nothing)"
	}
}

// A page follows every topic it shows over one socket: each is said to be
// followed (the page fetches then), changes to each are told by topic, one
// it leaves is told no more, and one that can't be followed is refused. An
// open socket holds the daemon up, until it's closed.
func TestSocket(t *testing.T) {
	root := t.TempDir()
	storetest.Own(t, root)
	ms := mission.Store{Root: root}
	one, _ := ms.Create("One")
	var held atomic.Int32
	hold := func() func() {
		held.Add(1)
		return func() { held.Add(-1) }
	}
	fw := &fakeWorkspace{title: "one"}
	s := startTest(t, root, fw, hold)
	c, _, err := dialSocket(t, s, s.Info().Origin())
	if err != nil {
		t.Fatal(err)
	}
	st := talkTo(t, c)

	st.ask(SocketAsk{Op: SocketFollow, Topic: "workspace"})
	if got := st.next(3 * time.Second); got != "following workspace" {
		t.Fatalf("following the workspace: %q", got)
	}
	st.ask(SocketAsk{Op: SocketFollow, Topic: "mission:" + one.Slug})
	if got := st.next(3 * time.Second); got != "following mission:"+one.Slug {
		t.Fatalf("following a mission: %q", got)
	}
	st.ask(SocketAsk{Op: SocketFollow, Topic: "nope"})
	if got := st.next(3 * time.Second); got != "refused nope: no such topic: nope" {
		t.Errorf("a topic that isn't: %q", got)
	}
	if held.Load() != 1 {
		t.Errorf("held %d, want 1 for the socket", held.Load())
	}

	fw.set("two")
	if got := st.next(3 * time.Second); got != "changed workspace" {
		t.Errorf("after the workspace changed: %q", got)
	}
	ms.AppendLog(one.Slug, mission.Event{Kind: mission.EventNote, Text: "hi"})
	if got := st.next(3 * time.Second); got != "changed mission:"+one.Slug {
		t.Errorf("after a write to the mission: %q", got)
	}

	st.ask(SocketAsk{Op: SocketLeave, Topic: "workspace"})
	time.Sleep(100 * time.Millisecond)
	fw.set("three")
	if got := st.next(500 * time.Millisecond); got != "(nothing)" {
		t.Errorf("a topic left: %q", got)
	}

	c.Close(websocket.StatusNormalClosure, "")
	for end := time.Now().Add(3 * time.Second); held.Load() != 0 && time.Now().Before(end); time.Sleep(10 * time.Millisecond) {
	}
	if held.Load() != 0 {
		t.Error("closing the socket didn't release the daemon")
	}
}

// Another site's page can't open a socket with our cookie: its Origin
// isn't ours.
func TestSocket_RefusesAnotherOrigin(t *testing.T) {
	s := startTest(t, t.TempDir(), &fakeWorkspace{}, nil)
	if _, res, err := dialSocket(t, s, "https://elsewhere.example"); err == nil {
		t.Fatal("another origin's socket was opened")
	} else if res == nil || res.StatusCode != http.StatusForbidden {
		t.Errorf("another origin: %v (%v)", res, err)
	}
}
