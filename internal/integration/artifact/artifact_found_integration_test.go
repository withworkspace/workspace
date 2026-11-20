package artifact_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/testutil"
)

// A promoted artifact's folder in a repo says it's an artifact, so every
// ws with the repo finds it there: the one that promoted it has it as its
// own (a1), and anyone else's is listed, found and read from the repo's
// files, by its folder's name. Its version is its folder's: a change to a
// file beside its page is a change to it.
func TestArtifactInARepo_IsFoundByEveryoneWithIt(t *testing.T) {
	w := homeWorkspace(t, "")
	app := testutil.OpsAt(w.Root, time.Now())
	ctx := context.Background()
	ground := testutil.CapsuleDir(w, "records", ".ground")

	// Promoted here: its folder has a thing.toml, and it's this machine's
	// own, not one found.
	addAt(t, w, time.Now(), "Horizon glow", glowV1, "Accent at 8% in dark.")
	testutil.Run(t, w, nil, "artifact", "promote", "a1")
	manifests, _ := filepath.Glob(filepath.Join(ground, "artifacts", "*-horizon-glow", "thing.toml"))
	if len(manifests) != 1 {
		t.Fatalf("the promoted artifact's thing.toml: %v", manifests)
	}
	said, _ := os.ReadFile(manifests[0])
	for _, want := range []string{`kind = "artifact"`, `title = "Horizon glow"`, `main = "index.html"`} {
		if !strings.Contains(string(said), want) {
			t.Errorf("its thing.toml doesn't say %s:\n%s", want, said)
		}
	}
	if found, err := app.FoundArtifacts(ctx); err != nil || len(found) != 0 {
		t.Fatalf("found, with only this machine's own promoted: %+v, %v", found, err)
	}

	// Someone else's, as their ws promoted it: on the default branch once
	// the repo's pulled.
	const folder = "artifacts/2026-11-01-team-board"
	const name = "records/" + folder + "/"
	put := func(file, text string) {
		t.Helper()
		full := filepath.Join(ground, folder, file)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	put("thing.toml", "kind = \"artifact\"\ntitle = \"Team board\"\nmain = \"index.html\"\n")
	put("artifact.toml", `title = "Team board"
status = "promoted"
made = 2026-11-01T10:00:00Z

[promoted]
at = 2026-11-02T09:00:00Z
by = "Sam"
version = 2
into = ""

[[versions]]
n = 1
at = 2026-11-01T10:00:00Z
by = "Sam"
note = "A board in columns."

[[versions]]
n = 2
at = 2026-11-02T08:00:00Z
by = "Sam"
note = "Swimlanes for each person."
`)
	put("index.html", "<!doctype html>\n<title>Team board</title>\n<link rel=stylesheet href=css/board.css>\n")
	testutil.CommitIn(t, ground, folder+"/css/board.css", "body { color: teal }", "Promote the team board")

	found, err := app.FoundArtifacts(ctx)
	if err != nil || len(found) != 1 {
		t.Fatalf("found: %+v, %v", found, err)
	}
	board := found[0]
	if board.Name != name || board.Where() != "records:"+folder+"/" || board.Title != "Team board" || board.Version == "" || board.Changed.IsZero() {
		t.Fatalf("the board, as found: %+v", board)
	}
	if board.Record == nil || board.Record.Promoted == nil || board.Record.Promoted.By != "Sam" || board.Record.Promoted.Version != 2 || len(board.Record.Versions) != 2 {
		t.Fatalf("what its record says: %+v", board.Record)
	}
	detail, err := app.FoundArtifact(ctx, "records:"+folder+"/")
	if err != nil || len(detail.Files) != 4 || detail.Size == 0 {
		t.Fatalf("its files: %+v, %v", detail.Files, err)
	}
	if css, blob, err := app.FoundArtifactAt(ctx, "records", board.Version, "css/board.css"); err != nil || string(css) != "body { color: teal }" || blob == "" {
		t.Fatalf("its stylesheet: %q, %v", css, err)
	}
	if _, _, err := app.FoundArtifactAt(ctx, "records", board.Version, "secret.txt"); err == nil {
		t.Error("a file it hasn't was read")
	}
	// What ws serves is what it found: a version that's no artifact's isn't read.
	if _, _, err := app.FoundArtifactAt(ctx, "records", strings.Repeat("0", 40), "index.html"); !errors.Is(err, ops.ErrNotFound) {
		t.Errorf("a version that's no artifact's: %v", err)
	}
	// Found by what a version of it changed, and by its title, by its folder's name.
	for _, words := range []string{"swimlanes", "team board"} {
		res := testutil.RunCommand(t, w.Root, nil, "find", words)
		if res.Err != nil || !strings.Contains(res.Stdout, name) {
			t.Errorf("find %s: %v\n%s", words, res.Err, res.Stdout)
		}
	}

	// The command line has it too, by where it lives: listed after this
	// machine's own, and shown with what its record says and its files.
	if res := testutil.RunCommand(t, w.Root, nil, "artifact", "list"); res.Err != nil || !strings.Contains(res.Stdout, "records:"+folder+"/ v2") || !strings.Contains(res.Stdout, "promoted by Sam") || !strings.Contains(res.Stdout, "a1 ") {
		t.Errorf("artifact list: %v\n%s", res.Err, res.Stdout)
	}
	shown := testutil.RunCommand(t, w.Root, nil, "show", "records:"+folder+"/")
	for _, want := range []string{`Artifact at records:` + folder + `/ — "Team board"`, "v2 was chosen", "Swimlanes for each person.", "css/board.css", filepath.Join(".ground", folder)} {
		if shown.Err != nil || !strings.Contains(shown.Stdout, want) {
			t.Errorf("show doesn't say %q: %v\n%s", want, shown.Err, shown.Stdout)
		}
	}

	// Its stylesheet alone changes: it's another version, and the one
	// before isn't served any more.
	testutil.CommitIn(t, ground, folder+"/css/board.css", "body { color: crimson }", "Restyle the board")
	found, err = app.FoundArtifacts(ctx)
	if err != nil || len(found) != 1 || found[0].Version == board.Version {
		t.Fatalf("found, restyled: %+v, %v (was at %s)", found, err, board.Version)
	}
	if css, _, err := app.FoundArtifactAt(ctx, "records", found[0].Version, "css/board.css"); err != nil || string(css) != "body { color: crimson }" {
		t.Fatalf("its stylesheet, restyled: %q, %v", css, err)
	}
	if _, _, err := app.FoundArtifactAt(ctx, "records", board.Version, "css/board.css"); !errors.Is(err, ops.ErrNotFound) {
		t.Errorf("the version before, once it's moved on: %v", err)
	}

	// Taken out of the repo, it's found no more.
	testutil.GitCmd(t, ground, "rm", "-r", "-q", folder)
	testutil.GitCmd(t, ground, "-c", "user.name=Test", "-c", "user.email=test@test.com", "commit", "-q", "-m", "Discard the board")
	if found, err := app.FoundArtifacts(ctx); err != nil || len(found) != 0 {
		t.Fatalf("found, once it's gone: %+v, %v", found, err)
	}
	if res := testutil.RunCommand(t, w.Root, nil, "find", "swimlanes"); strings.Contains(res.Stdout, name) {
		t.Errorf("found by its words once it's gone:\n%s", res.Stdout)
	}
}
