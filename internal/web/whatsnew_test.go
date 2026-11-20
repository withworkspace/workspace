package web

import (
	"net/http"
	"testing"

	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/release"
)

// What's new is served, and seen is said: a release's build that's seen
// its own release has nothing unseen, and a picture no entry names isn't
// fetched.
func TestWhatsNew(t *testing.T) {
	s := startTest(t, t.TempDir(), &fakeWorkspace{}, nil)
	setOps(s, testOps(s, ops.Options{WS: release.Upgrader{Version: "v1.3.0", Release: true}}))

	var n WhatsNewInfo
	if res := asks(t, s, "GET", "/api/whats-new", nil, &n); res.StatusCode != http.StatusOK {
		t.Fatalf("GET: %d", res.StatusCode)
	}
	if n.Version != "v1.3.0" || !n.Release || n.Unseen == nil || n.Entries == nil {
		t.Fatalf("got %+v", n)
	}
	if res := asks(t, s, "POST", "/api/whats-new/seen", struct{}{}, nil); res.StatusCode != http.StatusOK {
		t.Fatalf("seen: %d", res.StatusCode)
	}
	if res := asks(t, s, "GET", "/api/whats-new", nil, &n); res.StatusCode != http.StatusOK || len(n.Unseen) != 0 {
		t.Fatalf("after seen: %d, %+v", res.StatusCode, n.Unseen)
	}
	if res := asks(t, s, "GET", "/api/whats-new/pictures/nothing.png", nil, nil); res.StatusCode != http.StatusNotFound {
		t.Fatalf("a picture no entry names: %d", res.StatusCode)
	}
}
