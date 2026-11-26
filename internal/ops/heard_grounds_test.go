package ops

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/withworkspace/workspace/internal/events"
	"github.com/withworkspace/workspace/internal/space"
	"github.com/withworkspace/workspace/internal/workspace"
)

// What lands on a ground is heard once, by whom: the commits, and what
// they changed in its space (a document, a sketch). A ground heard of for
// the first time is only noted; a commit of yours is by you.
func TestHearGrounds(t *testing.T) {
	w, run := spaceFixture(t)
	ground := filepath.Join(w.Root(), "repos", "records", workspace.GroundDir)
	run(workspace.GroundDir, "config", "user.email", "me@example.com")
	commitAs := func(name, email, msg string, files map[string]string) {
		t.Helper()
		for p, text := range files {
			os.MkdirAll(filepath.Dir(filepath.Join(ground, p)), 0o755)
			os.WriteFile(filepath.Join(ground, p), []byte(text), 0o644)
		}
		for _, args := range [][]string{{"add", "-A"}, {"-c", "commit.gpgsign=false", "commit", "-q", "-m", msg}} {
			cmd := exec.Command("git", append([]string{"-C", ground}, args...)...)
			cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME="+name, "GIT_AUTHOR_EMAIL="+email, "GIT_COMMITTER_NAME="+name, "GIT_COMMITTER_EMAIL="+email)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("git %v: %v\n%s", args, err, out)
			}
		}
	}
	ctx := context.Background()

	if got, err := w.HearGrounds(ctx); err != nil || len(got) != 0 {
		t.Fatalf("first look: %v, %v", got, err)
	}

	manifest, _ := space.Manifest("sketch", "Release flow", "flow.excalidraw", "")
	commitAs("Brendan Ash", "brendan@example.com", "Draw the release flow", map[string]string{
		"sketches/flow/thing.toml":      string(manifest),
		"sketches/flow/flow.excalidraw": "{}",
	})
	commitAs("Brendan Ash", "brendan@example.com", "Say what the cache is, and decide the queue", map[string]string{
		"adr/0001-cache.md": "# Cache tokens per tenant\n\nFor an hour.\n",
		"adr/0002-queue.md": "# One queue\n",
	})
	got, err := w.HearGrounds(ctx)
	if err != nil {
		t.Fatal(err)
	}
	by := map[string][]events.Event{}
	for _, e := range got {
		if !e.IsHeard() || e.N == 0 {
			t.Errorf("not kept as heard: %+v", e)
		}
		by[e.Kind] = append(by[e.Kind], e)
	}
	landed := by[events.CommitsLanded]
	if len(landed) != 1 || len(landed[0].Commits) != 2 || landed[0].Repo != "records" || landed[0].Branch != "main" {
		t.Fatalf("commits.landed: %+v", landed)
	}
	if key, ok := events.PersonOf(landed[0].By); !ok || !strings.Contains(key, "brendan") {
		t.Errorf("landed by %q, not Brendan", landed[0].By)
	}
	if c := landed[0].Commits[0]; c.Subject != "Draw the release flow" || c.By != landed[0].By {
		t.Errorf("its first commit: %+v", c)
	}
	if things := by[events.ThingAdded]; len(things) != 1 || things[0].ThingKind != "sketch" || things[0].Title != "Release flow" ||
		things[0].Thing != "records/sketches/flow/" || things[0].By != landed[0].By {
		t.Errorf("thing.added: %+v", things)
	}
	docs := map[string]string{}
	for _, e := range by[events.DocChanged] {
		docs[e.Doc] = e.Status
	}
	if len(docs) != 2 || docs["records:adr/0001-cache.md"] != "changed" || docs["records:adr/0002-queue.md"] != "added" {
		t.Errorf("doc.changed: %v", docs)
	}

	if again, _ := w.HearGrounds(ctx); len(again) != 0 {
		t.Errorf("heard twice: %+v", again)
	}

	commitAs("Me", "me@example.com", "Change the sketch", map[string]string{"sketches/flow/flow.excalidraw": `{"elements":[]}`})
	got, _ = w.HearGrounds(ctx)
	kinds := map[string]events.Event{}
	for _, e := range got {
		kinds[e.Kind] = e
	}
	if e := kinds[events.CommitsLanded]; e.By != events.ByYou {
		t.Errorf("your commit landed by %q", e.By)
	}
	if e, ok := kinds[events.ThingChanged]; !ok || e.By != events.ByYou {
		t.Errorf("thing.changed: %+v", kinds)
	}
	if list, _ := events.List(w.Root(), events.Filter{Origin: events.Heard, Only: true}); len(list) != len(by[events.CommitsLanded])+len(by[events.ThingAdded])+len(by[events.DocChanged])+2 {
		t.Errorf("kept: %d", len(list))
	}
}

// In the daemon, what lands on a ground is heard as its head moves, not
// at the next follow of the grounds.
func TestHearGrounds_AsTheGroundMoves(t *testing.T) {
	w, _ := spaceFixture(t)
	ground := filepath.Join(w.Root(), "repos", "records", workspace.GroundDir)
	if _, err := w.HearGrounds(context.Background()); err != nil { // where it is, noted
		t.Fatal(err)
	}
	stop, done := make(chan struct{}), make(chan struct{})
	t.Cleanup(func() { close(stop); <-done })
	go func() {
		defer close(done)
		w.WatchGit(stop, t.Logf)
	}()
	waitFor(t, "the ground kept", func() bool { _, ok := w.caps.factsOf(ground); return ok })

	os.WriteFile(filepath.Join(ground, "adr", "0002-queue.md"), []byte("# One queue\n"), 0o644)
	for _, args := range [][]string{{"add", "-A"}, {"-c", "user.name=Ana", "-c", "user.email=ana@example.com", "-c", "commit.gpgsign=false", "commit", "-q", "-m", "Decide one queue"}} {
		if out, err := exec.Command("git", append([]string{"-C", ground}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	waitFor(t, "the commit heard", func() bool {
		got, _ := events.List(w.Root(), events.Filter{Kinds: []string{events.CommitsLanded}})
		return len(got) == 1 && got[0].Text == "Decide one queue"
	})
}
