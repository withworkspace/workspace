package web

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/doc"
	"github.com/withworkspace/workspace/internal/job"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/ops"
)

// docServer serves a mission "m" with a proposal to comment on, whose text
// the test can change, a review artifact and a Go file, which aren't
// documents to comment on.
func docServer(t *testing.T) (*Server, *string) {
	t.Helper()
	s := startTest(t, t.TempDir(), &fakeWorkspace{}, nil)
	if _, err := (mission.Store{Root: s.Root}).Create("M"); err != nil {
		t.Fatal(err)
	}
	text := "# Tokens\n\nWe cache tokens per user.\n"
	fake(s).artifact = func(slug, ref string) (*ops.DocText, error) {
		if slug != "m" {
			return nil, ErrNotFound
		}
		switch ref {
		case "records/tokens:proposal.md":
			return &ops.DocText{Ref: ref, Kind: "proposal", Label: "Tokens", Text: text, Markdown: true}, nil
		case "mission:reviews/r1.md":
			return &ops.DocText{Ref: ref, Kind: "review", Text: "# Review", Markdown: true}, nil
		case "api:main.go":
			return &ops.DocText{Ref: ref, Kind: "file", Text: "package main"}, nil
		}
		return nil, ErrNotFound
	}
	return s, &text
}

func threadWrite(t *testing.T, s *Server, method, path string, v any) (*http.Response, ThreadState) {
	t.Helper()
	data, _ := json.Marshal(v)
	res := do(t, s, method, path, true, func(r *http.Request) {
		r.Body = io.NopCloser(bytes.NewReader(data))
		r.ContentLength = int64(len(data))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Origin", s.Info().Origin())
	})
	var st ThreadState
	if res.StatusCode == 200 {
		json.NewDecoder(res.Body).Decode(&st)
	}
	return res, st
}

