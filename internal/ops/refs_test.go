package ops

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/doc"
	"github.com/withworkspace/workspace/internal/ref"
)

// groundRepo makes the fixture's frontend a repo with a default branch
// holding plan.md, and says its commit.
func groundRepo(t *testing.T, w *Workspace) string {
	t.Helper()
	ground := w.WS().MainWorktree("frontend")
	if err := os.MkdirAll(ground, 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(ground, "plan.md"), []byte("# Plan\n\nCache per user.\n"), 0o644)
	for _, args := range [][]string{{"init", "-q", "-b", "main"}, {"add", "."}, {"-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "-m", "plan"}} {
		if out, err := exec.Command("git", append([]string{"-C", ground}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	head, _ := exec.Command("git", "-C", ground, "rev-parse", "HEAD").Output()
	return string(head[:len(head)-1])
}

// What's written into a document has its names linked: a file's lines at
// the commit they were read at, a mission by ws:.
func TestLinkNames(t *testing.T) {
	w, _ := fixture(t, "")
	head := groundRepo(t, w)
	w.WS().Org = "acme"
	got := w.LinkNames("Per user (frontend/plan.md:3), for cache.\n")
	want := "Per user ([frontend/plan.md:3](https://github.com/acme/frontend/blob/" + head + "/plan.md#L3)), for [cache](ws:cache).\n"
	if got != want {
		t.Errorf("LinkNames:\n got  %q\n want %q", got, want)
	}
}

// What's said in a thread keeps what its names were when it was said: a
// file's lines with the commit they were of, a mission by its slug.
func TestTurnsKeepWhatTheyNamed(t *testing.T) {
	w, _ := fixture(t, "")
	head := groundRepo(t, w)

	sc := doc.OfMission("cache")
	text := []byte("# Plan\n\nCache per user.\n")
	c, err := w.OpenThread(sc, "frontend:plan.md", text, conversation.DocThread{Version: doc.Version(text), Line: 3, Quote: conversation.Quote{Exact: "per user"}, Kind: conversation.Ask, Body: "Why?"}, AsYou)
	if err != nil {
		t.Fatal(err)
	}
	c, err = w.Reply(sc, c.ID, text, "It's in frontend/plan.md:3, for cache; frontend/nope.go isn't.", "", AsCrew("c1"))
	if err != nil {
		t.Fatal(err)
	}
	got := c.Turns[len(c.Turns)-1].Refs
	want := []ref.Pinned{
		{Said: "frontend/plan.md:3", Ref: ref.Ref{Kind: ref.File, Repo: "frontend", Worktree: ref.Ground, Path: "plan.md", Line: 3, Commit: head}},
		{Said: "cache", Ref: ref.Ref{Kind: ref.Mission, Slug: "cache"}},
	}
	if len(got) != len(want) {
		t.Fatalf("refs = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("refs[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}
