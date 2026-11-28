package ops

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/doc"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/outbox"
)

// In the daemon, archiving a mission answers before its capsules are
// burned: they're burned behind, from the outbox, once the moment to undo
// it has passed, with those its conversations' agents lifted. Reopened in
// that moment, its capsules are kept.
func TestArchiveBehind(t *testing.T) {
	was := putAwayHold
	putAwayHold = 100 * time.Millisecond
	t.Cleanup(func() { putAwayHold = was })
	w := liftWorld(t)
	q := &outbox.Queue{Root: w.Root()}
	w.opts.Outbox = q
	w.PutAwayBehind(q)
	q.Resume()
	t.Cleanup(func() { q.Stop(time.Second) })
	there := func(name string) bool {
		_, err := os.Stat(filepath.Join(w.WS().RepoDir("api"), name))
		return err == nil
	}
	waitFor := func(what string, ok func() bool) {
		t.Helper()
		for range 400 {
			if ok() {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatalf("never: %s", what)
	}

	m, err := w.Missions.Create("Cache")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Lift("api", "cache-work", "", CapsuleOptions{Mission: m.Slug, By: AsYou}); err != nil {
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

	// Its work hasn't merged: refused, as without an outbox, until you
	// agree to burn it.
	_, err = w.Archive(m.Slug, false, AsYou)
	refusedWith(t, err, "unmerged-capsules")
	set, err := w.Archive(m.Slug, true, AsYou)
	if err != nil {
		t.Fatal(err)
	}
	if got := set.String(); got != "Archived; burning api/cache-work in a moment." {
		t.Errorf("said: %q", got)
	}
	if m, _ := w.Missions.Load(m.Slug); m.State != mission.Archived {
		t.Errorf("not archived: %q", m.State)
	}
	if !there("cache-work") || !there("bench") {
		t.Error("burned before the moment to undo it had passed")
	}
	if items := w.PuttingAway(m.Slug); len(items) != 2 || items[0].Doing != "burning api/cache-work" || items[0].State != outbox.Waiting {
		t.Errorf("its page shows: %+v", items)
	}
	waitFor("both burned", func() bool { return !there("cache-work") && !there("bench") && len(w.PuttingAway(m.Slug)) == 0 })
	if c, _ = store.Load(c.ID); len(c.Lifted()) != 0 || len(c.PutAway()) != 1 {
		t.Errorf("the conversation's after the archive: %+v", c.Capsules)
	}

	// Undone in the moment after: kept.
	n, err := w.Missions.Create("Flags")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Lift("api", "flags-work", "", CapsuleOptions{Mission: n.Slug, By: AsYou}); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Archive(n.Slug, true, AsYou); err != nil {
		t.Fatal(err)
	}
	back, err := w.Reopen(n.Slug, mission.Archived, AsYou)
	if err != nil {
		t.Fatal(err)
	}
	if got := back.String(); got != "Reopened; kept api/flags-work." {
		t.Errorf("said: %q", got)
	}
	time.Sleep(3 * putAwayHold)
	q.Wait()
	if !there("flags-work") {
		t.Error("burned, though reopened in time")
	}
	if items := w.PuttingAway(n.Slug); len(items) != 0 {
		t.Errorf("still to put away: %+v", items)
	}
}

// A burn that stopped partway (its .git gone, the folder left) is
// finished by the next, as a retry from the outbox is; a folder git never
// made, with things in it, isn't taken for one.
func TestBurnFinishesAnUnfinishedRemoval(t *testing.T) {
	w := liftWorld(t)
	if _, err := w.Lift("api", "half", "", CapsuleOptions{By: AsYou}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(w.WS().RepoDir("api"), "half")
	if err := os.Remove(filepath.Join(path, ".git")); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Burn("api", "half", BurnOptions{}); err != nil {
		t.Fatalf("finishing it: %v", err)
	}
	if exists(path) {
		t.Error("what was left is still there")
	}

	stray := filepath.Join(w.WS().RepoDir("api"), "stray")
	os.MkdirAll(stray, 0o755)
	os.WriteFile(filepath.Join(stray, "notes.txt"), []byte("mine\n"), 0o644)
	if _, err := w.Burn("api", "stray", BurnOptions{Force: true}); err == nil {
		t.Error("burned a folder git never made")
	}
	if !exists(filepath.Join(stray, "notes.txt")) {
		t.Error("a folder git never made lost what was in it")
	}
}

// In the daemon, resolving a conversation whose agent lifted a capsule
// answers before the capsule's put away; reopened in the moment after,
// it's kept, and resolved, put away behind.
func TestResolveBehind(t *testing.T) {
	was := putAwayHold
	putAwayHold = 100 * time.Millisecond
	t.Cleanup(func() { putAwayHold = was })
	w := liftWorld(t)
	q := &outbox.Queue{Root: w.Root()}
	w.opts.Outbox = q
	w.PutAwayBehind(q)
	q.Resume()
	t.Cleanup(func() { q.Stop(time.Second) })
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
	path := filepath.Join(w.WS().RepoDir("api"), "bench")
	sc := doc.OfMission(m.Slug)

	// Resolved and taken up again in the moment after: kept.
	if c, err = w.SetTalkStatus(sc, c.ID, conversation.Resolved, AsYou); err != nil || c.Status != conversation.Resolved {
		t.Fatalf("resolved: %+v %v", c, err)
	}
	if !exists(path) {
		t.Error("put away before the answer")
	}
	if _, err := w.SetTalkStatus(sc, c.ID, conversation.Open, AsYou); err != nil {
		t.Fatal(err)
	}
	time.Sleep(3 * putAwayHold)
	q.Wait()
	if !exists(path) {
		t.Error("put away, though taken up again in time")
	}

	if _, err := w.SetTalkStatus(sc, c.ID, conversation.Resolved, AsYou); err != nil {
		t.Fatal(err)
	}
	// Burned, then the conversation told: both, before it's done.
	for range 400 {
		if c, _ = store.Load(c.ID); !exists(path) && len(c.PutAway()) == 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if exists(path) {
		t.Fatal("never put away")
	}
	if len(c.Lifted()) != 0 || len(c.PutAway()) != 1 {
		t.Errorf("the conversation's after: %+v", c.Capsules)
	}
}
