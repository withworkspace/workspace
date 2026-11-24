package web

import (
	"bufio"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"testing/fstest"
	"time"

	"github.com/withworkspace/workspace/internal/config"
	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/job"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/store/storetest"
	"github.com/withworkspace/workspace/internal/workspace"
)

var testApp = fstest.MapFS{
	"index.html":    {Data: []byte("<!doctype html><title>app</title>")},
	"assets/app.js": {Data: []byte("console.log('app')")},
	// What the shell of a plugin's page holds: the web build writes it.
	"plugin-shell.css": {Data: []byte(":root{--bg:#fff;--ws-bg:var(--bg)}")},
}

// fakeWorkspace is a workspace with one mission, if it's given a title,
// whose title a test can change.
type fakeWorkspace struct {
	title      string
	root, slug string
}

// make makes the workspace's mission in root.
func (f *fakeWorkspace) make(t *testing.T, root string) {
	t.Helper()
	f.root = root
	if f.title == "" {
		return
	}
	m, err := (mission.Store{Root: root}).Create(f.title)
	if err != nil {
		t.Fatal(err)
	}
	f.slug = m.Slug
}

func (f *fakeWorkspace) set(title string) {
	(mission.Store{Root: f.root}).Update(f.slug, func(m *mission.Mission) error { m.Title = title; return nil })
}

func startTest(t *testing.T, root string, fw *fakeWorkspace, hold func() func()) *Server {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	storetest.Own(t, root)
	fw.make(t, root)
	s := &Server{Root: root, Hold: hold, Poll: 20 * time.Millisecond, Assets: testApp, listener: ln}
	s.App = testOps(s, ops.Options{})
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	return s
}

// testOps is s's workspace's operations, as the daemon has them (with no
// config but what cfg says, if it's given), on s's clock.
func testOps(s *Server, o ops.Options, cfg ...func(*config.Config, *workspace.Workspace)) *ops.Workspace {
	c, ws := &config.Config{}, &workspace.Workspace{Root: s.Root}
	for _, f := range cfg {
		f(c, ws)
	}
	if o.Now == nil {
		o.Now = s.now
	}
	return ops.New(c, ws, nil, o)
}

// fakeApp is s's operations, with some of what they read or do as a test
// says instead.
type fakeApp struct {
	*ops.Workspace
	artifact     func(slug, ref string) (*ops.DocText, error)
	doc          func(ref string) (*ops.DocText, error)
	saveDocument func(slug, ref, base, text, message string) (*ops.DocSaved, error)
	accept       func(slug, id string) (string, error)
	filePage     func(id string, r ops.PageFile) (*ops.PageFiling, error)
	openFile     func(ref string) (string, *ops.DocMission, error)
	savePage     func(id, message string) (*ops.DocSaved, error)
	bringIn      func(id string) (*ops.PageBroughtIn, error)
	capsuleDir   func(slug, repo, name string) (string, string, error)
	worktree     func(repo, name string) (string, string, error)
	repos        func() []ops.Repo
	worktrees    func(repo string) ([]ops.Worktree, error)
	crew         ops.CrewPages
	sendToCrew   func(slug, ref string, ids []string) (string, error)
	settle       func(slug, id string, n int, accept bool, e ops.ProposalEdits) (string, string, error)
	askPR        func(repo string, number int, q ops.PRQuestion) (*conversation.Conversation, error)
	dockPR       func(repo string, number int) (string, error)
}

func (f *fakeApp) Crew() ops.CrewPages {
	if f.crew != nil {
		return f.crew
	}
	return f.Workspace.Crew()
}

func (f *fakeApp) SendToCrew(slug, ref string, ids []string) (string, error) {
	if f.sendToCrew != nil {
		return f.sendToCrew(slug, ref, ids)
	}
	return f.Workspace.SendToCrew(slug, ref, ids)
}

func (f *fakeApp) SettleProposal(slug, id string, n int, accept bool, e ops.ProposalEdits) (string, string, error) {
	if f.settle != nil {
		return f.settle(slug, id, n, accept, e)
	}
	return f.Workspace.SettleProposal(slug, id, n, accept, e)
}

