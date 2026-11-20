package router

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/withworkspace/workspace/internal/runs"
)

// Ports are the router's, in the order they're tried: 80, so names need no
// port, and Fallback where 80 can't be had (another program has it, or the
// system keeps it for root, as Linux does).
var Ports = []int{80, Fallback}

// Fallback is the router's port when 80 can't be had.
const Fallback = 7080

// RetryEvery is how often a daemon that isn't the router tries to be.
var RetryEvery = 2 * time.Second

// yieldRest is how long a router that stepped down for a newer one waits
// before it tries to be the router again.
var yieldRest = 10 * time.Second

// path is where a router says it's one, and is asked to step down.
const path = "/.ws-router"

// Hello is what a router says at path.
type Hello struct {
	Router int `json:"router"` // its Version
	PID    int `json:"pid"`
}

// Router is the router, held by whichever daemon the system lets have its
// port.
type Router struct {
	// Dir is the route files' (Dir()).
	Dir string
	// Procs reads the user's processes, for which daemons are running:
	// nil is the system's.
	Procs func() (runs.Procs, error)
	Logf  func(string, ...any)

	mu    sync.Mutex
	table *table
	read  time.Time
	yield chan struct{}
}

func (rt *Router) logf(format string, a ...any) {
	if rt.Logf != nil {
		rt.Logf(format, a...)
	}
}

// Run tries to be the router until ctx is done: it listens on the first of
// Ports it can, and serves while it holds it. Where a router of ws holds a
// port already, it waits for that one; a router of an older version is
// asked to step down. A port another program holds, or the system keeps,
// is passed over for the next.
func (rt *Router) Run(ctx context.Context) {
	for ctx.Err() == nil {
		for _, p := range Ports {
			l, err := net.Listen("tcp", ":"+strconv.Itoa(p))
			if err == nil {
				rt.serve(ctx, l, p)
				break
			}
			if h, ok := WhoIs(p); ok {
				if h.Router < Version {
					rt.askYield(p)
				}
				break // a router of ws has it: wait for that one
			}
			// Kept by the system (permission denied), or another program's.
		}
		select {
		case <-ctx.Done():
		case <-time.After(RetryEvery):
		}
	}
}

func (rt *Router) serve(ctx context.Context, l net.Listener, port int) {
	rt.mu.Lock()
	rt.yield = make(chan struct{})
	yield := rt.yield
	rt.mu.Unlock()
	srv := &http.Server{Handler: rt, ReadHeaderTimeout: 10 * time.Second}
	rt.logf("router: serving names on :%d", port)
	done := make(chan struct{})
	go func() {
		srv.Serve(l)
		close(done)
	}()
	yielded := false
	select {
	case <-ctx.Done():
	case <-yield:
		yielded = true
	case <-done:
	}
	shut, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	srv.Shutdown(shut)
	cancel()
	if yielded {
		rt.logf("router: stepped down on :%d for a newer router", port)
		select {
		case <-ctx.Done():
		case <-time.After(yieldRest):
		}
	}
}

// WhoIs asks what holds port on this machine: a router of ws, and which.
func WhoIs(port int) (Hello, bool) {
	c := http.Client{Timeout: time.Second}
	resp, err := c.Get(fmt.Sprintf("http://127.0.0.1:%d%s", port, path))
	if err != nil {
		return Hello{}, false
	}
	defer resp.Body.Close()
	var h Hello
	if resp.StatusCode != http.StatusOK || json.NewDecoder(io.LimitReader(resp.Body, 1<<10)).Decode(&h) != nil || h.Router == 0 {
		return Hello{}, false
	}
	return h, true
}

func (rt *Router) askYield(port int) {
	c := http.Client{Timeout: time.Second}
	body := strings.NewReader(fmt.Sprintf(`{"router":%d}`, Version))
	resp, err := c.Post(fmt.Sprintf("http://127.0.0.1:%d%s/yield", port, path), "application/json", body)
	if err == nil {
		resp.Body.Close()
	}
}

