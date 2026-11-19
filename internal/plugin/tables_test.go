package plugin_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/withworkspace/plugin/protocol"
	"github.com/withworkspace/workspace/internal/plugin"
)

// boardKeys is a config with tables as a board would declare it: queries,
// and spaces with a map and a table in each.
var boardKeys = []plugin.ConfigKey{
	{Key: "site", Type: protocol.TypeURL},
	{Key: "queries", Type: protocol.TypeTable, Row: "Query", Fields: []plugin.ConfigKey{
		{Key: "label", Type: protocol.TypeString},
		{Key: "query", Type: protocol.TypeString, Required: true},
		{Key: "limit", Type: protocol.TypeNumber},
		{Key: "kind", Type: protocol.TypeEnum, Options: []string{"board", "list"}},
		{Key: "tags", Type: protocol.TypeList},
	}},
	{Key: "spaces", Type: protocol.TypeTable, Row: "Space key", Fields: []plugin.ConfigKey{
		{Key: "label", Type: protocol.TypeString},
		{Key: "columns", Type: protocol.TypeMap},
		{Key: "custom_fields", Type: protocol.TypeTable, Row: "Field", Fields: []plugin.ConfigKey{
			{Key: "field", Type: protocol.TypeString},
			{Key: "card", Type: protocol.TypeBool},
		}},
	}},
	{Key: "statuses", Type: protocol.TypeMap},
}