func (f *fakeApp) AsksPRs() bool { return f.askPR != nil || f.Workspace.AsksPRs() }

func (f *fakeApp) AskPR(repo string, number int, q ops.PRQuestion) (*conversation.Conversation, error) {
	if f.askPR != nil {
		return f.askPR(repo, number, q)
	}
	return f.Workspace.AskPR(repo, number, q)
}

func (f *fakeApp) DockPR(repo string, number int) (string, error) {
	if f.dockPR != nil {
		return f.dockPR(repo, number)
	}
	return f.Workspace.DockPR(repo, number)
}

// crewFuncs is crew's pages as a test says: what it doesn't say, there's
// no crew for.
type crewFuncs struct {
	ops.NoCrew
	say    func(slug, id, text string) (string, error)
	answer func(slug, id string, a ops.CrewAnswer) (string, error)
}

func (c crewFuncs) Say(slug, id, text string) (string, error) {
	if c.say != nil {
		return c.say(slug, id, text)
	}
	return c.NoCrew.Say(slug, id, text)
}

func (c crewFuncs) Answer(slug, id string, a ops.CrewAnswer) (string, error) {
	if c.answer != nil {
		return c.answer(slug, id, a)
	}
	return c.NoCrew.Answer(slug, id, a)
}

func (f *fakeApp) CapsuleDir(slug, repo, name string) (string, string, error) {
	if f.capsuleDir != nil {
		return f.capsuleDir(slug, repo, name)
	}
	return f.Workspace.CapsuleDir(slug, repo, name)
}

func (f *fakeApp) Worktree(repo, name string) (string, string, error) {
	if f.worktree != nil {
		return f.worktree(repo, name)
	}
	return f.Workspace.Worktree(repo, name)
}

func (f *fakeApp) Repos() []ops.Repo {
	if f.repos != nil {
		return f.repos()
	}
	return f.Workspace.Repos()
}

func (f *fakeApp) Worktrees(repo string) ([]ops.Worktree, error) {
	if f.worktrees != nil {
		return f.worktrees(repo)
	}
	return f.Workspace.Worktrees(repo)
}

func (f *fakeApp) Artifact(slug, ref string) (*ops.DocText, error) {
	if f.artifact != nil {
		return f.artifact(slug, ref)
	}
	return f.Workspace.Artifact(slug, ref)
}

func (f *fakeApp) Doc(ref string) (*ops.DocText, error) {
	if f.doc != nil {
		return f.doc(ref)
	}
	return f.Workspace.Doc(ref)
}

func (f *fakeApp) SaveDocument(slug, ref, base, text, message string) (*ops.DocSaved, error) {
	if f.saveDocument != nil {
		return f.saveDocument(slug, ref, base, text, message)
	}
	return f.Workspace.SaveDocument(slug, ref, base, text, message)
}

func (f *fakeApp) AcceptSuggestion(slug, id string) (string, error) {
	if f.accept != nil {
		return f.accept(slug, id)
	}
	return f.Workspace.AcceptSuggestion(slug, id)
}

func (f *fakeApp) FilePage(id string, r ops.PageFile) (*ops.PageFiling, error) {
	if f.filePage != nil {
		return f.filePage(id, r)
	}
	return f.Workspace.FilePage(id, r)
}

func (f *fakeApp) OpenFile(ref string) (string, *ops.DocMission, error) {
	if f.openFile != nil {
		return f.openFile(ref)
	}
	return f.Workspace.OpenFile(ref)
}

func (f *fakeApp) SavePageToFile(id, message string) (*ops.DocSaved, error) {
	if f.savePage != nil {
		return f.savePage(id, message)
	}
	return f.Workspace.SavePageToFile(id, message)
}

func (f *fakeApp) BringIn(id string) (*ops.PageBroughtIn, error) {
	if f.bringIn != nil {
		return f.bringIn(id)
	}
	return f.Workspace.BringIn(id)
}

