// frame-fixture is a plugin for trying ws's page for a plugin's app (the
// shell) and the frame protocol by hand, before the plugin framework
// builds apps itself. It's the least a plugin can be: it speaks the plugin
// protocol's JSON-RPC by hand, as internal/plugin/fakeplugin does, with no
// framework, registers two views and its app, and serves the app's files
// (ui/dist, built from ui/ and committed) and one JSON endpoint.
//
// Its app is on the published @withworkspace/ui-lib: two screens, a title
// for each, and a count of the ws:changed it's had. README.md says how to
// run it and what to look for.
package main

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"net"
	"net/http"
	"os"
	"sync/atomic"
	"time"

	"github.com/withworkspace/workspace/internal/acp"
)

//go:embed ui/dist
var files embed.FS

// The app's built files, as the plugin serves them under /ui/.
var dist = func() fs.FS {
	sub, err := fs.Sub(files, "ui/dist")
	if err != nil {
		panic(err) // ui/dist is embedded at build time
	}
	return sub
}()

// versioned is a file of the app's as the registration names it: its path
// under /ui/, with a hash of what's in it, so a browser asks again when
// it's been built again.
func versioned(name string) string {
	data, err := fs.ReadFile(dist, name)
	if err != nil {
		panic(err) // the app wasn't built: pnpm build, in ui/
	}
	sum := sha256.Sum256(data)
	return "/ui/" + name + "?v=" + hex.EncodeToString(sum[:4])
}

type thing struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Note string `json:"note"`
}

var things = []thing{
	{1, "Kettle", "On the hob since Tuesday."},
	{2, "Lantern", "The one from the deep link: /x/frame-fixture/things/2."},
	{3, "Compass", "Points at the sidebar."},
}

// poked counts ws frame-fixture poke: the app shows it, fetched again each
// time it's told what it shows is stale.
var poked atomic.Int64

// looking is who ws last said is looking (view/looking's params, as they
// came): ws frame-fixture looking prints it.
var looking atomic.Value

func main() {
	done := make(chan struct{})
	var conn *acp.Conn
	ready := make(chan struct{})
	conn = acp.NewConn(os.Stdin, os.Stdout, func(ctx context.Context, method string, params json.RawMessage) (any, error) {
		<-ready
		switch method {
		case "initialize":
			var p struct {
				View struct{ Listen string } `json:"view"`
			}
			if err := json.Unmarshal(params, &p); err != nil {
				return nil, err
			}
			os.Remove(p.View.Listen)
			ln, err := net.Listen("unix", p.View.Listen)
			if err != nil {
				return nil, err
			}
			go http.Serve(ln, serve()) //nolint:errcheck // ends with the process
			script := versioned("app.js")
			// FRAME_FIXTURE_SPEAKS=2, in the daemon's environment, makes the
			// app say it speaks only that frame protocol version, which ws
			// refuses. (The app takes ?speaks=2 on its page's address too,
			// which needs no restart.)
			if v := os.Getenv("FRAME_FIXTURE_SPEAKS"); v != "" {
				script += "&speaks=" + v
			}
			return map[string]any{"protocol": 1, "registration": map[string]any{
				"description": "A fixture for trying ws's page for a plugin's app, and the frame protocol",
				"views": []map[string]any{
					{"title": "Frame fixture", "icon": "box", "path": "/"},
					{"title": "Fixture: thing 2", "icon": "box", "path": "/things/2"},
				},
				"ui": map[string]string{"script": script, "style": versioned("app.css")},
				"commands": map[string]any{
					"short": "Try ws's page for a plugin's app",
					"subcommands": []map[string]string{
						{"name": "poke", "short": "Say what the view shows is stale: the open app is told (ws:changed)"},
						{"name": "looking", "short": "Print who ws last said is looking at the app, and at ws (view/looking)"},
					},
				},
				"agent": map[string]any{"guide": "`ws frame-fixture poke` tells the fixture's open app that what it shows is stale. It's for trying ws's plugin frame by hand: there's nothing in it for a mission."},
			}}, nil
		case "command/run":
			var p struct {
				ID   string   `json:"id"`
				Args []string `json:"args"`
			}
			if err := json.Unmarshal(params, &p); err != nil {
				return nil, err
			}
			out := func(stream, data string) {
				conn.Notify("command/output", map[string]string{"id": p.ID, "stream": stream, "data": data}) //nolint:errcheck // ws has gone
			}
			if len(p.Args) == 1 && p.Args[0] == "looking" {
				said, _ := looking.Load().(string)
				if said == "" {
					said = "ws hasn't said: nobody's looking"
				}
				out("stdout", said+"\n")
				return map[string]int{"exit": 0}, nil
			}
			if len(p.Args) != 1 || p.Args[0] != "poke" {
				out("stderr", "usage: ws frame-fixture poke|looking\n")
				return map[string]int{"exit": 2}, nil
			}
			n := poked.Add(1)
			conn.Notify("view/changed", map[string]any{}) //nolint:errcheck // ws has gone
			out("stdout", "Poked ("+itoa(n)+"): the open app is told what it shows is stale.\n")
			return map[string]int{"exit": 0}, nil
		case "config/changed":
			return nil, nil
		case "view/looking":
			looking.Store(string(params))
			return nil, nil
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

func itoa(n int64) string {
	b, _ := json.Marshal(n)
	return string(b)
}

// serve is the plugin's own HTTP, which ws passes /plugins/frame-fixture/…
// to: its app's files, and what its app asks it. It serves no HTML: a path
// that's neither is a plain 404.
func serve() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /ui/", http.StripPrefix("/ui/", http.FileServerFS(dist)))
	mux.HandleFunc("GET /api/things", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		json.NewEncoder(w).Encode(map[string]any{"things": things, "poked": poked.Load()}) //nolint:errcheck // the app has gone
	})
	return mux
}
