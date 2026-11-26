package cost

import (
	"bufio"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// root is where measuring writes, for every test run: it starts once.
var root string

func TestMain(m *testing.M) {
	root, _ = os.MkdirTemp("", "cost")
	os.MkdirAll(filepath.Join(root, ".ws"), 0o755)
	code := m.Run()
	os.RemoveAll(root)
	os.Exit(code)
}

func TestDoing(t *testing.T) {
	g := []byte("goroutine 42 [semacquire, 2 minutes]:\n" +
		"sync.runtime_SemacquireMutex(0x0?, 0x0?, 0x0?)\n\t/go/src/runtime/sema.go:95 +0x1c\n" +
		"github.com/withworkspace/workspace/internal/cost.Begin.func1()\n\t/x/cost.go:1 +0x1\n" +
		"github.com/withworkspace/workspace/internal/ops.(*keptGit).soon(0x14000)\n\t/x/capsulegit.go:10 +0x2\n" +
		"github.com/withworkspace/workspace/internal/web.(*Server).mission(0x1)\n\t/x/mission.go:3 +0x3\n")
	id, what, ok := doing(g)
	if !ok || id != 42 || what != "semacquire · ops.(*keptGit).soon" {
		t.Fatalf("doing = %d %q %v", id, what, ok)
	}
}

// A request is charged what its own goroutine waits on, not what it
// leaves to another; a long one says where it waited.
func TestWrap(t *testing.T) {
	os.Setenv(Env, "1")
	if !Start(root) {
		t.Fatal("not started")
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /things/{id}", func(w http.ResponseWriter, r *http.Request) {
		done := Begin(Git)
		time.Sleep(10 * time.Millisecond)
		done()
		Stmt()
		Stmt()
		behind := make(chan struct{})
		go func() { // not the request's: it's waited for, so it's in rest
			defer Begin(GH)()
			time.Sleep(40 * time.Millisecond)
			close(behind)
		}()
		<-behind
		w.Write([]byte("ok"))
	})
	srv := httptest.NewServer(Wrap(mux))
	defer srv.Close()
	res, err := http.Get(srv.URL + "/things/7?x=1")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()

	time.Sleep(20 * time.Millisecond)
	f, _ := os.Open(filepath.Join(root, ".ws", "measure.jsonl"))
	defer f.Close()
	var line Line
	for sc := bufio.NewScanner(f); sc.Scan(); {
		if strings.Contains(sc.Text(), `"pattern"`) {
			json.Unmarshal(sc.Bytes(), &line)
		}
	}
	if line.Pattern != "GET /things/{id}" || line.Path != "/things/7?x=1" || line.Bytes != 2 {
		t.Fatalf("line = %+v", line)
	}
	if line.Costs["git"].N != 1 || line.Costs["git"].Ms < 10 || line.Stmts != 2 {
		t.Fatalf("costs = %+v stmts %d", line.Costs, line.Stmts)
	}
	if _, ok := line.Costs["gh"]; ok {
		t.Fatalf("gh on another goroutine was charged: %+v", line.Costs)
	}
	if line.RestMs < 35 {
		t.Fatalf("rest = %v", line.RestMs)
	}
	var waited bool
	for what := range line.Waits {
		if strings.HasPrefix(what, "chan receive · cost.TestWrap") {
			waited = true
		}
	}
	if !waited {
		t.Fatalf("waits = %v", line.Waits)
	}
}