// fake is s's App as a fakeApp, made one if it isn't yet.
func fake(s *Server) *fakeApp {
	if f, ok := s.App.(*fakeApp); ok {
		return f
	}
	f := &fakeApp{Workspace: s.App.(*ops.Workspace)}
	s.App = f
	return f
}

// setOps gives s the operations w, keeping what a test has faked.
func setOps(s *Server, w *ops.Workspace) {
	defer s.views.forget() // changed in place, a fakeApp is the same App
	if f, ok := s.App.(*fakeApp); ok {
		f.Workspace = w
		return
	}
	s.App = w
}

// answersInto has s's operations note each conversation they have the
// daemon answer (of the kinds named; any, for none) as slug/id, into got.
func answersInto(s *Server, got *[]string, kinds ...string) {
	setOps(s, testOps(s, ops.Options{Jobs: ops.JobsOf(func(j job.Job) {
		if len(kinds) == 0 || slices.Contains(kinds, j.Kind) {
			*got = append(*got, j.Mission+"/"+j.ID)
		}
	}, job.Answer, job.Thread, job.Draft)}))
}

// idsAnsweredInto is answersInto, noting each by its id alone.
func idsAnsweredInto(s *Server, got *[]string, kinds ...string) {
	setOps(s, testOps(s, ops.Options{Jobs: ops.JobsOf(func(j job.Job) {
		if len(kinds) == 0 || slices.Contains(kinds, j.Kind) {
			*got = append(*got, j.ID)
		}
	}, job.Answer, job.Thread, job.Draft)}))
}

func noRedirects() *http.Client {
	return &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}

func do(t *testing.T, s *Server, method, path string, signedIn bool, edit func(*http.Request)) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(method, s.Info().Origin()+path, strings.NewReader("{}"))
	if signedIn {
		req.AddCookie(&http.Cookie{Name: s.cookieName(), Value: s.Info().Token})
	}
	if edit != nil {
		edit(req)
	}
	res, err := noRedirects().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { res.Body.Close() })
	return res
}

func body(res *http.Response) string {
	data, _ := io.ReadAll(res.Body)
	return string(data)
}

func TestDefaultPort(t *testing.T) {
	a, b := DefaultPort("/Users/me/work"), DefaultPort("/Users/me/work/")
	if a != b || a < portBase || a >= portBase+portRange {
		t.Errorf("ports = %d, %d", a, b)
	}
	if DefaultPort("/Users/me/other") == a {
		t.Log("two workspaces share a port by chance; the next free one is used")
	}
}

// .ws/web.json says where the UI is, readable only by you; the token
// outlives the server, so a signed-in browser stays signed in.
func TestStart_InfoAndToken(t *testing.T) {
	root := t.TempDir()
	s := startTest(t, root, &fakeWorkspace{}, nil)
	info, err := ReadInfo(root)
	if err != nil || info.Port != s.Info().Port || info.Pid != os.Getpid() || len(info.Token) != 64 {
		t.Fatalf("info = %+v, %v", info, err)
	}
	if st, _ := os.Stat(InfoPath(root)); st.Mode().Perm() != 0o600 {
		t.Errorf("web.json mode = %v", st.Mode().Perm())
	}
	s.Close()
	after, _ := ReadInfo(root)
	if after.Port != 0 || after.Pid != 0 || after.Token != info.Token {
		t.Errorf("after close = %+v", after)
	}
	again := startTest(t, root, &fakeWorkspace{}, nil)
	if again.Info().Token != info.Token {
		t.Error("restarting changed the token")
	}
}

// With no [web] port, the workspace's own port, or the next free one; a
// configured port that's taken is an error.
func TestListen_Ports(t *testing.T) {
	root := t.TempDir()
	own := DefaultPort(root)
	blocker, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(own))
	if err != nil {
		t.Skipf("port %d is busy already: %v", own, err)
	}
	defer blocker.Close()
	s := &Server{Root: root, Assets: testApp}
	s.App = testOps(s, ops.Options{})
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if p := s.Info().Port; p == own || p < portBase || p >= portBase+portRange {
		t.Errorf("port = %d with %d taken", p, own)
	}
	if err := (&Server{Root: t.TempDir(), Port: own, Assets: testApp}).Start(); err == nil || !strings.Contains(err.Error(), "[web] port") {
		t.Errorf("taken [web] port: %v", err)
	}
}

