package web

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/doc"
	"github.com/withworkspace/workspace/internal/ops"
)

func docWrite(t *testing.T, s *Server, method, path string, v any) (*http.Response, []byte) {
	t.Helper()
	data, _ := json.Marshal(v)
	res := do(t, s, method, path, true, func(r *http.Request) {
		r.Body = io.NopCloser(bytes.NewReader(data))
		r.ContentLength = int64(len(data))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Origin", s.Info().Origin())
	})
	body, _ := io.ReadAll(res.Body)
	return res, body
}

// A document's draft is kept as you write it and handed back when you
// come back to it; saving goes to the hook, and a clash comes back as
// data, not an error. An editable document keeps the version you open, to
// merge with.
func TestDocsAPI(t *testing.T) {
	s, text := docServer(t)
	ref := "records/tokens:proposal.md"
	q := "?ref=" + url.QueryEscape(ref)
	editable := fake(s).artifact
	fake(s).artifact = func(slug, r string) (*ops.DocText, error) {
		a, err := editable(slug, r)
		if a != nil {
			a.Editable = true
		}
		return a, err
	}

	var got draftReply
	json.NewDecoder(do(t, s, "GET", "/api/missions/m/draft"+q, true, nil).Body).Decode(&got)
	if got.Draft != nil {
		t.Errorf("a draft before writing one: %+v", got.Draft)
	}
	if res, _ := docWrite(t, s, "PUT", "/api/missions/m/draft", map[string]any{"ref": ref, "text": "x"}); res.StatusCode != http.StatusBadRequest {
		t.Errorf("a draft with no version: %d", res.StatusCode)
	}
	if res, _ := docWrite(t, s, "PUT", "/api/missions/nope/draft", map[string]any{"ref": ref, "base": "abc", "text": "x"}); res.StatusCode != http.StatusNotFound {
		t.Errorf("a draft for no mission: %d", res.StatusCode)
	}
	// A slug that climbs out of the missions folder (the router unescapes
	// %2F) is no mission, and writes nothing.
	for _, method := range []string{"PUT", "DELETE"} {
		if res, _ := docWrite(t, s, method, "/api/missions/..%2F../draft"+q, map[string]any{"ref": ref, "base": "abc", "text": "x"}); res.StatusCode != http.StatusNotFound {
			t.Errorf("%s of a draft through a slug that isn't one: %d", method, res.StatusCode)
		}
	}
	if res := do(t, s, "GET", "/api/missions/..%2F../draft"+q, true, nil); res.StatusCode != http.StatusNotFound {
		t.Errorf("reading a draft through a slug that isn't one: %d", res.StatusCode)
	}
	if _, err := os.Stat(filepath.Join(s.Root, "drafts")); err == nil {
		t.Error("a draft was written outside the missions folder")
	}
	if res, body := docWrite(t, s, "PUT", "/api/missions/m/draft", map[string]any{"ref": ref, "base": "abc", "text": "# Tokens\n\nPer tenant.\n"}); res.StatusCode != 200 {
		t.Fatalf("keeping a draft: %d %s", res.StatusCode, body)
	}
	json.NewDecoder(do(t, s, "GET", "/api/missions/m/draft"+q, true, nil).Body).Decode(&got)
	if got.Draft == nil || got.Draft.Text != "# Tokens\n\nPer tenant.\n" || got.Draft.Base != "abc" || got.Draft.At.IsZero() {
		t.Errorf("the draft back: %+v", got.Draft)
	}
	if res, _ := docWrite(t, s, "DELETE", "/api/missions/m/draft"+q, nil); res.StatusCode != 200 {
		t.Errorf("dropping it: %d", res.StatusCode)
	}
	got = draftReply{}
	json.NewDecoder(do(t, s, "GET", "/api/missions/m/draft"+q, true, nil).Body).Decode(&got)
	if got.Draft != nil {
		t.Errorf("dropped, but still %+v", got.Draft)
	}

	// Opening it keeps its version.
	do(t, s, "GET", "/api/missions/m/artifact"+q, true, nil)
	if kept, err := conversation.For(s.Root, "m").VersionText(doc.Version([]byte(*text))); err != nil || string(kept) != *text {
		t.Errorf("the version opened isn't kept: %q, %v", kept, err)
	}

	// Saving: what the hook says, or its clash.
	var asked DocSave
	fake(s).saveDocument = func(slug, ref, base, text, message string) (*ops.DocSaved, error) {
		asked = DocSave{Ref: ref, Base: base, Text: text, Message: message}
		if text == "clash" {
			return &ops.DocSaved{Version: "def", Conflict: &ops.DocConflict{Text: "<<<<<<< your edit", Clashes: 1}}, nil
		}
		return &ops.DocSaved{Message: "Saved", Version: "def"}, nil
	}
	if res, _ := docWrite(t, s, "POST", "/api/missions/m/artifact/save", map[string]any{"ref": ref, "base": "abc", "text": "  "}); res.StatusCode != http.StatusBadRequest {
		t.Errorf("saving it empty: %d", res.StatusCode)
	}
	res, body := docWrite(t, s, "POST", "/api/missions/m/artifact/save", map[string]any{"ref": ref, "base": "abc", "text": "# Tokens\n", "message": "Per tenant"})
	var saved DocSaved
	json.Unmarshal(body, &saved)
	if res.StatusCode != 200 || saved.Message != "Saved" || asked.Message != "Per tenant" || asked.Base != "abc" {
		t.Errorf("saving: %d %+v (asked %+v)", res.StatusCode, saved, asked)
	}
	res, body = docWrite(t, s, "POST", "/api/missions/m/artifact/save", map[string]any{"ref": ref, "base": "abc", "text": "clash"})
	saved = DocSaved{}
	json.Unmarshal(body, &saved)
	if res.StatusCode != 200 || saved.Conflict == nil || saved.Conflict.Clashes != 1 {
		t.Errorf("a clash: %d %s", res.StatusCode, body)
	}
}