func TestThreadsAPI(t *testing.T) {
	s, text := docServer(t)
	s.clock = func() time.Time { return time.Date(2026, 11, 9, 10, 0, 0, 0, time.UTC) }
	api := "/api/missions/m/threads"
	q := "?ref=" + url.QueryEscape("records/tokens:proposal.md")
	v := doc.Version([]byte(*text))

	// The artifact says its version, and that it can be commented on.
	var a ArtifactContent
	json.NewDecoder(do(t, s, "GET", "/api/missions/m/artifact"+q, true, nil).Body).Decode(&a)
	if a.Version != v || !a.Annotatable {
		t.Errorf("artifact = %+v", a)
	}
	json.NewDecoder(do(t, s, "GET", "/api/missions/m/artifact?ref=mission%3Areviews%2Fr1.md", true, nil).Body).Decode(&a)
	if a.Annotatable {
		t.Error("a review isn't a document to comment on")
	}

	res, st := threadWrite(t, s, "GET", api+q, nil)
	if res.StatusCode != 200 || st.Version != v || len(st.Threads) != 0 {
		t.Fatalf("empty: %d %+v", res.StatusCode, st)
	}
	for _, bad := range []string{"", "?ref=api%3Amain.go", "?ref=mission%3Areviews%2Fr1.md", "?ref=nope%3Ax.md"} {
		if res := do(t, s, "GET", api+bad, true, nil); res.StatusCode == 200 {
			t.Errorf("%q: %d", bad, res.StatusCode)
		}
	}

	var asked []string
	answersInto(s, &asked, job.Thread)
	open := map[string]any{"version": v, "line": 3, "quote": map[string]string{"exact": "per user"}, "kind": "ask", "body": "Why per user?"}
	res, st = threadWrite(t, s, "POST", api+q, open)
	if res.StatusCode != 200 || len(st.Threads) != 1 {
		t.Fatalf("open: %d %+v", res.StatusCode, st)
	}
	th := st.Threads[0]
	if th.ID != "t1" || th.Kind != "ask" || th.Anchor.Path != "proposal.md" || th.Anchor.Excerpt[0] != "We cache tokens per user." || th.Turns[0].Body != "Why per user?" {
		t.Errorf("t1 = %+v", th)
	}
	// A question is the agent's to answer; a note isn't.
	threadWrite(t, s, "POST", api+q, map[string]any{"version": v, "line": 1, "quote": map[string]string{"exact": "Tokens"}, "kind": "note", "body": "Nice title."})
	if strings.Join(asked, ",") != "m/t1" {
		t.Errorf("agent asked for %v", asked)
	}
	if res, _ := threadWrite(t, s, "POST", api+q, map[string]any{"version": v, "line": 9, "kind": "ask", "body": "x"}); res.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("a line past the end: %d", res.StatusCode)
	}
	if res, _ := threadWrite(t, s, "POST", api+q, map[string]any{"version": v, "line": 3, "kind": "blocking", "body": "x"}); res.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("a code review's kind: %d", res.StatusCode)
	}

	// You reply, resolve and reopen; the version replied on is kept.
	*text += "\nPer tenant, now.\n"
	if res, _ := threadWrite(t, s, "POST", api+q, open); res.StatusCode != http.StatusConflict {
		t.Errorf("a thread on an old version: %d", res.StatusCode)
	}
	res, st = threadWrite(t, s, "POST", api+"/t1/say"+q, map[string]any{"body": "Still?"})
	v2 := doc.Version([]byte(*text))
	if res.StatusCode != 200 || len(st.Threads[0].Turns) != 2 || st.Threads[0].Turns[1].Version != v2 || st.Version != v2 {
		t.Fatalf("reply: %d %+v", res.StatusCode, st)
	}
	if strings.Join(asked, ",") != "m/t1,m/t1" {
		t.Errorf("your reply didn't ask the agent: %v", asked)
	}
	res, st = threadWrite(t, s, "POST", api+"/t1/status"+q, map[string]any{"status": "resolved"})
	if res.StatusCode != 200 || st.Threads[0].Status != conversation.Resolved {
		t.Errorf("resolve: %d %+v", res.StatusCode, st)
	}
	if res, _ := threadWrite(t, s, "POST", api+"/t1/status"+q, map[string]any{"status": "done"}); res.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("a status that isn't: %d", res.StatusCode)
	}
	if res, _ := threadWrite(t, s, "POST", api+"/t9/say"+q, map[string]any{"body": "x"}); res.StatusCode != http.StatusNotFound {
		t.Errorf("no such thread: %d", res.StatusCode)
	}

	for _, ver := range []string{v, v2} {
		if res := do(t, s, "GET", api+"/version"+q+"&v="+ver, true, nil); res.StatusCode != 200 || !strings.Contains(body(res), "We cache tokens per user.") {
			t.Errorf("version %s: %d", ver, res.StatusCode)
		}
	}
	if res := do(t, s, "GET", api+"/version"+q+"&v=bbbbbbbbbbbb", true, nil); res.StatusCode != http.StatusNotFound {
		t.Errorf("a version no thread is on: %d", res.StatusCode)
	}
}

