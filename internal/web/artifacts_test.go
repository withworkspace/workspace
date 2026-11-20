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

// framedAt follows a frame's address, signed in, to where its files are:
// the address it's sent to.
func framedAt(t *testing.T, s *Server, frame string) string {
	t.Helper()
	res := do(t, s, "GET", frame, true, nil)
	to := res.Header.Get("Location")
	if res.StatusCode != http.StatusFound || !strings.HasPrefix(to, framedRoot) {
		t.Fatalf("%s: %d, sent to %q", frame, res.StatusCode, to)
	}
	return to
}

// A version is served from a route of its own, with a policy that runs
// its scripts in an origin of their own, lets nothing leave, and lets
// only the web UI frame it: in place of the guard's, which frames
// nothing and runs no inline script. Everything else keeps the guard's.
func TestFrame(t *testing.T) {
	s := startTest(t, t.TempDir(), &fakeWorkspace{}, nil)
	addArtifact(t, s.Root, "Spy", spy, "<!doctype html><title>Two</title><p>two")

	// Signed in, the frame's sent to where the version's files are, under
	// a key, with what its address says after ?, which is the page's to
	// read.
	at := framedAt(t, s, "/frame/a1/v1?t.density=compact")
	key := s.frameKey("/frame/a1/v1")
	if at != "/framed/"+key+"/a1/v1/?t.density=compact" || len(key) != 32 {
		t.Fatalf("sent to %q", at)
	}
	// There, it's the file as it was written, with ws.tweaks written in
	// after its doctype. The frame that asks sends no cookie: the key in
	// the address is what lets it be read.
	res := do(t, s, "GET", at, false, nil)
	if got := body(res); res.StatusCode != 200 || got != string(withTweaks([]byte(spy))) || !strings.HasPrefix(got, "<!doctype html><script>") || !strings.HasSuffix(got, strings.TrimPrefix(spy, "<!doctype html>")) {
		t.Fatalf("v1: %d %q", res.StatusCode, got)
	}
	// The frame is sandboxed, an origin of no one's, so to the browser its
	// requests are another site's: the key is what they're read by.
	if res := do(t, s, "GET", at, false, func(r *http.Request) {
		r.Header.Set("Sec-Fetch-Site", "cross-site")
		r.Header.Set("Sec-Fetch-Mode", "cors")
	}); res.StatusCode != 200 {
		t.Errorf("from the sandboxed frame: %d", res.StatusCode)
	}
	h := res.Header
	own := s.Info().Origin() + "/framed/" + key + "/a1/v1/"
	want := map[string]string{
		"Content-Type": "text/html; charset=utf-8",
		"Content-Security-Policy": "sandbox allow-scripts; default-src 'none'; script-src 'unsafe-inline' " + own + "; style-src 'unsafe-inline' " + own +
			"; img-src data: blob: " + own + "; font-src data: " + own + "; media-src data: blob: " + own + "; connect-src " + own + "; frame-ancestors 'self'",
		"X-Frame-Options":        "SAMEORIGIN",
		"X-Content-Type-Options": "nosniff",
		"Referrer-Policy":        "no-referrer",
		"Cache-Control":          "private, no-cache",
	}
	for k, v := range want {
		if got := h.Get(k); got != v {
			t.Errorf("%s: %q, want %q", k, got, v)
		}
	}
	// The only place it loads from is its own files: nothing else of this
	// server, and nowhere else.
	csp := strings.ReplaceAll(h.Get("Content-Security-Policy"), own, "OWN")
	for _, never := range []string{"allow-same-origin", "allow-popups", "allow-top-navigation", "allow-forms", "allow-modals", "http", "*", "'self';"} {
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
	if res := do(t, s, "GET", at, false, func(r *http.Request) { r.Header.Set("If-None-Match", etag) }); res.StatusCode != http.StatusNotModified {
		t.Errorf("the same again: %d, want 304", res.StatusCode)
	}
	res = do(t, s, "GET", framedAt(t, s, "/frame/a1/v2"), false, nil)
	if b := body(res); res.StatusCode != 200 || !strings.Contains(b, "<p>two") || res.Header.Get("ETag") == etag {
		t.Errorf("v2: %d %q %s", res.StatusCode, b, res.Header.Get("ETag"))
	}

	// The frame's own address is signed in, like any other request: the
	// frame's request is same-site, so the cookie goes with it.
	if res := do(t, s, "GET", "/frame/a1/v1", false, nil); res.StatusCode != http.StatusUnauthorized || strings.Contains(body(res), "Spy") {
		t.Errorf("signed out: %d", res.StatusCode)
	}
	if res := do(t, s, "GET", "/frame/a1/v1", true, func(r *http.Request) { r.Host = "evil.example:" + strconv.Itoa(s.Info().Port) }); res.StatusCode != http.StatusMisdirectedRequest {
		t.Errorf("foreign host: %d", res.StatusCode)
	}
	if res := do(t, s, "GET", at, false, func(r *http.Request) { r.Host = "evil.example:" + strconv.Itoa(s.Info().Port) }); res.StatusCode != http.StatusMisdirectedRequest {
		t.Errorf("its files, by a foreign host: %d", res.StatusCode)
	}
	// A key reads its own frame's files and no other's: not another
	// version's, not another's, and nothing else of the server's.
	for _, p := range []string{
		"/framed/" + key + "/a1/v2/",
		"/framed/" + s.frameKey("/frame/a1/v1;x") + "/a1/v1;x/",
		"/framed/" + s.frameKey("/frame/a1/latest") + "/a1/latest/",
		"/framed/" + strings.Repeat("0", 32) + "/a1/v1/",
		"/framed/" + key + "/",
		"/framed/" + key,
		"/framed/",
	} {
		if res := do(t, s, "GET", p, false, nil); res.StatusCode != http.StatusUnauthorized || strings.Contains(body(res), "Spy") {
			t.Errorf("%s, signed out: %d", p, res.StatusCode)
		}
	}
	if res := do(t, s, "GET", "/api/workspace", false, func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+key) }); res.StatusCode != http.StatusUnauthorized {
		t.Errorf("the API, by a frame's key: %d", res.StatusCode)
	}
	if res := do(t, s, "POST", at, false, func(r *http.Request) {
		r.Header.Set("Origin", s.Info().Origin())
		r.Header.Set("Content-Type", "application/json")
	}); res.StatusCode == http.StatusOK {
		t.Errorf("POST to its files: %d", res.StatusCode)
	}
	// A key that isn't the frame's, in a tab from before the token
	// changed: signed in, it's sent to the one that is.
	if res := do(t, s, "GET", "/framed/"+strings.Repeat("0", 32)+"/a1/v1/?t.x=1", true, nil); res.StatusCode != http.StatusFound || res.Header.Get("Location") != "/framed/"+key+"/a1/v1/?t.x=1" {
		t.Errorf("an old key, signed in: %d %q", res.StatusCode, res.Header.Get("Location"))
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

// site is a version that's more than its page: a stylesheet, a module
// and what it imports, an image, data, and a second page.
func site(sheet string) artifact.Files {
	return artifact.Files{
		{Path: "index.html", Data: []byte("<!doctype html>\n<title>Site</title>\n<link rel=\"stylesheet\" href=\"css/site.css\">\n<script type=\"module\" src=\"app.mjs\"></script>\n<img src=\"img/dot.png\">\n<a href=\"about.html\">About</a>\n")},
		{Path: "about.html", Data: []byte("<!doctype html>\n<title>About</title>\n<link rel=\"stylesheet\" href=\"css/site.css\">\n")},
		{Path: "css/site.css", Data: []byte(sheet)},
		{Path: "app.mjs", Data: []byte("import { n } from './lib/n.mjs';\nfetch('data.json');\n")},
		{Path: "lib/n.mjs", Data: []byte("export const n = 1;\n")},
		{Path: "data.json", Data: []byte(`{"n":1}`)},
		{Path: "img/dot.png", Data: []byte("\x89PNG\r\n\x1a\n\x00\x01\x02")},
		{Path: "notes.weird", Data: []byte("who knows")},
		{Path: "img/my logo.svg", Data: []byte("<svg xmlns=\"http://www.w3.org/2000/svg\"/>")},
	}
}

// A version's files are each served at their path under the frame's
// key, as what their names say they are, to whoever has the key: the
// frame asks for them with no cookie, and for a module, a font and a
// fetch with an origin that's no one's. A page among them gets ws.tweaks
// and the frame's policy.
func TestFrameFiles(t *testing.T) {
	s := startTest(t, t.TempDir(), &fakeWorkspace{}, nil)
	st := artifact.Store{Root: s.Root}
	at := time.Date(2026, 11, 12, 18, 46, 0, 0, time.UTC)
	if _, err := st.Add(context.Background(), artifact.New{Title: "Site", Files: site("h1 { color: tomato }\n"), Version: artifact.Version{By: "you"}, At: at}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := st.AddVersion(context.Background(), "a1", artifact.Revision{Files: site("h1 { color: teal }\n"), Version: artifact.Version{By: "you"}, At: at.Add(time.Minute)}); err != nil {
		t.Fatal(err)
	}
	base := strings.TrimSuffix(framedAt(t, s, "/frame/a1/v1"), "/")
	own := s.Info().Origin() + base + "/"
	for file, want := range map[string]struct{ ctype, has string }{
		"/":                  {"text/html; charset=utf-8", "<title>Site</title>"},
		"/index.html":        {"text/html; charset=utf-8", "<title>Site</title>"},
		"/about.html":        {"text/html; charset=utf-8", "<title>About</title>"},
		"/css/site.css":      {"text/css; charset=utf-8", "tomato"},
		"/app.mjs":           {"text/javascript; charset=utf-8", "import { n }"},
		"/lib/n.mjs":         {"text/javascript; charset=utf-8", "export const n"},
		"/data.json":         {"application/json", `{"n":1}`},
		"/img/dot.png":       {"image/png", "\x89PNG"},
		"/notes.weird":       {"application/octet-stream", "who knows"},
		"/img/my%20logo.svg": {"image/svg+xml", "<svg"},
	} {
		res := do(t, s, "GET", base+file, false, func(r *http.Request) { r.Header.Set("Origin", "null") })
		b := body(res)
		if res.StatusCode != 200 || res.Header.Get("Content-Type") != want.ctype || !strings.Contains(b, want.has) {
			t.Errorf("%s: %d %q %.40q", file, res.StatusCode, res.Header.Get("Content-Type"), b)
		}
		if res.Header.Get("Access-Control-Allow-Origin") != "*" || res.Header.Get("X-Content-Type-Options") != "nosniff" || res.Header.Get("ETag") == "" {
			t.Errorf("%s: %v", file, res.Header)
		}
		if csp := res.Header.Get("Content-Security-Policy"); !strings.HasPrefix(csp, "sandbox allow-scripts; default-src 'none'") || !strings.Contains(csp, "script-src 'unsafe-inline' "+own+";") {
			t.Errorf("%s's policy: %s", file, csp)
		}
		if page := strings.HasSuffix(want.ctype, "html; charset=utf-8"); page != strings.Contains(b, "<script>") {
			t.Errorf("%s: ws.tweaks written in: %v", file, !page)
		}
	}
	// v2's are v2's, under its own key.
	two := framedAt(t, s, "/frame/a1/v2")
	if res := do(t, s, "GET", two+"css/site.css", false, nil); res.StatusCode != 200 || !strings.Contains(body(res), "teal") {
		t.Errorf("v2's stylesheet: %d", res.StatusCode)
	}
	// What isn't one of its files says so, and nothing outside it is
	// reached by a path.
	for _, file := range []string{"/nope.css", "/css", "/css/", "/.git/HEAD", "/artifact.toml", "/..%2f..%2fartifact.toml", "/css/../../v2/index.html"} {
		res := do(t, s, "GET", base+file, false, nil)
		if b := body(res); res.StatusCode == 200 || strings.Contains(b, "teal") || strings.Contains(b, "versions") {
			t.Errorf("%s: %d %.60q", file, res.StatusCode, b)
		}
	}
	// The same file again is a 304, without its bytes.
	res := do(t, s, "GET", base+"/img/dot.png", false, nil)
	if res := do(t, s, "GET", base+"/img/dot.png", false, func(r *http.Request) { r.Header.Set("If-None-Match", res.Header.Get("ETag")) }); res.StatusCode != http.StatusNotModified {
		t.Errorf("the image again: %d", res.StatusCode)
	}

	// The source is each file, by its path, with the version's files to
	// choose from; one that isn't text says so.
	var src ArtifactSource
	json.NewDecoder(do(t, s, "GET", "/api/artifacts/a1/source?v=2&file=css/site.css", true, nil).Body).Decode(&src)
	if src.Version != 2 || src.File != "css/site.css" || src.Text != "h1 { color: teal }\n" || src.Binary || len(src.Files) != 9 || src.Files[0].Path != "about.html" {
		t.Errorf("a file's source %+v", src)
	}
	src = ArtifactSource{}
	json.NewDecoder(do(t, s, "GET", "/api/artifacts/a1/source?file=img/dot.png", true, nil).Body).Decode(&src)
	if !src.Binary || src.Text != "" || src.Size != 11 {
		t.Errorf("an image's source %+v", src)
	}
	if res := do(t, s, "GET", "/api/artifacts/a1/source?file=nope.css", true, nil); res.StatusCode != http.StatusNotFound {
		t.Errorf("a file it hasn't: %d", res.StatusCode)
	}
	// And the two versions differ by the one file that changed.
	var c ArtifactCompare
	json.NewDecoder(do(t, s, "GET", "/api/artifacts/compare?a=a1.v1&b=a1.v2", true, nil).Body).Decode(&c)
	if len(c.Files) != 1 || c.Files[0].File.Path != "css/site.css" || c.Files[0].File.Status != "modified" || c.Files[0].File.Additions != 1 || c.Files[0].File.Deletions != 1 {
		t.Errorf("v1 against v2: %+v", c)
	}
	var d ArtifactDetail
	json.NewDecoder(do(t, s, "GET", "/api/artifacts/a1", true, nil).Body).Decode(&d)
	if len(d.All) != 2 || d.All[0].Files != 9 || d.All[0].Size != site("h1 { color: tomato }\n").Size() {
		t.Errorf("versions %+v", d.All)
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
		a.Home != nil || !strings.HasPrefix(a.Where, ".ws/artifacts/") || !strings.HasSuffix(a.Where, "-horizon-glow/") || len(a.From) != 1 || a.From[0].From != "glow" || a.From[0].Title != "The glow" {
		t.Errorf("listed %+v (latest %+v)", a, a.Latest)
	}

	var d ArtifactDetail
	res := do(t, s, "GET", "/api/artifacts/a1", true, nil)
	if err := json.NewDecoder(res.Body).Decode(&d); err != nil || res.StatusCode != 200 {
		t.Fatalf("detail: %d %v", res.StatusCode, err)
	}
	if len(d.All) != 2 || d.All[0].N != 1 || d.All[0].By != "you" || d.All[1].BasedOn != 1 || d.All[1].By != "c1" || d.All[1].Size == 0 || d.All[1].Files != 1 || d.All[0].Note != "v1's note" {
		t.Errorf("versions %+v", d.All)
	}
	// With no repo for promoted ones, promoting it would leave it here.
	if d.HomeOffered != "" {
		t.Errorf("offered a home, %q, with none to offer", d.HomeOffered)
	}
	for _, p := range []string{"/api/artifacts/a9", "/api/artifacts/nope", "/api/artifacts/a1.v1"} {
		if res := do(t, s, "GET", p, true, nil); res.StatusCode != http.StatusNotFound {
			t.Errorf("%s: %d", p, res.StatusCode)
		}
	}

	var src ArtifactSource
	json.NewDecoder(do(t, s, "GET", "/api/artifacts/a1/source?v=1", true, nil).Body).Decode(&src)
	if src.Version != 1 || src.Text != "<!doctype html><title>One</title>" || src.File != "index.html" || len(src.Files) != 1 || src.Files[0].Path != "index.html" {
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

// Two versions compare by their files, each that differs diffed as a
// review's file is: of one artifact or of two, a's against b's, each
// named as it was read (the latest, where a ref has no version), with
// b's lines for what's unchanged between the changes.
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
	if code != 200 || c.A != "a1.v1" || c.B != "a1.v2" || len(c.Files) != 1 {
		t.Fatalf("v1 against v2: %d %+v", code, c)
	}
	if f := c.Files[0].File; f.Path != "index.html" || f.Status != "modified" || f.Additions != 2 || f.Deletions != 1 || len(f.Hunks) != 1 {
		t.Fatalf("v1 against v2: %+v", f)
	}
	if lines := c.Files[0].Lines; len(lines) != 4 || lines[3] != "<p>light</p>" {
		t.Errorf("v2's lines %q", lines)
	}
	// Swapped, it's the other way about.
	if c, _ := compare("a=a1.v2&b=a1.v1"); len(c.Files) != 1 || c.Files[0].File.Additions != 1 || c.Files[0].File.Deletions != 2 || len(c.Files[0].Lines) != 3 {
		t.Errorf("v2 against v1: %+v", c)
	}
	// Two artifacts, the latest of each where no version's named.
	if c, code := compare("a=a1&b=a2"); code != 200 || c.A != "a1.v2" || c.B != "a2.v1" || len(c.Files) != 1 || c.Files[0].File.Additions == 0 {
		t.Errorf("a1 against a2: %d %+v", code, c)
	}
	// The same version: nothing differs.
	if c, code := compare("a=a1.v2&b=a1.v2"); code != 200 || c.Files == nil || len(c.Files) != 0 {
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

	// One proposed with files is kept beside the artifacts, the proposal
	// naming them, and its frame is sent to where they are, as a
	// version's is: the workspace's conversation's, and a mission's.
	files := site("h1 { color: tomato }\n")
	tree, err := (artifact.Store{Root: s.Root}).Propose(context.Background(), files, "Site", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	withFiles := conversation.Proposal{Kind: conversation.ProposeArtifact, Title: "Site", Note: "With files.", Tree: tree, Size: files.Size(),
		Files: []conversation.ProposedFile{{Path: "index.html", Size: 1}}, By: conversation.Agent, At: time.Now()}
	if _, err := talks.Propose("w1", withFiles); err != nil {
		t.Fatal(err)
	}
	if _, err := mt.Propose(c.ID, withFiles); err != nil {
		t.Fatal(err)
	}
	for _, frame := range []string{"/frame/w1/proposals/3", "/frame/missions/glow/" + c.ID + "/proposals/2"} {
		base := framedAt(t, s, frame)
		if want := "/framed/" + s.frameKey(frame) + strings.TrimPrefix(frame, "/frame") + "/"; base != want {
			t.Errorf("%s: sent to %q, want %q", frame, base, want)
		}
		res := do(t, s, "GET", base, false, nil)
		if b := body(res); res.StatusCode != 200 || !strings.Contains(b, "<title>Site</title>") || !strings.Contains(b, "<script>") || !strings.Contains(res.Header.Get("Content-Security-Policy"), "connect-src "+s.Info().Origin()+base) {
			t.Errorf("%s's page: %d %.60q %s", frame, res.StatusCode, b, res.Header.Get("Content-Security-Policy"))
		}
		if res := do(t, s, "GET", base+"css/site.css", false, nil); res.StatusCode != 200 || !strings.Contains(body(res), "tomato") || res.Header.Get("Content-Type") != "text/css; charset=utf-8" {
			t.Errorf("%s's stylesheet: %d", frame, res.StatusCode)
		}
		if res := do(t, s, "GET", base+"nope.css", false, nil); res.StatusCode != http.StatusNotFound {
			t.Errorf("%s's nope.css: %d", frame, res.StatusCode)
		}
	}
	// One proposal's key reads no other's.
	other := strings.Replace(framedAt(t, s, "/frame/w1/proposals/3"), "/proposals/3/", "/proposals/2/", 1)
	if res := do(t, s, "GET", other, false, nil); res.StatusCode != http.StatusUnauthorized {
		t.Errorf("another proposal, by this one's key: %d", res.StatusCode)
	}
	// The conversation's sent with what its files are, not where they're
	// kept.
	json.Unmarshal([]byte(body(do(t, s, "GET", "/api/conversations/w1", true, nil))), &d)
	if len(d.Proposals) != 3 || d.Proposals[2].Tree != "" || len(d.Proposals[2].Files) != 1 {
		t.Errorf("proposals: %+v", d.Proposals)
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
	if code := send(t, s, "/api/artifacts/clear", "{}", &cleared); code != 200 || len(cleared.Local) != 1 || cleared.Local[0] != "a2" {
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

	var e struct{ Error string }
	done = ArtifactDone{}
	if code := send(t, s, "/api/artifacts/a1/promote", `{"version": 1}`, &done); code != 200 || done.Artifact.Status != "promoted" || done.Artifact.Promoted.Version != 1 || done.Artifact.Expires != nil || done.Home != "" || done.Artifact.Home != nil {
		t.Errorf("promote: %d %+v", code, done)
	}
	// A home that isn't a folder of one of the workspace's repos is said,
	// and nothing's promoted to it.
	if code := send(t, s, "/api/artifacts/a1/promote", `{"home": "nowhere:artifacts/glow/"}`, &e); code != http.StatusUnprocessableEntity || !strings.Contains(e.Error, "nowhere isn't one of the workspace's repos") {
		t.Errorf("a home in no repo: %d %q", code, e.Error)
	}
	if code := send(t, s, "/api/artifacts/a1/promote", `{"home": "artifacts/glow"}`, &e); code != http.StatusUnprocessableEntity || !strings.Contains(e.Error, "isn't a folder in a repo") {
		t.Errorf("a home that's no folder: %d %q", code, e.Error)
	}
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
