package ops

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/withworkspace/workspace/internal/config"
	"github.com/withworkspace/workspace/internal/git"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/store/storetest"
	"github.com/withworkspace/workspace/internal/workspace"
)

// spaceFixture is a workspace whose records repo is a space, as ws makes
// one: a bare clone, its ground on main with one document, and a capsule.
// It gives the workspace, and a way to run git in a folder of the repo.
func spaceFixture(t *testing.T) (*Workspace, func(dir string, args ...string)) {
	t.Helper()
	root := t.TempDir()
	storetest.Own(t, root)
	repo := filepath.Join(root, "repos", "records")
	run := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", filepath.Join(repo, dir), "-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false"}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
		}
	}
	os.MkdirAll(filepath.Join(repo, "seed", "adr"), 0o755)
	os.WriteFile(filepath.Join(repo, "seed", "adr", "0001-cache.md"), []byte("# Cache tokens per tenant\n"), 0o644)
	run("", "init", "-q", "--bare", "--initial-branch=main", "origin.git")
	run("seed", "init", "-q", "--initial-branch=main")
	run("seed", "add", ".")
	run("seed", "commit", "-q", "-m", "first")
	run("seed", "push", "-q", filepath.Join(repo, "origin.git"), "main")
	run("", "clone", "-q", "--bare", filepath.Join(repo, "origin.git"), ".bare")
	run(".bare", "config", "remote.origin.fetch", "+refs/heads/*:refs/remotes/origin/*")
	run(".bare", "fetch", "-q", "origin")
	run(".bare", "worktree", "add", "-q", filepath.Join(repo, workspace.GroundDir), "main")
	run(".bare", "worktree", "add", "-q", "-b", "note", filepath.Join(repo, "note"), "origin/main")
	// Not capsules: what made the repo.
	os.RemoveAll(filepath.Join(repo, "seed"))
	os.RemoveAll(filepath.Join(repo, "origin.git"))
	ws := &workspace.Workspace{Root: root, RepoNames: []string{"records"}, Records: "records", DefaultBranch: "main"}
	return New(&config.Config{}, ws, nil, Options{}), run
}

// holdLooks has the looks behind reads wait to be run (the returned
// func runs those asked for, and says how many there were).
func holdLooks(t *testing.T) func() int {
	var held []func()
	was := behind
	behind = func(look func()) { held = append(held, look) }
	t.Cleanup(func() { behind = was })
	return func() int {
		n := len(held)
		for _, look := range held {
			look()
		}
		held = nil
		return n
	}
}

// The pages' reads of a space answer from what's kept, starting no
// process, and ask for a look behind them; the look reads the ground into
// the records and the space's activity, and says it's found something.
func TestKeptSpaces_ReadsStartNoProcessAndALookSaysWhatsNew(t *testing.T) {
	w, run := spaceFixture(t)
	looks := holdLooks(t)
	ctx := context.Background()
	stamp := w.Stamp()

	before := git.Started()
	list, err := w.SpaceList(ctx)
	if err != nil || len(list) != 1 || list[0].Count != 0 {
		t.Fatalf("first list: %+v, %v", list, err)
	}
	a, err := w.SpaceActivity(ctx, "records")
	if err != nil || !a.Unread {
		t.Fatalf("first activity: %+v, %v", a, err)
	}
	if _, _, err := w.DocCitations(ctx, "records", "adr/0001-cache.md"); err != nil {
		t.Fatal(err)
	}
	if n := git.Started() - before; n != 0 {
		t.Errorf("the first reads started git %d times, want none", n)
	}
	if w.SpacesSeen() != "" {
		t.Errorf("seen before any look: %q", w.SpacesSeen())
	}
	if n := looks(); n != 1 {
		t.Errorf("%d looks asked for, want one for all three reads", n)
	}
	seen := w.SpacesSeen()
	if seen == "" || w.Stamp() == stamp {
		t.Fatalf("the look that read the space didn't move spacesSeen (%q) or the stamp", seen)
	}
	select {
	case <-w.Seen():
	default:
		t.Error("the look that read the space didn't say so on Seen")
	}

	before = git.Started()
	list, _ = w.SpaceList(ctx)
	a, _ = w.SpaceActivity(ctx, "records")
	if len(list) != 1 || list[0].Count != 1 || a.Unread || len(a.Lately) != 1 || a.Lately[0].Docs[0].Title != "Cache tokens per tenant" {
		t.Fatalf("once read: %+v, %+v", list, a)
	}
	if n := git.Started() - before; n != 0 {
		t.Errorf("reads of what's kept started git %d times, want none", n)
	}
	if n := looks(); n != 0 {
		t.Errorf("%d looks asked for of a space that hasn't moved", n)
	}

	// The ground moves (a pull, say): the read's given what was, and the
	// look behind it reads the new document in.
	os.MkdirAll(filepath.Join(w.WS().MainWorktree("records"), "notes"), 0o755)
	os.WriteFile(filepath.Join(w.WS().MainWorktree("records"), "notes", "new.md"), []byte("# A new note\n"), 0o644)
	run(workspace.GroundDir, "add", ".")
	run(workspace.GroundDir, "commit", "-q", "-m", "a note")
	before = git.Started()
	if list, _ = w.SpaceList(ctx); list[0].Count != 1 {
		t.Fatalf("read after the ground moved: %+v, want as it was", list)
	}
	if n := git.Started() - before; n != 0 {
		t.Errorf("the read after the ground moved started git %d times, want none", n)
	}
	if n := looks(); n != 1 {
		t.Fatalf("%d looks asked for after the ground moved, want 1", n)
	}
	if w.SpacesSeen() == seen {
		t.Fatal("the look that read the new document didn't move spacesSeen")
	}
	seen = w.SpacesSeen()
	if list, _ = w.SpaceList(ctx); list[0].Count != 2 {
		t.Errorf("after the look: %+v", list)
	}

	// Its activity's moved too; read again behind a read, it's found once.
	if a, _ = w.SpaceActivity(ctx, "records"); len(a.Lately) != 1 {
		t.Fatalf("activity after the ground moved: %+v, want as it was", a.Lately)
	}
	looks()
	if a, _ = w.SpaceActivity(ctx, "records"); len(a.Lately) != 2 || a.Lately[0].Docs[0].Title != "A new note" {
		t.Errorf("activity after the look: %+v", a.Lately)
	}
	if w.SpacesSeen() == seen {
		t.Error("the look that found the new commit didn't move spacesSeen")
	}

	// A look that finds nothing new says nothing.
	seen = w.SpacesSeen()
	floor := spaceFloor
	spaceFloor = 0
	t.Cleanup(func() { spaceFloor = floor })
	w.SpaceActivity(ctx, "records")
	looks()
	if w.SpacesSeen() != seen {
		t.Errorf("a look that found nothing new moved spacesSeen: %q, was %q", w.SpacesSeen(), seen)
	}
}

