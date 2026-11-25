package ops

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/config"
	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/doc"
	"github.com/withworkspace/workspace/internal/git"
	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/store/storetest"
	"github.com/withworkspace/workspace/internal/workspace"
)

// liftWorld is a workspace with one repo, api, as ws keeps one: a bare
// clone of a remote, its .ground on main, with a file in it.
func liftWorld(t *testing.T) *Workspace {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	storetest.Own(t, root)
	run := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", filepath.Join(root, dir), "-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false"}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
		}
	}
	for _, d := range []string{"origin.git", "seed", "repos/api"} {
		os.MkdirAll(filepath.Join(root, d), 0o755)
	}
	run("origin.git", "init", "-q", "--bare", "--initial-branch=main")
	run("seed", "init", "-q", "--initial-branch=main")
	os.WriteFile(filepath.Join(root, "seed", "main.go"), []byte("package main\n"), 0o644)
	run("seed", "add", ".")
	run("seed", "commit", "-q", "-m", "start")
	run("seed", "remote", "add", "origin", filepath.Join(root, "origin.git"))
	run("seed", "push", "-q", "origin", "main")
	run("repos/api", "clone", "-q", "--bare", filepath.Join(root, "origin.git"), ".bare")
	run("repos/api/.bare", "config", "remote.origin.fetch", "+refs/heads/*:refs/remotes/origin/*")
	run("repos/api/.bare", "config", "user.name", "t")
	run("repos/api/.bare", "config", "user.email", "t@example.com")
	run("repos/api/.bare", "fetch", "-q", "origin")
	run("repos/api/.bare", "worktree", "add", "-q", filepath.Join(root, "repos/api/.ground"), "main")
	ws := &workspace.Workspace{Root: root, RepoNames: []string{"api"}, DefaultBranch: "main", Boarded: map[string][]string{}}
	return New(&config.Config{}, ws, nil, Options{})
}

// sandboxing is a harness that holds conversations with a sandbox for
// their shell.
type sandboxing struct{ fakeConverser }

func (*sandboxing) Sandboxes() bool { return true }

