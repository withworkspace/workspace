package web

import (
	"bufio"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/find"
	"github.com/withworkspace/workspace/internal/mission"
)

// findLines reads what GET /api/find answered: its groups by kind, in the
// order they came, and its last line.
func findLines(t *testing.T, s *Server, query url.Values) (groups map[string]find.Group, order []string, last map[string]any) {
	t.Helper()
	res := do(t, s, "GET", "/api/find?"+query.Encode(), true, nil)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("find %v: %d %s", query, res.StatusCode, body(res))
	}
	if ct := res.Header.Get("Content-Type"); ct != "application/x-ndjson" {
		t.Errorf("content type %q", ct)
	}
	groups = map[string]find.Group{}
	sc := bufio.NewScanner(res.Body)
	for sc.Scan() {
		var probe map[string]any
		if err := json.Unmarshal(sc.Bytes(), &probe); err != nil {
			t.Fatalf("line %q: %v", sc.Text(), err)
		}
		if _, isGroup := probe["kind"]; !isGroup {
			last = probe
			continue
		}
		var g find.Group
		json.Unmarshal(sc.Bytes(), &g)
		groups[g.Kind] = g
		order = append(order, g.Kind)
	}
	return groups, order, last
}

func TestFindAPI(t *testing.T) {
	w := newReviewsWorld(t)
	// Nothing to look through: Find isn't there.
	if res := do(t, w.s, "GET", "/api/find?q=a", true, nil); res.StatusCode != http.StatusNotFound {
		t.Errorf("with no index: %d", res.StatusCode)
	}
	// The workspace's repo, api, with its ground: the remote's checkout.
	w.s.Find = &find.Index{RepoAt: acmeRepo, Worktrees: func() ([]find.Worktree, error) {
		return []find.Worktree{{Repo: "api", Name: find.Ground, Dir: w.remote}}, nil
	}}
	if res := do(t, w.s, "GET", "/api/find?q=a", false, nil); res.StatusCode != http.StatusUnauthorized {
		t.Errorf("not signed in: %d", res.StatusCode)
	}

	// Opening Find warms it.
	_, _, last := findLines(t, w.s, url.Values{})
	if last["done"] != true {
		t.Errorf("warm ended with %v", last)
	}
	// Before Reviews has been opened there are no pull requests in hand,
	// and Find doesn't ask GitHub for them.
	groups, _, _ := findLines(t, w.s, url.Values{"q": {"two"}})
	if g, ok := groups[find.KindPulls]; !ok || len(g.Results) != 0 || g.At != nil {
		t.Errorf("pull requests before the list's read: %+v", g)
	}
	var list ReviewsList
	if code := getInto(t, w.s, "/api/reviews", &list); code != http.StatusOK {
		t.Fatalf("reviews: %d", code)
	}

	groups, order, last := findLines(t, w.s, url.Values{"q": {"a.go"}})
	if len(order) != 4 || order[0] != find.KindPulls || order[1] != find.KindRef || order[2] != find.KindCapsules || order[3] != find.KindFiles || last["done"] != true {
		t.Errorf("groups came %v, then %v", order, last)
	}
	if g := groups[find.KindFiles]; len(g.Results) != 1 || g.Results[0].Ref != "api/a.go" || g.Results[0].Kind != find.KindFile {
		t.Errorf("files: %+v", g)
	}
	// A pull request by a word of its title, its number, and its address.
	for _, q := range []string{"two", "5", "#5", "api#5", "alice"} {
		g := findLines0(t, w.s, q)[find.KindPulls]
		if len(g.Results) != 1 || g.Results[0].Ref != "api#5" || g.Results[0].Author != "alice" || g.Results[0].State == "" || g.At == nil {
			t.Errorf("%q: %+v", q, g)
		}
	}
	for q, ref := range map[string]string{
		"https://github.com/acme/api/pull/5": "api#5",
		"api#5":                              "api#5",
		"api/a.go:2":                         "api/a.go:2",
		"https://github.com/acme/api/blob/main/a.go#L2-L3": "api/a.go:2-3",
	} {
		g := findLines0(t, w.s, q)[find.KindRef]
		if len(g.Results) != 1 || g.Results[0].Ref != ref || !g.Results[0].Exact {
			t.Errorf("%q as a ref: %+v", q, g)
		}
	}
	// Only the kinds asked for, and as many as asked for.
	os.WriteFile(filepath.Join(w.remote, "b.go"), []byte("package a\n"), 0o644)
	gitT(t, w.remote, "add", "-A")
	gitT(t, w.remote, "commit", "-q", "-m", "b")
	w.s.Find = &find.Index{RepoAt: acmeRepo, Worktrees: w.s.Find.Worktrees}
	groups, order, _ = findLines(t, w.s, url.Values{"q": {"go"}, "kinds": {"files"}, "limit": {"1"}})
	if len(order) != 1 || len(groups[find.KindFiles].Results) != 1 || groups[find.KindFiles].More != 1 {
		t.Errorf("one file of two: %v %+v", order, groups[find.KindFiles])
	}
}

func findLines0(t *testing.T, s *Server, q string) map[string]find.Group {
	t.Helper()
	groups, _, _ := findLines(t, s, url.Values{"q": {q}})
	return groups
}

// TestFindAPI_Said: what ws records is a group of its own, found by what
// it says, led to where in it says it, and sent only when something is.
func TestFindAPI_Said(t *testing.T) {
	w := newReviewsWorld(t)
	w.s.Find = &find.Index{RepoAt: acmeRepo, Worktrees: func() ([]find.Worktree, error) { return nil, nil }}
	missions := mission.Store{Root: w.s.Root}
	m, err := missions.Create("Cache tokens")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := missions.Decide(m.Slug, mission.Decision{What: "Key the cache by tenant", Why: "Tenants share nothing", By: "you", At: time.Now()}); err != nil {
		t.Fatal(err)
	}
	said := func(q string) (g SaidGroup, ok bool) {
		t.Helper()
		res := do(t, w.s, "GET", "/api/find?"+url.Values{"q": {q}}.Encode(), true, nil)
		sc := bufio.NewScanner(res.Body)
		for sc.Scan() {
			var probe struct{ Kind string }
			json.Unmarshal(sc.Bytes(), &probe)
			if probe.Kind == "said" {
				json.Unmarshal(sc.Bytes(), &g)
				ok = true
			}
		}
		return g, ok
	}
	g, ok := said("tenants")
	if !ok || len(g.Results) != 1 {
		t.Fatalf("said: %+v", g)
	}
	if f := g.Results[0]; f.Ref != "d1" || f.Scope != m.Slug || f.Thing != "decision" || f.Title != "Key the cache by tenant" || f.Where != "mission "+m.Slug || !strings.Contains(f.Hint, "Tenants share nothing") {
		t.Errorf("the decision: %+v", f)
	}
	if _, ok := said("quidditch"); ok {
		t.Error("a group with nothing in it was sent")
	}
}

// acmeRepo is the tests' GitHub: every repo is acme's, by its name here.
func acmeRepo(owner, name string) (string, bool) { return name, strings.EqualFold(owner, "acme") }