// A document a mission records opens as that mission's, and what git says
// of the mission's capsules is what's kept: opened again, nothing's asked
// of git.
func TestDoc_RecordedByAsksNothingOfAMissionThatHasntMoved(t *testing.T) {
	w, _ := spaceFixture(t)
	m, err := w.Missions.Create("Write a note")
	if err != nil {
		t.Fatal(err)
	}
	w.Missions.Update(m.Slug, func(m *mission.Mission) error {
		m.Capsules = append(m.Capsules, mission.Capsule{Repo: "records", Name: "note", Branch: "note"})
		return nil
	})
	note := filepath.Join(w.WS().RepoDir("records"), "note", "notes", "x.md")
	os.MkdirAll(filepath.Dir(note), 0o755)
	os.WriteFile(note, []byte("# X\n"), 0o644)
	if err := w.Missions.AddArtifact(m.Slug, mission.Artifact{Ref: "records/note:notes/x.md"}); err != nil {
		t.Fatal(err)
	}
	d, err := w.Doc("records/note:notes/x.md")
	if err != nil || d.Mission == nil || d.Mission.Slug != m.Slug {
		t.Fatalf("first open: %+v, %v", d, err)
	}
	before := git.Started()
	if d, err = w.Doc("records/note:notes/x.md"); err != nil || d.Mission == nil {
		t.Fatalf("opened again: %+v, %v", d, err)
	}
	if n := git.Started() - before; n != 0 {
		t.Errorf("opening it again started git %d times, want none", n)
	}
}

// The artifacts found in the repos are listed from what's kept too: the
// read starts no process and asks for a look, which reads the spaces.
func TestKeptSpaces_FoundArtifactsStartNoProcess(t *testing.T) {
	w, _ := spaceFixture(t)
	looks := holdLooks(t)
	before := git.Started()
	if found, err := w.FoundArtifacts(context.Background()); err != nil || len(found) != 0 {
		t.Fatalf("first read: %+v, %v", found, err)
	}
	if n := git.Started() - before; n != 0 {
		t.Errorf("the first read started git %d times, want none", n)
	}
	if n := looks(); n != 1 {
		t.Fatalf("%d looks asked for, want 1", n)
	}
	before = git.Started()
	if _, err := w.FoundArtifacts(context.Background()); err != nil {
		t.Fatal(err)
	}
	if n := git.Started() - before; n != 0 {
		t.Errorf("a read of spaces that haven't moved started git %d times, want none", n)
	}
	if n := looks(); n != 0 {
		t.Errorf("%d looks asked for of spaces that haven't moved", n)
	}
}