// A conversation's agent lifts a capsule of its conversation's own: it
// joins no mission and isn't boarded, a sandboxed run may write in it and
// in git's folders for it, and it's put away with the conversation, what
// wasn't committed kept on its branch, to come back when it's lifted
// again.
func TestConversationLiftsItsOwnCapsules(t *testing.T) {
	w := liftWorld(t)
	store := conversation.ForWorkspace(w.Root())
	c, err := store.OpenTalk(conversation.You, "How slow are our tests?", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	made, err := w.LiftForConversation("", c.ID, "api", "parallel-tests", "", CapsuleOptions{By: AsYou})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(w.WS().RepoDir("api"), "parallel-tests")
	if made.Path != path || made.Mission != "" || w.WS().IsBoarded("api", "parallel-tests") {
		t.Errorf("made %+v, boarded %v", made, w.WS().IsBoarded("api", "parallel-tests"))
	}
	if _, err := os.Stat(filepath.Join(path, "main.go")); err != nil {
		t.Fatalf("no capsule at %s: %v", path, err)
	}
	c, _ = store.Load(c.ID)
	if len(c.Capsules) != 1 || c.Capsules[0].Ref() != "api/parallel-tests" || c.Capsules[0].Branch != "parallel-tests" || len(c.Lifted()) != 1 {
		t.Fatalf("the conversation's capsules: %+v", c.Capsules)
	}
	if by := store.LiftedBy("api", "parallel-tests"); by == nil || by.ID != c.ID {
		t.Errorf("lifted by %+v", by)
	}
	_, err = w.LiftForConversation("", c.ID, "api", "parallel-tests", "", CapsuleOptions{})
	refusedWith(t, err, "capsule-exists")

	// A sandboxed run writes in a folder of its own and the capsule, and
	// beside them where git keeps the capsule; not the rest of the repo's
	// git folder, nor its ground.
	run := harness.ConverseRun{Dir: w.Root(), Talk: true}
	said := w.sandboxFor(store, c, &sandboxing{}, &run)
	if said == nil || !run.Sandbox || !run.Lift || len(run.Write) != 2 || run.Write[0] != said.Own || run.Write[1] != path {
		t.Fatalf("the run: %+v, told %+v", run, said)
	}
	if info, err := os.Stat(said.Own); err != nil || !info.IsDir() || strings.HasPrefix(said.Own, w.Root()) {
		t.Errorf("its own folder %s: %v", said.Own, err)
	}
	bare := w.WS().BareDir("api")
	for _, want := range []string{filepath.Join(bare, "worktrees", "parallel-tests"), filepath.Join(bare, "objects"), filepath.Join(bare, "refs")} {
		if !slices.Contains(run.Beside, want) {
			t.Errorf("git can't write %s: %v", want, run.Beside)
		}
	}
	for _, never := range []string{bare, filepath.Join(bare, "config"), filepath.Join(bare, "hooks"), filepath.Join(bare, "worktrees"), filepath.Join(bare, "worktrees", ".ground"), w.WS().MainWorktree("api")} {
		if slices.Contains(run.Beside, never) || slices.Contains(run.Write, never) {
			t.Errorf("it may write %s", never)
		}
	}
	// There's room in the repo's folder, and in git's for its capsules,
	// for one it lifts as the run goes on.
	for _, want := range []string{w.WS().RepoDir("api"), filepath.Join(bare, "worktrees")} {
		if !slices.Contains(run.Room, want) {
			t.Errorf("no room in %s: %v", want, run.Room)
		}
	}
	if got := w.ConversationFolders("", c.ID); !slices.Equal(got, run.Write) {
		t.Errorf("its tools write in %v, its shell in %v", got, run.Write)
	}
	if len(said.Capsules) != 1 || said.Capsules[0].Path != path || len(said.Away) != 0 || said.NoLift != "" {
		t.Errorf("what it's told: %+v", said)
	}
	// A harness with no sandbox, or a conversation that doesn't talk, runs
	// as before.
	plain := harness.ConverseRun{Dir: w.Root(), Talk: true}
	if w.sandboxFor(store, c, &fakeConverser{}, &plain) != nil || plain.Sandbox || plain.Lift || plain.Write != nil {
		t.Errorf("sandboxed on a harness with none: %+v", plain)
	}

	// Put away, with something not committed: the capsule goes, its
	// branch holds what was there.
	os.WriteFile(filepath.Join(path, "notes.txt"), []byte("12s serial, 4s with -p 8\n"), 0o644)
	c, err = w.SetTalkStatus(doc.Scope{}, c.ID, conversation.Resolved, AsYou)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err == nil {
		t.Error("the capsule's still there, its conversation put away")
	}
	if len(c.Lifted()) != 0 || len(c.PutAway()) != 1 || !c.PutAway()[0].Kept || c.Status != conversation.Resolved {
		t.Errorf("put away: %+v (%s)", c.Capsules, c.Status)
	}
	if out, err := git.Output(bare, "show", "--stat", "--format=%s", "parallel-tests"); err != nil || !strings.Contains(out, "As it was when "+c.ID+" was put away") || !strings.Contains(out, "notes.txt") {
		t.Errorf("its branch: %s, %v", out, err)
	}
	away := harness.ConverseRun{Dir: w.Root(), Talk: true}
	if said := w.sandboxFor(store, c, &sandboxing{}, &away); len(away.Write) != 1 || len(said.Capsules) != 0 || len(said.Away) != 1 || said.Away[0].Name != "parallel-tests" || len(w.ConversationFolders("", c.ID)) != 1 {
		t.Errorf("put away, the run: %+v, told %+v", away, said)
	}

	// Taken up again, lifting it brings it back as it was left.
	if _, err := store.SetStatus(c.ID, conversation.You, conversation.Open, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := w.LiftForConversation("", c.ID, "api", "parallel-tests", "origin/main", CapsuleOptions{}); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(filepath.Join(path, "notes.txt")); string(data) != "12s serial, 4s with -p 8\n" {
		t.Errorf("back without what was in it: %q", data)
	}
	if c, _ = store.Load(c.ID); len(c.Capsules) != 1 || len(c.Lifted()) != 1 {
		t.Errorf("back: %+v", c.Capsules)
	}
}

// On a mission's Talk tab, a capsule its agent lifts is the
// conversation's, not the mission's, and goes when the mission's archived.
// An ask nobody's kept lifts none.
func TestConversationCapsulesArentTheMissions(t *testing.T) {
	w := liftWorld(t)
	m, err := w.Missions.Create("Cache")
	if err != nil {
		t.Fatal(err)
	}
	store := conversation.For(w.Root(), m.Slug)
	c, err := store.OpenTalk(conversation.You, "Is this change slower?", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.LiftForConversation(m.Slug, c.ID, "api", "bench", "", CapsuleOptions{Mission: m.Slug}); err != nil {
		t.Fatal(err)
	}
	if m, _ = w.Missions.Load(m.Slug); len(m.Capsules) != 0 {
		t.Errorf("the mission has it: %+v", m.Capsules)
	}
	if _, err := w.Archive(m.Slug, false, AsYou); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(w.WS().RepoDir("api"), "bench")); err == nil {
		t.Error("the capsule's still there, its mission archived")
	}
	if c, _ = store.Load(c.ID); len(c.Lifted()) != 0 || len(c.PutAway()) != 1 {
		t.Errorf("after the archive: %+v", c.Capsules)
	}

	ask, err := conversation.ForWorkspace(w.Root()).OpenAsk(conversation.Anchor{Doc: "api:README.md"}, conversation.You, "What's this?", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	_, err = w.LiftForConversation("", ask.ID, "api", "nope", "", CapsuleOptions{})
	refusedWith(t, err, "conversation-holds-none")
	run := harness.ConverseRun{Dir: w.Root(), Talk: true}
	if said := w.sandboxFor(conversation.ForWorkspace(w.Root()), ask, &sandboxing{}, &run); said == nil || said.NoLift == "" || run.Lift || !run.Sandbox {
		t.Errorf("an ask's run: %+v, told %+v", run, said)
	}
}

// A mission a conversation proposes in a capsule that's lifted already (by
// the conversation's agent, here) is given that capsule, joined to it, not
// refused one as there already; it's the mission's then, so putting the
// conversation away leaves it be.
func TestProposedMissionJoinsACapsuleLiftedAlready(t *testing.T) {
	w := liftWorld(t)
	store := conversation.ForWorkspace(w.Root())
	c, err := store.OpenTalk(conversation.You, "How slow are our tests?", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.LiftForConversation("", c.ID, "api", "parallel-tests", "", CapsuleOptions{By: AsYou}); err != nil {
		t.Fatal(err)
	}
	c, err = store.Propose(c.ID, conversation.Proposal{Kind: conversation.ProposeMission, By: conversation.Agent, Title: "Parallel tests", Brief: "## Goal\nRun the tests in parallel.\n", Capsules: []string{"api/parallel-tests"}})
	if err != nil {
		t.Fatal(err)
	}
	said, err := w.SettleProposalIn(store, c.ID, len(c.Proposals), true, ProposalEdits{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(said, "Joined api/parallel-tests, lifted already.") || strings.Contains(said, "Couldn't") {
		t.Errorf("accepting it said %q", said)
	}
	m, ok := w.Missions.FindByCapsule("api", "parallel-tests")
	if !ok || m.Title != "Parallel tests" {
		t.Fatalf("the capsule's mission: %+v, %v", m, ok)
	}

	if _, err := w.SetTalkStatus(doc.Scope{}, c.ID, conversation.Resolved, AsYou); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(w.WS().RepoDir("api"), "parallel-tests", "main.go")); err != nil {
		t.Errorf("the mission's capsule went with the conversation: %v", err)
	}
}