// Serving is the port the machine's router is serving names on, or 0 for
// none.
func Serving() int {
	servingMu.Lock()
	defer servingMu.Unlock()
	if time.Since(servingAt) < 5*time.Second {
		return servingPort
	}
	servingPort, servingAt = 0, time.Now()
	for _, p := range Ports {
		if _, ok := WhoIs(p); ok {
			servingPort = p
			break
		}
	}
	return servingPort
}

var (
	servingMu   sync.Mutex
	servingPort int
	servingAt   time.Time
)

// Link is the address of a name on the router serving on port.
func Link(host string, port int) string {
	if port == 80 {
		return "http://" + host + "/"
	}
	return fmt.Sprintf("http://%s:%d/", host, port)
}

// table is what the router serves, from the route files.
type table struct {
	files []File
	newer []File
	names map[string]string // by root
	// by name: each workspace's file, by the name it goes by.
	byName map[string]File
}

// look is the route files as they are now, read again at most each second.
func (rt *Router) look() *table {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if rt.table != nil && time.Since(rt.read) < time.Second {
		return rt.table
	}
	read := rt.Procs
	if read == nil {
		read = runs.ReadProcs
	}
	procs, err := read()
	if err != nil {
		procs = runs.Procs{}
	}
	files, newer := Read(rt.Dir, procs)
	t := &table{files: files, newer: newer, names: Names(files), byName: map[string]File{}}
	for _, f := range files {
		t.byName[t.names[f.Root]] = f
	}
	rt.table, rt.read = t, time.Now()
	return t
}

