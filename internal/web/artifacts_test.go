package web

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/artifact"
	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/mission"
)

// spy is a version whose script reaches for everything the frame keeps
// from it: ws's cookie, storage, and ws's API.
const spy = `<!doctype html>
<title>Spy</title>
<pre id="out"></pre>
<script>
const out = [];
try { out.push("cookie: " + JSON.stringify(document.cookie)); } catch (e) { out.push("cookie: " + e.name); }
try { localStorage.setItem("k", "v"); out.push("localStorage: " + localStorage.getItem("k")); } catch (e) { out.push("localStorage: " + e.name); }
fetch("/api/workspace").then(r => r.text()).then(t => out.push("fetch: " + t.slice(0, 40)), e => out.push("fetch: " + e.name));
document.getElementById("out").textContent = out.join("\n");
</script>`

// addArtifact adds an artifact with versions, as you would by hand.
func addArtifact(t *testing.T, root, title string, versions ...string) artifact.Entry {
	t.Helper()
	st := artifact.Store{Root: root}
	at := time.Date(2026, 11, 12, 18, 46, 0, 0, time.UTC)
	e, err := st.Add(context.Background(), artifact.New{Title: title, HTML: []byte(versions[0]), Version: artifact.Version{By: "you", Note: "v1's note"}, At: at})
	if err != nil {
		t.Fatal(err)
	}
	for i, v := range versions[1:] {
		at = at.Add(time.Minute)
		if e, _, err = st.AddVersion(context.Background(), e.ID, artifact.Revision{HTML: []byte(v), Version: artifact.Version{By: "c1", From: "glow", FromTitle: "The glow", Note: "v" + strconv.Itoa(i+2) + "'s note"}, At: at}); err != nil {
			t.Fatal(err)
		}
	}
	return e
}

// A version is served from a route of its own, with a policy that runs
// its scripts in an origin of their own, lets nothing leave, and lets
// only the web UI frame it: in place of the guard's, which frames
// nothing and runs no inline script. Everything else keeps the guard's.
func TestFrame(t *testing.T) {
	s := startTest(t, t.TempDir(), &fakeWorkspace{}, nil)
	addArtifact(t, s.Root, "Spy", spy, "<!doctype html><title>Two</title><p>two")

	// The file as it was written, with ws.tweaks written in after its
	// doctype: whatever its address says after ? is the page's to read.
	res := do(t, s, "GET", "/frame/a1/v1?t.density=compact", true, nil)
	if got := body(res); res.StatusCode != 200 || got != string(withTweaks([]byte(spy))) || !strings.HasPrefix(got, "<!doctype html><script>") || !strings.HasSuffix(got, strings.TrimPrefix(spy, "<!doctype html>")) {
		t.Fatalf("v1: %d %q", res.StatusCode, got)
	}
	h := res.Header
	want := map[string]string{
		"Content-Type":            "text/html; charset=utf-8",
		"Content-Security-Policy": "sandbox allow-scripts; default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; img-src data: blob:; font-src data:; media-src data: blob:; frame-ancestors 'self'",
		"X-Frame-Options":         "SAMEORIGIN",
		"X-Content-Type-Options":  "nosniff",
		"Referrer-Policy":         "no-referrer",
		"Cache-Control":           "private, no-cache",
	}
	for k, v := range want {
		if got := h.Get(k); got != v {
			t.Errorf("%s: %q, want %q", k, got, v)
		}
	}
	csp := h.Get("Content-Security-Policy")
	for _, never := range []string{"allow-same-origin", "allow-popups", "allow-top-navigation", "allow-forms", "allow-modals", "connect-src", "http", "*"} {
		if strings.Contains(csp, never) {
			t.Errorf("the frame's policy has %q: %s", never, csp)
		}
	}
	if len(h.Values("Content-Security-Policy")) != 1 || len(h.Values("X-Frame-Options")) != 1 {
		t.Errorf("the guard's headers are still there too: %v", h)
	}

	// It's asked again each time (what a3.v2 names can change, with the
	// workspace made again), and the same file is a 304.
	etag := h.Get("ETag")
	if etag == "" {
		t.Fatal("no ETag")
	}
	if res := do(t, s, "GET", "/frame/a1/v1", true, func(r *http.Request) { r.Header.Set("If-None-Match", etag) }); res.StatusCode != http.StatusNotModified {
		t.Errorf("the same again: %d, want 304", res.StatusCode)
	}
	res = do(t, s, "GET", "/frame/a1/v2", true, nil)
	if b := body(res); res.StatusCode != 200 || !strings.Contains(b, "<p>two") || res.Header.Get("ETag") == etag {
		t.Errorf("v2: %d %q %s", res.StatusCode, b, res.Header.Get("ETag"))
	}

	// Signed in, like any other request: the frame's request is same-site,
	// so the cookie goes with it, and only you are served the HTML.
	if res := do(t, s, "GET", "/frame/a1/v1", false, nil); res.StatusCode != http.StatusUnauthorized || strings.Contains(body(res), "Spy") {
		t.Errorf("signed out: %d", res.StatusCode)
	}
	if res := do(t, s, "GET", "/frame/a1/v1", true, func(r *http.Request) { r.Host = "evil.example:" + strconv.Itoa(s.Info().Port) }); res.StatusCode != http.StatusMisdirectedRequest {
		t.Errorf("foreign host: %d", res.StatusCode)
	}

	// What isn't there says so in the frame, under the frame's policy.
	for _, p := range []string{"/frame/a1/v9", "/frame/a9/v1", "/frame/a1/1", "/frame/a1.v1/v1", "/frame/p1/v1"} {
		res := do(t, s, "GET", p, true, nil)
		if res.StatusCode != http.StatusNotFound || res.Header.Get("Content-Security-Policy") != FramePolicy || res.Header.Get("Cache-Control") != "no-store" {
			t.Errorf("%s: %d %q", p, res.StatusCode, res.Header.Get("Content-Security-Policy"))
		}
	}
	if res := do(t, s, "POST", "/frame/a1/v1", true, func(r *http.Request) {
		r.Header.Set("Origin", s.Info().Origin())
		r.Header.Set("Content-Type", "application/json")
	}); res.StatusCode == http.StatusOK {
		t.Errorf("POST: %d", res.StatusCode)
	}

	// The app and the API keep the guard's: framed by nothing.
	for _, p := range []string{"/artifacts/a1", "/api/artifacts/a1"} {
		res := do(t, s, "GET", p, true, nil)
		if res.Header.Get("X-Frame-Options") != "DENY" || !strings.Contains(res.Header.Get("Content-Security-Policy"), "frame-ancestors 'none'") {
			t.Errorf("%s: %v", p, res.Header)
		}
	}
}

