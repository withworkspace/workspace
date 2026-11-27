package router

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/runs"
)

func TestNamesSettleAClashWithoutRefusing(t *testing.T) {
	early, late := time.Now().Add(-time.Hour), time.Now()
	n := Names([]File{
		{Root: "/b/ws-test", Name: "ws-test", Created: late},
		{Root: "/a/ws-test", Name: "ws-test", Created: early},
		{Root: "/c/shop", Name: "shop", Created: late},
	})
	if n["/a/ws-test"] != "ws-test" || n["/c/shop"] != "shop" {
		t.Errorf("names %v", n)
	}
	if got := n["/b/ws-test"]; got != "ws-test-"+hashOf("/b/ws-test", 4) {
		t.Errorf("the later one is %q", got)
	}
}

func TestAGivenNameIsKeptWhenTheOtherGoes(t *testing.T) {
	dir := t.TempDir()
	early, late := time.Now().Add(-time.Hour), time.Now()
	a := File{Root: "/a/shop", Name: "shop", Created: early}
	b := File{Root: "/b/shop", Name: "shop", Created: late}
	a.Given = Give(dir, a, runs.Procs{})
	Write(dir, a)
	b.Given = Give(dir, b, runs.Procs{})
	Write(dir, b)
	if a.Given != "shop" || b.Given != "shop-"+hashOf("/b/shop", 4) {
		t.Fatalf("given %q and %q", a.Given, b.Given)
	}
	Remove(dir, a.Root) // a's daemon stops
	if got := Give(dir, b, runs.Procs{}); got != b.Given {
		t.Errorf("b became %q when a went, want it kept as %q", got, b.Given)
	}
	files, _ := Read(dir, runs.Procs{})
	if n := Names(files)[b.Root]; n != b.Given {
		t.Errorf("the router calls b %q", n)
	}
	// a comes back: shop is free, as b was given another.
	a.Given = ""
	if got := Give(dir, a, runs.Procs{}); got != "shop" {
		t.Errorf("a came back as %q", got)
	}
}

func TestLabels(t *testing.T) {
	for in, want := range map[string]string{"My_Shop": "my-shop", "..x..": "x", "": ""} {
		if got := Label(in); got != want {
			t.Errorf("Label(%q) = %q, want %q", in, got, want)
		}
	}
	if got := RunKey("web", ".ground", "api"); got != "web.ground.api" {
		t.Errorf("RunKey = %s", got)
	}
	if got := NameFor("/x/Projects/Shop Front", ""); got != "shop-front" {
		t.Errorf("NameFor = %s", got)
	}
}

func TestReadSkipsAGoneDaemonsFile(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	Write(dir, File{PID: 10, Began: now, Root: "/a", Name: "a"})
	Write(dir, File{PID: 20, Began: now, Root: "/b", Name: "b"})
	Write(dir, File{PID: 30, Began: now.Add(-time.Hour), Root: "/c", Name: "c"}) // its pid is another's now
	procs := runs.ProcsOf([]harness.Proc{{PID: 10, Start: now}, {PID: 30, Start: now}})
	files, _ := Read(dir, procs)
	if len(files) != 1 || files[0].Root != "/a" {
		t.Fatalf("files %v", files)
	}
	if left, _ := os.ReadDir(dir); len(left) != 1 {
		t.Errorf("the gone daemons' files are still there: %d", len(left))
	}
}

