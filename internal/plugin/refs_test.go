package plugin_test

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

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

// thingsIn is what ws keeps in its graph under a plugin: each thing's
// name with its title and where it stands.
func thingsIn(t *testing.T, db *store.DB, name string) []string {
	t.Helper()
	var out []string
	err := db.Read(context.Background(), func(tx *store.Tx) error {
		rows, err := tx.Query(`SELECT name, title, state FROM things WHERE origin LIKE ? ORDER BY name`, string(plugin.Origin(name))+"/%")
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var n, title, state string
			if err := rows.Scan(&n, &title, &state); err != nil {
				return err
			}
			out = append(out, strings.TrimSpace(n+"="+title+" "+state))
		}
		return rows.Err()
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// placesIn counts the places words are kept at under a plugin.
func placesIn(t *testing.T, db *store.DB, name string) (n, latest int) {
	t.Helper()
	err := db.Read(context.Background(), func(tx *store.Tx) error {
		return tx.QueryRow(`SELECT count(*), coalesce(max(id), 0) FROM places WHERE origin LIKE ?`, string(plugin.Origin(name))+"/%").Scan(&n, &latest)
	})
	if err != nil {
		t.Fatal(err)
	}
	return n, latest
}

// What a plugin tells ws of its things is kept in the graph, with what
// they say, under the plugin: named as it registered, and no other way.
func TestAPluginsThingsAreKeptUnderIt(t *testing.T) {
	root := t.TempDir()
	db := storetest.Own(t, root)
	issue := protocol.Ref{Kind: "issue", Prefixes: []string{"RAM-", "RAP-"}}
	if _, err := plugin.KeepRefs(root, "jira", []protocol.Ref{issue, {Kind: "sprint", Prefixes: []string{"SPRINT-"}}}); err != nil {
		t.Fatal(err)
	}
	// Something of ws's own, by a name a plugin might use.
	err := db.Write(context.Background(), func(tx *store.Tx) error {
		return tx.Thing(store.Origin("mission:RAP-1"), store.Thing{Key: store.Key{Name: "RAP-1"}, Kind: "mission", Title: "ws's own"})
	})
	if err != nil {
		t.Fatal(err)
	}
	when := time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC)
	refused, err := plugin.PutThings(root, "jira", []protocol.Thing{
		{Kind: "issue", Ref: "RAM-1", Title: "Token  refresh\nis flaky", State: "In Progress", Changed: when, Text: "The **clock** should be injected."},
		{Kind: "issue", Ref: "RAM-2", Title: "Cache the sessions"},
		{Kind: "sprint", Ref: "SPRINT-4", Title: "Sprint 4"},
		{Kind: "issue", Ref: "ram-3", Title: "Lower case"},
		{Kind: "issue", Ref: "SPRINT-5", Title: "Another kind's name"},
		{Kind: "epic", Ref: "RAM-6", Title: "A kind it didn't register"},
		{Kind: "issue", Ref: "RAM-7", Title: "  "},
		{Kind: "issue", Ref: "RAP-1", Title: "ws has one of its own"},
	})
	if err != nil {
		t.Fatal(err)
	}
	why := map[string]string{}
	for _, r := range refused {
		why[r.Kind+" "+r.Ref] = r.Why
	}
	for ref, want := range map[string]string{
		"issue ram-3":    "isn't one of issue's names",
		"issue SPRINT-5": "isn't one of issue's names",
		"epic RAM-6":     "ws keeps no ref of jira's called epic",
		"issue RAM-7":    "it has no title",
		"issue RAP-1":    "ws has something of its own called RAP-1",
	} {
		if !strings.Contains(why[ref], want) {
			t.Errorf("%s refused as %q, want %q", ref, why[ref], want)
		}
	}
	if len(refused) != 5 {
		t.Errorf("refused: %+v", refused)
	}
	want := []string{"RAM-1=Token refresh is flaky In Progress", "RAM-2=Cache the sessions", "SPRINT-4=Sprint 4"}
	if got := thingsIn(t, db, "jira"); !slices.Equal(got, want) {
		t.Errorf("kept %q, want %q", got, want)
	}
	if got, err := plugin.ListThings(root, "jira", "issue"); err != nil || !slices.Equal(got, []string{"RAM-1", "RAM-2"}) {
		t.Errorf("issues listed: %v, %v", got, err)
	}
	if got, _ := plugin.ListThings(root, "linear", "issue"); len(got) != 0 {
		t.Errorf("another plugin's issues: %v", got)
	}

	// It's found by what it says, and by its name.
	for _, typed := range []string{"clock injected", "RAM-1", "flaky"} {
		found, _, err := db.Find(context.Background(), store.Said{Words: typed})
		if err != nil || len(found) == 0 || found[0].Thing.Key.Name != "RAM-1" || found[0].Thing.Kind != plugin.ThingKind || found[0].Thing.Title != "Token refresh is flaky" {
			t.Errorf("finding %q: %+v, %v", typed, found, err)
		}
	}

	// Told again as it is, nothing's written; told otherwise, it's put in
	// place of what was there.
	_, before := placesIn(t, db, "jira")
	if _, err := plugin.PutThings(root, "jira", []protocol.Thing{{Kind: "issue", Ref: "RAM-1", Title: "Token refresh is flaky", State: "In Progress", Changed: when, Text: "The **clock** should be injected."}}); err != nil {
		t.Fatal(err)
	}
	if n, after := placesIn(t, db, "jira"); n != 3 || after != before {
		t.Errorf("told again as it is: %d places, the latest %d, was %d", n, after, before)
	}
	if _, err := plugin.PutThings(root, "jira", []protocol.Thing{{Kind: "issue", Ref: "RAM-1", Title: "Token refresh is flaky", State: "Done", Changed: when.Add(time.Hour)}}); err != nil {
		t.Fatal(err)
	}
	if got := thingsIn(t, db, "jira"); got[0] != "RAM-1=Token refresh is flaky Done" {
		t.Errorf("told otherwise: %q", got)
	}
	if n, _ := placesIn(t, db, "jira"); n != 3 {
		t.Errorf("told otherwise: %d places, want one each", n)
	}
	if found, _, _ := db.Find(context.Background(), store.Said{Words: "clock injected"}); len(found) != 0 {
		t.Errorf("what it said before is still found: %+v", found)
	}

	// Removed by name; one that isn't there is nothing to remove.
	if err := plugin.RemoveThings(root, "jira", "issue", []string{"RAM-2", "RAM-99"}); err != nil {
		t.Fatal(err)
	}
	if got, _ := plugin.ListThings(root, "jira", "issue"); !slices.Equal(got, []string{"RAM-1"}) {
		t.Errorf("after removing RAM-2: %v", got)
	}

	// A kind the plugin registers no more, and a prefix it lets go of,
	// take their things with them.
	if _, err := plugin.KeepRefs(root, "jira", []protocol.Ref{{Kind: "issue", Prefixes: []string{"RAP-"}}}); err != nil {
		t.Fatal(err)
	}
	if got := thingsIn(t, db, "jira"); len(got) != 0 {
		t.Errorf("kept once it names them no more: %q", got)
	}
	if n, _ := placesIn(t, db, "jira"); n != 0 {
		t.Errorf("%d places kept once it names them no more", n)
	}
	// And ws's own is as it was, throughout.
	var title string
	db.Read(context.Background(), func(tx *store.Tx) error { //nolint:errcheck // the scan says
		return tx.QueryRow(`SELECT title FROM things WHERE name = 'RAP-1'`).Scan(&title)
	})
	if title != "ws's own" {
		t.Errorf("ws's own RAP-1 is %q", title)
	}
}

// Removing a plugin forgets the things it told ws of, and what they say:
// another plugin's stay.
func TestForgetTakesAPluginsThingsWithIt(t *testing.T) {
	root := t.TempDir()
	db := storetest.Own(t, root)
	for name, prefix := range map[string]string{"jira": "RAM-", "jira2": "ENG-"} {
		if _, err := plugin.KeepRefs(root, name, []protocol.Ref{{Kind: "issue", Prefixes: []string{prefix}}}); err != nil {
			t.Fatal(err)
		}
		if refused, err := plugin.PutThings(root, name, []protocol.Thing{{Kind: "issue", Ref: prefix + "1", Title: "A lantern", Text: "On the shelf."}}); err != nil || len(refused) != 0 {
			t.Fatalf("%s: %v, %v", name, refused, err)
		}
	}
	if err := plugin.Forget(root, "jira"); err != nil {
		t.Fatal(err)
	}
	if got := thingsIn(t, db, "jira"); len(got) != 0 {
		t.Errorf("jira's things after it's forgotten: %q", got)
	}
	found, _, err := db.Find(context.Background(), store.Said{Words: "lantern shelf"})
	if err != nil || len(found) != 1 || found[0].Thing.Key.Name != "ENG-1" {
		t.Errorf("found after jira's forgotten: %+v, %v", found, err)
	}
}

// ws and the framework, on telling ws of things: the plugin's sync
// mirrors what it has, and ws keeps it, finds it, and forgets what's gone.
func TestKitTellsWSOfItsThings(t *testing.T) {
	root, h := keeping(t)
	db := storetest.Own(t, root)
	h.Answer = func(_ context.Context, who, method string, params json.RawMessage) (any, error) {
		if answer, ok, err := plugin.AnswerThings(root, who, method, params); ok {
			return answer, err
		}
		return nil, plugin.NoMethod(method)
	}
	addKit(t, root)
	if _, err := h.Refresh("kit"); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) {
		t.Helper()
		if exit, err := h.Run(context.Background(), "kit", args, root, plugin.Terminal{}, func(string, string) {}); err != nil || exit != 0 {
			t.Fatalf("ws kit %v: exit %d, %v", args, exit, err)
		}
	}
	// Its sync runs as it starts, the moment it's answered initialize:
	// what it tells ws waits for ws to have taken what it registered.
	waitFor(t, func() bool { return len(thingsIn(t, db, "kit")) == 1 })
	if got := thingsIn(t, db, "kit"); got[0] != "KIT-1=The first thing Open" {
		t.Errorf("kept: %q", got)
	}
	found, _, err := db.Find(context.Background(), store.Said{Words: "lantern"})
	if err != nil || len(found) != 1 || found[0].Thing.Key.Name != "KIT-1" {
		t.Errorf("found by what it says: %+v, %v", found, err)
	}
	// One more, then one fewer: all of them told again each time.
	run("more")
	run("sync")
	if got := thingsIn(t, db, "kit"); len(got) != 2 || got[1] != "KIT-2=The second thing" {
		t.Errorf("with a second: %q", got)
	}
	run("more", "less")
	run("sync")
	if got := thingsIn(t, db, "kit"); len(got) != 1 {
		t.Errorf("with the second gone: %q", got)
	}
	// Removed and forgotten, nothing of it is left.
	h.Stop("kit")
	if err := plugin.Remove(root, "kit", false); err != nil {
		t.Fatal(err)
	}
	if err := plugin.Forget(root, "kit"); err != nil {
		t.Fatal(err)
	}
	if got := thingsIn(t, db, "kit"); len(got) != 0 {
		t.Errorf("kept once it's removed: %q", got)
	}
}

func waitFor(t *testing.T, ok func() bool) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); !ok(); time.Sleep(10 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("it didn't happen in five seconds")
		}
	}
}