// Your own edit, posted with a thread, is a suggestion when the text you
// highlighted is in the markdown once (and the agent isn't asked); else
// it's said in the thread for the agent. Accepting goes to the hook;
// declining is recorded.
func TestThreadSuggestions(t *testing.T) {
	s, text := docServer(t)
	api := "/api/missions/m/threads"
	q := "?ref=" + url.QueryEscape("records/tokens:proposal.md")
	v := doc.Version([]byte(*text))
	var asked []string
	idsAnsweredInto(s, &asked, job.Thread)
	var accepted []string
	fake(s).accept = func(slug, id string) (string, error) {
		accepted = append(accepted, id)
		*text = strings.Replace(*text, "per user", "per tenant", 1)
		return "Accepted: written to proposal.md, and resolved " + id + ".", nil
	}

	res, st := threadWrite(t, s, "POST", api+q, map[string]any{"version": v, "line": 3, "quote": map[string]string{"exact": "per user"}, "kind": "ask", "body": "", "suggestion": "per tenant"})
	if res.StatusCode != 200 || len(asked) != 0 {
		t.Fatalf("your edit: %d, asked %v", res.StatusCode, asked)
	}
	th := st.Threads[0]
	if th.Kind != conversation.Change || len(th.Suggestions) != 1 || th.Suggestions[0].By != conversation.You || th.Suggestions[0].From != "per user" || th.Suggestions[0].To != "per tenant" {
		t.Errorf("your edit = %+v", th)
	}
	// Text that isn't in the markdown as it reads goes to the agent.
	res, st = threadWrite(t, s, "POST", api+q, map[string]any{"version": v, "line": 3, "quote": map[string]string{"exact": "We cache tokens per user, everywhere"}, "kind": "change", "body": "Shorter.", "suggestion": "We cache them."})
	if res.StatusCode != 200 || strings.Join(asked, ",") != "t2" || len(st.Threads[1].Suggestions) != 0 || !strings.Contains(st.Threads[1].Turns[0].Body, "It should read: We cache them.") {
		t.Errorf("an edit that isn't one: %d %v %+v", res.StatusCode, asked, st.Threads[1])
	}
	// An edit made in the editor says the markdown it replaces, whole
	// lines of it, apart from the words it's on.
	res, st = threadWrite(t, s, "POST", api+q, map[string]any{"version": v, "line": 1, "quote": map[string]string{"exact": "Tokens"}, "kind": "change", "body": "", "from": "# Tokens\n", "suggestion": "# Token cache\n"})
	if th := st.Threads[2]; res.StatusCode != 200 || len(th.Suggestions) != 1 || th.Suggestions[0].From != "# Tokens\n" || th.Suggestions[0].To != "# Token cache\n" || th.Anchor.Quote.Exact != "Tokens" || strings.Join(asked, ",") != "t2" {
		t.Errorf("an edit from the editor: %d %+v (asked %v)", res.StatusCode, th, asked)
	}
	conversation.For(s.Root, "m").SetStatus("t3", conversation.You, conversation.Resolved, time.Now())

	res, st = threadWrite(t, s, "POST", api+"/t1/suggestion"+q, map[string]any{"action": "accept"})
	if res.StatusCode != 200 || strings.Join(accepted, ",") != "t1" || !strings.Contains(st.Message, "Accepted") || st.Version != doc.Version([]byte(*text)) {
		t.Errorf("accept: %d %+v", res.StatusCode, st)
	}
	conversation.For(s.Root, "m").Suggest("t2", conversation.Suggestion{From: "per tenant", To: "per org", By: conversation.Agent}, []byte(*text))
	res, st = threadWrite(t, s, "POST", api+"/t2/suggestion"+q, map[string]any{"action": "decline"})
	if res.StatusCode != 200 || st.Threads[1].Suggestions[0].State != conversation.SuggestDeclined {
		t.Errorf("decline: %d %+v", res.StatusCode, st.Threads[1])
	}
	if res, _ := threadWrite(t, s, "POST", api+"/t2/suggestion"+q, map[string]any{"action": "decline"}); res.StatusCode != http.StatusNotFound {
		t.Errorf("nothing waiting: %d", res.StatusCode)
	}
	if res, _ := threadWrite(t, s, "POST", api+"/t2/suggestion"+q, map[string]any{"action": "maybe"}); res.StatusCode != http.StatusBadRequest {
		t.Errorf("an action that isn't: %d", res.StatusCode)
	}
}

// Approving a document records it at the version you read, and is refused
// while a Change thread on it is open, or if it's changed since you read
// it. The state says the approval and the stage the document was made for.
func TestApproveDoc(t *testing.T) {
	s, text := docServer(t)
	base := fake(s).artifact
	fake(s).artifact = func(slug, ref string) (*ops.DocText, error) {
		a, err := base(slug, ref)
		if err == nil {
			a.Stage = &ops.DocStage{N: 1, Title: "Write the proposal", State: "current"}
		}
		return a, err
	}
	api := "/api/missions/m/threads"
	q := "?ref=" + url.QueryEscape("records/tokens:proposal.md")
	v := doc.Version([]byte(*text))

	threadWrite(t, s, "POST", api+q, map[string]any{"version": v, "line": 3, "quote": map[string]string{"exact": "per user"}, "kind": "change", "body": "Per tenant."})
	threadWrite(t, s, "POST", api+q, map[string]any{"version": v, "line": 1, "quote": map[string]string{"exact": "Tokens"}, "kind": "ask", "body": "Why?"})
	if res, _ := threadWrite(t, s, "POST", api+"/approve"+q, map[string]any{"version": v}); res.StatusCode != http.StatusUnprocessableEntity || !strings.Contains(body(res), "t1 asks for a change") {
		t.Errorf("approving with a change open: %d", res.StatusCode)
	}
	threadWrite(t, s, "POST", api+"/t1/status"+q, map[string]any{"status": "resolved"})
	if res, _ := threadWrite(t, s, "POST", api+"/approve"+q, map[string]any{"version": "aaaaaaaaaaaa"}); res.StatusCode != http.StatusConflict {
		t.Errorf("approving a version that isn't the document's: %d", res.StatusCode)
	}
	// A question left open doesn't hold it up.
	res, st := threadWrite(t, s, "POST", api+"/approve"+q, map[string]any{"version": v})
	if res.StatusCode != 200 || st.Approved == nil || st.Approved.Version != v || st.Approved.By != conversation.You || st.Message != "Approved proposal.md." {
		t.Fatalf("approve: %d %+v", res.StatusCode, st)
	}
	if st.Stage == nil || st.Stage.N != 1 || st.Stage.State != "current" {
		t.Errorf("stage = %+v", st.Stage)
	}
	*text += "\nMore.\n"
	if _, st := threadWrite(t, s, "GET", api+q, nil); st.Approved == nil || st.Approved.Version != v || st.Version == v {
		t.Errorf("after a change: %+v", st)
	}
}