// A router over one workspace, "shop", whose web UI and run web.feat.api
// are test servers, run api.feat.api has ended, and repo api's silo is on
// .ground with the recipe stack.
func routed(t *testing.T) (*Router, *httptest.Server, *httptest.Server) {
	t.Helper()
	dir := t.TempDir()
	webUI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprintf(w, "web UI at %s", r.Host) }))
	run := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "web run at %s, forwarded for %s", r.Host, r.Header.Get("X-Forwarded-Host"))
	}))
	t.Cleanup(webUI.Close)
	t.Cleanup(run.Close)
	Write(dir, File{PID: os.Getpid(), Root: "/x/shop", Name: "shop", Created: time.Now(), Web: portOf(webUI), Routes: map[string]Route{
		"web.feat.api":   {Port: portOf(run)},
		"api.feat.api":   {Ended: "exited (1) at 14:05"},
		"stack.silo.api": {Port: portOf(run)},
	}, Silos: map[string]Silo{"api": {Target: ".ground", Recipes: []string{"stack"}}}})
	return &Router{Dir: dir, Procs: func() (runs.Procs, error) { return runs.Procs{}, nil }}, webUI, run
}

func portOf(s *httptest.Server) int {
	u, _ := url.Parse(s.URL)
	p, _ := strconv.Atoi(u.Port())
	return p
}

func ask(rt *Router, host, remote string) (int, string) {
	r := httptest.NewRequest("GET", "http://"+host+"/", nil)
	r.RemoteAddr = remote
	w := httptest.NewRecorder()
	rt.ServeHTTP(w, r)
	b, _ := io.ReadAll(w.Result().Body)
	return w.Code, string(b)
}

func TestServe(t *testing.T) {
	rt, _, _ := routed(t)
	const me = "127.0.0.1:50000"
	for _, c := range []struct {
		host, remote string
		code         int
		has          string
	}{
		{"shop.localhost", me, 200, "web UI at shop.localhost"},
		{"web.feat.api.shop.localhost:7080", me, 200, "web run at web.feat.api.shop.localhost:7080, forwarded for web.feat.api.shop.localhost:7080"},
		{"WEB.feat.api.shop.localhost.", "[::1]:50000", 200, "web run"},
		{"web.feat.api.shop.localhost", "192.168.1.20:50000", 403, "only answers this machine"},
		{"evil.example", me, 421, "under .localhost"},
		{"api.feat.api.shop.localhost", me, 410, "exited (1) at 14:05"},
		{"nope.feat.api.shop.localhost", me, 404, "web.feat.api"},
		{"other.localhost", me, 404, "shop.localhost"},
		{"stack.silo.api.shop.localhost", me, 200, "web run"},
		{"stack.ground.api.shop.localhost", me, 200, "web run"},
		{"stack.feat.api.shop.localhost", me, 409, "Point silo at feat"},
	} {
		code, body := ask(rt, c.host, c.remote)
		if code != c.code || !strings.Contains(body, c.has) {
			t.Errorf("%s from %s: %d %q, want %d with %q", c.host, c.remote, code, body, c.code, c.has)
		}
	}
	_, body := ask(rt, "stack.feat.api.shop.localhost", me)
	if !strings.Contains(body, "http://shop.localhost/silo/api?then=http%3A%2F%2Fstack.silo.api.shop.localhost%2F&amp;to=feat") {
		t.Errorf("the silo page's link: %s", body)
	}
}

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

// Two daemons try to be the router: one is, and when it goes, the other
// takes over.
func TestThePortIsTheLock(t *testing.T) {
	oldPorts, oldRetry := Ports, RetryEvery
	p := freePort(t)
	Ports, RetryEvery = []int{p}, 50*time.Millisecond
	t.Cleanup(func() { Ports, RetryEvery = oldPorts, oldRetry })
	rt1, _, _ := routed(t)
	rt2 := &Router{Dir: rt1.Dir, Procs: rt1.Procs}
	ctx1, stop1 := context.WithCancel(context.Background())
	ctx2 := t.Context()
	go rt1.Run(ctx1)
	waitFor(t, func() bool { _, ok := WhoIs(p); return ok })
	go rt2.Run(ctx2)
	time.Sleep(200 * time.Millisecond) // it tries, and waits
	stop1()
	start := time.Now()
	waitFor(t, func() bool {
		resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/", p))
		if err != nil {
			return false
		}
		resp.Body.Close()
		return true
	})
	if d := time.Since(start); d > 3*time.Second {
		t.Errorf("took over after %s", d)
	}
}