func TestAuth(t *testing.T) {
	s := startTest(t, t.TempDir(), &fakeWorkspace{}, nil)
	if res := do(t, s, "GET", "/auth?t=wrong", false, nil); res.StatusCode != http.StatusForbidden {
		t.Errorf("wrong token: %d", res.StatusCode)
	}
	res := do(t, s, "GET", "/auth?t="+s.Info().Token+"&next=/missions/x", false, nil)
	if res.StatusCode != http.StatusSeeOther || res.Header.Get("Location") != "/missions/x" {
		t.Errorf("sign in: %d to %q", res.StatusCode, res.Header.Get("Location"))
	}
	var cookie *http.Cookie
	for _, c := range res.Cookies() {
		if c.Name == s.cookieName() {
			cookie = c
		}
	}
	if cookie == nil || cookie.Value != s.Info().Token || !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode {
		t.Errorf("cookie = %+v", cookie)
	}
	if res.Header.Get("Referrer-Policy") != "no-referrer" {
		t.Error("the sign-in URL carries the token: no referrer")
	}
	// A sign-in link can't send the browser elsewhere.
	for _, next := range []string{"//evil.example", "https://evil.example", "/\\evil.example", ""} {
		res := do(t, s, "GET", "/auth?t="+s.Info().Token+"&next="+next, false, nil)
		if res.Header.Get("Location") != "/" {
			t.Errorf("next %q went to %q", next, res.Header.Get("Location"))
		}
	}
	// Two workspaces' cookies don't collide (cookies ignore the port).
	other := startTest(t, t.TempDir(), &fakeWorkspace{}, nil)
	if other.cookieName() == s.cookieName() {
		t.Error("two workspaces share a cookie name")
	}
}

func TestGuard(t *testing.T) {
	s := startTest(t, t.TempDir(), &fakeWorkspace{title: "Cache tokens"}, nil)

	res := do(t, s, "GET", "/api/workspace", false, nil)
	if res.StatusCode != http.StatusUnauthorized || !strings.Contains(body(res), "ws web") {
		t.Errorf("signed out API: %d", res.StatusCode)
	}
	if res := do(t, s, "GET", "/missions/m", false, nil); res.StatusCode != http.StatusUnauthorized || strings.Contains(body(res), "<title>app") {
		t.Errorf("signed out page: %d", res.StatusCode)
	}

	res = do(t, s, "GET", "/api/workspace", true, nil)
	var ws Workspace
	if err := json.NewDecoder(res.Body).Decode(&ws); err != nil || res.StatusCode != 200 || ws.Missions[0].Title != "Cache tokens" {
		t.Errorf("signed in: %d %+v %v", res.StatusCode, ws, err)
	}
	bearer := func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+s.Info().Token) }
	if res := do(t, s, "GET", "/api/workspace", false, bearer); res.StatusCode != 200 {
		t.Errorf("bearer: %d", res.StatusCode)
	}

	// DNS rebinding: a name that resolves here isn't us.
	if res := do(t, s, "GET", "/api/workspace", true, func(r *http.Request) { r.Host = "evil.example:" + strconv.Itoa(s.Info().Port) }); res.StatusCode != http.StatusMisdirectedRequest {
		t.Errorf("foreign host: %d", res.StatusCode)
	}
	if res := do(t, s, "GET", "/api/workspace", true, func(r *http.Request) { r.Host = "localhost:" + strconv.Itoa(s.Info().Port) }); res.StatusCode != 200 {
		t.Errorf("localhost: %d", res.StatusCode)
	}

	// Writes need our Origin (another port on 127.0.0.1 is another site's
	// page, and SameSite doesn't stop it) and JSON.
	json := func(origin string) func(*http.Request) {
		return func(r *http.Request) {
			r.Header.Set("Content-Type", "application/json")
			if origin != "" {
				r.Header.Set("Origin", origin)
			}
		}
	}
	if res := do(t, s, "POST", "/api/anything", true, json("")); res.StatusCode != http.StatusForbidden {
		t.Errorf("no origin: %d", res.StatusCode)
	}
	if res := do(t, s, "POST", "/api/anything", true, json("http://127.0.0.1:3000")); res.StatusCode != http.StatusForbidden {
		t.Errorf("another port's page: %d", res.StatusCode)
	}
	if res := do(t, s, "POST", "/api/anything", true, func(r *http.Request) {
		r.Header.Set("Origin", s.Info().Origin())
		r.Header.Set("Content-Type", "text/plain")
	}); res.StatusCode != http.StatusUnsupportedMediaType {
		t.Errorf("a form post: %d", res.StatusCode)
	}
	if res := do(t, s, "POST", "/api/anything", true, json(s.Info().Origin())); res.StatusCode != http.StatusNotFound {
		t.Errorf("an allowed write reaches the API: %d", res.StatusCode)
	}
}

