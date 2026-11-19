package plugin_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/withworkspace/plugin/protocol"
	"github.com/withworkspace/workspace/internal/plugin"
	"github.com/withworkspace/workspace/internal/plugin/kitplugin"
)

// These start a plugin written with the framework, so each is ws and the
// framework speaking to each other: where they stop agreeing, one fails.

func addKit(t *testing.T, root string) {
	t.Helper()
	if _, err := plugin.Add(root, kitplugin.Install(t, "kit"), false); err != nil {
		t.Fatal(err)
	}
}

func TestKitRegistersWhatItDeclared(t *testing.T) {
	root, h := workspace(t)
	addKit(t, root)
	st, err := h.Refresh("kit")
	if err != nil {
		t.Fatal(err)
	}
	r := st.Registration
	if st.State != plugin.Ready || r.Description != "A plugin written with the framework, for tests" {
		t.Errorf("started: %+v", st)
	}
	if len(r.Views) != 2 || r.Views[0].Title != "Kit" || r.Views[0].Path != "/" || r.Views[1].Path != "/board" || r.Views[1].Icon != "kanban" {
		t.Errorf("views = %+v", r.Views)
	}
	if r.UI == nil || !strings.HasPrefix(r.UI.Script, "/ui/app.js?v=") || !plugin.ValidUIPath(r.UI.Script) {
		t.Errorf("ui = %+v", r.UI)
	}
	// Its own seven, and the framework's sync, for a plugin with syncs.
	if r.CommandName("kit") != "kit" || len(r.Commands.Subcommands) != 8 {
		t.Errorf("commands = %+v", r.Commands)
	}
	if r.Agent == nil || !strings.Contains(r.Agent.Guide, "ws kit hi") || len(r.Agent.Roles) != 1 || r.Agent.Roles[0] != "crew" {
		t.Errorf("agent = %+v", r.Agent)
	}
	keys := map[string]plugin.ConfigKey{}
	for _, k := range r.Config {
		keys[k.Key] = k
	}
	if keys["greeting"].Default != "Hello" || !keys["token"].Secret() {
		t.Errorf("config = %+v", r.Config)
	}
	// A table, with its rows' fields, one of them a table in turn.
	if s := keys["shelves"]; s.Type != protocol.TypeTable || s.Row != "Shelf" || len(s.Fields) != 3 || !s.Fields[0].Required || s.Fields[2].Type != protocol.TypeTable || len(s.Fields[2].Fields) != 1 {
		t.Errorf("a table = %+v", s)
	}
	if len(r.Syncs) != 1 || r.Syncs[0].Name != "things" || r.Syncs[0].Every != 3600 {
		t.Errorf("syncs = %+v", r.Syncs)
	}
}