func TestANewerRouterIsYieldedTo(t *testing.T) {
	oldRest := yieldRest
	yieldRest = time.Hour
	t.Cleanup(func() { yieldRest = oldRest })
	rt, _, _ := routed(t)
	rt.yield = make(chan struct{})
	ask := func(version int, edit func(*http.Request)) int {
		r := httptest.NewRequest("POST", "http://127.0.0.1/.ws-router/yield", strings.NewReader(fmt.Sprintf(`{"router":%d}`, version)))
		r.RemoteAddr = "127.0.0.1:1"
		r.Header.Set("Content-Type", "application/json")
		if edit != nil {
			edit(r)
		}
		w := httptest.NewRecorder()
		rt.ServeHTTP(w, r)
		return w.Code
	}
	if code := ask(1, nil); code != http.StatusConflict {
		t.Errorf("one of the same version: %d", code)
	}

	// A page in a browser here comes from this machine too: whatever it
	// says, it isn't a router.
	for name, edit := range map[string]func(*http.Request){
		"another site's page": func(r *http.Request) { r.Header.Set("Origin", "https://evil.example") },
		"a sandboxed page":    func(r *http.Request) { r.Header.Set("Origin", "null") },
		"a page's fetch":      func(r *http.Request) { r.Header.Set("Sec-Fetch-Site", "cross-site") },
		"a form":              func(r *http.Request) { r.Header.Set("Content-Type", "text/plain") },
		"a rebound name":      func(r *http.Request) { r.Host = "evil.example" },
	} {
		if code := ask(Version+1, edit); code == http.StatusOK {
			t.Errorf("%s was yielded to", name)
		}
	}
	select {
	case <-rt.yield:
		t.Fatal("yielded to a page")
	default:
	}

	yield := rt.yield
	if code := ask(Version+1, nil); code != http.StatusOK {
		t.Errorf("a newer router: %d", code)
	}
	select {
	case <-yield:
	default:
		t.Error("a newer router wasn't yielded to")
	}
}

// What a router says it is, to a name pointed here by another site, is
// nothing.
func TestTheRouterAnswersItsOwnNames(t *testing.T) {
	rt, _, _ := routed(t)
	for host, want := range map[string]int{
		"127.0.0.1:7080": http.StatusOK,
		"localhost":      http.StatusOK,
		"[::1]:80":       http.StatusOK,
		"evil.example":   http.StatusMisdirectedRequest,
	} {
		r := httptest.NewRequest("GET", "http://"+host+"/.ws-router", nil)
		r.RemoteAddr = "127.0.0.1:1"
		w := httptest.NewRecorder()
		rt.ServeHTTP(w, r)
		if w.Code != want {
			t.Errorf("%s: %d, want %d", host, w.Code, want)
		}
	}
}

func waitFor(t *testing.T, ok func() bool) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if ok() {
			return
		}
	}
	t.Fatal("it never happened")
}