// The list is the workspace's, an artifact's page reads its versions, and
// a version's source is its HTML.
func TestArtifactsAPI(t *testing.T) {
	fw := &fakeWorkspace{title: "The glow"}
	s := startTest(t, t.TempDir(), fw, nil)
	addArtifact(t, s.Root, "Horizon glow", "<!doctype html><title>One</title>", "<!doctype html><title>Two</title>")
	if err := (mission.Store{Root: s.Root}).AddArtifact(fw.slug, mission.Artifact{Ref: "a1", Kind: "artifact", Label: "Old title", Crew: "c1"}); err != nil {
		t.Fatal(err)
	}

	var ws Workspace
	if err := json.NewDecoder(do(t, s, "GET", "/api/workspace", true, nil).Body).Decode(&ws); err != nil {
		t.Fatal(err)
	}
	if len(ws.Artifacts) != 1 {
		t.Fatalf("listed %+v", ws.Artifacts)
	}
	a := ws.Artifacts[0]
	if a.ID != "a1" || a.Title != "Horizon glow" || a.Status != "live" || a.Versions != 2 || a.Latest == nil || a.Latest.N != 2 || a.Latest.Note != "v2's note" ||
		a.Place != artifact.Local || !strings.HasSuffix(a.Where, "-horizon-glow/") || len(a.From) != 1 || a.From[0].From != "glow" || a.From[0].Title != "The glow" {
		t.Errorf("listed %+v (latest %+v)", a, a.Latest)
	}

	var d ArtifactDetail
	res := do(t, s, "GET", "/api/artifacts/a1", true, nil)
	if err := json.NewDecoder(res.Body).Decode(&d); err != nil || res.StatusCode != 200 {
		t.Fatalf("detail: %d %v", res.StatusCode, err)
	}
	if len(d.All) != 2 || d.All[0].N != 1 || d.All[0].By != "you" || d.All[1].BasedOn != 1 || d.All[1].By != "c1" || d.All[1].Size == 0 || d.All[0].Note != "v1's note" {
		t.Errorf("versions %+v", d.All)
	}
	for _, p := range []string{"/api/artifacts/a9", "/api/artifacts/nope", "/api/artifacts/a1.v1"} {
		if res := do(t, s, "GET", p, true, nil); res.StatusCode != http.StatusNotFound {
			t.Errorf("%s: %d", p, res.StatusCode)
		}
	}

	var src ArtifactSource
	json.NewDecoder(do(t, s, "GET", "/api/artifacts/a1/source?v=1", true, nil).Body).Decode(&src)
	if src.Version != 1 || src.Text != "<!doctype html><title>One</title>" {
		t.Errorf("v1's source %+v", src)
	}
	json.NewDecoder(do(t, s, "GET", "/api/artifacts/a1/source", true, nil).Body).Decode(&src)
	if src.Version != 2 || !strings.Contains(src.Text, "Two") {
		t.Errorf("the latest's source %+v", src)
	}
	if res := do(t, s, "GET", "/api/artifacts/a1/source?v=9", true, nil); res.StatusCode != http.StatusNotFound {
		t.Errorf("v9: %d", res.StatusCode)
	}
	if res := do(t, s, "GET", "/api/artifacts/a1/source?v=x", true, nil); res.StatusCode != http.StatusBadRequest {
		t.Errorf("v=x: %d", res.StatusCode)
	}

	// The mission's page lists it with its versions, titled as it is now.
	var m MissionDetail
	json.NewDecoder(do(t, s, "GET", "/api/missions/"+fw.slug, true, nil).Body).Decode(&m)
	if len(m.Artifacts) != 1 || m.Artifacts[0].Ref != "a1" || m.Artifacts[0].Versions != 2 || m.Artifacts[0].Label != "Horizon glow" || m.Artifacts[0].State != "live" {
		t.Errorf("the mission's %+v", m.Artifacts)
	}
}

