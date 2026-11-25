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
	// What its things are made of, found by and drawn from is kept with it.
	if k := kept[0]; len(k.Fields) != 6 || k.Typed(protocol.FieldTitle) != "title" || k.Typed(protocol.FieldState) != "status" || len(k.Search) != 2 || len(k.Views[protocol.ViewCard]) != 3 || len(k.Views[protocol.ViewPane]) != 5 {
		t.Errorf("kept: fields %+v, search %+v, views %+v", k.Fields, k.Search, k.Views)
	}

	ctx := context.Background()
	shown, err := h.Show(ctx, "kit", "thing", "KIT-1", false)
	if f := shown.Fields; err != nil || f["title"] != "The first thing" || f["status"] != "Open" || f["owner"] != "Dana" || !strings.HasPrefix(f["body"], "It says hello") || shown.Changed.IsZero() {
		t.Errorf("KIT-1: %+v, %v", shown, err)
	}
	if shown, err := h.Show(ctx, "kit", "thing", "KIT-1", true); err != nil || !strings.Contains(shown.Fields["body"], "in full, goodbye") {
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
	if shown, err := c.Show(ctx, "kit", "thing", "KIT-1", false); err != nil || shown.Fields["title"] != "The first thing" {
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

// issueKind is a kind as a plugin registers one: named RAM-n and RAP-n,
// made of a title, a status, an assignee and a body, found by its
// assignee and the start of its body.
var issueKind = protocol.Ref{
	Kind: "issue", Title: "Jira issue", Prefixes: []string{"RAM-", "RAP-"},
	Fields: []protocol.Field{
		{Name: "title", Type: protocol.FieldTitle}, {Name: "status", Type: protocol.FieldState},
		{Name: "assignee", Type: protocol.FieldText, Label: "Assignee"}, {Name: "body", Type: protocol.FieldMarkdown},
	},
	Search: []protocol.Searched{{Field: "assignee"}, {Field: "body", Max: 40}},
}

func issue(ref string, fields map[string]string) protocol.Thing {
	return protocol.Thing{Kind: "issue", Ref: ref, Fields: fields}
}

// What a plugin tells ws of its things is kept as it's told, a value for
// each of its kind's fields, under the plugin: named as it registered,
// and no other way. What ws reads of one is made from that.
func TestAPluginsThingsAreKeptUnderIt(t *testing.T) {
	root := t.TempDir()
	db := storetest.Own(t, root)
	sprint := protocol.Ref{Kind: "sprint", Prefixes: []string{"SPRINT-"}, Fields: []protocol.Field{{Name: "name", Type: protocol.FieldTitle}}}
	if _, err := plugin.KeepRefs(root, "jira", []protocol.Ref{issueKind, sprint, {Kind: "epic", Prefixes: []string{"EPIC-"}}}); err != nil {
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
	flaky := protocol.Thing{Kind: "issue", Ref: "RAM-1", Changed: when, Fields: map[string]string{
		"title": "Token  refresh\nis flaky", "status": "In Progress", "assignee": "Dana",
		"body": "The **clock** should be injected, as the kettle is. Past forty bytes: lantern.", "colour": "not a field of its kind",
	}}
	refused, err := plugin.PutThings(root, "jira", []protocol.Thing{
		flaky,
		issue("RAM-2", map[string]string{"title": "Cache the sessions"}),
		{Kind: "sprint", Ref: "SPRINT-4", Fields: map[string]string{"name": "Sprint 4"}},
		issue("ram-3", map[string]string{"title": "Lower case"}),
		issue("SPRINT-5", map[string]string{"title": "Another kind's name"}),
		{Kind: "story", Ref: "RAM-6", Fields: map[string]string{"title": "A kind it didn't register"}},
		issue("RAM-7", map[string]string{"title": "  ", "status": "Open"}),
		issue("RAP-1", map[string]string{"title": "ws has one of its own"}),
		{Kind: "epic", Ref: "EPIC-1", Fields: map[string]string{"title": "Its kind has no fields"}},
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
		"story RAM-6":    "ws keeps no ref of jira's called story",
		"issue RAM-7":    "it has nothing for title, its title",
		"issue RAP-1":    "ws has something of its own called RAP-1",
		"epic EPIC-1":    "epic has no title field",
	} {
		if !strings.Contains(why[ref], want) {
			t.Errorf("%s refused as %q, want %q", ref, why[ref], want)
		}
	}
	if len(refused) != 6 {
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
	// As it was told, for what its kind has a field for: what ws draws
	// it from without asking.
	rec, ok, err := plugin.ThingOf(root, "jira", "issue", "RAM-1")
	if err != nil || !ok || !rec.Changed.Equal(when) || len(rec.Fields) != 4 || rec.Fields["title"] != "Token refresh is flaky" || rec.Fields["assignee"] != "Dana" || !strings.HasPrefix(rec.Fields["body"], "The **clock**") {
		t.Errorf("RAM-1 as kept: %+v, %v, %v", rec, ok, err)
	}
	if _, ok, _ := plugin.ThingOf(root, "jira", "issue", "RAM-99"); ok {
		t.Error("ws has RAM-99, which it wasn't told of")
	}

	finds := func(typed string) string {
		t.Helper()
		found, _, err := db.Find(context.Background(), store.Said{Words: typed})
		if err != nil {
			t.Fatal(err)
		}
		var names []string
		for _, f := range found {
			if f.Thing.Kind == plugin.ThingKind {
				names = append(names, f.Thing.Key.Name)
			}
		}
		return strings.Join(names, " ")
	}
	// It's found by what its kind says it's found by, as far as it says,
	// and by its title and its name whatever that is.
	for typed, want := range map[string]string{"clock injected": "RAM-1", "dana": "RAM-1", "RAM-1": "RAM-1", "flaky": "RAM-1", "lantern": "", "progress": ""} {
		if got := finds(typed); got != want {
			t.Errorf("finding %q: %q, want %q", typed, got, want)
		}
	}

	// Told again as it is, nothing's written; told otherwise, it's put in
	// place of what was there.
	_, before := placesIn(t, db, "jira")
	if _, err := plugin.PutThings(root, "jira", []protocol.Thing{flaky}); err != nil {
		t.Fatal(err)
	}
	if n, after := placesIn(t, db, "jira"); n != 3 || after != before {
		t.Errorf("told again as it is: %d places, the latest %d, was %d", n, after, before)
	}
	done := protocol.Thing{Kind: "issue", Ref: "RAM-1", Changed: when.Add(time.Hour), Fields: map[string]string{"title": "Token refresh is flaky", "status": "Done", "body": "Fixed: the lantern's lit."}}
	if _, err := plugin.PutThings(root, "jira", []protocol.Thing{done}); err != nil {
		t.Fatal(err)
	}
	if got := thingsIn(t, db, "jira"); got[0] != "RAM-1=Token refresh is flaky Done" {
		t.Errorf("told otherwise: %q", got)
	}
	if n, _ := placesIn(t, db, "jira"); n != 3 {
		t.Errorf("told otherwise: %d places, want one each", n)
	}
	if got := finds("clock injected") + finds("dana"); got != "" {
		t.Errorf("what it said before is still found: %q", got)
	}

	// What it's found by is ws's to make: when its kind says otherwise,
	// it's made again from what was told, and nothing's told twice.
	wider := issueKind
	wider.Search = []protocol.Searched{{Field: "status"}, {Field: "body"}}
	if _, err := plugin.KeepRefs(root, "jira", []protocol.Ref{wider, sprint}); err != nil {
		t.Fatal(err)
	}
	for typed, want := range map[string]string{"lantern lit": "RAM-1", "done": "RAM-1", "cache sessions": "RAM-2"} {
		if got := finds(typed); got != want {
			t.Errorf("found by what its kind now says, %q: %q, want %q", typed, got, want)
		}
	}
	// With none said, it's every field but the title, which is found
	// anyway.
	plain := issueKind
	plain.Search = nil
	if _, err := plugin.KeepRefs(root, "jira", []protocol.Ref{plain, sprint}); err != nil {
		t.Fatal(err)
	}
	if got := finds("fixed lantern") + " " + finds("done"); got != "RAM-1 RAM-1" {
		t.Errorf("found by every field: %q", got)
	}

	// Removed by name; one that isn't there is nothing to remove.
	if err := plugin.RemoveThings(root, "jira", "issue", []string{"RAM-2", "RAM-99"}); err != nil {
		t.Fatal(err)
	}
	if got, _ := plugin.ListThings(root, "jira", "issue"); !slices.Equal(got, []string{"RAM-1"}) {
		t.Errorf("after removing RAM-2: %v", got)
	}

	// A field that's its title no more leaves nothing to call it by, a
	// kind the plugin registers no more and a prefix it lets go of name
	// nothing: their things go.
	untitled := issueKind
	untitled.Fields = []protocol.Field{{Name: "summary", Type: protocol.FieldTitle}, {Name: "body", Type: protocol.FieldMarkdown}}
	untitled.Search = nil
	if _, err := plugin.KeepRefs(root, "jira", []protocol.Ref{untitled}); err != nil {
		t.Fatal(err)
	}
	if got := thingsIn(t, db, "jira"); len(got) != 0 {
		t.Errorf("kept once it names them no more: %q", got)
	}
	if n, _ := placesIn(t, db, "jira"); n != 0 {
		t.Errorf("%d places kept once it names them no more", n)
	}
	if got, _ := plugin.ListThings(root, "jira", ""); len(got) != 0 {
		t.Errorf("records kept once it names them no more: %v", got)
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
		kind := issueKind
		kind.Prefixes, kind.Search = []string{prefix}, nil
		if _, err := plugin.KeepRefs(root, name, []protocol.Ref{kind}); err != nil {
			t.Fatal(err)
		}
		if refused, err := plugin.PutThings(root, name, []protocol.Thing{issue(prefix+"1", map[string]string{"title": "A lantern", "body": "On the shelf."})}); err != nil || len(refused) != 0 {
			t.Fatalf("%s: %v, %v", name, refused, err)
		}
	}
	if err := plugin.Forget(root, "jira"); err != nil {
		t.Fatal(err)
	}
	if got := thingsIn(t, db, "jira"); len(got) != 0 {
		t.Errorf("jira's things after it's forgotten: %q", got)
	}
	if _, ok, _ := plugin.ThingOf(root, "jira", "issue", "RAM-1"); ok {
		t.Error("jira's RAM-1 is kept after it's forgotten")
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
		if exit, err := h.Run(context.Background(), "kit", args, root, plugin.Terminal{}, nil, func(string, string) {}); err != nil || exit != 0 {
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

// linksIn is what ws's graph has a plugin's things pointing at: each as
// "from how to".
func linksIn(t *testing.T, db *store.DB, name string) []string {
	t.Helper()
	var out []string
	err := db.Read(context.Background(), func(tx *store.Tx) error {
		rows, err := tx.Query(`SELECT src_name, kind, dst_name FROM links WHERE origin LIKE ? ORDER BY src_name, kind, dst_name`, string(plugin.Origin(name))+"/%")
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var src, kind, dst string
			if err := rows.Scan(&src, &kind, &dst); err != nil {
				return err
			}
			out = append(out, src+" "+kind+" "+dst)
		}
		return rows.Err()
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// A field that names other things makes links in the graph, as its field
// says it points: to the plugin's own things, another plugin's, and pull
// requests, and to nothing ws doesn't read as a name. They go with the
// thing.
func TestAThingsFieldsLinkItToWhatTheyName(t *testing.T) {
	root := t.TempDir()
	db := storetest.Own(t, root)
	linked := issueKind
	linked.Search = nil
	linked.Fields = append(slices.Clone(issueKind.Fields),
		protocol.Field{Name: "parent", Type: protocol.FieldRef, Label: "Parent", Link: protocol.LinkIn},
		protocol.Field{Name: "links", Type: protocol.FieldRefs, Label: "Links"},
		protocol.Field{Name: "epic", Type: protocol.FieldRef, Link: "sideways"}) // a way of pointing ws doesn't know
	if _, err := plugin.KeepRefs(root, "jira", []protocol.Ref{linked}); err != nil {
		t.Fatal(err)
	}
	if _, err := plugin.KeepRefs(root, "linear", []protocol.Ref{{Kind: "issue", Prefixes: []string{"ENG-"}}}); err != nil {
		t.Fatal(err)
	}
	refused, err := plugin.PutThings(root, "jira", []protocol.Thing{
		issue("RAM-1", map[string]string{"title": "The parent"}),
		issue("RAM-2", map[string]string{"title": "A child", "parent": "RAM-1", "links": "RAM-3, ENG-7\ndemo#12 RAM-2 ram-9 nothing UTF-8 RAM-3", "epic": "RAP-4"}),
	})
	if err != nil || len(refused) != 0 {
		t.Fatalf("put: %v, %v", refused, err)
	}
	want := []string{"RAM-2 in RAM-1", "RAM-2 mentions ENG-7", "RAM-2 mentions RAM-3", "RAM-2 mentions RAP-4", "RAM-2 mentions demo#12"}
	if got := linksIn(t, db, "jira"); !slices.Equal(got, want) {
		t.Errorf("links %q, want %q", got, want)
	}
	// What points at the parent is found from the parent: the child, in it.
	err = db.Read(context.Background(), func(tx *store.Tx) error {
		to, err := tx.LinksTo(store.Key{Name: "RAM-1"})
		if err != nil || len(to) != 1 || to[0].Src.Name != "RAM-2" || to[0].Kind != store.In || to[0].From.Kind != plugin.ThingKind || to[0].From.Title != "A child" {
			t.Errorf("what points at RAM-1: %+v, %v", to, err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// Told otherwise, its links are what it now names; removed, none.
	if _, err := plugin.PutThings(root, "jira", []protocol.Thing{issue("RAM-2", map[string]string{"title": "A child", "links": "RAM-3"})}); err != nil {
		t.Fatal(err)
	}
	if got := linksIn(t, db, "jira"); !slices.Equal(got, []string{"RAM-2 mentions RAM-3"}) {
		t.Errorf("links once told otherwise: %q", got)
	}
	if err := plugin.RemoveThings(root, "jira", "issue", []string{"RAM-2"}); err != nil {
		t.Fatal(err)
	}
	if got := linksIn(t, db, "jira"); len(got) != 0 {
		t.Errorf("links once it's removed: %q", got)
	}
}

// ws and the framework, on actions: one's run by its id, asks what it
// wants answered, and does it once it has; ws runs none the plugin didn't
// register, and one on a thing only by a name that's the kind's.
func TestKitActionsAreRunFromWS(t *testing.T) {
	root, h := keeping(t)
	db := storetest.Own(t, root)
	h.Answer = func(_ context.Context, who, method string, params json.RawMessage) (any, error) {
		if answer, ok, err := plugin.AnswerThings(root, who, method, params); ok {
			return answer, err
		}
		return nil, plugin.NoMethod(method)
	}
	addKit(t, root)
	st, err := h.Refresh("kit")
	if err != nil || len(st.Registration.Actions) != 1 || st.Registration.Actions[0].ID != "ping" || len(st.Registration.Refs[0].Actions) != 1 {
		t.Fatalf("started: %+v, %v", st.Registration, err)
	}
	waitFor(t, func() bool { return len(thingsIn(t, db, "kit")) == 1 })
	// What its first thing names is in the graph, from its sync.
	if got := linksIn(t, db, "kit"); !slices.Equal(got, []string{"KIT-1 mentions KIT-2", "KIT-1 mentions demo#12"}) {
		t.Errorf("links: %q", got)
	}
	ctx := context.Background()
	act := func(in protocol.ActionRunParams) (protocol.ActionRunResult, error) { return h.Act(ctx, "kit", in) }

	// The plugin's own: asks, then says.
	r, err := act(protocol.ActionRunParams{ID: "ping"})
	if err != nil || r.Ask == nil || r.Ask.Type != protocol.AskText || r.Ask.Title != "Who's asking?" {
		t.Fatalf("ping: %+v, %v", r, err)
	}
	if r, err := act(protocol.ActionRunParams{ID: "ping", Answers: []string{"ws"}}); err != nil || r.Ask != nil || r.Say != "pong, ws" {
		t.Errorf("ping, answered: %+v, %v", r, err)
	}
	// On a thing: asks where to, moves it, and tells ws of it as it now is.
	move := protocol.ActionRunParams{ID: "move", Kind: "thing", Ref: "KIT-1"}
	if r, err := act(move); err != nil || r.Ask == nil || r.Ask.Type != protocol.AskPick || len(r.Ask.Options) != 2 {
		t.Fatalf("move: %+v, %v", r, err)
	}
	move.Answers = []string{"Closed"}
	if r, err := act(move); err != nil || r.Say != "KIT-1 is Closed" || r.Open != "/thing/KIT-1" {
		t.Errorf("move, answered: %+v, %v", r, err)
	}
	if got := thingsIn(t, db, "kit"); len(got) != 1 || got[0] != "KIT-1=The first thing Closed" {
		t.Errorf("as ws has it once it's moved: %q", got)
	}
	// One that fails says why.
	if _, err := act(protocol.ActionRunParams{ID: "move", Kind: "thing", Ref: "KIT-2", Answers: []string{"Closed"}}); err == nil || !strings.Contains(err.Error(), "only the first thing moves") {
		t.Errorf("one that fails: %v", err)
	}
	// None it didn't register, of that kind, or by a name that isn't the
	// kind's: the plugin isn't asked.
	for what, in := range map[string]protocol.ActionRunParams{
		"one it hasn't":                {ID: "delete"},
		"a thing's, with no thing":     {ID: "move"},
		"its own, on a thing":          {ID: "ping", Kind: "thing", Ref: "KIT-1"},
		"a kind it hasn't":             {ID: "move", Kind: "sprint", Ref: "KIT-1"},
		"a name that isn't its kind's": {ID: "move", Kind: "thing", Ref: "OTHER-1"},
	} {
		if _, err := act(in); !errors.Is(err, plugin.ErrNoAction) {
			t.Errorf("%s: %v", what, err)
		}
	}
}

// A kind's things can live in a repo: ws finds one there, asks the
// plugin what its file is found by, and keeps it as the plugin's, by its
// folder. It's ws's: the plugin's own telling doesn't reach it.
func TestKitReadsAThingWSFound(t *testing.T) {
	root, h := keeping(t)
	db := storetest.Own(t, root)
	h.Answer = func(_ context.Context, who, method string, params json.RawMessage) (any, error) {
		if answer, ok, err := plugin.AnswerThings(root, who, method, params); ok {
			return answer, err
		}
		return nil, plugin.NoMethod(method)
	}
	addKit(t, root)
	st, err := h.Refresh("kit")
	if err != nil {
		t.Fatal(err)
	}
	if r := st.Registration.Refs[0].Remote; r == nil || *r != (protocol.Remote{Path: "things/{slug}/", Main: "{slug}.txt", Reads: true, Exports: true}) {
		t.Fatalf("its remote, as registered: %+v", r)
	}
	waitFor(t, func() bool { return len(thingsIn(t, db, "kit")) == 1 }) // its own, told as it starts
	ctx := context.Background()
	const name = "records/things/hearth/"

	fields, err := h.ReadRemote(ctx, "kit", protocol.RemoteReadParams{Kind: "thing", Name: name, Title: "The hearth", File: "hearth.txt", Content: []byte("It burns with a slow ember.")})
	if err != nil || fields["body"] != "It burns with a slow ember." || fields["owner"] != "whoever wrote hearth.txt" {
		t.Fatalf("read: %v, %v", fields, err)
	}
	// A file it can't read, and a kind it doesn't read: each is said.
	if _, err := h.ReadRemote(ctx, "kit", protocol.RemoteReadParams{Kind: "thing", Name: name, File: "hearth.txt", Content: []byte("torn")}); !errors.Is(err, plugin.ErrCantRead) || !strings.Contains(err.Error(), "the page is torn") {
		t.Errorf("a file it can't read: %v", err)
	}
	if _, err := h.ReadRemote(ctx, "kit", protocol.RemoteReadParams{Kind: "sprint", Name: name, File: "a.txt"}); !errors.Is(err, plugin.ErrNoRead) {
		t.Errorf("a kind it doesn't read: %v", err)
	}

	// Kept as the plugin's, called what its manifest says.
	fields["title"] = "The hearth"
	if err := plugin.KeepFound(root, "kit", []plugin.Found{{Kind: "thing", Name: name, Version: "v1", Fields: fields}, {Kind: "sprint", Name: "records/sprints/a/", Version: "v1", Fields: map[string]string{"title": "Not a kind of its"}}}, nil); err != nil {
		t.Fatal(err)
	}
	found, _, err := db.Find(ctx, store.Said{Words: "ember"})
	if err != nil || len(found) != 1 || found[0].Thing.Key.Name != name || found[0].Thing.Title != "The hearth" {
		t.Fatalf("found by what its file says: %+v, %v", found, err)
	}
	if got, err := plugin.Findings(root); err != nil || len(got) != 1 || got[0] != (plugin.Finding{Plugin: "kit", Kind: "thing", Name: name, Version: "v1"}) {
		t.Fatalf("what ws found: %+v, %v", got, err)
	}
	if f, ok, _ := plugin.FoundNamed(root, name); !ok || f.Plugin != "kit" || f.Kind != "thing" {
		t.Errorf("found by its name: %+v, %v", f, ok)
	}
	// The plugin's own telling doesn't reach it: its sync tells ws of all
	// it has, and what ws found is still there after.
	if names, _ := plugin.ListThings(root, "kit", "thing"); len(names) != 1 || names[0] != "KIT-1" {
		t.Errorf("what the plugin told ws of: %q", names)
	}
	if exit, err := h.Run(ctx, "kit", []string{"sync"}, root, plugin.Terminal{}, nil, func(string, string) {}); err != nil || exit != 0 {
		t.Fatalf("ws kit sync: exit %d, %v", exit, err)
	}
	if rec, ok, _ := plugin.ThingOf(root, "kit", "thing", name); !ok || rec.Found != "v1" || rec.Fields["body"] != "It burns with a slow ember." {
		t.Errorf("after the plugin's sync: %+v, %v", rec, ok)
	}
	// Registered again as it was: it stays. Gone from where it was found:
	// forgotten.
	if _, err := h.Refresh("kit"); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := plugin.ThingOf(root, "kit", "thing", name); !ok {
		t.Error("forgotten when the plugin started again")
	}
	if err := plugin.KeepFound(root, "kit", nil, []plugin.Finding{{Plugin: "kit", Kind: "thing", Name: name}}); err != nil {
		t.Fatal(err)
	}
	if found, _, _ := db.Find(ctx, store.Said{Words: "ember"}); len(found) != 0 {
		t.Errorf("found after it went: %+v", found)
	}
}

// ws asks a plugin for one of its own things as its folder in a repo is
// to hold it, and tells it where it is: what it gives after is made from
// what it was told, and from what's there now where that's changed.
func TestKitGivesAThingForARepo(t *testing.T) {
	root, h := keeping(t)
	addKit(t, root)
	st, err := h.Refresh("kit")
	if err != nil {
		t.Fatal(err)
	}
	if r := st.Registration.Refs[0].Remote; r == nil || !r.Exports {
		t.Fatalf("its remote, as registered: %+v", r)
	}
	ctx := context.Background()

	given, err := h.ExportRemote(ctx, "kit", protocol.RemoteExportParams{Kind: "thing", Ref: "KIT-1"})
	if err != nil || given.Name != "" || given.Base != "" || given.Title != "The first thing" || string(given.Main) != "It says hello." ||
		len(given.Files) != 1 || given.Files[0].Name != "{slug}.svg" || !given.Files[0].Picture || given.RemotePackage.Problem() != "" {
		t.Fatalf("given: %+v, %v", given, err)
	}
	// One it hasn't, and a kind it doesn't give: each is said.
	if _, err := h.ExportRemote(ctx, "kit", protocol.RemoteExportParams{Kind: "thing", Ref: "KIT-2"}); !errors.Is(err, plugin.ErrNoThing) {
		t.Errorf("one it hasn't: %v", err)
	}
	if _, err := h.ExportRemote(ctx, "kit", protocol.RemoteExportParams{Kind: "sprint", Ref: "SPRINT-1"}); !errors.Is(err, plugin.ErrNoExport) {
		t.Errorf("a kind it doesn't give: %v", err)
	}

	// Told where it is: what it gives next is from there.
	at := protocol.RemoteThing{Kind: "thing", Name: "records/things/the-first-thing/", Repo: "records", Path: "things/the-first-thing/", Title: "The first thing", Main: "the-first-thing.txt", Version: "3f9c2a"}
	if err := h.SavedRemote(ctx, "kit", protocol.RemoteSavedParams{Kind: "thing", Ref: "KIT-1", Thing: at}); err != nil {
		t.Fatal(err)
	}
	if given, err := h.ExportRemote(ctx, "kit", protocol.RemoteExportParams{Kind: "thing", Ref: "KIT-1"}); err != nil || given.Name != at.Name || given.Base != "3f9c2a" {
		t.Errorf("given once it's told where it is: %+v, %v", given, err)
	}
	// Changed there since: it's given what's there, and gives the two together.
	now := at
	now.Version = "9e8d7c"
	given, err = h.ExportRemote(ctx, "kit", protocol.RemoteExportParams{Kind: "thing", Ref: "KIT-1", Current: &protocol.RemoteOpened{RemoteThing: now, Content: []byte("it waves.")}})
	if err != nil || given.Base != "9e8d7c" || string(given.Main) != "It says hello. And: it waves." {
		t.Errorf("given with what's there now: %+v, %v", given, err)
	}
}

// A plugin says where each of its things stands, in words every kind
// shares, and since when: ws keeps it, weighs what's found of the thing
// by it, and takes a change to it alone as a change.
func TestAPluginsThingStandsAsItSays(t *testing.T) {
	root := t.TempDir()
	db := storetest.Own(t, root)
	if _, err := plugin.KeepRefs(root, "jira", []protocol.Ref{issueKind}); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)
	month := 30 * 24 * time.Hour
	told := func(ref, standing string, since time.Time) protocol.Thing {
		return protocol.Thing{Kind: "issue", Ref: ref, Changed: now.Add(-3 * month), Standing: standing, Since: since,
			Fields: map[string]string{"title": "Retry the webhook", "status": "whatever Jira calls it"}}
	}
	if _, err := plugin.PutThings(root, "jira", []protocol.Thing{
		told("RAM-1", protocol.StandingOpen, time.Time{}),
		told("RAM-2", protocol.StandingSettled, now.Add(-2*month)),
		told("RAM-3", protocol.StandingGone, now.Add(-month)),
		told("RAM-4", "", time.Time{}),
		told("RAM-5", "not one ws knows", time.Time{}),
	}); err != nil {
		t.Fatal(err)
	}
	stands := func() map[string]store.Stands {
		t.Helper()
		out := map[string]store.Stands{}
		err := db.Read(context.Background(), func(tx *store.Tx) error {
			things, err := tx.ThingsAt(store.Key{Name: "RAM-1"}, store.Key{Name: "RAM-2"}, store.Key{Name: "RAM-3"}, store.Key{Name: "RAM-4"}, store.Key{Name: "RAM-5"})
			for k, th := range things {
				out[k.Name] = th.Stands
			}
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	got := stands()
	for ref, want := range map[string]store.Stands{
		"RAM-1": {Standing: store.InForce},
		"RAM-2": {Standing: store.Settled, Since: now.Add(-2 * month)},
		"RAM-3": {Standing: store.Gone, Since: now.Add(-month)},
		"RAM-4": {Standing: store.InForce},
		"RAM-5": {Standing: store.InForce},
	} {
		if g := got[ref]; g.Standing != want.Standing || !g.Since.Equal(want.Since) {
			t.Errorf("%s stands %+v, want %+v", ref, g, want)
		}
	}
	if rec, _, _ := plugin.ThingOf(root, "jira", "issue", "RAM-2"); rec.Standing != protocol.StandingSettled || !rec.Since.Equal(now.Add(-2*month)) {
		t.Errorf("RAM-2 as kept: %+v", rec)
	}
	found, _, err := db.Find(context.Background(), store.Said{Words: "retry the webhook", Now: now})
	if err != nil {
		t.Fatal(err)
	}
	at := map[string]int{}
	for i, f := range found {
		at[f.Thing.Key.Name] = i
	}
	if !(at["RAM-1"] < at["RAM-2"] && at["RAM-2"] < at["RAM-3"]) {
		t.Errorf("found %v: what's open, then what's settled, then what's gone", at)
	}
	// Reopened: its standing alone changes, and that's a change.
	if _, err := plugin.PutThings(root, "jira", []protocol.Thing{told("RAM-2", protocol.StandingOpen, time.Time{})}); err != nil {
		t.Fatal(err)
	}
	if g := stands()["RAM-2"]; g.Standing != store.InForce || !g.Since.IsZero() {
		t.Errorf("RAM-2 reopened stands %+v", g)
	}
}
