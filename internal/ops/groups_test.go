package ops

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/withworkspace/workspace/internal/store/storetest"
)

// groupWorld is a workspace with the team's api and web in platform,
// which the team hides, docs in no group, and notes, yours, in ws.local.toml
// with nothing but its table.
func groupWorld(t *testing.T) (*Workspace, string) {
	t.Helper()
	root := t.TempDir()
	storetest.Own(t, root)
	os.WriteFile(filepath.Join(root, "ws.toml"), []byte(`[workspace]
org = "acme"
default_branch = "main"

[repos.api]
group = "platform"

[repos.web]
group = "platform"

[repos.docs]

[groups.platform]
hidden = true
`), 0o644)
	os.WriteFile(filepath.Join(root, "ws.local.toml"), []byte("[repos.notes]\n"), 0o644)
	return Open(root, Options{}), root
}

func groupNamed(gs []Group, name string) (Group, bool) {
	for _, g := range gs {
		if g.Name == name {
			return g, true
		}
	}
	return Group{}, false
}

// A group is hidden as the team says until you say otherwise, and then as
// you say, with the team's beside it; taken back, it's the team's again.
// Shown, its repos that aren't cloned are.
func TestShowGroup_YoursOverTheTeams(t *testing.T) {
	w, root := groupWorld(t)
	g, _ := groupNamed(w.Groups(), "platform")
	if !g.Hidden || !g.TeamHidden || g.Chosen || strings.Join(g.Repos, ",") != "api,web" {
		t.Fatalf("platform: %+v", g)
	}
	source(t, "acme/api", "acme/web")

	shown := false
	done, err := w.ShowGroup(context.Background(), AsYou, "platform", &shown, true)
	if err != nil {
		t.Fatal(err)
	}
	if done.Hidden || !done.Chosen {
		t.Errorf("shown: %+v", done)
	}
	for _, repo := range []string{"api", "web"} {
		if _, err := os.Stat(filepath.Join(root, "repos", repo, ".ground")); err != nil {
			t.Errorf("%s isn't cloned: %v", repo, err)
		}
	}
	if g, _ := groupNamed(w.Groups(), "platform"); g.Hidden || !g.TeamHidden || !g.Chosen {
		t.Errorf("after: %+v", g)
	}

	if _, err := w.ShowGroup(context.Background(), AsYou, "platform", nil, true); err != nil {
		t.Fatal(err)
	}
	if g, _ := groupNamed(w.Groups(), "platform"); !g.Hidden || g.Chosen {
		t.Errorf("reset: %+v", g)
	}
	if _, err := w.ShowGroup(context.Background(), AsYou, "nowhere", &shown, false); err == nil {
		t.Error("there's no group nowhere")
	}
	if _, err := w.ShowGroup(context.Background(), AsCrew("c1"), "platform", &shown, false); err == nil {
		t.Error("crew shouldn't show groups")
	}
}

// A repo of the team's can be put in another group for you, and back in
// the team's, which takes yours away; not in none, which ws.local.toml
// can't say over ws.toml's group. A repo of yours can be in any, or none,
// and stays yours.
func TestGroupRepo_ForYou(t *testing.T) {
	w, root := groupWorld(t)
	ctx := context.Background()
	local := func() string {
		b, _ := os.ReadFile(filepath.Join(root, "ws.local.toml"))
		return string(b)
	}

	if _, err := w.GroupRepo(ctx, AsYou, "api", "mine"); err != nil {
		t.Fatal(err)
	}
	if ws := w.WS(); ws.GroupOf("api") != "mine" || ws.TeamGroups["api"] != "platform" {
		t.Errorf("api: in %s over %q", ws.GroupOf("api"), ws.TeamGroups["api"])
	}
	if g, ok := groupNamed(w.Groups(), "mine"); !ok || strings.Join(g.Repos, ",") != "api" {
		t.Errorf("mine: %+v", g)
	}
	if _, err := w.GroupRepo(ctx, AsYou, "api", ""); err == nil || !strings.Contains(err.Error(), "not in none") {
		t.Errorf("api in none: %v", err)
	}
	done, err := w.GroupRepo(ctx, AsYou, "api", "platform")
	if err != nil || !done.Team {
		t.Fatalf("back in platform: %+v, %v", done, err)
	}
	if strings.Contains(local(), "api") {
		t.Errorf("ws.local.toml still says api:\n%s", local())
	}

	if _, err := w.GroupRepo(ctx, AsYou, "notes", "mine"); err != nil {
		t.Fatal(err)
	}
	if _, err := w.GroupRepo(ctx, AsYou, "notes", ""); err != nil {
		t.Fatal(err)
	}
	if ws := w.WS(); !ws.Yours["notes"] || ws.GroupOf("notes") != "" {
		t.Errorf("notes: yours %v, in %q\n%s", ws.Yours["notes"], ws.GroupOf("notes"), local())
	}
	if _, err := w.GroupRepo(ctx, AsYou, "docs", "a group"); err == nil {
		t.Error("a group's name has no spaces")
	}
}

// A group's display_name and order, in ws.toml, are what it's called and
// where it comes: Groups are in their order, then by name, and a repo
// carries its group's. Shown or hidden for you, a group keeps them.
func TestGroups_DisplayNameAndOrder(t *testing.T) {
	root := t.TempDir()
	storetest.Own(t, root)
	os.WriteFile(filepath.Join(root, "ws.toml"), []byte(`[workspace]
org = "acme"
default_branch = "main"

[repos.api]
group = "platform"

[repos.site]
group = "web"

[repos.tools]
group = "aaa"

[groups.platform]
display_name = "Platform team"
order = 2

[groups.web]
display_name = "Web"
order = 1
`), 0o644)
	w := Open(root, Options{})
	var names []string
	for _, g := range w.Groups() {
		names = append(names, g.Name)
	}
	if got := strings.Join(names, ","); got != "aaa,web,platform" {
		t.Fatalf("groups in order: %s", got)
	}
	g, _ := groupNamed(w.Groups(), "platform")
	if g.Label != "Platform team" || g.Order != 2 {
		t.Fatalf("platform: %+v", g)
	}
	if g, _ := groupNamed(w.Groups(), "aaa"); g.Label != "aaa" {
		t.Fatalf("aaa's label is its name: %+v", g)
	}

	hidden := true
	if _, err := w.ShowGroup(context.Background(), AsYou, "platform", &hidden, false); err != nil {
		t.Fatal(err)
	}
	w = Open(root, Options{})
	if g, _ := groupNamed(w.Groups(), "platform"); !g.Hidden || g.Label != "Platform team" || g.Order != 2 {
		t.Fatalf("platform hidden for you: %+v", g)
	}
}
