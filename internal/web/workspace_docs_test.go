package web

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/doc"
	"github.com/withworkspace/workspace/internal/ops"
)

// A document outside a mission (any markdown file in a repo) is read here,
// and edited as a page. The threads made on it, and the draft kept of it,
// before it was are kept to read, and the draft may be thrown away; nothing
// else is written through /api/docs.
func TestDocsOutsideAMission(t *testing.T) {
	s, _ := docServer(t)
	text := "# How loud\n\nA greeting is loud when asked.\n"
	fake(s).doc = func(ref string) (*ops.DocText, error) {
		switch ref {
		case "records:adr/0002-how-loud.md":
			return &ops.DocText{Ref: ref, Kind: "doc", State: "landed", Text: text, Markdown: true,
				In: &ops.DocIn{Repo: "records", Worktree: ".ground", Path: "adr/0002-how-loud.md"}}, nil
		case "records:main.go":
			return &ops.DocText{Ref: ref, Kind: "doc", Text: "package main"}, nil
		}
		return nil, ErrNotFound
	}
	q := "?ref=" + url.QueryEscape("records:adr/0002-how-loud.md")
	v := doc.Version([]byte(text))

	var a ArtifactContent
	res := do(t, s, "GET", "/api/docs"+q, true, nil)
	json.NewDecoder(res.Body).Decode(&a)
	if res.StatusCode != 200 || a.Version != v || !a.Annotatable || a.In == nil || a.In.Repo != "records" {
		t.Fatalf("the document: %d %+v", res.StatusCode, a)
	}
	if res := do(t, s, "GET", "/api/docs?ref=nope%3Ax.md", true, nil); res.StatusCode != http.StatusNotFound {
		t.Errorf("no such file: %d", res.StatusCode)
	}
	if res := do(t, s, "GET", "/api/docs/threads?ref=records%3Amain.go", true, nil); res.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("code isn't a document to comment on: %d", res.StatusCode)
	}
	if res := do(t, s, "GET", "/api/docs"+q, false, nil); res.StatusCode != http.StatusUnauthorized {
		t.Errorf("not signed in: %d", res.StatusCode)
	}

	// Threads made on it before: read, with the version they were made on.
	if _, err := conversation.ForWorkspace(s.Root).OpenThread(conversation.NewThread{
		Kind: conversation.Ask, By: conversation.You, Body: "Asked by whom?",
		Anchor: conversation.Anchor{Doc: "records:adr/0002-how-loud.md", Path: "adr/0002-how-loud.md", Version: v, Line: 3, EndLine: 3, Quote: &conversation.Quote{Exact: "when asked"}},
	}, time.Now()); err != nil {
		t.Fatal(err)
	}
	conversation.ForWorkspace(s.Root).KeepVersion([]byte(text))
	var st ThreadState
	res = do(t, s, "GET", "/api/docs/threads"+q, true, nil)
	json.NewDecoder(res.Body).Decode(&st)
	if res.StatusCode != 200 || len(st.Threads) != 1 || st.Threads[0].ID != "w1" {
		t.Fatalf("the threads: %d %+v", res.StatusCode, st)
	}
	if res := do(t, s, "GET", "/api/docs/threads/version"+q+"&v="+v, true, nil); res.StatusCode != 200 || !strings.Contains(body(res), "loud when asked") {
		t.Errorf("the version it was made on: %d", res.StatusCode)
	}

	// Nothing's written: no new threads, turns, suggestions or saves.
	for _, w := range []struct{ method, path string }{
		{"POST", "/api/docs/threads"}, {"POST", "/api/docs/threads/w1/say"}, {"POST", "/api/docs/threads/w1/status"},
		{"POST", "/api/docs/threads/w1/suggestion"}, {"POST", "/api/docs/save"}, {"PUT", "/api/docs/draft"},
	} {
		if res, _ := docWrite(t, s, w.method, w.path+q, map[string]any{}); res.StatusCode == 200 {
			t.Errorf("%s %s: %d", w.method, w.path, res.StatusCode)
		}
	}
	// A draft from before is read, and thrown away.
	if err := (doc.Drafts{Root: s.Root}).Save(doc.Draft{Ref: "records:adr/0002-how-loud.md", Base: v, Text: "Louder.", At: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if res := do(t, s, "GET", "/api/docs/draft"+q, true, nil); res.StatusCode != 200 || !strings.Contains(body(res), "Louder.") {
		t.Errorf("the draft: %d", res.StatusCode)
	}
	if res, _ := docWrite(t, s, "DELETE", "/api/docs/draft"+q, nil); res.StatusCode != 200 {
		t.Errorf("dropping the draft: %d", res.StatusCode)
	}
}