func TestKitCommandsAndConfig(t *testing.T) {
	root, h := workspace(t)
	addKit(t, root)
	run := func(args ...string) (int, *output) {
		t.Helper()
		o := &output{}
		exit, err := h.Run(context.Background(), "kit", args, root, plugin.Terminal{Width: 80}, o.write)
		if err != nil {
			t.Fatal(err)
		}
		return exit, o
	}
	if exit, o := run("hi"); exit != 0 || o.stdout() != "Hello, ws\n" || !strings.Contains(o.errOut.String(), "to stderr") {
		t.Errorf("hi: exit %d, %q, %q", exit, o.stdout(), o.errOut.String())
	}

	st, _ := h.Refresh("kit")
	for _, k := range st.Registration.Config {
		if k.Key == "greeting" {
			if err := plugin.SetValue(root, "kit", k, "Howdy"); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := h.ConfigChanged("kit"); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, o := run("hi"); o.stdout() == "Howdy, ws\n" {
			break
		} else if time.Now().After(deadline) {
			t.Fatalf("after config/changed: %q", o.stdout())
		}
		time.Sleep(20 * time.Millisecond)
	}

	if exit, _ := run("fail"); exit != 3 {
		t.Errorf("fail: exit %d", exit)
	}
	if exit, _ := run("nothing-like-it"); exit == 0 {
		t.Error("a command it doesn't have succeeded")
	}
}

func TestKitTellsWS(t *testing.T) {
	root, h := workspace(t)
	addKit(t, root)
	if _, err := h.Refresh("kit"); err != nil {
		t.Fatal(err)
	}
	// Its sync runs as it starts, says how it went, and says its view is
	// stale.
	deadline := time.Now().Add(5 * time.Second)
	for st := h.Status("kit"); st.Syncs["things"].At.IsZero() || st.Changed == 0; st = h.Status("kit") {
		if time.Now().After(deadline) {
			t.Fatalf("no sync/state heard: %+v", h.Status("kit").Syncs)
		}
		time.Sleep(20 * time.Millisecond)
	}
	before := h.Status("kit").Changed
	if exit, err := h.Run(context.Background(), "kit", []string{"stale"}, "", plugin.Terminal{}, func(string, string) {}); err != nil || exit != 0 {
		t.Fatalf("stale: %d, %v", exit, err)
	}
	if exit, err := h.Run(context.Background(), "kit", []string{"fail"}, "", plugin.Terminal{}, func(string, string) {}); err != nil || exit != 3 {
		t.Fatalf("fail: %d, %v", exit, err)
	}
	// Notifications sent before a command's answer are heard before it.
	st := h.Status("kit")
	if st.Changed != before+1 || st.Badges["/board"] != 3 {
		t.Errorf("changed %d, badges %v", st.Changed, st.Badges)
	}
	if st.Error != "it went wrong" {
		t.Errorf("last error = %q", st.Error)
	}
}

// Who's looking reaches the framework: a plugin that starts while
// someone is, is told as it starts; one that's running, as it changes;
// and nobody's looking only once they've been gone a moment.
func TestKitIsToldWhosLooking(t *testing.T) {
	root, h := workspace(t)
	h.LookLinger = 300 * time.Millisecond
	addKit(t, root)
	looking := func() string {
		t.Helper()
		o := &output{}
		if exit, err := h.Run(context.Background(), "kit", []string{"looking"}, root, plugin.Terminal{}, o.write); err != nil || exit != 0 {
			t.Fatalf("looking: %d, %v", exit, err)
		}
		return strings.TrimSpace(o.stdout())
	}
	eventually := func(want string) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for got := looking(); got != want; got = looking() {
			if time.Now().After(deadline) {
				t.Fatalf("the plugin was told %q, not %q", got, want)
			}
			time.Sleep(10 * time.Millisecond)
		}
	}

	// A page of ws's is showing before the plugin's started.
	page := h.Look("")
	if st := h.Status("kit"); st.State != plugin.Stopped {
		t.Fatalf("a page showing started the plugin: %+v", st)
	}
	eventually("app false, ws true")

	// Its app comes on screen, in another page.
	app := h.Look("kit")
	eventually("app true, ws true")

	// That page goes, and for a moment the plugin isn't told: whoever it
	// was may be back.
	app()
	app() // (gone twice is gone once)
	if got := looking(); got != "app true, ws true" {
		t.Errorf("straight after its page went, the plugin was told %q", got)
	}
	eventually("app false, ws true")

	// Back within the moment, it never hears they went: not when its page
	// comes back, and not when another page of ws's is told of meanwhile.
	app = h.Look("kit")
	eventually("app true, ws true")
	app()
	other := h.Look("")
	time.Sleep(50 * time.Millisecond) // the other page's telling is done
	if got := looking(); got != "app true, ws true" {
		t.Errorf("another page showing, straight after its own went, had the plugin told %q", got)
	}
	app = h.Look("kit")
	other()
	time.Sleep(h.LookLinger + 100*time.Millisecond)
	if got := looking(); got != "app true, ws true" {
		t.Errorf("its page went and came back within the moment, and the plugin was told %q", got)
	}

	app()
	page()
	eventually("app false, ws false")
}