// What a run needs answers below the run's own name, at the path it's
// mounted on: the same origin, with the rest of the name the run's.
func TestAMountAnswersBelowTheRunsOwnName(t *testing.T) {
	dir := t.TempDir()
	said := func(what string) *httptest.Server {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, "%s: %s?%s at %s", what, r.URL.Path, r.URL.RawQuery, r.Host)
		}))
		t.Cleanup(s.Close)
		return s
	}
	web, api, auth, outside := said("web"), said("api"), said("auth"), said("outside")
	Write(dir, File{PID: os.Getpid(), Root: "/x/shop", Name: "shop", Created: time.Now(), Web: 1, Routes: map[string]Route{
		"web.feat.front": {Port: portOf(web), Mounts: []Mount{
			{Path: "/api", Need: "api", Run: "server.ground.back", Port: portOf(api)},
			{Path: "/api/auth", Need: "auth", Port: portOf(auth), Strip: true},
			{Path: "/cdn", Need: "cdn", URL: outside.URL + "/assets", Strip: true},
			{Path: "/down", Need: "search", Run: "search.ground.back"},
		}},
	}})
	rt := &Router{Dir: dir, Procs: func() (runs.Procs, error) { return runs.Procs{}, nil }}
	get := func(path string) (int, string) {
		r := httptest.NewRequest("GET", "http://web.feat.front.shop.localhost"+path, nil)
		r.RemoteAddr = "127.0.0.1:50000"
		w := httptest.NewRecorder()
		rt.ServeHTTP(w, r)
		b, _ := io.ReadAll(w.Result().Body)
		return w.Code, string(b)
	}
	outsideHost := strings.TrimPrefix(outside.URL, "http://")
	for _, c := range []struct {
		path string
		code int
		has  string
	}{
		{"/", 200, "web: /? at web.feat.front.shop.localhost"},
		{"/apiary", 200, "web: /apiary?"}, // a path that only begins the same isn't the mount's
		{"/api", 200, "api: /api? at web.feat.front.shop.localhost"},
		{"/api/users?page=2", 200, "api: /api/users?page=2 at web.feat.front.shop.localhost"},
		{"/api/auth/login", 200, "auth: /login?"}, // the longer mount, its path taken off
		{"/api/auth", 200, "auth: /?"},
		{"/cdn/app.css?v=1", 200, "outside: /assets/app.css?v=1 at " + outsideHost}, // an address outside ws is asked by its own name
		{"/down/q", 502, "search</b> (search.ground.back) is mounted at <b>/down</b> and not running"},
	} {
		code, body := get(c.path)
		if code != c.code || !strings.Contains(body, c.has) {
			t.Errorf("%s: %d %q, want %d with %q", c.path, code, body, c.code, c.has)
		}
	}
}

func TestMountPath(t *testing.T) {
	for in, want := range map[string]string{"/api": "/api", "api/": "/api", " /api/v1/ ": "/api/v1", "/": "", "": ""} {
		if got := MountPath(in); got != want {
			t.Errorf("MountPath(%q) = %q, want %q", in, got, want)
		}
	}
}

// A dev server that binds localhost can get ::1 alone: its run is up, as
// ws counts it, so the router reaches it there.
func TestARunOnTheOtherLoopbackIsReached(t *testing.T) {
	l, err := net.Listen("tcp", "[::1]:0")
	if err != nil {
		t.Skip("no IPv6 loopback here:", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	if c, err := net.Dial("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port))); err == nil {
		c.Close()
		l.Close()
		t.Skip("something else has the port on 127.0.0.1")
	}
	run := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprintf(w, "run at %s", r.Host) })}
	go run.Serve(l)
	t.Cleanup(func() { run.Close() })

	dir := t.TempDir()
	Write(dir, File{PID: os.Getpid(), Root: "/x/shop", Name: "shop", Created: time.Now(), Web: 1, Routes: map[string]Route{
		"web.feat.front": {Port: port, Mounts: []Mount{{Path: "/api", Need: "api", Port: port}}},
		"old.feat.front": {Port: freePort(t)},
	}})
	rt := &Router{Dir: dir, Procs: func() (runs.Procs, error) { return runs.Procs{}, nil }}
	const me = "127.0.0.1:50000"
	for _, host := range []string{"web.feat.front.shop.localhost", "web.feat.front.shop.localhost/api"} {
		r := httptest.NewRequest("GET", "http://"+host, nil)
		r.RemoteAddr = me
		w := httptest.NewRecorder()
		rt.ServeHTTP(w, r)
		if b := w.Body.String(); w.Code != 200 || b != "run at web.feat.front.shop.localhost" {
			t.Errorf("%s: %d %q, want the run on ::1", host, w.Code, b)
		}
	}
	// With nothing on either, it's still the page that says so.
	if code, body := ask(rt, "old.feat.front.shop.localhost", me); code != 502 || !strings.Contains(body, "Not up yet") {
		t.Errorf("a run on neither loopback: %d %q, want Not up yet", code, body)
	}
}
