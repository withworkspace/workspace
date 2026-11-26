package reads_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/withworkspace/workspace/internal/cost"
	"github.com/withworkspace/workspace/internal/testutil"
	"github.com/withworkspace/workspace/internal/web"
)

// A read starts no git (the rule: what a page reads is what the daemon
// keeps, answered at once; what it has to ask git is asked behind the
// answer, and the page told when it's in). Every GET the web UI serves is
// asked of a workspace with a mission, a capsule and crew, twice: a page's
// first read and the next. One that runs git on its own goroutine fails,
// unless it's below, with why: the first read (firstReads) or any
// (anyRead).
func TestReads_StartNoGit(t *testing.T) {
	var mu sync.Mutex
	gits := map[string]map[string][]string{} // by read ("first", "again"), by route: who ran git
	stop := cost.Observe(func(l cost.Line) {
		if l.Costs["git"].N == 0 {
			return
		}
		mu.Lock()
		defer mu.Unlock()
		if gits[l.Tag] == nil {
			gits[l.Tag] = map[string][]string{}
		}
		for _, b := range l.By {
			if strings.HasPrefix(b.Who, "git ") {
				gits[l.Tag][l.Pattern] = append(gits[l.Tag][l.Pattern], fmt.Sprintf("%s ×%d", b.Who, b.N))
			}
		}
	})
	t.Cleanup(stop)

	w, _ := testutil.OrchestrateFixture(t)
	s := testutil.WebServer(t, w.Root)
	// The mission's page is open, as it is when its tabs read: it follows
	// the mission, which has what its tabs read next read behind it.
	follow(t, s, "mission:cache")
	time.Sleep(time.Second) // the daemon's first look at what it keeps

	get := func(path, read string) int {
		req, _ := http.NewRequest("GET", s.Info().Origin()+path, nil)
		req.Header.Set("Authorization", "Bearer "+s.Info().Token)
		req.Header.Set(cost.TagHeader, read)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		res.Body.Close()
		return res.StatusCode
	}
	asked := 0
	for _, route := range s.Routes() {
		pattern, ok := strings.CutPrefix(route, "GET ")
		if !ok || pattern == "/" || strings.HasPrefix(pattern, "/auth") {
			continue
		}
		path := fill(pattern)
		get(path, "first")
		get(path, "again")
		asked++
	}
	if asked < 90 {
		t.Fatalf("asked only %d routes", asked)
	}

	mu.Lock()
	defer mu.Unlock()
	for _, read := range []string{"first", "again"} {
		routes := make([]string, 0, len(gits[read]))
		for r := range gits[read] {
			routes = append(routes, r)
		}
		sort.Strings(routes)
		for _, r := range routes {
			pattern := strings.TrimPrefix(r, "GET ")
			why, ok := anyRead[pattern]
			if !ok && read == "first" {
				why, ok = firstReads[pattern]
			}
			if ok {
				t.Logf("%s runs git on its %s read, allowed for now (%s): %s", r, read, why, strings.Join(gits[read][r], ", "))
				continue
			}
			t.Errorf("%s runs git on its %s read: %s", r, read, strings.Join(gits[read][r], ", "))
		}
	}
}

// anyRead and firstReads are the reads that still run git, each with
// why: the lists shrink, and nothing joins them without a reason.
var (
	anyRead = map[string]string{
		"/api/repos/{repo}/worktrees/{name}/nav/now":          "where code navigation's names are of: git status of the working tree, each read",
		"/api/missions/{slug}/capsules/{repo}/{name}/nav/now": "as the worktree's nav/now",
	}
	firstReads = map[string]string{}
)

// fill is pattern with each of its wildcards given a value the fixture
// has: the mission cache, its capsule backend/cache-lookup, its crew c1.
// A file's routes are asked of the repo's README.
func fill(pattern string) string {
	values := map[string]string{
		"slug": "cache", "repo": "backend", "name": "cache-lookup", "worktree": "cache-lookup",
		"id": "c1", "number": "1", "n": "1", "kind": "thing", "ref": "x", "version": "1", "tree": "cache-lookup",
		"rest...": "x",
	}
	if strings.HasPrefix(pattern, "/api/apps/") || strings.Contains(pattern, "/apps/") || strings.HasPrefix(pattern, "/api/plugins/") || strings.HasPrefix(pattern, "/plugin-frame/") || strings.HasPrefix(pattern, "/api/whats-new/") {
		values["name"] = "x"
	}
	out := pattern
	for k, v := range values {
		out = strings.ReplaceAll(out, "{"+k+"}", v)
	}
	switch {
	case strings.HasSuffix(out, "/file"), strings.HasSuffix(out, "/raw"), strings.HasSuffix(out, "/blob"), strings.HasSuffix(out, "/file-diff"):
		out += "?path=README.md"
	case strings.HasSuffix(out, "/definition"), strings.HasSuffix(out, "/references"):
		out += "?path=README.md&line=1&col=1"
	}
	return out
}

// follow has a page's socket on s follow topic until t ends.
func follow(t *testing.T, s *web.Server, topic string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	h := http.Header{}
	h.Set("Origin", s.Info().Origin())
	h.Set("Authorization", "Bearer "+s.Info().Token)
	c, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(s.Info().Origin(), "http")+"/api/socket", &websocket.DialOptions{HTTPHeader: h})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.CloseNow() })
	ask, _ := json.Marshal(web.SocketAsk{Op: web.SocketFollow, Topic: topic})
	if err := c.Write(ctx, websocket.MessageText, ask); err != nil {
		t.Fatal(err)
	}
	go func() { // what it's told is let be
		for {
			if _, _, err := c.Read(context.Background()); err != nil {
				return
			}
		}
	}()
}
