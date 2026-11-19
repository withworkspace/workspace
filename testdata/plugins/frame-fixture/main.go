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
	"bytes"
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
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
			// It tells ws of its things, so ws has them without asking: Find
			// finds the Lantern by what it says. ws answers once it's
			// taken what's registered below.
			go func() {
				told := []map[string]any{}
				for _, t := range things {
					told = append(told, map[string]any{"kind": "thing", "ref": "THING-" + itoa(int64(t.ID)), "fields": fieldsOf(t, false)})
				}
				conn.Call(context.Background(), "thing/put", map[string]any{"things": told}, nil) //nolint:errcheck // a ws from before it could be told
			}()
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
						{"name": "said", "short": "Print what ws shows of a ref, as it answers a plugin (show): said THING-2, or said <mission> <ref>"},
						{"name": "talk", "short": "Have ws start a conversation with what you'd say first (conversation/start): talk \"Let's shelve THING-2\""},
					},
				},
				"agent": map[string]any{"guide": "`ws frame-fixture poke` tells the fixture's open app that what it shows is stale. It's for trying ws's plugin frame by hand: there's nothing in it for a mission."},
				// How its things are named: THING-2 is the Lantern, wherever
				// it's written in ws. What one is made of, what it's found
				// by, and how ws draws one under its name (card) and beside
				// the list in Find (pane), all as data. ws asks for one it
				// wasn't told of with ref/show.
				"refs": []map[string]any{{
					"kind": "thing", "title": "Fixture thing", "icon": "box", "prefixes": []string{"THING-"},
					"address": "https://fixture.example/things/{ref}", "path": "/things/{ref}",
					"fields": []map[string]string{
						{"name": "name", "type": "title"},
						{"name": "state", "type": "state"},
						{"name": "number", "type": "text", "label": "Number"},
						{"name": "note", "type": "markdown"},
						// What it names: ws links them in its graph, and
						// draws each as what it names.
						{"name": "part_of", "type": "ref", "label": "Part of", "link": "in"},
						{"name": "beside", "type": "refs", "label": "Beside"},
					},
					"search": []map[string]any{{"field": "note", "max": 2000}},
					"views": map[string]any{
						"card": []map[string]any{
							{"block": "title", "field": "name"}, {"block": "subtitle", "field": "key"}, {"block": "badge", "field": "state"},
							{"block": "markdown", "field": "note", "lines": 2},
						},
						"pane": []map[string]any{
							{"block": "title", "field": "name"}, {"block": "subtitle", "field": "key"}, {"block": "badge", "field": "state"},
							{"block": "field", "field": "number"}, {"block": "field", "field": "part_of"}, {"block": "field", "field": "beside"},
							{"block": "markdown", "field": "note"},
						},
					},
					// What can be done to one, from Find.
					"actions": []map[string]string{{"id": "move", "title": "Move to…"}},
				}},
				// What it can do, from Find.
				"actions": []map[string]string{{"id": "dust", "title": "Dust the shelf", "icon": "sparkle", "hint": "Every thing on it"}},
			}}, nil
		case "action/run":
			var p struct {
				ID, Kind, Ref string
				Answers       []string
			}
			if err := json.Unmarshal(params, &p); err != nil {
				return nil, err
			}
			switch {
			case p.Kind == "" && p.ID == "dust":
				return map[string]any{"say": "Dusted " + itoa(int64(len(things))) + " things"}, nil
			case p.Kind == "thing" && p.ID == "move":
				// It asks where to before it does anything: ws asks, and
				// runs it again with the answer.
				if len(p.Answers) == 0 {
					return map[string]any{"ask": map[string]any{"title": "Move " + p.Ref + " to", "type": "pick", "options": []map[string]string{
						{"value": "On the shelf", "label": "The shelf"}, {"value": "On the hob", "label": "The hob", "hint": "Where the kettle goes"}, {"value": "In the attic", "label": "The attic"},
					}}}, nil
				}
				for _, t := range things {
					if p.Ref == "THING-"+itoa(int64(t.ID)) {
						places.Store(t.ID, p.Answers[0])
						// ws is told of it as it now is.
						conn.Call(ctx, "thing/put", map[string]any{"things": []map[string]any{{"kind": "thing", "ref": p.Ref, "fields": fieldsOf(t, false)}}}, nil) //nolint:errcheck // a ws from before it could be told
						return map[string]any{"say": t.Name + " is " + strings.ToLower(p.Answers[0][:1]) + p.Answers[0][1:], "open": "/things/" + p.Ref}, nil
					}
				}
				return nil, &acp.RPCError{Code: -32001, Message: "the fixture has no " + p.Ref + " to move"}
			}
			return nil, &acp.RPCError{Code: acp.CodeInvalidParams, Message: "the fixture has no action " + p.ID}
		case "ref/show":
			var p struct {
				Kind, Ref string
				Full      bool
			}
			if err := json.Unmarshal(params, &p); err != nil {
				return nil, err
			}
			for _, t := range things {
				if p.Kind == "thing" && p.Ref == "THING-"+itoa(int64(t.ID)) {
					return map[string]any{"fields": fieldsOf(t, p.Full)}, nil
				}
			}
			// One it has and never tells ws of: found by its name alone.
			if p.Kind == "thing" && p.Ref == "THING-4" {
				return map[string]any{"fields": map[string]string{"name": "Sextant", "state": "In the attic", "number": "4", "note": "ws wasn't told of this one: the fixture was asked."}}, nil
			}
			return nil, &acp.RPCError{Code: -32001, Message: "the fixture has no " + p.Ref}
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
			if len(p.Args) >= 2 && p.Args[0] == "said" {
				// What ws show shows, asked of ws as a plugin asks: one of
				// the workspace's by its ref, or one of a mission's own.
				ask := map[string]any{"ref": p.Args[1]}
				if len(p.Args) > 2 {
					ask = map[string]any{"mission": p.Args[1], "ref": p.Args[2]}
				}
				var shown json.RawMessage
				if err := conn.Call(context.Background(), "show", ask, &shown); err != nil {
					out("stderr", err.Error()+"\n")
					return map[string]int{"exit": 1}, nil
				}
				var pretty bytes.Buffer
				json.Indent(&pretty, shown, "", "  ") //nolint:errcheck // ws's own JSON
				out("stdout", pretty.String()+"\n")
				return map[string]int{"exit": 0}, nil
			}
			if len(p.Args) >= 2 && p.Args[0] == "talk" {
				// A conversation, started for the person with what the
				// plugin has them say first.
				var started struct{ ID, Path string }
				if err := conn.Call(context.Background(), "conversation/start", map[string]any{"message": strings.Join(p.Args[1:], " ")}, &started); err != nil {
					out("stderr", err.Error()+"\n")
					return map[string]int{"exit": 1}, nil
				}
				out("stdout", started.ID+" "+started.Path+"\n")
				return map[string]int{"exit": 0}, nil
			}
			if len(p.Args) != 1 || p.Args[0] != "poke" {
				out("stderr", "usage: ws frame-fixture poke|looking|said <ref>|talk <message>\n")
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

// fieldsOf is a thing as ws is told of it: a value for each of the
// fields its kind registers.
func fieldsOf(t thing, full bool) map[string]string {
	note := t.Note
	if full {
		note += "\n\nThat's all the fixture has of it."
	}
	place := "On the shelf"
	if p, ok := places.Load(t.ID); ok {
		place = p.(string)
	}
	out := map[string]string{"name": t.Name, "state": place, "number": itoa(int64(t.ID)), "note": note}
	// The compass is part of the lantern, which sits beside the others.
	switch t.ID {
	case 2:
		out["beside"] = "THING-1, THING-3"
	case 3:
		out["part_of"] = "THING-2"
	}
	return out
}

// places is where each thing is, once it's been moved (ws frame-fixture's
// Move to…, from Find).
var places sync.Map

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
