package plugin_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/withworkspace/plugin/protocol"
	"github.com/withworkspace/workspace/internal/plugin"
	"github.com/withworkspace/workspace/internal/plugin/kitplugin"
	"github.com/withworkspace/workspace/internal/store"
	"github.com/withworkspace/workspace/internal/store/storetest"
)

// keeping is a workspace whose host keeps how plugins' things are named
// in its records, as the daemon's does.
func keeping(t *testing.T) (string, *plugin.Host) {
	t.Helper()
	root, h := workspace(t)
	storetest.Own(t, root)
	h.Keep = func(name string, refs []protocol.Ref) ([]string, error) { return plugin.KeepRefs(root, name, refs) }
	return root, h
}

func prefixesOf(t *testing.T, root string) map[string]string {
	t.Helper()
	kept, err := plugin.Refs(root)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, k := range kept {
		out[k.Plugin+"/"+k.Kind] = strings.Join(k.Prefixes, " ")
	}
	return out
}

func TestRefsAreKeptByThePluginThatRegisteredThem(t *testing.T) {
	root := t.TempDir()
	storetest.Own(t, root)
	issue := protocol.Ref{Kind: "issue", Title: "Jira issue", Prefixes: []string{"RAM-", "RAP-"}, Address: "https://acme.atlassian.net/browse/{ref}", Path: "/issue/{ref}"}
	sprint := protocol.Ref{Kind: "sprint", Prefixes: []string{"SPRINT-"}}

	refused, err := plugin.KeepRefs(root, "jira", []protocol.Ref{issue, sprint})
	if err != nil || len(refused) != 0 {
		t.Fatalf("keeping jira's: %v, %v", refused, err)
	}
	kept, err := plugin.Refs(root)
	if err != nil || len(kept) != 2 || kept[0].Plugin != "jira" || kept[0].Kind != "issue" || kept[1].Kind != "sprint" || kept[0].Registered.IsZero() {
		t.Fatalf("kept: %+v, %v", kept, err)
	}
	if n := kept[0].Named(); n.Plugin != "jira" || n.What != "issue" || !n.Names("RAP-7") || n.Names("SPRINT-7") || n.Address != issue.Address {
		t.Errorf("as it's named: %+v", n)
	}
	first := kept[0].Registered

	// The first to register a prefix keeps it: another plugin's ref
	// names by what's left, and one with nothing left isn't kept.
	refused, err = plugin.KeepRefs(root, "linear", []protocol.Ref{
		{Kind: "issue", Prefixes: []string{"RAM-", "ENG-"}},
		{Kind: "cycle", Prefixes: []string{"SPRINT-"}},
		{Kind: "Bad", Prefixes: []string{"X-"}},
		{Kind: "doc", Prefixes: []string{"ENG-", "DOC-"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	said := strings.Join(refused, "\n")
	for _, want := range []string{`"issue" doesn't name by RAM-: that's the plugin jira's already`, `"cycle" doesn't name by SPRINT-`, `"Bad" is left out`, `"doc" doesn't name by ENG-: that's its ref issue's already`} {
		if !strings.Contains(said, want) {
			t.Errorf("refused doesn't say %q:\n%s", want, said)
		}
	}
	got := prefixesOf(t, root)
	want := map[string]string{"jira/issue": "RAM- RAP-", "jira/sprint": "SPRINT-", "linear/issue": "ENG-", "linear/doc": "DOC-"}
	if len(got) != len(want) {
		t.Errorf("kept %v, want %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("kept %s as %q, want %q", k, got[k], v)
		}
	}

	// Registered again as it was, nothing's written: it's from when it was.
	if _, err := plugin.KeepRefs(root, "jira", []protocol.Ref{issue, sprint}); err != nil {
		t.Fatal(err)
	}
	if kept, _ := plugin.RefsOf(root, "jira"); len(kept) != 2 || !kept[0].Registered.Equal(first) {
		t.Errorf("registered again: %+v, first %v", kept, first)
	}
	// Registered differently, what it registered before is replaced: a
	// prefix it lets go of is another's to take.
	if _, err := plugin.KeepRefs(root, "jira", []protocol.Ref{{Kind: "issue", Prefixes: []string{"RAP-"}}}); err != nil {
		t.Fatal(err)
	}
	if refused, err := plugin.KeepRefs(root, "linear", []protocol.Ref{{Kind: "issue", Prefixes: []string{"RAM-", "ENG-"}}}); err != nil || len(refused) != 0 {
		t.Errorf("taking what jira let go of: %v, %v", refused, err)
	}
	if got := prefixesOf(t, root); got["jira/issue"] != "RAP-" || got["linear/issue"] != "RAM- ENG-" || len(got) != 2 {
		t.Errorf("after: %v", got)
	}
}

func TestForgetRemovesWhatWSKeptOfAPlugin(t *testing.T) {
	root := t.TempDir()
	db := storetest.Own(t, root)
	for _, name := range []string{"jira", "linear"} {
		prefix := strings.ToUpper(name[:3]) + "-"
		if _, err := plugin.KeepRefs(root, name, []protocol.Ref{{Kind: "issue", Prefixes: []string{prefix}}}); err != nil {
			t.Fatal(err)
		}
		// What a plugin has in the graph is kept under it.
		err := db.Write(context.Background(), func(tx *store.Tx) error {
			return tx.Thing(plugin.Origin(name), store.Thing{Key: store.Key{Name: prefix + "1"}, Kind: "issue", Title: "One of " + name + "'s"})
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := plugin.Forget(root, "jira"); err != nil {
		t.Fatal(err)
	}
	if got := prefixesOf(t, root); len(got) != 1 || got["linear/issue"] != "LIN-" {
		t.Errorf("kept after forgetting jira: %v", got)
	}
	var things []string
	err := db.Read(context.Background(), func(tx *store.Tx) error {
		rows, err := tx.Query(`SELECT name FROM things ORDER BY name`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var n string
			if err := rows.Scan(&n); err != nil {
				return err
			}
			things = append(things, n)
		}
		return rows.Err()
	})
	if err != nil || !slices.Equal(things, []string{"LIN-1"}) {
		t.Errorf("in the graph after forgetting jira: %v, %v", things, err)
	}
	// Forgetting one ws keeps nothing of is nothing.
	if err := plugin.Forget(root, "nobody"); err != nil {
		t.Error(err)
	}
}

// ws and the framework, on refs: what the plugin declared is kept as it
// starts, and what one of its things is, it says when asked.
func TestKitRegistersRefsAndShowsAThing(t *testing.T) {
	root, h := keeping(t)
	addKit(t, root)
	st, err := h.Refresh("kit")
	if err != nil || len(st.Registration.Refs) != 1 || len(st.Refused) != 0 {
		t.Fatalf("started: %+v, %v", st, err)
	}
	kept, err := plugin.RefsOf(root, "kit")
	if err != nil || len(kept) != 1 {
		t.Fatalf("kept: %+v, %v", kept, err)
	}
	if k := kept[0]; k.Kind != "thing" || k.Title != "Kit thing" || !slices.Equal(k.Prefixes, []string{"KIT-"}) || k.Address != "https://kit.example/things/{ref}" || k.Path != "/thing/{ref}" || k.Icon != "box" {
		t.Errorf("kept: %+v", k)
	}

	ctx := context.Background()
	shown, err := h.Show(ctx, "kit", "thing", "KIT-1", false)
	if err != nil || shown.Title != "The first thing" || shown.State != "Open" || len(shown.Fields) != 1 || shown.Fields[0].Value != "Dana" || shown.Markdown != "It says hello." {
		t.Errorf("KIT-1: %+v, %v", shown, err)
	}
	if shown, err := h.Show(ctx, "kit", "thing", "KIT-1", true); err != nil || !strings.Contains(shown.Markdown, "in full, goodbye") {
		t.Errorf("KIT-1 in full: %+v, %v", shown, err)
	}
	// One the plugin has none of, and names that aren't its own: it isn't
	// asked about those at all.
	for name, why := range map[string]string{
		"KIT-2":    "kit has no thing called KIT-2",
		"KIT-x":    "KIT-x isn't a name kit registered for its thing",
		"OTHER-1":  "isn't a name kit registered",
		"KIT-1/..": "isn't a name kit registered",
	} {
		if _, err := h.Show(ctx, "kit", "thing", name, false); !errors.Is(err, plugin.ErrNoThing) || !strings.Contains(err.Error(), why) {
			t.Errorf("%s: %v, want %q", name, err, why)
		}
	}
	if _, err := h.Show(ctx, "kit", "sprint", "KIT-1", false); !errors.Is(err, plugin.ErrNoThing) {
		t.Errorf("a kind it didn't register: %v", err)
	}

	// Over the daemon's socket, as a command asks.
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
	if shown, err := c.Show(ctx, "kit", "thing", "KIT-1", false); err != nil || shown.Title != "The first thing" {
		t.Errorf("over the socket: %+v, %v", shown, err)
	}
	if _, err := c.Show(ctx, "kit", "thing", "KIT-2", false); !errors.Is(err, plugin.ErrNoThing) || err.Error() != "kit has no thing called KIT-2" {
		t.Errorf("over the socket, one it hasn't: %v", err)
	}
}

// A second plugin that names things as the first does keeps none of the
// first's prefixes, and says so where its status is read.
func TestAPrefixIsTheFirstPluginsToRegisterIt(t *testing.T) {
	root, h := keeping(t)
	addKit(t, root)
	if _, err := h.Refresh("kit"); err != nil {
		t.Fatal(err)
	}
	if _, err := plugin.Add(root, kitplugin.Install(t, "second"), false); err != nil {
		t.Fatal(err)
	}
	st, err := h.Refresh("second")
	if err != nil || st.State != plugin.Ready {
		t.Fatalf("second: %+v, %v", st, err)
	}
	if len(st.Refused) != 1 || !strings.Contains(st.Refused[0], "KIT-: that's the plugin kit's already") {
		t.Errorf("refused: %v", st.Refused)
	}
	if got := prefixesOf(t, root); len(got) != 1 || got["kit/thing"] != "KIT-" {
		t.Errorf("kept: %v", got)
	}
	// Removed and forgotten, its prefix is free again.
	h.Stop("kit")
	if err := plugin.Remove(root, "kit", false); err != nil {
		t.Fatal(err)
	}
	if err := plugin.Forget(root, "kit"); err != nil {
		t.Fatal(err)
	}
	if st, err := h.Refresh("second"); err != nil || len(st.Refused) != 0 {
		t.Errorf("second, once kit's gone: %+v, %v", st, err)
	}
	if got := prefixesOf(t, root); len(got) != 1 || got["second/thing"] != "KIT-" {
		t.Errorf("kept once kit's gone: %v", got)
	}
}