// Another site's page has our cookie along when it sends the browser to
// a link (Lax), and when it's a page on another port of this machine, or
// a run under the workspace's name (the same site): what that reaches is
// the app's pages, followed as a link, and nothing that does anything.
func TestCrossSite(t *testing.T) {
	s := startTest(t, t.TempDir(), &fakeWorkspace{title: "Cache tokens"}, nil)
	from := func(site, mode string) func(*http.Request) {
		return func(r *http.Request) {
			r.Header.Set("Sec-Fetch-Site", site)
			r.Header.Set("Sec-Fetch-Mode", mode)
		}
	}
	for _, c := range []struct {
		method, path, site, mode string
		code                     int
	}{
		{"GET", "/api/workspace", "cross-site", "navigate", http.StatusForbidden},
		{"GET", "/api/workspace", "same-site", "no-cors", http.StatusForbidden},
		{"GET", "/api/workspace", "same-site", "navigate", http.StatusForbidden},
		{"GET", "/plugins/jira/do-something", "cross-site", "navigate", http.StatusForbidden},
		{"GET", "/plugins/jira/do-something", "same-site", "no-cors", http.StatusForbidden},
		{"GET", "/missions/m", "cross-site", "no-cors", http.StatusForbidden},
		{"POST", "/missions/m", "cross-site", "navigate", http.StatusForbidden},
		// A link to the app from elsewhere still opens it.
		{"GET", "/missions/m", "cross-site", "navigate", http.StatusOK},
		{"GET", "/missions/m", "same-site", "navigate", http.StatusOK},
		// The app's own, the browser's own (a typed address, a bookmark),
		// and a client that isn't a browser.
		{"GET", "/api/workspace", "same-origin", "cors", http.StatusOK},
		{"GET", "/api/workspace", "none", "navigate", http.StatusOK},
		{"GET", "/api/workspace", "", "", http.StatusOK},
	} {
		if res := do(t, s, c.method, c.path, true, from(c.site, c.mode)); res.StatusCode != c.code {
			t.Errorf("%s %s, %s %s: %d, want %d", c.method, c.path, c.site, c.mode, res.StatusCode, c.code)
		}
	}
	res := do(t, s, "GET", "/api/workspace", true, from("cross-site", "no-cors"))
	if !strings.Contains(body(res), "another site") {
		t.Errorf("says why: %d", res.StatusCode)
	}
}

// The app's files are served as they are, hashed assets for good; any
// other path is the app (it routes), but not under /api.
func TestApp(t *testing.T) {
	s := startTest(t, t.TempDir(), &fakeWorkspace{}, nil)
	res := do(t, s, "GET", "/missions/m", true, nil)
	if b := body(res); res.StatusCode != 200 || !strings.Contains(b, "<title>app") || res.Header.Get("Cache-Control") != "no-cache" {
		t.Errorf("app route: %d %q", res.StatusCode, b)
	}
	res = do(t, s, "GET", "/assets/app.js", true, nil)
	if body(res) != "console.log('app')" || !strings.Contains(res.Header.Get("Cache-Control"), "immutable") {
		t.Errorf("asset: %q", res.Header.Get("Cache-Control"))
	}
	if res := do(t, s, "GET", "/api/nope", true, nil); res.StatusCode != http.StatusNotFound || !strings.Contains(body(res), `"error"`) {
		t.Errorf("unknown API: %d", res.StatusCode)
	}
}

