package web

import (
	"encoding/json"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/find"
	"github.com/withworkspace/workspace/internal/mission"
)

// TestAroundAPI: what's around a thing is what links to it, or from it,
// as the find window draws each (a decision that names a mission, led to
// in its own mission), with how it's linked; and a ref is needed.
func TestAroundAPI(t *testing.T) {
	w := newReviewsWorld(t)
	w.s.Find = &find.Index{RepoAt: acmeRepo, Worktrees: func() ([]find.Worktree, error) { return nil, nil }}
	missions := mission.Store{Root: w.s.Root}
	a, err := missions.Create("Cache tokens")
	if err != nil {
		t.Fatal(err)
	}
	b, err := missions.Create("Refresh tokens")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := missions.Decide(b.Slug, mission.Decision{What: "Wait for " + a.Slug, Why: "It keys the cache", By: "you", At: time.Now()}); err != nil {
		t.Fatal(err)
	}
	res := do(t, w.s, "GET", "/api/around?"+url.Values{"ref": {a.Slug}}.Encode(), true, nil)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("around: %d", res.StatusCode)
	}
	var got struct{ Results []NearFound }
	json.NewDecoder(res.Body).Decode(&got)
	var d *NearFound
	for i, n := range got.Results {
		if n.Scope == b.Slug && n.Ref == "d1" {
			d = &got.Results[i]
		}
	}
	if d == nil || d.How != "mentions" || d.Out || d.Thing != "decision" || d.Title != "Wait for "+a.Slug || d.Kind != "said" {
		t.Errorf("the decision that names it: %+v in %+v", d, got.Results)
	}
	if res := do(t, w.s, "GET", "/api/around", true, nil); res.StatusCode != http.StatusBadRequest {
		t.Errorf("no ref: %d", res.StatusCode)
	}
}
