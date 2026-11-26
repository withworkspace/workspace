package web

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/ops"
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

// openSocket is a page's socket on s, from its own origin, signed in.
func openSocket(t *testing.T, s *Server) *socketTalk {
	t.Helper()
	c, _, err := dialSocket(t, s, s.Info().Origin())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.CloseNow() })
	return talkTo(t, c)
}

// followOver is a page's socket on s following topic (as a page showing
// app, for the plugins'), once it's said to be followed.
func followOver(t *testing.T, s *Server, topic string, app ...string) *socketTalk {
	t.Helper()
	st := openSocket(t, s)
	ask := SocketAsk{Op: SocketFollow, Topic: topic}
	if len(app) > 0 {
		ask.App = app[0]
	}
	st.ask(ask)
	if got := st.next(3 * time.Second); got != "following "+topic {
		t.Fatalf("following %s: %q", topic, got)
	}
	return st
}

// socketTalk is a page's side of a socket: what it asks, and what it's
// told next (or "(nothing)" within the time given).
type socketTalk struct {
	t    *testing.T
	c    *websocket.Conn
	said chan SocketSaid
	last SocketSaid // what was told last
	// passed is what nextOf has passed over, by topic.
	passed map[string]int
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

// nextOf is what it's told next of topic, within the time given: what's
// told of other topics meanwhile (a write to a mission is a change to the
// workspace's view too, told in no order) is passed over.
func (st *socketTalk) nextOf(topic string, within time.Duration) string {
	deadline := time.Now().Add(within)
	for {
		left := time.Until(deadline)
		if left <= 0 {
			return "(nothing)"
		}
		got := st.next(left)
		if got == "(nothing)" || got == "(closed)" || st.last.Topic == topic {
			return got
		}
		if st.passed == nil {
			st.passed = map[string]int{}
		}
		st.passed[st.last.Topic]++
	}
}

func (st *socketTalk) next(within time.Duration) string {
	select {
	case m, ok := <-st.said:
		if !ok {
			return "(closed)"
		}
		st.last = m
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
	if got := st.nextOf("mission:"+one.Slug, 3*time.Second); got != "changed mission:"+one.Slug {
		t.Errorf("after a write to the mission: %q", got)
	}
	// The write is a change to the workspace's view too, told before the
	// mission's or after it. One still to come is waited for: told as the
	// workspace is left, it would be taken for one told after.
	if st.passed["workspace"] == 0 {
		st.nextOf("workspace", 3*time.Second)
	}

	st.ask(SocketAsk{Op: SocketLeave, Topic: "workspace"})
	time.Sleep(100 * time.Millisecond)
	fw.set("three")
	if got := st.nextOf("workspace", 500*time.Millisecond); got != "(nothing)" {
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

// A conversation's page follows its own topic: told at once of a turn said
// in it (the records say so), not of another's, and within a second of its
// agent writing its steps as it answers, which the records don't see. So
// nothing need read it again and again while it's answered.
func TestSocket_FollowsAConversation(t *testing.T) {
	root := t.TempDir()
	storetest.Own(t, root)
	ms := mission.Store{Root: root}
	m, _ := ms.Create("One")
	talks := conversation.For(root, m.Slug)
	now := time.Now()
	c, err := talks.OpenTalk("you", "Why?", now)
	if err != nil {
		t.Fatal(err)
	}
	other, _ := talks.OpenTalk("you", "And this?", now)
	s := startTest(t, root, &fakeWorkspace{}, nil)
	conn, _, err := dialSocket(t, s, s.Info().Origin())
	if err != nil {
		t.Fatal(err)
	}
	st := talkTo(t, conn)
	topic := "conversation:" + m.Slug + "/" + c.ID
	st.ask(SocketAsk{Op: SocketFollow, Topic: topic})
	if got := st.next(3 * time.Second); got != "following "+topic {
		t.Fatalf("following: %q", got)
	}

	talks.Say(other.ID, conversation.Turn{By: conversation.You, At: now, Body: "elsewhere"})
	if got := st.next(500 * time.Millisecond); got != "(nothing)" {
		t.Errorf("a turn in another conversation: %q", got)
	}
	talks.Say(c.ID, conversation.Turn{By: conversation.You, At: now, Body: "Go on."})
	if got := st.next(500 * time.Millisecond); got != "changed "+topic {
		t.Errorf("a turn said in it: %q", got)
	}
	// It's told what changed: the turn after the one it had, and the rest
	// as it is now, as of the records' change.
	op := st.last.Conversation
	if op == nil || op.From != 1 || len(op.Turns) != 1 || op.Turns[0].Body != "Go on." || op.Conversation.ID != c.ID || op.Conversation.Turns != nil || op.Version == 0 {
		t.Fatalf("the change: %+v", op)
	}
	// Being answered, its agent's steps are told as its run.
	talks.StartAnswering(c.ID, os.Getpid())
	if got := st.next(500 * time.Millisecond); got != "changed "+topic || st.last.Conversation == nil || !st.last.Conversation.Conversation.Answering || len(st.last.Conversation.Turns) != 0 {
		t.Errorf("answering: %q, %+v", got, st.last.Conversation)
	}
	step := `{"type":"assistant","message":{"content":[{"type":"text","text":"Reading cache.go."}]}}` + "\n"
	os.WriteFile(talks.RunPath(c.ID), []byte(step), 0o644)
	if got := st.next(3 * time.Second); got != "changed "+topic || st.last.Conversation == nil || st.last.Conversation.Run == nil || st.last.Conversation.Run.Said != "Reading cache.go." {
		t.Errorf("its agent's steps: %q, %+v", got, st.last.Conversation)
	}
	// What changes nothing a page sees is told nothing.
	talks.SetGuideGiven(c.ID, conversation.GuideGiven{})
	if got := st.next(500 * time.Millisecond); got != "(nothing)" {
		t.Errorf("a change it doesn't show: %q", got)
	}

	for _, bad := range []string{"conversation:" + m.Slug, "conversation:a/b/c", "conversation:"} {
		st.ask(SocketAsk{Op: SocketFollow, Topic: bad})
		if got := st.next(3 * time.Second); !strings.HasPrefix(got, "refused "+bad) {
			t.Errorf("%q: %q", bad, got)
		}
	}
}

// Told the workspace has changed, every open page asks for its view at
// once: it's built once, when the workspace's hub looks at it, and served
// from that to them all. A write a page makes has it built afresh.
func TestWorkspaceView_BuiltOncePerChange(t *testing.T) {
	root := t.TempDir()
	storetest.Own(t, root)
	ms := mission.Store{Root: root}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{Root: root, Poll: 5 * time.Second, Assets: testApp, listener: ln}
	s.App = testOps(s, ops.Options{})
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	c, _, err := dialSocket(t, s, s.Info().Origin())
	if err != nil {
		t.Fatal(err)
	}
	st := talkTo(t, c)
	st.ask(SocketAsk{Op: SocketFollow, Topic: "workspace"})
	if got := st.next(3 * time.Second); got != "following workspace" {
		t.Fatalf("following: %q", got)
	}
	built := func(key string) int {
		s.views.mu.Lock()
		defer s.views.mu.Unlock()
		return s.views.built[key]
	}
	before := built("workspace")
	ms.Create("Pushed")
	if got := st.next(3 * time.Second); got != "changed workspace" {
		t.Fatalf("after a mission's made: %q", got)
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			res := do(t, s, "GET", "/api/workspace", true, nil)
			var ws Workspace
			json.NewDecoder(res.Body).Decode(&ws)
			res.Body.Close()
			if len(ws.Missions) != 1 {
				t.Errorf("a page was served %d missions", len(ws.Missions))
			}
		})
	}
	wg.Wait()
	if n := built("workspace") - before; n != 1 {
		t.Errorf("built %d times for one change and eight pages, want once", n)
	}
	asks(t, s, "POST", "/api/focus", map[string]any{"pins": []any{}}, nil).Body.Close()
	do(t, s, "GET", "/api/workspace", true, nil).Body.Close()
	if n := built("workspace") - before; n != 2 {
		t.Errorf("after a write: built %d times in all, want twice", n)
	}

	// A mission's page, and what else shows the mission (its pane, its
	// crew's page), ask for it at once when it changes: built once.
	slug := "pushed"
	before = built("mission:" + slug)
	ms.AppendLog(slug, mission.Event{Kind: mission.EventNote, Text: "hi"})
	for range 4 {
		wg.Go(func() {
			res := do(t, s, "GET", "/api/missions/"+slug, true, nil)
			var d MissionDetail
			json.NewDecoder(res.Body).Decode(&d)
			res.Body.Close()
			if d.Slug != slug {
				t.Errorf("a page was served %q", d.Slug)
			}
		})
	}
	wg.Wait()
	if n := built("mission:"+slug) - before; n != 1 {
		t.Errorf("built the mission %d times for four asking at once, want once", n)
	}
	if code := do(t, s, "GET", "/api/missions/nope", true, nil).StatusCode; code != http.StatusNotFound {
		t.Errorf("a mission that isn't: %d", code)
	}
}

// A conversation that isn't a talk (a pass, a cell) is followed as one is,
// and told it's changed with nothing more: what's shown of it is worked out
// of it, so the page reads it again.
func TestSocket_FollowsAConversationThatIsntATalk(t *testing.T) {
	root := t.TempDir()
	storetest.Own(t, root)
	talks := conversation.ForWorkspace(root)
	c, err := talks.OpenCell("p1", "Summarise it.", "", "", "", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	s := startTest(t, root, &fakeWorkspace{}, nil)
	conn, _, err := dialSocket(t, s, s.Info().Origin())
	if err != nil {
		t.Fatal(err)
	}
	st := talkTo(t, conn)
	topic := "conversation:/" + c.ID
	st.ask(SocketAsk{Op: SocketFollow, Topic: topic})
	if got := st.next(3 * time.Second); got != "following "+topic {
		t.Fatalf("following: %q", got)
	}
	talks.Say(c.ID, conversation.Turn{By: conversation.Agent, At: time.Now(), Body: "Done."})
	if got := st.next(3 * time.Second); got != "changed "+topic || st.last.Conversation != nil {
		t.Errorf("a turn said in it: %q, %+v", got, st.last.Conversation)
	}
}

// A document being edited is followed by its editor, and told when its
// file's written (crew committing to it): the editor reads it then.
func TestSocket_FollowsADocument(t *testing.T) {
	s := startTest(t, t.TempDir(), &fakeWorkspace{}, nil)
	path := filepath.Join(t.TempDir(), "adr.md")
	os.WriteFile(path, []byte("# ADR\n"), 0o644)
	s.App = &fakeApp{Workspace: s.App.(*ops.Workspace), doc: func(ref string) (*ops.DocText, error) {
		return &ops.DocText{Ref: ref, Path: path}, nil
	}}
	conn, _, err := dialSocket(t, s, s.Info().Origin())
	if err != nil {
		t.Fatal(err)
	}
	st := talkTo(t, conn)
	topic := "doc:/demo:docs/adr.md"
	st.ask(SocketAsk{Op: SocketFollow, Topic: topic})
	if got := st.next(3 * time.Second); got != "following "+topic {
		t.Fatalf("following: %q", got)
	}
	time.Sleep(20 * time.Millisecond)
	os.WriteFile(path, []byte("# ADR\n\nDecided.\n"), 0o644)
	if got := st.next(3 * time.Second); got != "changed "+topic {
		t.Errorf("its file written: %q", got)
	}
}

// An app's topic names it, and its worktree for one of a worktree's.
func TestAppTopicKey(t *testing.T) {
	for _, c := range []struct {
		key  string
		name string
		p    ops.AppPlace
		ok   bool
	}{
		{"docs", "docs", ops.AtWorkspace, true},
		{"web@demo/feat", "web", ops.AppPlace{Repo: "demo", Worktree: "feat"}, true},
		{"web@demo", "", ops.AppPlace{}, false},
		{"web@demo/a/b", "", ops.AppPlace{}, false},
		{"", "", ops.AppPlace{}, false},
	} {
		name, p, ok := appOfKey(c.key)
		if name != c.name || p != c.p || ok != c.ok {
			t.Errorf("%q: %q %+v %v", c.key, name, p, ok)
		}
		if ok && appTopicKey(name, p) != c.key {
			t.Errorf("%q made again: %q", c.key, appTopicKey(name, p))
		}
	}
}

// While a page follows the workspace, its view is served as the hub keeps
// it: a read builds nothing, however long since it was built, and however
// many there are.
func TestWorkspaceView_ServedAsKeptWhileFollowed(t *testing.T) {
	root := t.TempDir()
	storetest.Own(t, root)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{Root: root, Poll: 50 * time.Millisecond, Assets: testApp, listener: ln}
	s.App = testOps(s, ops.Options{})
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	c, _, err := dialSocket(t, s, s.Info().Origin())
	if err != nil {
		t.Fatal(err)
	}
	st := talkTo(t, c)
	st.ask(SocketAsk{Op: SocketFollow, Topic: "workspace"})
	if got := st.next(3 * time.Second); got != "following workspace" {
		t.Fatalf("following: %q", got)
	}
	built := func() int {
		s.views.mu.Lock()
		defer s.views.mu.Unlock()
		return s.views.built["workspace"]
	}
	before := built()
	version := func(res *http.Response) int64 {
		v, _ := strconv.ParseInt(res.Header.Get(versionHeader), 10, 64)
		return v
	}
	var was int64
	for range 5 {
		time.Sleep(60 * time.Millisecond) // past a poll each time
		res := do(t, s, "GET", "/api/workspace", true, nil)
		res.Body.Close()
		if was = version(res); was == 0 {
			t.Fatal("a read didn't say its version")
		}
	}
	if n := built() - before; n != 0 {
		t.Errorf("reads built the view %d times, want none", n)
	}
	(mission.Store{Root: root}).Create("Pushed")
	if got := st.next(3 * time.Second); got != "changed workspace" {
		t.Fatalf("after a mission's made: %q", got)
	}
	told := st.last.Version
	if told <= was {
		t.Errorf("told it's at version %d, after %d", told, was)
	}
	res := do(t, s, "GET", "/api/workspace", true, nil)
	if v := version(res); v != told {
		t.Errorf("read as of version %d, told %d", v, told)
	}
	var ws Workspace
	json.NewDecoder(res.Body).Decode(&ws)
	res.Body.Close()
	if len(ws.Missions) != 1 {
		t.Errorf("told it changed, a page was served %d missions, want 1", len(ws.Missions))
	}
}

// What the workspace's view is built from says when it moves: a file a
// command writes under .ws (a mission's brief) reaches a page that
// follows the workspace as it's written, not at the view's next sweep.
func TestWorkspaceView_ToldOfAFileAsItsWritten(t *testing.T) {
	root := t.TempDir()
	storetest.Own(t, root)
	ms := mission.Store{Root: root}
	m, err := ms.Create("Pushed")
	if err != nil {
		t.Fatal(err)
	}
	brief := filepath.Join(ms.Dir(), m.Slug, "brief.md")
	os.MkdirAll(filepath.Dir(brief), 0o755)
	os.WriteFile(brief, []byte("## Goal\nFirst.\n"), 0o644)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{Root: root, Poll: 50 * time.Millisecond, Sweep: time.Hour, Assets: testApp, listener: ln}
	s.App = testOps(s, ops.Options{})
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	c, _, err := dialSocket(t, s, s.Info().Origin())
	if err != nil {
		t.Fatal(err)
	}
	st := talkTo(t, c)
	st.ask(SocketAsk{Op: SocketFollow, Topic: "workspace"})
	if got := st.next(3 * time.Second); got != "following workspace" {
		t.Fatalf("following: %q", got)
	}
	time.Sleep(300 * time.Millisecond) // the folders watched, and anything the start moved told
	for st.next(100*time.Millisecond) != "(nothing)" {
	}
	os.WriteFile(brief, []byte("## Goal\nSecond, and longer.\n"), 0o644)
	if got := st.next(3 * time.Second); got != "changed workspace" {
		t.Fatalf("after the brief's written: %q", got)
	}
}
