// Package fakeplugin is a plugin for tests: it speaks the protocol a real
// one does, from the test's own binary. A test package's TestMain calls
// RunIfAsked first, and Install puts it in a workspace.
package fakeplugin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/acp"
)

// The environment that makes a test's binary a plugin, and says how it
// behaves.
const (
	envRun      = "WS_FAKE_PLUGIN"
	envRequire  = "WS_FAKE_PLUGIN_REQUIRE" // a key it requires
	envCommand  = "WS_FAKE_PLUGIN_COMMAND" // the word after ws it asks for
	envProtocol = "WS_FAKE_PLUGIN_PROTOCOL"
)

// RunIfAsked runs the plugin and exits, if this process was started as
// one. Otherwise it returns, and the tests run.
func RunIfAsked() {
	if os.Getenv(envRun) == "" {
		return
	}
	run()
	os.Exit(0)
}

// Options are how an installed fake behaves.
type Options struct {
	// Require is a key it requires ("token", a secret), "" for none.
	Require string
	// Command is the word after ws it asks for; "" for its own name.
	Command string
	// Protocol is the version it says it speaks; 0 for the right one.
	Protocol int
}

// Install puts a fake plugin called name in a folder, and returns the
// folder, for plugin.Add. Every fake a test starts behaves as o says.
func Install(t *testing.T, name string, o Options) string {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := fmt.Sprintf("name = %q\nversion = \"1.2.3\"\ncommand = [%q]\n", name, self)
	if err := os.WriteFile(filepath.Join(dir, "plugin.toml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(envRun, "1")
	t.Setenv(envRequire, o.Require)
	t.Setenv(envCommand, o.Command)
	t.Setenv(envProtocol, "")
	if o.Protocol != 0 {
		t.Setenv(envProtocol, fmt.Sprint(o.Protocol))
	}
	return dir
}

func run() {
	var mu sync.Mutex
	config := map[string]any{}
	done := make(chan struct{})

	var conn *acp.Conn
	ready := make(chan struct{})
	conn = acp.NewConn(os.Stdin, os.Stdout, func(ctx context.Context, method string, params json.RawMessage) (any, error) {
		<-ready
		switch method {
		case "initialize":
			var p struct {
				Plugin struct{ Name string } `json:"plugin"`
				Config map[string]any        `json:"config"`
				View   struct{ Listen, Base string }
			}
			if err := json.Unmarshal(params, &p); err != nil {
				return nil, err
			}
			mu.Lock()
			config = p.Config
			mu.Unlock()
			os.Remove(p.View.Listen)
			ln, err := net.Listen("unix", p.View.Listen)
			if err != nil {
				return nil, err
			}
			go http.Serve(ln, view(&mu, &config)) //nolint:errcheck // ends with the process
			reg := map[string]any{
				"description": "A plugin for tests",
				"views":       []map[string]any{{"title": "Fake", "icon": "box", "path": "/"}},
				"commands":    map[string]any{"name": os.Getenv(envCommand), "short": "A fake's commands"},
				"agent":       map[string]any{"guide": "Use `ws " + p.Plugin.Name + "`."},
				"config":      []map[string]any{{"key": "greeting", "title": "Greeting", "type": "string", "default": "Hello"}},
			}
			if key := os.Getenv(envRequire); key != "" {
				reg["config"] = append(reg["config"].([]map[string]any), map[string]any{"key": key, "title": "Token", "type": "secret", "required": true})
			}
			protocol := 1
			if v := os.Getenv(envProtocol); v != "" {
				fmt.Sscan(v, &protocol)
			}
			return map[string]any{"protocol": protocol, "registration": reg}, nil
		case "config/changed":
			var p struct {
				Config map[string]any `json:"config"`
			}
			if json.Unmarshal(params, &p) == nil {
				mu.Lock()
				config = p.Config
				mu.Unlock()
			}
			return nil, nil
		case "command/run":
			var p struct {
				ID   string   `json:"id"`
				Args []string `json:"args"`
				Dir  string   `json:"dir"`
			}
			if err := json.Unmarshal(params, &p); err != nil {
				return nil, err
			}
			out := func(stream, data string) {
				conn.Notify("command/output", map[string]string{"id": p.ID, "stream": stream, "data": data}) //nolint:errcheck // ws has gone
			}
			if len(p.Args) == 0 {
				out("stdout", "usage\n")
				return map[string]int{"exit": 0}, nil
			}
			switch p.Args[0] {
			case "echo":
				out("stdout", strings.Join(p.Args[1:], " ")+"\n")
			case "dir":
				out("stdout", p.Dir+"\n")
			case "fail":
				out("stderr", "it failed\n")
				return map[string]int{"exit": 3}, nil
			case "config":
				mu.Lock()
				data, _ := json.Marshal(config)
				mu.Unlock()
				out("stdout", string(data)+"\n")
			case "sleep":
				out("stdout", "sleeping\n")
				<-ctx.Done()
				return nil, ctx.Err()
			case "badge":
				conn.Notify("view/badge", map[string]any{"path": "/", "count": 3}) //nolint:errcheck // ws has gone
			case "changed":
				conn.Notify("view/changed", map[string]any{}) //nolint:errcheck // ws has gone
			case "crash":
				os.Exit(7)
			default:
				out("stderr", "unknown command "+p.Args[0]+"\n")
				return map[string]int{"exit": 2}, nil
			}
			return map[string]int{"exit": 0}, nil
		case "shutdown":
			go func() {
				time.Sleep(10 * time.Millisecond) // after the answer's written
				close(done)
			}()
			return nil, nil
		}
		return nil, &acp.RPCError{Code: acp.CodeMethodNotFound, Message: "no such method"}
	})
	close(ready)
	select {
	case <-done:
	case <-conn.Done(): // ws closed stdin
	}
}

// view is the fake's pages.
func view(mu *sync.Mutex, config *map[string]any) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		greeting, _ := (*config)["greeting"].(string)
		mu.Unlock()
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Content-Security-Policy", "default-src 'none'") // ws's replaces it
		fmt.Fprintf(w, "<h1>%s from the fake</h1><p>prefix=%s cookie=%q</p>", greeting, r.Header.Get("X-Forwarded-Prefix"), r.Header.Get("Cookie"))
	})
	mux.HandleFunc("GET /go", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/there", http.StatusSeeOther)
	})
	mux.HandleFunc("GET /there", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "there") //nolint:errcheck // a test's page
	})
	mux.HandleFunc("POST /form", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "posted "+r.FormValue("q")) //nolint:errcheck // a test's page
	})
	return mux
}