// The app's page is served with the accent as set on its <html>, which
// is :root, as --accent-pick, so the page has it before it first paints;
// with none, it's served as built, and foundations.css's default applies.
func TestAppAccent(t *testing.T) {
	built, err := os.ReadFile("../../web/index.html")
	if err != nil {
		t.Fatal(err)
	}
	s := startTest(t, t.TempDir(), &fakeWorkspace{}, nil)
	s.Assets = fstest.MapFS{"index.html": {Data: built}}
	served := func(path string) string {
		t.Helper()
		res := do(t, s, "GET", path, true, nil)
		if res.StatusCode != 200 || res.Header.Get("Cache-Control") != "no-cache" {
			t.Fatalf("%s: %d, %q", path, res.StatusCode, res.Header.Get("Cache-Control"))
		}
		return body(res)
	}
	accent := func(a string) {
		setOps(s, testOps(s, ops.Options{}, func(c *config.Config, _ *workspace.Workspace) { c.UI.Accent = a }))
	}

	if b := served("/"); b != string(built) || strings.Contains(b, "--accent-pick") {
		t.Errorf("none set: served\n%s", b)
	}

	accent("#2A9D8F")
	for _, path := range []string{"/", "/index.html", "/missions/m", "/settings"} {
		b := served(path)
		if !strings.Contains(b, `<html lang="en" style="--accent-pick: #2a9d8f">`) {
			t.Errorf("%s with an accent: served\n%s", path, b)
		}
		if strings.Replace(b, ` style="--accent-pick: #2a9d8f"`, "", 1) != string(built) {
			t.Errorf("%s: more than the accent changed:\n%s", path, b)
		}
	}

	// What isn't a hex colour never reaches the page, however the config
	// was made.
	for _, bad := range []string{"violet", `#fff" onload="alert(1)`, "#fff;}</style><script>x()</script>", "url(x)"} {
		accent(bad)
		if b := served("/"); b != string(built) {
			t.Errorf("%q: served\n%s", bad, b)
		}
	}
}

// Only a hex colour is put on a page's <html>, whatever it's given, and a
// page with no <html> is left as it is.
func TestWithAccent(t *testing.T) {
	for _, c := range []struct{ page, accent, want string }{
		{`<!doctype html><html lang="en"><head>`, "#abc", `<!doctype html><html lang="en" style="--accent-pick: #abc"><head>`},
		{"<HTML>\n<head>", "#A1B2C3", "<HTML style=\"--accent-pick: #A1B2C3\">\n<head>"},
		{"<html\n  lang=\"en\"\n>", "#123456", "<html\n  lang=\"en\"\n style=\"--accent-pick: #123456\">"},
		{"<!doctype html><title>app</title>", "#123456", "<!doctype html><title>app</title>"},
		{"<htmlx><html>", "#123456", "<htmlx><html style=\"--accent-pick: #123456\">"},
		{"<html>", "", "<html>"},
		{"<html>", "#12345", "<html>"},
		{"<html>", "#1234567", "<html>"},
		{"<html>", `#fff" x="`, "<html>"},
		{"<html>", "#ggg", "<html>"},
		{"<html>", "red", "<html>"},
	} {
		if got := string(withAccent([]byte(c.page), c.accent)); got != c.want {
			t.Errorf("withAccent(%q, %q) = %q, want %q", c.page, c.accent, got, c.want)
		}
	}
}