func TestKitServesItsApp(t *testing.T) {
	root, h := workspace(t)
	addKit(t, root)
	sock, err := h.ViewSocket("kit")
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return net.Dial("unix", sock)
	}}}
	get := func(path string) (int, string, string) {
		t.Helper()
		res, err := client.Get("http://plugin" + path)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		body, _ := io.ReadAll(res.Body)
		return res.StatusCode, res.Header.Get("Content-Type"), string(body)
	}
	script := h.Status("kit").Registration.UI.Script
	if code, ctype, body := get(script); code != 200 || !strings.HasPrefix(ctype, "text/javascript") || body != kitplugin.AppJS {
		t.Errorf("%s: %d %s %q", script, code, ctype, body)
	}
	if code, _, body := get("/api/greeting"); code != 200 || !strings.Contains(body, `"greeting":"Hello"`) {
		t.Errorf("/api/greeting: %d %q", code, body)
	}
}

// What the kit asks ws reaches Answer, said by which plugin, and comes
// back in the protocol's types, errors and all.
func TestKitAsksWS(t *testing.T) {
	root, h := workspace(t)
	var answering atomic.Bool
	var mu sync.Mutex
	var asked []string
	h.Answer = func(ctx context.Context, who, method string, params json.RawMessage) (any, error) {
		if !answering.Load() {
			return nil, plugin.NoMethod(method)
		}
		mu.Lock()
		asked = append(asked, who+" "+method+" "+string(params))
		mu.Unlock()
		switch method {
		case protocol.MethodMissionList:
			return protocol.MissionListResult{Missions: []protocol.Mission{{Slug: "cache", State: protocol.MissionActive, Created: time.Date(2026, 11, 18, 9, 0, 0, 0, time.UTC),
				Headline: &protocol.Headline{Court: protocol.CourtAgent, Text: "c1 is working"}, Capsules: []protocol.MissionCapsule{}, Path: "/missions/cache"}}}, nil
		case protocol.MethodMissionShow:
			var in protocol.MissionShowParams
			json.Unmarshal(params, &in) //nolint:errcheck // a test's
			if in.Slug != "cache" {
				return nil, plugin.NotFound("there's no mission " + in.Slug)
			}
			return protocol.MissionDetail{Mission: protocol.Mission{Slug: "cache"}, Brief: "## Goal"}, nil
		case protocol.MethodRepoList:
			return protocol.RepoListResult{Repos: []protocol.Repo{{Name: "api", Capsules: []protocol.Capsule{{Name: ".ground", Ground: true}}}}}, nil
		case protocol.MethodConversationStart:
			return protocol.ConversationStarted{ID: "w4", Path: "/conversations/w4"}, nil
		case protocol.MethodShow:
			var in protocol.ShowParams
			json.Unmarshal(params, &in) //nolint:errcheck // a test's
			if in.Ref != "KIT-1" {
				return nil, plugin.NotFound("ws has nothing called " + in.Ref)
			}
			return protocol.Shown{Ref: in.Ref, Mission: in.Mission, Head: "KIT-1: A thing", Body: []string{}, Links: []protocol.ShownLink{},
				Said: []protocol.Said{{Ref: "cache", Kind: "mission", How: protocol.SaidAbout, Standing: true, Path: "/missions/cache"}}}, nil
		}
		return nil, plugin.NoMethod(method)
	}
	addKit(t, root)
	run := func(args ...string) string {
		t.Helper()
		o := &output{}
		exit, err := h.Run(context.Background(), "kit", args, root, plugin.Terminal{}, o.write)
		if err != nil || exit != 0 {
			t.Fatalf("ask %v: exit %d, %v, %s", args, exit, err, o.errOut.String())
		}
		return o.stdout()
	}

	// A ws that answers nothing: the plugin's told to update ws.
	if got := run("ask", "repos"); !strings.Contains(got, "unsupported: ws test can't answer repo/list") {
		t.Errorf("unanswered: %q", got)
	}
	answering.Store(true)
	if got := run("ask", "missions", "--archived"); !strings.Contains(got, `"slug":"cache","title":"","state":"active","created":"2026-11-18T09:00:00Z"`) || !strings.Contains(got, `"headline":{"court":"agent","text":"c1 is working"}`) {
		t.Errorf("missions: %s", got)
	}
	if got := run("ask", "mission", "cache"); !strings.Contains(got, `"brief":"## Goal"`) {
		t.Errorf("mission: %s", got)
	}
	if got := run("ask", "mission", "nope"); !strings.Contains(got, "not found: there's no mission nope") {
		t.Errorf("no such mission: %s", got)
	}
	if got := run("ask", "repos"); !strings.Contains(got, `"ground":true`) {
		t.Errorf("repos: %s", got)
	}
	if got := run("ask", "show", "KIT-1"); !strings.Contains(got, `"head":"KIT-1: A thing"`) || !strings.Contains(got, `"said":[{"ref":"cache","kind":"mission","title":"","standing":true,"how":"about"`) || !strings.Contains(got, `"path":"/missions/cache"`) {
		t.Errorf("show: %s", got)
	}
	if got := run("ask", "show", "d1", "--mission", "cache"); !strings.Contains(got, "not found: ws has nothing called d1") {
		t.Errorf("show what isn't there: %s", got)
	}
	if got := run("ask", "talk", "Let's talk about KIT-1."); !strings.Contains(got, `{"id":"w4","path":"/conversations/w4"}`) {
		t.Errorf("a conversation started: %s", got)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(asked) != 7 || asked[0] != `kit mission/list {"archived":true}` || asked[1] != `kit mission/show {"slug":"cache"}` || asked[4] != `kit show {"ref":"KIT-1"}` || asked[5] != `kit show {"ref":"d1","mission":"cache"}` || asked[6] != `kit conversation/start {"message":"Let's talk about KIT-1."}` {
		t.Errorf("asked = %q", asked)
	}
}

// A plugin built on the kit is given a table's rows in the order they
// were written, the team's with yours laid over them, each field as its
// type and with its default: from its first start, when ws hasn't yet
// heard what it declares, and after a change.
func TestKitReadsItsTablesAsRows(t *testing.T) {
	root, h := workspace(t)
	team, _ := os.ReadFile(filepath.Join(root, "ws.toml"))
	team = append(team, []byte(`
[plugin.kit.shelves.top]
label = "Top shelf"

[plugin.kit.shelves.top.hooks.left]
for = "the lantern"

[plugin.kit.shelves.bottom]
label = "Bottom shelf"
holds = 9

[plugin.kit.shelves.broken]
holds = 1
`)...)
	if err := os.WriteFile(filepath.Join(root, "ws.toml"), team, 0o644); err != nil {
		t.Fatal(err)
	}
	addKit(t, root)
	run := func(args ...string) string {
		t.Helper()
		o := &output{}
		if exit, err := h.Run(context.Background(), "kit", args, root, plugin.Terminal{Width: 80}, o.write); err != nil || exit != 0 {
			t.Fatalf("%v: exit %d, %v, %s", args, exit, err, o.errOut.String())
		}
		return o.stdout()
	}
	// Written top then bottom, which isn't their order by name; the row
	// without the label it needs isn't given.
	want := "top: Top shelf holds 3 [left for the lantern]\nbottom: Bottom shelf holds 9\n"
	deadline := time.Now().Add(5 * time.Second)
	for {
		got := run("shelves")
		if got == want {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("shelves, as first given:\n%s\nwant:\n%s", got, want)
		}
		time.Sleep(20 * time.Millisecond)
	}

	// Yours over the team's: a field of one row, a row of your own, and
	// one of theirs left out.
	st, _ := h.Refresh("kit")
	for typed, value := range map[string]any{"shelves.bottom.holds": 2.0, "shelves.mine.label": "My shelf", "shelves.top.enabled": false} {
		path, _ := plugin.SplitPath(typed)
		at, err := plugin.Find(st.Registration.Config, path)
		if err != nil {
			t.Fatal(err)
		}
		if err := plugin.SetAt(root, "kit", at, value); err != nil {
			t.Fatal(err)
		}
	}
	if err := h.ConfigChanged("kit"); err != nil {
		t.Fatal(err)
	}
	want = "bottom: Bottom shelf holds 2\nmine: My shelf holds 3\n"
	deadline = time.Now().Add(5 * time.Second)
	for {
		got := run("shelves")
		if got == want {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("shelves, after a change:\n%s\nwant:\n%s", got, want)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// What a kit plugin proposes: listed for agents once it's registered,
// its guide asked for, what's written refused or drafted, and items done
// in order, each told what an earlier one made, nothing done twice under
// its key; through the daemon's socket, as ws propose and accepting reach
// it.
func TestKitProposes(t *testing.T) {
	root, h := workspace(t)
	addKit(t, root)
	if _, err := h.Refresh("kit"); err != nil {
		t.Fatal(err)
	}
	if got := plugin.Proposables(root); len(got) != 1 || got[0].Name() != "kit:notes" || got[0].When != "Notes to keep." {
		t.Errorf("proposables: %+v", got)
	}
	ln, err := h.Listen()
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go h.Serve(ln)
	c, err := plugin.Dial(root)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	ctx := context.Background()
	draft := func(input string) (plugin.Drafted, error) {
		var raw json.RawMessage
		if input != "" {
			raw = json.RawMessage(input)
		}
		return c.Draft(ctx, "kit", protocol.ProposalDraftParams{ID: "notes", Input: raw, Conversation: "w1"})
	}
	if d, err := draft(""); err != nil || !strings.Contains(d.Guide, "JSON list") || d.Proposes.Title != "Kit notes" {
		t.Errorf("the guide: %+v, %v", d, err)
	}
	if d, err := draft(`[{"text": ""}]`); err != nil || d.Refused != "every note has text" {
		t.Errorf("refused: %+v, %v", d, err)
	}
	if _, err := c.Draft(ctx, "kit", protocol.ProposalDraftParams{ID: "bugs", Input: json.RawMessage(`[]`)}); !errors.Is(err, plugin.ErrNoProposes) || !strings.Contains(err.Error(), "kit:notes") {
		t.Errorf("what it doesn't propose: %v", err)
	}
	d, err := draft(`[{"text": "First"}, {"text": "Second", "under": "#1"}]`)
	if err != nil || d.Proposal == nil || len(d.Proposal.Items) != 2 || d.Proposal.Items[1].Rows[0].Value != "#1" {
		t.Fatalf("drafted: %+v, %v", d, err)
	}
	accept := func(made map[int]string, ns ...int) protocol.ProposalAcceptResult {
		t.Helper()
		in := protocol.ProposalAcceptParams{ID: "notes", Key: "workspace/w1/1", Made: made, Conversation: "w1"}
		for _, n := range ns {
			in.Items = append(in.Items, protocol.AcceptItem{N: n, Payload: d.Proposal.Items[n-1].Payload})
		}
		out, err := c.Accept(ctx, "kit", in)
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	if out := accept(nil, 2); len(out.Items) != 1 || !strings.Contains(out.Items[0].Problem, "item 1") {
		t.Errorf("the second before the first: %+v", out)
	}
	if out := accept(nil, 1); out.Items[0].Made != "NOTE-1" {
		t.Errorf("the first: %+v", out)
	}
	if out := accept(map[int]string{1: "NOTE-1"}, 2); out.Items[0].Made != "NOTE-2" || out.Items[0].Say != "NOTE-2 under NOTE-1" {
		t.Errorf("the second: %+v", out)
	}
	// Asked again, under the same key: as it was, nothing made again.
	if out := accept(nil, 1, 2); out.Items[0].Made != "NOTE-1" || out.Items[1].Made != "NOTE-2" {
		t.Errorf("asked again: %+v", out)
	}
}
