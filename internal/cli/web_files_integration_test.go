package cli_test

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/withworkspace/workspace/internal/cli"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/web"
)

// Colours are as ws's terminal shows them: hex as it is, xterm numbers as
// the colour they draw.
func TestFiles_Colors(t *testing.T) {
	for in, want := range map[string]string{"63": "#5f5fff", "43": "#00d7af", "209": "#ff875f", "#ab12cd": "#ab12cd", "244": "#808080", "1": "#800000", "blue": "", "300": ""} {
		if got := cli.CSSColor(in); got != want {
			t.Errorf("cssColor(%q) = %q, want %q", in, got, want)
		}
	}
}

// The Files page lists the workspace's repos as ws labels them, with
// their capsules; a repo's ground and capsules with their branches; and
// reads only those.
func TestFiles_Repos(t *testing.T) {
	w, _ := orchestrateFixture(t)
	repos, err := cli.WebRepos(w.Root)()
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]ops.Repo{}
	for _, r := range repos {
		byName[r.Name] = r
	}
	be, fe := byName["backend"], byName["frontend"]
	if len(repos) != 2 || be.Label != "backend" || !be.Cloned || len(be.Capsules) != 1 || be.Capsules[0] != "cache-lookup" || len(fe.Capsules) != 0 {
		t.Fatalf("repos = %+v", repos)
	}
	if be.Color == "" || fe.Color == "" || be.Color == fe.Color {
		t.Errorf("each repo has a colour of its own: %q, %q", be.Color, fe.Color)
	}

	wts, err := cli.WebWorktrees(w.Root)("backend")
	if err != nil || len(wts) != 2 || !wts[0].Ground || wts[0].Name != ".ground" || wts[0].Branch != "main" || wts[1].Name != "cache-lookup" {
		t.Fatalf("worktrees = %+v, %v", wts, err)
	}
	if _, err := cli.WebWorktrees(w.Root)("nope"); !errors.Is(err, web.ErrNotFound) {
		t.Errorf("no such repo: %v", err)
	}

	at := cli.WebWorktree(w.Root)
	if dir, base, err := at("backend", ".ground"); err != nil || dir != filepath.Join(w.Root, "repos", "backend", ".ground") || base != "" {
		t.Errorf("the ground: %q %q %v", dir, base, err)
	}
	if dir, base, err := at("backend", "cache-lookup"); err != nil || dir != filepath.Join(w.Root, "repos", "backend", "cache-lookup") || base != "origin/main" {
		t.Errorf("a capsule: %q %q %v", dir, base, err)
	}
	for _, bad := range [][2]string{{"backend", "../frontend"}, {"backend", ".bare"}, {"backend", "nope"}, {"nope", ".ground"}, {"../backend", ".ground"}} {
		if _, _, err := at(bad[0], bad[1]); !errors.Is(err, web.ErrNotFound) {
			t.Errorf("%s/%s: %v", bad[0], bad[1], err)
		}
	}
}
