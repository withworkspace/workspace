package web

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/ops"
)

// A mission's page has its verification bundles, each file served from the
// bundle's own route as it is: only the files the bundle names, a video in
// ranges to seek in, text as text, and nothing an SVG could run.
func TestBundles(t *testing.T) {
	s := startTest(t, t.TempDir(), &fakeWorkspace{}, nil)
	store := mission.Store{Root: s.Root}
	if _, err := store.Create("M"); err != nil {
		t.Fatal(err)
	}
	store.StartCrew("m", "shell", "")
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "after.png"), []byte("\x89PNG\r\n\x1a\nafter"), 0o644)
	os.WriteFile(filepath.Join(dir, "before.png"), []byte("\x89PNG\r\n\x1a\nbefore"), 0o644)
	os.WriteFile(filepath.Join(dir, "run.webm"), []byte("0123456789video"), 0o644)
	os.WriteFile(filepath.Join(dir, "out.txt"), []byte("<b>hit</b>\n"), 0o644)
	written := "## Summary\nIt's cached.\n\n## Try it\n- try: `just dev`\n- look for: one request\n\n" +
		"## Checks\n1. met: the page shows the cached value\n   - shows: `after.png` After\n     - before: `before.png`\n   - shows: `run.webm` Loading it twice\n     - made by: `npm run shots`\n   - shows: `out.txt` What it printed\n" +
		"2. can't tell: whether it evicts\n   - ask: Add a test for eviction\n\n## Not verified\n- Safari.\n"
	if _, err := s.App.(*ops.Workspace).HandOverBundle("m", "c1", written, dir); err != nil {
		t.Fatal(err)
	}

	var d MissionDetail
	json.NewDecoder(do(t, s, "GET", "/api/missions/m", true, nil).Body).Decode(&d)
	if len(d.Bundles) != 1 {
		t.Fatalf("bundles = %+v", d.Bundles)
	}
	b := d.Bundles[0]
	if b.ID != "b1" || b.Crew != "c1" || b.By != "shell" || !b.Own || b.Stale || b.Summary != "It's cached." || len(b.NotVerified) != 1 {
		t.Errorf("bundle = %+v", b)
	}
	if len(b.Try) != 2 || b.Try[0].Command != "just dev" || !b.Try[1].Look {
		t.Errorf("try = %+v", b.Try)
	}
	if len(b.Checks) != 2 || b.Checks[0].Verdict != mission.CheckMet || b.Checks[1].Verdict != mission.CheckCantTell || len(b.Checks[1].Asks) != 1 {
		t.Fatalf("checks = %+v", b.Checks)
	}
	shows := b.Checks[0].Shows
	if len(shows) != 3 || shows[0].Kind != "image" || shows[0].URL != "/api/missions/m/bundles/b1/file/after.png" || shows[0].Before == nil || shows[0].Before.URL != "/api/missions/m/bundles/b1/file/before.png" ||
		shows[1].Kind != "video" || shows[1].MadeBy != "npm run shots" || shows[2].Kind != "text" || shows[2].Caption != "What it printed" {
		t.Fatalf("shows = %+v", shows)
	}

	res := do(t, s, "GET", shows[0].URL, true, nil)
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != 200 || res.Header.Get("Content-Type") != "image/png" || !strings.Contains(res.Header.Get("Content-Security-Policy"), "sandbox") || !strings.HasSuffix(string(body), "after") {
		t.Errorf("image: %d %v %q", res.StatusCode, res.Header, body)
	}
	res = do(t, s, "GET", shows[1].URL, true, func(r *http.Request) { r.Header.Set("Range", "bytes=10-") })
	body, _ = io.ReadAll(res.Body)
	if res.StatusCode != http.StatusPartialContent || res.Header.Get("Content-Type") != "video/webm" || string(body) != "video" {
		t.Errorf("a video's range: %d %q", res.StatusCode, body)
	}
	res = do(t, s, "GET", shows[2].URL, true, nil)
	body, _ = io.ReadAll(res.Body)
	if res.StatusCode != 200 || !strings.HasPrefix(res.Header.Get("Content-Type"), "text/plain") || res.Header.Get("X-Content-Type-Options") != "nosniff" || string(body) != "<b>hit</b>\n" {
		t.Errorf("text: %d %v %q", res.StatusCode, res.Header, body)
	}
	for _, path := range []string{"/api/missions/m/bundles/b1/file/brief.md", "/api/missions/m/bundles/b1/file/..%2F..%2Fbrief.md", "/api/missions/m/bundles/b9/file/after.png", "/api/missions/x/bundles/b1/file/after.png"} {
		if res := do(t, s, "GET", path, true, nil); res.StatusCode != http.StatusNotFound {
			t.Errorf("%s: %d, want not found", path, res.StatusCode)
		}
	}
	if res := do(t, s, "GET", shows[0].URL, false, nil); res.StatusCode == 200 {
		t.Error("served without signing in")
	}
}