func loopback(remote string) bool {
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func (rt *Router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// It listens on every interface (macOS lets a user have 80 no other
	// way), so it answers this machine alone.
	if !loopback(r.RemoteAddr) {
		http.Error(w, "ws's router answers this machine alone", http.StatusForbidden)
		return
	}
	if r.URL.Path == path {
		writeJSON(w, Hello{Router: Version, PID: syscallPID()})
		return
	}
	if r.URL.Path == path+"/yield" && r.Method == http.MethodPost {
		var h Hello
		json.NewDecoder(io.LimitReader(r.Body, 1<<10)).Decode(&h)
		if h.Router > Version {
			writeJSON(w, Hello{Router: Version, PID: syscallPID()})
			rt.mu.Lock()
			if rt.yield != nil {
				close(rt.yield)
				rt.yield = nil
			}
			rt.mu.Unlock()
			return
		}
		http.Error(w, "not newer", http.StatusConflict)
		return
	}
	host, port := r.Host, ""
	if h, p, err := net.SplitHostPort(r.Host); err == nil {
		host, port = h, p
	}
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	if !strings.HasSuffix(host, Suffix) {
		// Another site's name pointed here (DNS rebinding) gets nothing.
		http.Error(w, "ws's router answers names under .localhost alone", http.StatusMisdirectedRequest)
		return
	}
	labels := strings.Split(strings.TrimSuffix(host, Suffix), ".")
	t := rt.look()
	wsName := labels[len(labels)-1]
	f, ok := t.byName[wsName]
	if !ok {
		rt.unknown(w, t, host, port)
		return
	}
	key := strings.Join(labels[:len(labels)-1], ".")
	if key == "" {
		proxy(w, r, f.Web)
		return
	}
	if rte, ok := f.Routes[key]; ok {
		if rte.Port > 0 {
			through(w, r, rte)
			return
		}
		notice(w, http.StatusGone, key+" has ended", fmt.Sprintf("<p><b>%s</b> %s.</p><p>Start it again with <code>ws run</code>, or from its mission's Work tab.</p>%s", esc(key), esc(rte.Ended), listRuns(f, wsName, port)))
		return
	}
	if len(labels) == 4 {
		if s, ok := f.Silos[labels[2]]; ok && contains(s.Recipes, labels[0]) {
			rt.silo(w, r, f, wsName, port, labels, s)
			return
		}
	}
	notice(w, http.StatusNotFound, "Nothing runs at "+host, fmt.Sprintf("<p>Nothing in <b>%s</b> is called <b>%s</b>.</p>%s", esc(wsName), esc(key), listRuns(f, wsName, port)))
}

// silo answers a name for a silo's recipe in a capsule (labels: run,
// capsule, repo, workspace): through to the silo, when it's pointed there,
// or a page that offers to point it, which is the workspace's to do.
// Nothing's switched by a request alone: two tabs would fight over it, and
// a link's preview would move it.
func (rt *Router) silo(w http.ResponseWriter, r *http.Request, f File, wsName, port string, labels []string, s Silo) {
	run, capsule, repo := labels[0], labels[1], labels[2]
	siloKey := run + ".silo." + repo
	if Label(runs.Label(s.Target)) == capsule {
		if rte, ok := f.Routes[siloKey]; ok && rte.Port > 0 {
			through(w, r, rte)
			return
		}
	}
	portNum, _ := strconv.Atoi(port)
	if portNum == 0 {
		portNum = 80
	}
	then := Link(Host(siloKey, wsName), portNum)
	point := Link(Host("", wsName), portNum) + "silo/" + url.PathEscape(repo) + "?" + url.Values{"to": {capsule}, "then": {then}}.Encode()
	notice(w, http.StatusConflict, run+" runs in the silo",
		fmt.Sprintf("<p><b>%s</b> runs in %s's silo, which is on <b>%s</b>, not <b>%s</b>.</p><p><a class=\"button\" href=\"%s\">Point the silo at %s</a></p><p>Or go to it as it is: <a href=\"%s\">%s</a></p>",
			esc(run), esc(repo), esc(runs.Label(s.Target)), esc(capsule), esc(point), esc(capsule), esc(then), esc(strings.TrimSuffix(strings.TrimPrefix(then, "http://"), "/"))))
}

func (rt *Router) unknown(w http.ResponseWriter, t *table, host, port string) {
	var b strings.Builder
	names := make([]string, 0, len(t.byName))
	for n := range t.byName {
		names = append(names, n)
	}
	sort.Strings(names)
	portNum, _ := strconv.Atoi(port)
	if portNum == 0 {
		portNum = 80
	}
	if len(names) > 0 {
		b.WriteString("<p>The workspaces with names here:</p><ul>")
		for _, n := range names {
			fmt.Fprintf(&b, "<li><a href=\"%s\">%s</a></li>", esc(Link(Host("", n), portNum)), esc(Host("", n)))
		}
		b.WriteString("</ul>")
	} else {
		b.WriteString("<p>No workspace has names here: <code>[router] enabled = true</code> in a workspace's ws.local.toml gives it some.</p>")
	}
	for _, f := range t.newer {
		fmt.Fprintf(&b, "<p>%s is a newer ws's, whose names this router can't read: restart this router's ws onto the newer one.</p>", esc(f.Root))
	}
	notice(w, http.StatusNotFound, "No workspace is "+host, b.String())
}

func listRuns(f File, wsName, port string) string {
	var keys []string
	for k, r := range f.Routes {
		if r.Port > 0 {
			keys = append(keys, k)
		}
	}
	portNum, _ := strconv.Atoi(port)
	if portNum == 0 {
		portNum = 80
	}
	var b strings.Builder
	fmt.Fprintf(&b, "<p><a href=\"%s\">%s's web UI</a></p>", esc(Link(Host("", wsName), portNum)), esc(wsName))
	if len(keys) == 0 {
		b.WriteString("<p>Nothing runs in it now.</p>")
		return b.String()
	}
	sort.Strings(keys)
	b.WriteString("<p>What runs in it:</p><ul>")
	for _, k := range keys {
		fmt.Fprintf(&b, "<li><a href=\"%s\">%s</a></li>", esc(Link(Host(k, wsName), portNum)), esc(k))
	}
	b.WriteString("</ul>")
	return b.String()
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if Label(x) == s {
			return true
		}
	}
	return false
}

