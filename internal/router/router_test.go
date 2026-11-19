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
		{"web.feat.api.shop.localhost", "192.168.1.20:50000", 403, "this machine alone"},
		{"evil.example", me, 421, "under .localhost"},
		{"api.feat.api.shop.localhost", me, 410, "exited (1) at 14:05"},
		{"nope.feat.api.shop.localhost", me, 404, "web.feat.api"},
		{"other.localhost", me, 404, "shop.localhost"},
		{"stack.silo.api.shop.localhost", me, 200, "web run"},
		{"stack.ground.api.shop.localhost", me, 200, "web run"},
		{"stack.feat.api.shop.localhost", me, 409, "Point the silo at feat"},
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
	ctx2, stop2 := context.WithCancel(context.Background())
	defer stop2()
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
	r := httptest.NewRequest("POST", "http://x/.ws-router/yield", strings.NewReader(`{"router":1}`))
	r.RemoteAddr = "127.0.0.1:1"
	w := httptest.NewRecorder()
	rt.ServeHTTP(w, r)
	if w.Code != http.StatusConflict {
		t.Errorf("one of the same version: %d", w.Code)
	}
	yield := rt.yield
	r = httptest.NewRequest("POST", "http://x/.ws-router/yield", strings.NewReader(fmt.Sprintf(`{"router":%d}`, Version+1)))
	r.RemoteAddr = "127.0.0.1:1"
	w = httptest.NewRecorder()
	rt.ServeHTTP(w, r)
	select {
	case <-yield:
	default:
		t.Error("a newer router wasn't yielded to")
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