// A bundle whose capsule has moved on since says it's stale, and where the
// capsule is now.
func TestBundleOf_Stale(t *testing.T) {
	v := ops.BundleView{Bundle: mission.Bundle{ID: "b1", Capsule: "web/x", Commit: "abc1234def"}}
	if b := bundleOf("m", v, "abc1234"); b.Stale {
		t.Errorf("stale at its own commit: %+v", b)
	}
	if b := bundleOf("m", v, "99f0000"); !b.Stale || b.Head != "99f0000" {
		t.Errorf("not stale past it: %+v", b)
	}
	if b := bundleOf("m", v, ""); b.Stale {
		t.Errorf("stale with no capsule to compare: %+v", b)
	}
}

// What ws ran of a bundle's step is its commands; what was judged goes
// with the bundle's own check, even where the judge couldn't be asked and
// it's ws that says so.
func TestBundleOf_RanAndJudged(t *testing.T) {
	v := ops.BundleView{
		Bundle:     mission.Bundle{ID: "b1", Own: true, Checks: []mission.BundleCheck{{N: 1, Check: "it's cached", Verdict: mission.CheckMet, Said: "it is"}}},
		StepChecks: []mission.Check{{Text: "it's cached"}, {Text: "`go build ./...` passes", Command: "go build ./...", Expect: mission.CheckPasses}},
		Ran: &mission.CheckRound{Commit: "abc", Results: []mission.CheckResult{
			{Check: "it's cached", Result: mission.CheckCantTell, Evidence: "the judge couldn't be asked", By: "ws"},
			{Check: "`go build ./...` passes", Result: mission.CheckMet, Evidence: "passed in 1s", By: "ws"},
		}},
	}
	b := bundleOf("m", v, "")
	if len(b.Ran) != 1 || b.Ran[0].N != 2 || b.Ran[0].Result != mission.CheckMet {
		t.Errorf("ran = %+v", b.Ran)
	}
	if j := b.Checks[0].Judged; j == nil || j.Result != mission.CheckCantTell {
		t.Errorf("judged = %+v", j)
	}
	// A verifier's bundle has its own word of its checks already: what's
	// shown beside it is only what ws ran.
	v.Own = false
	if b := bundleOf("m", v, ""); b.Checks[0].Judged != nil || len(b.Ran) != 1 {
		t.Errorf("a verifier's: judged %+v, ran %+v", b.Checks[0].Judged, b.Ran)
	}
}

// A mission's page approves a bundle, sends one back and marks what's
// been looked at, as you: each through the page's one way of acting.
func TestBundles_ActedOnFromThePage(t *testing.T) {
	s := startTest(t, t.TempDir(), &fakeWorkspace{}, nil)
	store := mission.Store{Root: s.Root}
	if _, err := store.Create("M"); err != nil {
		t.Fatal(err)
	}
	store.StartCrew("m", "shell", "")
	written := "## Summary\nIt's cached.\n\n## Checks\n1. unmet: it isn't cached yet\n2. can't tell: whether it evicts\n\n## Not verified\nNothing.\n"
	app := s.App.(*ops.Workspace)
	if _, err := app.HandOverBundle("m", "c1", written, t.TempDir()); err != nil {
		t.Fatal(err)
	}
	bundle := func() Bundle {
		var d MissionDetail
		json.NewDecoder(do(t, s, "GET", "/api/missions/m", true, nil).Body).Decode(&d)
		return d.Bundles[len(d.Bundles)-1]
	}
	if _, err := ActOn(app, "m", ActRequest{Verb: "bundle-looked", Bundle: "b1", Check: "c2", Looked: true}); err != nil {
		t.Fatal(err)
	}
	if b := bundle(); strings.Join(b.Looked, ",") != "c2" || b.Verdict != "" {
		t.Errorf("looked: %+v", b)
	}
	if _, err := ActOn(app, "m", ActRequest{Verb: "bundle-send-back", Bundle: "b1"}); err == nil {
		t.Error("sent back with nothing said")
	}
	said, err := ActOn(app, "m", ActRequest{Verb: "bundle-send-back", Bundle: "b1", Notes: []BundleNote{{Check: 1, Text: "cache it"}}})
	if err != nil || !strings.HasPrefix(said, "Sent b1 back") {
		t.Fatalf("sent back: %q, %v", said, err)
	}
	if b := bundle(); b.Verdict != mission.BundleSentBack || len(b.Notes) != 1 || b.Notes[0].Check != 1 || b.VerdictAt == nil {
		t.Errorf("sent back: %+v", b)
	}
	if _, err := app.HandOverBundle("m", "c1", strings.Replace(written, "1. unmet: it isn't cached yet", "1. unmet: still not", 1), t.TempDir()); err != nil {
		t.Fatal(err)
	}
	said, err = ActOn(app, "m", ActRequest{Verb: "bundle-approve", Bundle: "b2"})
	if err != nil || said != "Approved b2." {
		t.Fatalf("approved: %q, %v", said, err)
	}
	if b := bundle(); b.ID != "b2" || b.Verdict != mission.BundleApproved {
		t.Errorf("approved: %+v", b)
	}
}