// An open page holds the daemon up, and hears when what it shows
// changes.
func TestEvents(t *testing.T) {
	var held atomic.Int32
	hold := func() func() {
		held.Add(1)
		return func() { held.Add(-1) }
	}
	fw := &fakeWorkspace{title: "one"}
	s := startTest(t, t.TempDir(), fw, hold)

	req, _ := http.NewRequest("GET", s.Info().Origin()+"/api/events", nil)
	req.AddCookie(&http.Cookie{Name: s.cookieName(), Value: s.Info().Token})
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if res.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatalf("content type %q", res.Header.Get("Content-Type"))
	}
	events := make(chan string, 10)
	go func() {
		sc := bufio.NewScanner(res.Body)
		for sc.Scan() {
			if name, ok := strings.CutPrefix(sc.Text(), "event: "); ok {
				events <- name
			}
		}
		close(events)
	}()
	next := func() string {
		select {
		case e := <-events:
			return e
		case <-time.After(3 * time.Second):
			return "(nothing)"
		}
	}
	if e := next(); e != "hello" {
		t.Fatalf("first event %q", e)
	}
	if held.Load() != 1 {
		t.Errorf("held %d", held.Load())
	}
	fw.set("two")
	if e := next(); e != "workspace" {
		t.Errorf("after a change: %q", e)
	}
	res.Body.Close()
	deadline := time.Now().Add(3 * time.Second)
	for held.Load() != 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if held.Load() != 0 {
		t.Error("closing the page didn't release the daemon")
	}
}

// Pages follow one stream, by topic: one that isn't there, or whose
// service this server hasn't, is not found.
func TestStreamTopics(t *testing.T) {
	s := startTest(t, t.TempDir(), &fakeWorkspace{}, nil)
	for _, topic := range []string{"nope", "reviews", "team"} {
		if code := do(t, s, "GET", "/api/events?topic="+topic, true, nil).StatusCode; code != http.StatusNotFound {
			t.Errorf("topic %s: %d, want 404", topic, code)
		}
	}
}

// A mission's page follows its own topic: told at once of a write to its
// records (before any look could have seen it), not of another mission's,
// and at the next look of its brief changing, which the records can't see.
func TestMissionEvents(t *testing.T) {
	root := t.TempDir()
	storetest.Own(t, root)
	ms := mission.Store{Root: root}
	one, _ := ms.Create("One")
	other, _ := ms.Create("Other")
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{Root: root, Poll: time.Second, Assets: testApp, listener: ln}
	s.App = testOps(s, ops.Options{})
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)

	req, _ := http.NewRequest("GET", s.Info().Origin()+"/api/events?topic=mission:"+one.Slug, nil)
	req.AddCookie(&http.Cookie{Name: s.cookieName(), Value: s.Info().Token})
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	events := make(chan string, 10)
	go func() {
		sc := bufio.NewScanner(res.Body)
		for sc.Scan() {
			if name, ok := strings.CutPrefix(sc.Text(), "event: "); ok {
				events <- name
			}
		}
		close(events)
	}()
	next := func(within time.Duration) string {
		select {
		case e := <-events:
			return e
		case <-time.After(within):
			return "(nothing)"
		}
	}
	if e := next(3 * time.Second); e != "hello" {
		t.Fatalf("first event %q", e)
	}
	ms.AppendLog(other.Slug, mission.Event{Kind: mission.EventNote, Text: "elsewhere"})
	if e := next(300 * time.Millisecond); e != "(nothing)" {
		t.Errorf("another mission's write: %q", e)
	}
	ms.AppendLog(one.Slug, mission.Event{Kind: mission.EventNote, Text: "hi"})
	if e := next(300 * time.Millisecond); e != "mission:"+one.Slug {
		t.Errorf("a write to its log: %q", e)
	}
	ms.WriteBrief(one.Slug, "# One\n\n## Goal\nSaid now.\n")
	if e := next(3 * time.Second); e != "mission:"+one.Slug {
		t.Errorf("its brief changing: %q", e)
	}
}