// through passes the request to a run: to the run itself, or, for a path
// one of its mounts has, to what's mounted there.
func through(w http.ResponseWriter, r *http.Request, rte Route) {
	m, ok := rte.mount(r.URL.Path)
	if !ok {
		proxy(w, r, rte.Port)
		return
	}
	if m.Strip {
		// The copy the proxy sends is made from these: what was asked
		// for, less the mount.
		r.URL.Path = "/" + strings.TrimPrefix(strings.TrimPrefix(r.URL.Path, m.Path), "/")
		if r.URL.RawPath != "" {
			r.URL.RawPath = "/" + strings.TrimPrefix(strings.TrimPrefix(r.URL.RawPath, m.Path), "/")
		}
	}
	what := m.Need
	if what == "" {
		what = "What's"
	}
	switch {
	case m.URL != "":
		target, err := url.Parse(m.URL)
		if err != nil || target.Host == "" {
			notice(w, http.StatusBadGateway, what+" has no address", fmt.Sprintf("<p><b>%s</b> is mounted at <b>%s</b>, at an address that can't be read: <code>%s</code>.</p>", esc(what), esc(m.Path), esc(m.URL)))
			return
		}
		proxyTo(w, r, target, true)
	case m.Port > 0:
		proxy(w, r, m.Port)
	default:
		run := ""
		if m.Run != "" {
			run = " (" + m.Run + ")"
		}
		notice(w, http.StatusBadGateway, what+" isn't running", fmt.Sprintf("<p><b>%s</b>%s is mounted here at <b>%s</b>, and isn't running.</p><p>Starting this run again with <code>ws run</code> starts it too.</p>", esc(what), esc(run), esc(m.Path)))
	}
}

// proxy passes the request to what listens on port, its name kept as its
// Host: a dev server sees the name it was asked for.
func proxy(w http.ResponseWriter, r *http.Request, port int) {
	proxyTo(w, r, &url.URL{Scheme: "http", Host: "127.0.0.1:" + strconv.Itoa(port)}, false)
}

// proxyTo passes the request to target. A run of ws's is asked by the name
// the request came for; an address outside ws (itsHost) by its own, as it
// knows no other, and with its own path before what's asked for.
func proxyTo(w http.ResponseWriter, r *http.Request, target *url.URL, itsHost bool) {
	p := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(target)
			if !itsHost {
				pr.Out.Host = pr.In.Host
			}
			pr.SetXForwarded()
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			if errors.Is(err, syscall.ECONNREFUSED) || strings.Contains(err.Error(), "connection refused") {
				at := "its port (" + target.Port() + ")"
				if itsHost {
					at = target.Host
				}
				notice(w, http.StatusBadGateway, "Not up yet", fmt.Sprintf("<p>Nothing's listening on %s yet. If it's just started, reload in a moment; <code>ws run logs</code> says what it's doing.</p>", esc(at)))
				return
			}
			notice(w, http.StatusBadGateway, "Couldn't reach it", "<p>"+esc(err.Error())+"</p>")
		},
	}
	p.ServeHTTP(w, r)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func esc(s string) string { return html.EscapeString(s) }

// notice is a page of the router's own, drawn in both schemes.
func notice(w http.ResponseWriter, status int, title, body string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	fmt.Fprintf(w, `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><meta name="color-scheme" content="light dark"><title>%s</title>
<style>
:root{--bg:#f7f7f8;--card:#fff;--fg:#1d1d22;--dim:#6b6b76;--line:#e3e3e8}
@media (prefers-color-scheme:dark){:root{--bg:#121216;--card:#1b1b21;--fg:#ececf1;--dim:#9a9aa6;--line:#2c2c35}}
body{margin:0;background:var(--bg);color:var(--fg);font:15px/1.5 system-ui,-apple-system,sans-serif;display:flex;justify-content:center;padding:12vh 16px}
main{background:var(--card);border:1px solid var(--line);border-radius:12px;padding:24px 28px;max-width:560px;width:100%%;box-sizing:border-box}
h1{font-size:18px;margin:0 0 12px}p,li{color:var(--dim)}b{color:var(--fg)}a{color:var(--fg)}code{font-size:13px}
a.button{display:inline-block;background:var(--fg);color:var(--bg);text-decoration:none;padding:6px 14px;border-radius:8px}
small{color:var(--dim)}
</style></head><body><main><h1>%s</h1>%s<p><small>ws's router</small></p></main></body></html>`, esc(title), esc(title), body)
}

func syscallPID() int { return syscall.Getpid() }
