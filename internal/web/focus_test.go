package web

import (
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"testing"
)

// You pin, unpin and order your focus from the page; the workspace
// carries it, so every page's sidebar has it.
func TestFocusAPI(t *testing.T) {
	s := startTest(t, t.TempDir(), &fakeWorkspace{title: "Cache"}, nil)
	names := func(pins []FocusPin) []string {
		var out []string
		for _, p := range pins {
			out = append(out, p.Name+" "+p.Kind+" "+p.Title)
		}
		return out
	}
	post := func(v any) (int, FocusDone, string) {
		res, data := docWrite(t, s, "POST", "/api/focus", v)
		var d FocusDone
		json.Unmarshal(data, &d)
		return res.StatusCode, d, string(data)
	}

	if code, d, raw := post(map[string]string{"pin": "cache"}); code != http.StatusOK || d.Said != "Pinned cache." || !slices.Equal(names(d.Focus), []string{"cache mission Cache"}) {
		t.Fatalf("pin: %d %s", code, raw)
	}
	if code, d, raw := post(map[string]string{"pin": "demo#12"}); code != http.StatusOK || len(d.Focus) != 2 {
		t.Fatalf("pin a PR: %d %s", code, raw)
	}
	if code, d, raw := post(map[string][]string{"order": {"demo#12"}}); code != http.StatusOK || !slices.Equal(names(d.Focus), []string{"demo#12  ", "cache mission Cache"}) {
		t.Fatalf("order: %d %s", code, raw)
	}

	res := do(t, s, "GET", "/api/workspace", true, nil)
	var ws Workspace
	json.NewDecoder(res.Body).Decode(&ws)
	if !slices.Equal(names(ws.Focus), []string{"demo#12  ", "cache mission Cache"}) {
		t.Errorf("the workspace's focus: %+v", ws.Focus)
	}

	if code, d, raw := post(map[string]string{"unpin": "demo#12"}); code != http.StatusOK || d.Said != "Unpinned demo#12." || len(d.Focus) != 1 {
		t.Errorf("unpin: %d %s", code, raw)
	}
	if code, _, raw := post(map[string]string{"pin": "nothing-here"}); code != http.StatusUnprocessableEntity || !strings.Contains(raw, "nothing named nothing-here") {
		t.Errorf("pinning what isn't there: %d %s", code, raw)
	}
	if code, _, raw := post(map[string]any{"pin": "cache", "unpin": "cache"}); code != http.StatusBadRequest {
		t.Errorf("pin and unpin at once: %d %s", code, raw)
	}
}
