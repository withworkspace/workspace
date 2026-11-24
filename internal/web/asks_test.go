package web

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/withworkspace/workspace/internal/config"
	"github.com/withworkspace/workspace/internal/job"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/workspace"
)

// Asking about a document from its ask bar starts a conversation about
// it, with the passage you picked out and the lines it's on, and has the
// agent answer; it's followed and said to as any is, and it isn't listed.
func TestAskAPI(t *testing.T) {
	s := startTest(t, t.TempDir(), &fakeWorkspace{}, nil)
	notes := filepath.Join(s.Root, "repos", "records", ".ground", "notes")
	os.MkdirAll(notes, 0o755)
	os.WriteFile(filepath.Join(notes, "tokens.md"), []byte("# Tokens\n\n## Caching\n\nWe cache tokens\n*per user*, for an hour.\n"), 0o644)
	var asked []string
	setOps(s, testOps(s, ops.Options{Jobs: ops.JobsOf(func(j job.Job) { asked = append(asked, j.Kind+" "+j.Mission+"/"+j.ID) }, job.Answer, job.Thread, job.Draft)}, func(c *config.Config, ws *workspace.Workspace) {
		ws.RepoNames = []string{"records"}
	}))

	for payload, want := range map[string]int{
		`{"doc":"records:notes/tokens.md","body":" "}`: http.StatusUnprocessableEntity,
		`{"body":"Why?"}`: http.StatusUnprocessableEntity,
		`{"doc":"records:notes/nothing.md","body":"Why?"}`: http.StatusNotFound,
	} {
		if res, _ := talkWrite(t, s, "/api/asks", json.RawMessage(payload)); res.StatusCode != want {
			t.Errorf("%s: %d, want %d", payload, res.StatusCode, want)
		}
	}
	res, d := talkWrite(t, s, "/api/asks", map[string]any{
		"doc": "records:notes/tokens.md", "body": "Why per user?", "section": "Caching",
		"quote": map[string]string{"exact": "We cache tokens per user", "suffix": ", for an hour."},
	})
	if res.StatusCode != 200 || d.Conversation == nil || d.ID != "w1" || !d.Ephemeral || d.What != "doc" {
		t.Fatalf("ask: %d %+v", res.StatusCode, d.Conversation)
	}
	a := d.Anchor
	if a.Doc != "records:notes/tokens.md" || a.Path != "notes/tokens.md" || a.Section != "Caching" || a.Quote == nil || a.Line != 5 || a.EndLine != 6 || strings.Join(a.Excerpt, "|") != "We cache tokens|*per user*, for an hour." || a.Version == "" {
		t.Errorf("about: %+v", a)
	}
	if res, d := talkWrite(t, s, "/api/conversations/w1/say", map[string]any{"body": "And per tenant?"}); res.StatusCode != 200 || len(d.Turns) != 2 || !d.Ephemeral {
		t.Errorf("a follow-up: %d %+v", res.StatusCode, d.Conversation)
	}
	if strings.Join(asked, ",") != "answer /w1,answer /w1" {
		t.Errorf("the agent was asked %v", asked)
	}
	if talks := Talks(s.Root); len(talks) != 0 {
		t.Errorf("listed: %+v", talks)
	}

	// Kept, it's listed, about the document and the passage.
	if res, _ := talkWrite(t, s, "/api/conversations/w9/keep", nil); res.StatusCode != http.StatusNotFound {
		t.Errorf("keeping w9: %d", res.StatusCode)
	}
	if res, d := talkWrite(t, s, "/api/conversations/w1/keep", nil); res.StatusCode != 200 || d.Ephemeral || d.Kept == nil {
		t.Fatalf("keep: %d %+v", res.StatusCode, d.Conversation)
	}
	talks := Talks(s.Root)
	if len(talks) != 1 || talks[0].ID != "w1" || talks[0].Doc != "records:notes/tokens.md" || talks[0].Section != "Caching" || talks[0].Quote != "We cache tokens per user" {
		t.Errorf("listed: %+v", talks)
	}
}