// The workspace's pages are told at once of a write to the records that
// names the workspace (a mission made, a conversation's turn), before any
// look at the files could have seen it: here the next is five seconds off.
func TestWorkspaceEvents_ToldAtOnceOfAWriteToTheRecords(t *testing.T) {
	root := t.TempDir()
	storetest.Own(t, root)
	ms := mission.Store{Root: root}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{Root: root, Poll: 5 * time.Second, Assets: testApp, listener: ln}
	s.App = testOps(s, ops.Options{})
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)

	req, _ := http.NewRequest("GET", s.Info().Origin()+"/api/events?topic=workspace", nil)
	req.AddCookie(&http.Cookie{Name: s.cookieName(), Value: s.Info().Token})
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	events := make(chan string, 10)
	go func() {
		sc := bufio.NewScanner(res.Body)
		for sc.Scan() {
			if name, ok := strings.CutPrefix(sc.Text(), "event: "); ok {
				events <- name
			}
		}
		close(events)
	}()
	next := func(within time.Duration) string {
		select {
		case e := <-events:
			return e
		case <-time.After(within):
			return "(nothing)"
		}
	}
	if e := next(3 * time.Second); e != "hello" {
		t.Fatalf("first event %q", e)
	}
	one, err := ms.Create("One")
	if err != nil {
		t.Fatal(err)
	}
	if e := next(2 * time.Second); e != "workspace" {
		t.Errorf("a mission made: %q", e)
	}
	ms.AppendLog(one.Slug, mission.Event{Kind: mission.EventNote, Text: "hi"})
	if e := next(2 * time.Second); e != "workspace" {
		t.Errorf("a write to its log: %q", e)
	}
}

// The workspace's menu shuts the daemon down: the page is told it has
// been, then the daemon goes. Without a daemon to close there's no such
// thing, and like any write it's refused from another site's page.
func TestShutdown_ClosesTheDaemon(t *testing.T) {
	root := t.TempDir()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	storetest.Own(t, root)
	(&fakeWorkspace{}).make(t, root)
	closed := make(chan string, 1)
	s := &Server{Root: root, Poll: 20 * time.Millisecond, Assets: testApp, listener: ln, Shutdown: func(reason string) { closed <- reason }}
	s.App = testOps(s, ops.Options{})
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	write := func(origin string) func(*http.Request) {
		return func(r *http.Request) {
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("Origin", origin)
		}
	}

	if res := do(t, s, "POST", "/api/shutdown", true, write("http://127.0.0.1:3000")); res.StatusCode != http.StatusForbidden {
		t.Errorf("another site's page: %d", res.StatusCode)
	}
	select {
	case <-closed:
		t.Fatal("closed from another site's page")
	default:
	}
	if res := do(t, s, "POST", "/api/shutdown", true, write(s.Info().Origin())); res.StatusCode != http.StatusAccepted {
		t.Fatalf("shutdown: %d %s", res.StatusCode, body(res))
	}
	select {
	case reason := <-closed:
		if reason != "closed from the web UI" {
			t.Errorf("reason %q", reason)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("the daemon wasn't closed")
	}

	none := startTest(t, t.TempDir(), &fakeWorkspace{}, nil)
	if res := do(t, none, "POST", "/api/shutdown", true, write(none.Info().Origin())); res.StatusCode != http.StatusNotFound {
		t.Errorf("no daemon to close: %d", res.StatusCode)
	}
}

// The workspace says which ws serves it, for the web UI's menu: the
// daemon's own, and nothing where it wasn't told.
func TestWorkspaceSaysItsVersion(t *testing.T) {
	s := startTest(t, t.TempDir(), &fakeWorkspace{title: "The glow"}, nil)
	asked := func() map[string]any {
		t.Helper()
		var ws map[string]any
		if err := json.NewDecoder(do(t, s, "GET", "/api/workspace", true, nil).Body).Decode(&ws); err != nil {
			t.Fatal(err)
		}
		return ws
	}
	if v, ok := asked()["version"]; ok {
		t.Errorf("version = %v, with none given", v)
	}
	s.Version = "v1.4.0"
	s.views.forget() // a running server's version doesn't change but here
	if v := asked()["version"]; v != "v1.4.0" {
		t.Errorf("version = %v, want v1.4.0", v)
	}
}