// Two versions compare by their sources, diffed as a review's file is:
// of one artifact or of two, a's against b's, each named as it was read
// (the latest, where a ref has no version), with b's lines for what's
// unchanged between the changes.
func TestArtifactCompare(t *testing.T) {
	s := startTest(t, t.TempDir(), &fakeWorkspace{}, nil)
	addArtifact(t, s.Root, "Horizon glow", "<!doctype html>\n<title>Glow</title>\n<style>body { background: #000; }</style>\n",
		"<!doctype html>\n<title>Glow</title>\n<style>body { background: #111; }</style>\n<p>light</p>\n")
	addArtifact(t, s.Root, "Another take", "<!doctype html>\n<title>Take</title>\n")

	compare := func(q string) (ArtifactCompare, int) {
		t.Helper()
		var c ArtifactCompare
		res := do(t, s, "GET", "/api/artifacts/compare?"+q, true, nil)
		if res.StatusCode == 200 {
			if err := json.NewDecoder(res.Body).Decode(&c); err != nil {
				t.Fatal(err)
			}
		}
		return c, res.StatusCode
	}
	c, code := compare("a=a1.v1&b=a1.v2")
	if code != 200 || c.A != "a1.v1" || c.B != "a1.v2" || c.File.Path != "a1.v2.html" || c.File.Additions != 2 || c.File.Deletions != 1 || len(c.File.Hunks) != 1 {
		t.Fatalf("v1 against v2: %d %+v", code, c)
	}
	if len(c.Lines) != 4 || c.Lines[3] != "<p>light</p>" {
		t.Errorf("v2's lines %q", c.Lines)
	}
	// Swapped, it's the other way about.
	if c, _ := compare("a=a1.v2&b=a1.v1"); c.File.Additions != 1 || c.File.Deletions != 2 || len(c.Lines) != 3 {
		t.Errorf("v2 against v1: %+v", c)
	}
	// Two artifacts, the latest of each where no version's named.
	if c, code := compare("a=a1&b=a2"); code != 200 || c.A != "a1.v2" || c.B != "a2.v1" || c.File.Additions == 0 {
		t.Errorf("a1 against a2: %d %+v", code, c)
	}
	// The same version: no changes.
	if c, code := compare("a=a1.v2&b=a1.v2"); code != 200 || len(c.File.Hunks) != 0 {
		t.Errorf("the same: %d %+v", code, c)
	}
	for q, want := range map[string]int{"a=a1.v1": 400, "a=nope&b=a1": 400, "a=a1.v9&b=a1.v1": 404, "a=a1&b=a9": 404} {
		if _, code := compare(q); code != want {
			t.Errorf("%s: %d, want %d", q, code, want)
		}
	}
}

