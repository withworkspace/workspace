// Package kitplugin is a plugin for tests written with the framework
// plugins are written with (github.com/withworkspace/plugin), run from the
// test's own binary. Where fakeplugin speaks the protocol by hand, to try
// what a plugin might send, this is the framework's own side of it: a test
// with it fails when ws and the framework stop agreeing. A test package's
// TestMain calls RunIfAsked first, and Install puts it in a workspace.
package kitplugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"testing/fstest"
	"time"

	"github.com/withworkspace/plugin"
	"github.com/withworkspace/plugin/protocol"
	"github.com/withworkspace/plugin/ui"
)

// The environment that makes a test's binary the plugin.
const envRun = "WS_KIT_PLUGIN"

// RunIfAsked runs the plugin and exits, if this process was started as
// one. Otherwise it returns, and the tests run.
func RunIfAsked() {
	if os.Getenv(envRun) == "" {
		return
	}
	run()
	os.Exit(0)
}

// AppJS is the app's entry, as the plugin serves it.
const AppJS = `console.log("kit")`

func run() {
	p := plugin.New(fstest.MapFS{"ui/dist/app.js": {Data: []byte(AppJS)}})
	p.Describe("A plugin written with the framework, for tests")
	greeting := p.Config.String("greeting", "Greeting", plugin.Default("Hello"))
	p.Config.Secret("token", "Token")

	p.UI("ui/dist")
	p.View("Kit", plugin.Icon("box"))
	p.View("Board", plugin.Icon("kanban"), plugin.At("/board"))
	p.HandleFunc("GET /api/greeting", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"greeting": greeting.Get()}) //nolint:errcheck // to a test
	})
	// Its things, named KIT-1 and so on (or as its config says), each a
	// title, where it stands, who has it and what it says. The one it
	// has is KIT-1; KIT-3 it has too, and never tells ws of.
	prefixes := p.Config.List("prefixes", "What its things' names start with", plugin.Default([]string{"KIT-"}))
	var status atomic.Value // where the first thing stands: an action moves it
	status.Store("Open")
	first := func(name string) plugin.Thing {
		prefix, _, _ := strings.Cut(name, "-")
		return plugin.Thing{Name: name, Changed: time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC), Fields: map[string]string{
			"title": "The first thing", "status": status.Load().(string), "owner": "Dana", "body": "It says hello, in a **lantern**'s light.", "colour": "not one of its fields",
			// What it names: another of its own, a pull request, and
			// words that name nothing.
			"links": prefix + "-2, demo#12 nothing-here " + name,
		}}
	}
	var thing *plugin.Ref
	p.Setup(func() {
		thing = p.Ref("thing", "Kit thing").
			Prefix(prefixes.Get()...).Address("https://kit.example/things/{ref}").Opens("/thing/{ref}").Icon("box").
			Fields(plugin.Title("title"), plugin.State("status"), plugin.Text("owner", "Owner"), plugin.Markdown("body"),
				plugin.Names("parent", "Parent", protocol.LinkIn), plugin.NamesSeveral("links", "Links")).
			Search(plugin.All("owner"), plugin.Upto("body", 200)).
			Card(ui.Title("title"), ui.Subtitle(ui.Key), ui.Badge("status")).
			Pane(ui.Title("title"), ui.Subtitle(ui.Key), ui.Badge("status"), ui.Field("owner"), ui.Markdown("body")).
			// One can live in a repo, a text file in a folder of its own:
			// what's in the file is what it says, unless the page is torn.
			Remote(plugin.Remote{Path: "things/{slug}/", Main: "{slug}.txt", Read: func(_ context.Context, f plugin.RemoteFile) (map[string]string, error) {
				if strings.Contains(string(f.Content), "torn") {
					return nil, errors.New("the page is torn")
				}
				return map[string]string{"body": string(f.Content), "owner": "whoever wrote " + f.File}, nil
			}}).
			// Something to do to one: it asks where to, moves it, and
			// tells ws of it as it now is.
			Action("move", "Move to…", func(a *plugin.Act) error {
				to, err := a.Pick("Move "+a.Name+" to", plugin.Choices("Open", "Closed")...)
				if err != nil {
					return err
				}
				if _, n, _ := strings.Cut(a.Name, "-"); n != "1" {
					return errors.New("only the first thing moves")
				}
				status.Store(to)
				if err := thing.Put(a.Context(), first(a.Name)); err != nil && !errors.Is(err, plugin.ErrUnsupported) {
					return err
				}
				a.Open("/thing/" + a.Name)
				return a.Say(a.Name + " is " + to)
			}).
			Show(func(_ context.Context, a plugin.Asked) (plugin.Thing, error) {
				_, n, _ := strings.Cut(a.Name, "-")
				switch n {
				case "1":
					t := first(a.Name)
					if a.Full {
						t.Fields["body"] += "\n\nAnd, in full, goodbye."
					}
					return t, nil
				case "3":
					return plugin.Thing{Name: a.Name, Fields: map[string]string{"title": "The third thing", "status": "Off the board"}}, nil
				}
				return plugin.Thing{}, plugin.ErrNotFound
			})
	})
	p.Guide("Use `ws kit hi` to be greeted.", "crew")
	// Its sync tells ws of the things it has: the one, named by its first
	// prefix, and a second it's been told of (ws kit more), so ws has
	// them without asking. A ws that can't be told is one from before it.
	var more atomic.Bool
	p.Sync("things", time.Hour, func(ctx context.Context) error {
		if len(prefixes.Get()) == 0 {
			return nil
		}
		things := []plugin.Thing{first(prefixes.Get()[0] + "1")}
		if more.Load() {
			things = append(things, plugin.Thing{Name: prefixes.Get()[0] + "2", Fields: map[string]string{"title": "The second thing", "body": "It says goodbye.", "parent": prefixes.Get()[0] + "1"}})
		}
		if err := thing.Mirror(ctx, things); err != nil && !errors.Is(err, plugin.ErrUnsupported) {
			return err
		}
		return nil
	})
	// Something it can do, from Find.
	p.Action("ping", "Ping the kit", func(a *plugin.Act) error {
		who, err := a.Ask("Who's asking?", "A name")
		if err != nil {
			return err
		}
		return a.Say("pong, " + who)
	}).Icon("box").Hint("It answers")
	// Something a conversation's agent may propose: notes, each made with
	// the next number, one under another (#N) saying so.
	var notes atomic.Int32
	p.Proposes("notes", "Kit notes").Icon("box").
		When("Notes to keep.").
		Guide(func(context.Context) (string, error) {
			return `A JSON list of notes: [{"text": "…", "under": "#1"}]`, nil
		}).
		Draft(func(d *plugin.Draft) error {
			var in []struct {
				Text  string `json:"text"`
				Under string `json:"under,omitempty"`
			}
			if err := d.Decode(&in); err != nil {
				return err
			}
			for _, n := range in {
				if n.Text == "" {
					return d.Refuse("every note has text")
				}
				d.Item(n.Text, n).Row("Under", n.Under)
			}
			return nil
		}).
		Accept(func(a *plugin.Accepting) error {
			var n struct {
				Text  string `json:"text"`
				Under string `json:"under,omitempty"`
			}
			if err := a.Decode(&n); err != nil {
				return err
			}
			under, err := a.Resolve(n.Under)
			if err != nil {
				return err
			}
			made := fmt.Sprintf("NOTE-%d", notes.Add(1))
			a.Made(made)
			if under != "" {
				a.Say(made + " under " + under)
			}
			return nil
		})
	p.Command("more", "Has a second thing, from the next sync on", func(c *plugin.Call) error {
		more.Store(c.Arg(0) != "less")
		return c.Say("noted")
	}).Args("[less]")

	// A table: shelves, each with a label, what's on it, and its hooks.
	shelves := p.Config.Table("shelves", "Shelves", plugin.RowName("Shelf"))
	shelfLabel := shelves.String("label", "Label", plugin.Required)
	shelfHolds := shelves.Number("holds", "How many it holds", plugin.Default(3))
	hooks := shelves.Table("hooks", "Hooks", plugin.RowName("Hook"))
	hookFor := hooks.String("for", "What hangs on it")
	p.Command("shelves", "Says its shelves, a row a line, as config has them", func(c *plugin.Call) error {
		for _, row := range shelves.Rows() {
			line := fmt.Sprintf("%s: %s holds %v", row.Name, shelfLabel.In(row), shelfHolds.In(row))
			for _, hook := range hooks.RowsIn(row) {
				line += fmt.Sprintf(" [%s for %s]", hook.Name, hookFor.In(hook))
			}
			fmt.Fprintln(c.Stdout(), line)
		}
		return nil
	})
	p.Command("cat", "Says whether its input's piped, then prints it, prompting first", func(c *plugin.Call) error {
		fmt.Fprintf(c.Stdout(), "piped %v\n> ", c.Piped())
		_, err := io.Copy(c.Stdout(), c.Stdin())
		return err
	})
	p.Command("hi", "Says the greeting", func(c *plugin.Call) error {
		fmt.Fprintln(c.Stderr(), "to stderr")
		return c.Say(greeting.Get() + ", ws")
	})
	p.Command("stale", "Says its view is stale, with a badge", func(c *plugin.Call) error {
		p.Changed()
		p.Badge("/board", 3)
		return c.Say("told")
	})
	p.Command("looking", "Says who ws has said is looking", func(c *plugin.Call) error {
		l := p.Looking()
		return c.Say(fmt.Sprintf("app %t, ws %t", l.App, l.WS))
	})
	p.Command("fail", "Logs an error and exits 3", func(c *plugin.Call) error {
		p.Log.Error("it went wrong")
		return plugin.Exit(3, errors.New("failed"))
	})
	p.Command("ask", "Asks ws for missions, a mission, repos or what it shows of a ref, or has it start a conversation, and prints the answer as JSON", func(c *plugin.Call) error {
		var answer any
		var err error
		switch c.Arg(0) {
		case "missions":
			answer, err = p.Missions(c.Context(), c.Flag("archived"))
		case "mission":
			answer, err = p.Mission(c.Context(), c.Arg(1))
		case "repos":
			answer, err = p.Repos(c.Context())
		case "show":
			answer, err = p.ShowIn(c.Context(), c.Option("mission"), c.Arg(1))
		case "talk":
			answer, err = p.StartConversation(c.Context(), c.Arg(1))
		}
		switch {
		case errors.Is(err, plugin.ErrNotFound):
			return c.Say("not found: " + err.Error())
		case errors.Is(err, plugin.ErrUnsupported):
			return c.Say("unsupported: " + err.Error())
		case err != nil:
			return err
		}
		return json.NewEncoder(c.Stdout()).Encode(answer)
	}).Args("missions|mission SLUG|repos|show REF|talk MESSAGE").Flag("archived", "archived missions too").Option("mission", "the mission a ref shown is one of")
	p.Run()
}

// Install puts the plugin, called name, in a folder, and returns the
// folder, for plugin.Add.
func Install(t *testing.T, name string) string {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := fmt.Sprintf("name = %q\nversion = \"0.1.0\"\ncommand = [%q]\n", name, self)
	if err := os.WriteFile(filepath.Join(dir, "plugin.toml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(envRun, "1")
	return dir
}