// files writes a workspace's two config files.
func files(t *testing.T, team, yours string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "ws.toml"), []byte("[workspace]\norg = \"acme\"\n\n"+team), 0o644); err != nil {
		t.Fatal(err)
	}
	if yours != "" {
		if err := os.WriteFile(filepath.Join(root, "ws.local.toml"), []byte(yours), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// sent is a plugin's config as it's sent it: JSON, as the protocol has it.
func sent(t *testing.T, root string) (string, []plugin.ConfigProblem) {
	t.Helper()
	shaped, problems, err := plugin.Shaped(root, "jira", boardKeys)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(plugin.Wire(shaped))
	if err != nil {
		t.Fatal(err)
	}
	return string(data), problems
}

const teamBoard = `[plugin.jira]
site = "https://example.atlassian.net"

[plugin.jira.queries.initiatives]
label = "Initiatives"
query = 'project = RAP AND ("Product Impacts" = "Climate Risk")'

[plugin.jira.queries.bugs]
label = "Bugs"
query = 'project = RAM, type = Bug'
limit = 20

[plugin.jira.spaces.rap]
label = "Risk Platform"
custom_fields = { "Product Impacts" = { card = true } }

[plugin.jira.spaces.ram]
label = "Risk Modelling"
columns = { "Ready for Development" = "To Do" }

[plugin.jira.spaces.ram.custom_fields.Team]
card = true

[plugin.jira.spaces.ram.custom_fields."Story Points"]
field = "customfield_10016"
`

// A plugin is given its tables as rows, in the order they were written,
// each field as its type: a row within a row and a map among them.
func TestTablesAreSentAsRowsInTheOrderWritten(t *testing.T) {
	got, problems := sent(t, files(t, teamBoard, ""))
	if len(problems) != 0 {
		t.Errorf("problems in a config that has none: %v", problems)
	}
	want := `{"queries":[` +
		`{"name":"initiatives","values":{"label":"Initiatives","query":"project = RAP AND (\"Product Impacts\" = \"Climate Risk\")"}},` +
		`{"name":"bugs","values":{"label":"Bugs","limit":20,"query":"project = RAM, type = Bug"}}],` +
		`"site":"https://example.atlassian.net",` +
		`"spaces":[` +
		`{"name":"rap","values":{"custom_fields":[{"name":"Product Impacts","values":{"card":true}}],"label":"Risk Platform"}},` +
		`{"name":"ram","values":{"columns":{"Ready for Development":"To Do"},"custom_fields":[{"name":"Team","values":{"card":true}},{"name":"Story Points","values":{"field":"customfield_10016"}}],"label":"Risk Modelling"}}]}`
	if got != want {
		t.Errorf("sent:\n%s\nwant:\n%s", got, want)
	}
}

// Yours is laid over the team's row by row, then field by field: a row of
// yours changes that row and leaves the team's others, a row only yours
// comes after theirs, and a team's row is taken away with enabled =
// false.
func TestYoursIsLaidOverTheTeamsRowByRow(t *testing.T) {
	got, problems := sent(t, files(t, teamBoard, `[plugin.jira.queries.mine]
query = "assignee = currentUser()"

[plugin.jira.queries.bugs]
label = "My bugs"

[plugin.jira.queries.initiatives]
enabled = false

[plugin.jira.spaces.ram]
columns = { "In Review" = "Review" }

[plugin.jira.spaces.ram.custom_fields.Team]
card = false
`))
	if len(problems) != 0 {
		t.Errorf("problems: %v", problems)
	}
	for _, want := range []string{
		// The team's order, with yours after; the one switched off gone.
		`"queries":[{"name":"bugs","values":{"label":"My bugs","limit":20,"query":"project = RAM, type = Bug"}},{"name":"mine","values":{"query":"assignee = currentUser()"}}]`,
		// The team's other space is as it was.
		`{"name":"rap","values":{"custom_fields":[{"name":"Product Impacts","values":{"card":true}}],"label":"Risk Platform"}}`,
		// A field of a row of a row, changed, beside the row's others; a
		// map laid over entry by entry.
		`"columns":{"In Review":"Review","Ready for Development":"To Do"}`,
		`"custom_fields":[{"name":"Team","values":{"card":false}},{"name":"Story Points","values":{"field":"customfield_10016"}}]`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("sent lacks %s:\n%s", want, got)
		}
	}
	if strings.Contains(got, "initiatives") || strings.Contains(got, "enabled") {
		t.Errorf("a row switched off is sent:\n%s", got)
	}
}

// What's wrong in a table is said, with where: a field that isn't
// declared and a value of the wrong type are left out, and a row without
// a field it needs isn't given to the plugin.
func TestWhatsWrongInATableIsSaid(t *testing.T) {
	got, problems := sent(t, files(t, `[plugin.jira]
statuses = { "Done" = 3 }

[plugin.jira.queries.bugs]
lable = "Bugs"
query = "project = RAM"
limit = "many"
kind = "grid"
tags = ["a", 2]

[plugin.jira.queries.empty]
label = "No query"

[plugin.jira.queries.off]
enabled = false

[plugin.jira.spaces]
ram = "Risk Modelling"

[plugin.jira.spaces.rap.custom_fields.Team]
card = "yes"
colour = "red"
`, ""))
	var said []string
	for _, p := range problems {
		said = append(said, p.String())
	}
	want := []string{
		`queries.bugs.kind is one of: board, list: it's "grid"`,
		`queries.bugs.lable isn't a field of queries (it has: label, query, limit, kind, tags)`,
		`queries.bugs.limit is a number: it's "many"`,
		`queries.bugs.tags is a list of text: it has 2 in it`,
		`queries.empty has no query, which a row of queries needs: the row isn't given to the plugin`,
		`spaces.ram is a row of spaces, written as a table of its fields: it's "Risk Modelling"`,
		`spaces.rap.custom_fields.Team.card is true or false: it's "yes"`,
		`spaces.rap.custom_fields.Team.colour isn't a field of spaces.rap.custom_fields (it has: field, card)`,
		`statuses is text by text: Done is 3`,
	}
	if !slices.Equal(said, want) {
		t.Errorf("problems:\n%s\nwant:\n%s", strings.Join(said, "\n"), strings.Join(want, "\n"))
	}
	// What's right is given: the row with a typo, less the typo, and
	// the row within a row, less what's wrong in it. A row switched off
	// needs nothing, and nothing's said of it.
	if want := `{"queries":[{"name":"bugs","values":{"query":"project = RAM"}}],"spaces":[{"name":"rap","values":{"custom_fields":[{"name":"Team","values":{}}]}}]}`; got != want {
		t.Errorf("sent:\n%s\nwant:\n%s", got, want)
	}
}

// A key the plugin doesn't declare is passed as the files have it: a
// plugin from before it declared its tables reads them as it did.
func TestWhatIsntDeclaredIsPassedAsItIs(t *testing.T) {
	root := files(t, teamBoard, "")
	shaped, problems, err := plugin.Shaped(root, "jira", []plugin.ConfigKey{{Key: "site", Type: protocol.TypeURL}})
	if err != nil || len(problems) != 0 {
		t.Fatalf("%v %v", err, problems)
	}
	data, _ := json.Marshal(plugin.Wire(shaped))
	if got := string(data); !strings.Contains(got, `"queries":{"bugs":{"label":"Bugs","limit":20,"query":"project = RAM, type = Bug"},"initiatives":{`) {
		t.Errorf("an undeclared table: %s", got)
	}
}

// A path is typed a part at a time, quoted where a part has a dot or a
// space, and leads to a key, a row, a row's field or an entry of a map.
func TestPathsLeadToFieldsOfRows(t *testing.T) {
	for typed, want := range map[string][]string{
		"site":             {"site"},
		"spaces.ram.label": {"spaces", "ram", "label"},
		`spaces.ram.custom_fields."Story Points".card`: {"spaces", "ram", "custom_fields", "Story Points", "card"},
		`spaces.ram.columns."v1.2 done"`:               {"spaces", "ram", "columns", "v1.2 done"},
	} {
		got, err := plugin.SplitPath(typed)
		if err != nil || !slices.Equal(got, want) {
			t.Errorf("SplitPath(%s) = %q, %v", typed, got, err)
		}
		if back := plugin.JoinPath(got); back != typed {
			t.Errorf("JoinPath(%q) = %s, want %s", got, back, typed)
		}
	}
	for _, bad := range []string{"", "spaces..label", `spaces."ram`, "spaces."} {
		if got, err := plugin.SplitPath(bad); err == nil {
			t.Errorf("SplitPath(%q) = %q, and no error", bad, got)
		}
	}

	find := func(typed string) (plugin.Place, error) {
		path, err := plugin.SplitPath(typed)
		if err != nil {
			t.Fatal(err)
		}
		return plugin.Find(boardKeys, path)
	}
	if at, err := find("site"); err != nil || at.Key.Type != protocol.TypeURL || at.Row || at.Entry {
		t.Errorf("a key: %+v, %v", at, err)
	}
	if at, err := find("queries.bugs.limit"); err != nil || at.Key.Type != protocol.TypeNumber {
		t.Errorf("a row's field: %+v, %v", at, err)
	}
	if at, err := find("queries.bugs"); err != nil || !at.Row || at.Table.Key != "queries" {
		t.Errorf("a row: %+v, %v", at, err)
	}
	if at, err := find(`spaces.ram.custom_fields."Story Points".card`); err != nil || at.Key.Type != protocol.TypeBool {
		t.Errorf("a field of a row of a row: %+v, %v", at, err)
	}
	if at, err := find("spaces.ram.custom_fields.Team"); err != nil || !at.Row || at.Table.Key != "custom_fields" {
		t.Errorf("a row of a row: %+v, %v", at, err)
	}
	if at, err := find(`spaces.ram.columns."In Review"`); err != nil || !at.Entry || at.Key.Type != protocol.TypeMap {
		t.Errorf("an entry of a row's map: %+v, %v", at, err)
	}
	if at, err := find("statuses.Done"); err != nil || !at.Entry {
		t.Errorf("an entry of a map: %+v, %v", at, err)
	}
	if at, err := find("queries.bugs.enabled"); err != nil || at.Key.Type != protocol.TypeBool {
		t.Errorf("a row's enabled: %+v, %v", at, err)
	}
	for typed, want := range map[string]string{
		"sight":                     "sight isn't in the plugin's config (it has: site, queries, spaces, statuses)",
		"queries":                   "queries is a table: name a row and a field of it, as queries.<query>.<field>",
		"queries.bugs.lable":        "queries.bugs.lable isn't in queries.bugs (it has: label, query, limit, kind, tags)",
		"queries.bugs.label.more":   "queries.bugs.label is text, and has nothing in it to name",
		"site.more":                 "site is text, and has nothing in it to name",
		"spaces.ram.columns":        "",
		"spaces.ram.columns.a.b":    "spaces.ram.columns is text by text, and has nothing in it to name",
		"spaces.ram.custom_fields":  "spaces.ram.custom_fields is a table: name a row and a field of it, as spaces.ram.custom_fields.<field>.<field>",
		"queries.bugs.enabled.more": "queries.bugs.enabled is true or false, and has nothing in it",
	} {
		_, err := find(typed)
		switch {
		case want == "" && err != nil:
			t.Errorf("%s: %v", typed, err)
		case want != "" && (err == nil || err.Error() != want):
			t.Errorf("%s: %v, want %s", typed, err, want)
		}
	}
}

// Setting a row's field writes it in your own file, under the row: the
// team's is as it was, and what's sent has yours over it. Unsetting takes
// it out again, and a row with nothing left in it goes with it.
func TestARowsFieldIsSetInYourOwnFile(t *testing.T) {
	root := files(t, teamBoard, "")
	set := func(typed string, value any) {
		t.Helper()
		path, err := plugin.SplitPath(typed)
		if err != nil {
			t.Fatal(err)
		}
		at, err := plugin.Find(boardKeys, path)
		if err != nil {
			t.Fatal(err)
		}
		if err := plugin.SetAt(root, "jira", at, value); err != nil {
			t.Fatalf("setting %s: %v", typed, err)
		}
	}
	set("queries.bugs.label", "Defects, all of them")
	set("queries.mine.query", "assignee = currentUser(), open")
	set(`spaces.ram.custom_fields."Story Points".card`, true)
	set(`spaces.ram.columns."In Review"`, "Review")
	set("queries.bugs.limit", 5.0)

	team, _ := os.ReadFile(filepath.Join(root, "ws.toml"))
	if string(team) != "[workspace]\norg = \"acme\"\n\n"+teamBoard {
		t.Errorf("the team's file changed:\n%s", team)
	}
	got, problems := sent(t, root)
	if len(problems) != 0 {
		t.Errorf("problems: %v", problems)
	}
	for _, want := range []string{
		`{"name":"bugs","values":{"label":"Defects, all of them","limit":5,"query":"project = RAM, type = Bug"}}`,
		`{"name":"mine","values":{"query":"assignee = currentUser(), open"}}`,
		`"columns":{"In Review":"Review","Ready for Development":"To Do"}`,
		`{"name":"Story Points","values":{"card":true,"field":"customfield_10016"}}`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("sent lacks %s:\n%s", want, got)
		}
	}
	if !plugin.Yours(root, "jira", []string{"queries", "mine"}) || plugin.Yours(root, "jira", []string{"queries", "initiatives"}) || !plugin.Yours(root, "jira", []string{"queries", "bugs", "label"}) {
		t.Error("which rows are yours")
	}

	set("queries.bugs.label", nil)
	set("queries.bugs.limit", nil)
	set("queries.mine", nil)
	got, _ = sent(t, root)
	if !strings.Contains(got, `{"name":"bugs","values":{"label":"Bugs","limit":20,"query":"project = RAM, type = Bug"}}`) || strings.Contains(got, "mine") {
		t.Errorf("after unsetting:\n%s", got)
	}
	local, _ := os.ReadFile(filepath.Join(root, "ws.local.toml"))
	if strings.Contains(string(local), "queries") {
		t.Errorf("a table left empty is still in your file:\n%s", local)
	}
}