// What was decided in a thread is recorded as a decision from it, about
// the document and its stage, and its draft settled; or the draft is
// skipped. Resolving asks for a draft.
func TestRecordDecision(t *testing.T) {
	s, text := docServer(t)
	base := fake(s).artifact
	fake(s).artifact = func(slug, ref string) (*ops.DocText, error) {
		a, err := base(slug, ref)
		if err == nil {
			a.Stage = &ops.DocStage{N: 2, Title: "Propose", State: "current"}
		}
		return a, err
	}
	var resolved []string
	answersInto(s, &resolved, job.Draft)
	api := "/api/missions/m/threads"
	q := "?ref=" + url.QueryEscape("records/tokens:proposal.md")
	v := doc.Version([]byte(*text))
	threadWrite(t, s, "POST", api+q, map[string]any{"version": v, "line": 3, "quote": map[string]string{"exact": "per user"}, "kind": "ask", "body": "Why per user?"})
	threadWrite(t, s, "POST", api+q, map[string]any{"version": v, "line": 3, "quote": map[string]string{"exact": "per user"}, "kind": "ask", "body": "And?"})
	store := conversation.For(s.Root, "m")
	store.Say("t1", conversation.Turn{By: conversation.Agent, At: time.Now(), Body: "Tokens are per tenant."})
	threadWrite(t, s, "POST", api+"/t1/status"+q, map[string]any{"status": "resolved"})
	if strings.Join(resolved, ",") != "m/t1" {
		t.Errorf("resolving asked for drafts of %v", resolved)
	}
	store.DraftDecision("t1", conversation.Draft{What: "Key per tenant.", Why: "Tokens are.", By: conversation.Agent, At: time.Now()})

	res, st := threadWrite(t, s, "POST", api+"/t1/decision"+q, map[string]any{"action": "record", "what": "Key the cache per tenant.", "why": "Tokens are per tenant."})
	if res.StatusCode != 200 || st.Threads[0].Draft == nil || st.Threads[0].Draft.State != conversation.DraftRecorded || st.Threads[0].Draft.Decision != "d1" || st.Message != "Recorded d1: Key the cache per tenant." {
		t.Fatalf("record: %d %+v", res.StatusCode, st)
	}
	all, _ := (mission.Store{Root: s.Root}).Decisions("m")
	if len(all) != 1 || all[0].From != "t1" || all[0].By != "you" || strings.Join(all[0].About, " ") != "records/tokens:proposal.md s2" || all[0].Why != "Tokens are per tenant." {
		t.Errorf("decisions = %+v", all)
	}
	if res, _ := threadWrite(t, s, "POST", api+"/t1/decision"+q, map[string]any{"action": "record", "what": "Again."}); res.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("recording twice: %d", res.StatusCode)
	}
	if res, _ := threadWrite(t, s, "POST", api+"/t2/decision"+q, map[string]any{"action": "record", "what": " "}); res.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("recording nothing: %d", res.StatusCode)
	}
	if res, st := threadWrite(t, s, "POST", api+"/t2/decision"+q, map[string]any{"action": "skip"}); res.StatusCode != 200 || st.Threads[1].Draft == nil || st.Threads[1].Draft.State != conversation.DraftSkipped {
		t.Errorf("skip: %d %+v", res.StatusCode, st.Threads)
	}
}