// An artifact a conversation proposed runs from a route of its own, from
// the conversation's records, under the same policy as a version: in the
// card under the reply, and opened larger. The conversation itself is
// sent without the HTML, which only the frame serves.
func TestProposalFrame(t *testing.T) {
	s := startTest(t, t.TempDir(), &fakeWorkspace{}, nil)
	talks := conversation.ForWorkspace(s.Root)
	if _, err := talks.OpenTalk(conversation.You, "Radial horizon gradient", time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := talks.Propose("w1", conversation.Proposal{Kind: conversation.ProposeDecision, What: "Keep it faint.", By: conversation.Agent, At: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if _, err := talks.Propose("w1", conversation.Proposal{Kind: conversation.ProposeArtifact, Title: "Spy", Note: "Reaches for everything.", HTML: spy, Size: len(spy), By: conversation.Agent, At: time.Now()}); err != nil {
		t.Fatal(err)
	}

	res := do(t, s, "GET", "/frame/w1/proposals/2", true, nil)
	if got := body(res); res.StatusCode != 200 || got != string(withTweaks([]byte(spy))) {
		t.Fatalf("the proposal: %d %q", res.StatusCode, got)
	}
	for k, v := range map[string]string{"Content-Type": "text/html; charset=utf-8", "Content-Security-Policy": FramePolicy, "X-Frame-Options": "SAMEORIGIN", "Cache-Control": "private, no-cache"} {
		if got := res.Header.Values(k); len(got) != 1 || got[0] != v {
			t.Errorf("%s: %q, want %q", k, got, v)
		}
	}
	if etag := res.Header.Get("ETag"); etag == "" {
		t.Error("no ETag")
	} else if res := do(t, s, "GET", "/frame/w1/proposals/2", true, func(r *http.Request) { r.Header.Set("If-None-Match", etag) }); res.StatusCode != http.StatusNotModified {
		t.Errorf("the same again: %d, want 304", res.StatusCode)
	}
	if res := do(t, s, "GET", "/frame/w1/proposals/2", false, nil); res.StatusCode != http.StatusUnauthorized || strings.Contains(body(res), "Spy") {
		t.Errorf("signed out: %d", res.StatusCode)
	}
	// Not an artifact, not there, not a conversation: a notice, in the
	// frame's policy.
	for _, p := range []string{"/frame/w1/proposals/1", "/frame/w1/proposals/3", "/frame/w9/proposals/1", "/frame/w1/proposals/x", "/frame/t1/proposals/1", "/frame/missions/nope/t1/proposals/1"} {
		res := do(t, s, "GET", p, true, nil)
		if res.StatusCode != http.StatusNotFound || res.Header.Get("Content-Security-Policy") != FramePolicy || strings.Contains(body(res), "Spy") {
			t.Errorf("%s: %d %q", p, res.StatusCode, res.Header.Get("Content-Security-Policy"))
		}
	}

	// The conversation's sent without the HTML: its size says what it is.
	res = do(t, s, "GET", "/api/conversations/w1", true, nil)
	b := body(res)
	if res.StatusCode != 200 || strings.Contains(b, "localStorage") {
		t.Fatalf("the conversation: %d %s", res.StatusCode, b)
	}
	var d struct {
		Proposals []conversation.Proposal `json:"proposals"`
	}
	json.Unmarshal([]byte(b), &d)
	if len(d.Proposals) != 2 || d.Proposals[1].Size != len(spy) || d.Proposals[1].Note != "Reaches for everything." || d.Proposals[1].HTML != "" {
		t.Errorf("proposals: %+v", d.Proposals)
	}
	// Declined, it isn't an artifact, but it stays in the conversation,
	// with its frame.
	if _, err := talks.SettleProposal("w1", 2, conversation.ProposalDeclined, "", conversation.You, time.Now()); err != nil {
		t.Fatal(err)
	}
	if res := do(t, s, "GET", "/frame/w1/proposals/2", true, nil); res.StatusCode != 200 {
		t.Errorf("declined: %d", res.StatusCode)
	}

	// A mission's conversation's, under the mission.
	if _, err := (mission.Store{Root: s.Root}).Create("Glow"); err != nil {
		t.Fatal(err)
	}
	mt := conversation.For(s.Root, "glow")
	c, err := mt.OpenTalk(conversation.You, "Show me", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := mt.Propose(c.ID, conversation.Proposal{Kind: conversation.ProposeArtifact, Title: "Glow", Note: "A first.", HTML: "<!doctype html><title>Glow</title>", By: conversation.Agent, At: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if res := do(t, s, "GET", "/frame/missions/glow/"+c.ID+"/proposals/1", true, nil); res.StatusCode != 200 || body(res) != string(withTweaks([]byte("<!doctype html><title>Glow</title>"))) || res.Header.Get("Content-Security-Policy") != FramePolicy {
		t.Errorf("a mission's: %d", res.StatusCode)
	}
}

// send is a signed-in request with a JSON body, decoded into out.
func send(t *testing.T, s *Server, path, payload string, out any) int {
	t.Helper()
	res := do(t, s, "POST", path, true, func(r *http.Request) {
		r.Body = io.NopCloser(strings.NewReader(payload))
		r.ContentLength = int64(len(payload))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Origin", s.Info().Origin())
	})
	if out != nil {
		json.NewDecoder(res.Body).Decode(out)
	}
	return res.StatusCode
}

// An artifact's lifetime, as the web UI asks it: each says when it
// expires, and Promote, Discard, Restore and Clear expired do it, as you.
func TestArtifactLifetimeAPI(t *testing.T) {
	s := startTest(t, t.TempDir(), &fakeWorkspace{}, nil)
	addArtifact(t, s.Root, "Horizon glow", "<!doctype html><title>One</title>", "<!doctype html><title>Two</title>")
	old := time.Now().Add(-31 * 24 * time.Hour)
	if _, err := (artifact.Store{Root: s.Root}).Add(context.Background(), artifact.New{Title: "Old sketch", HTML: []byte("<!doctype html><title>Old</title>"), Version: artifact.Version{By: "you"}, At: old}); err != nil {
		t.Fatal(err)
	}

	var ws Workspace
	json.NewDecoder(do(t, s, "GET", "/api/workspace", true, nil).Body).Decode(&ws)
	byID := map[string]ArtifactInfo{}
	for _, a := range ws.Artifacts {
		byID[a.ID] = a
	}
	if a := byID["a1"]; a.Expires == nil || a.Expired || !a.Expires.Equal(time.Date(2026, 11, 12, 18, 47, 0, 0, time.UTC).Add(artifact.DefaultExpiry)) {
		t.Errorf("a1 expires %v (%v)", a.Expires, a.Expired)
	}
	if a := byID["a2"]; a.Expires == nil || !a.Expired {
		t.Errorf("a2, 31 days old: expires %v (%v)", a.Expires, a.Expired)
	}

	var cleared ArtifactsCleared
	if code := send(t, s, "/api/artifacts/clear", "{}", &cleared); code != 200 || len(cleared.Local) != 1 || cleared.Local[0] != "a2" || len(cleared.Repo) != 0 {
		t.Errorf("clear: %d %+v", code, cleared)
	}
	var done ArtifactDone
	if code := send(t, s, "/api/artifacts/a2/restore", "{}", &done); code != 200 || done.Artifact.Status != "live" || done.Artifact.Expired || done.Artifact.Expires == nil {
		t.Errorf("restore: %d %+v", code, done.Artifact)
	}
	done = ArtifactDone{}
	if code := send(t, s, "/api/artifacts/a2/discard", `{"reason": "Not now."}`, &done); code != 200 || done.Artifact.Status != "discarded" || !done.Artifact.Restorable || done.Artifact.Discarded.Reason != "Not now." {
		t.Errorf("discard: %d %+v", code, done.Artifact)
	}

	done = ArtifactDone{}
	if code := send(t, s, "/api/artifacts/a1/promote", `{"version": 1}`, &done); code != 200 || done.Artifact.Status != "promoted" || done.Artifact.Promoted.Version != 1 || done.Artifact.Expires != nil {
		t.Errorf("promote: %d %+v", code, done.Artifact)
	}
	var e struct{ Error string }
	if code := send(t, s, "/api/artifacts/a1/promote", `{"into": "nosuch"}`, &e); code != http.StatusUnprocessableEntity || !strings.Contains(e.Error, "neither a mission nor a document") {
		t.Errorf("into nothing that's there: %d %q", code, e.Error)
	}
	if code := send(t, s, "/api/artifacts/a2/promote", `{}`, &e); code != http.StatusConflict || !strings.Contains(e.Error, "restore it first") {
		t.Errorf("promoting one discarded: %d %q", code, e.Error)
	}
	if code := send(t, s, "/api/artifacts/a9/discard", `{}`, nil); code != http.StatusNotFound {
		t.Errorf("a9: %d", code)
	}
}
