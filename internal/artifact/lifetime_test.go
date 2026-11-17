package artifact

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A live artifact expires expire_after after its last version, or after
// it was put back if that's later; promoted, discarded, gone, or with a
// version in flight, it doesn't; and the list warns in its last week.
func TestExpiry(t *testing.T) {
	c := Config{ExpireAfter: 30 * 24 * time.Hour}
	last := day
	e := Entry{Place: Local, Artifact: Artifact{Status: Live, Made: day, Versions: []Version{{N: 1, At: last}}}}
	at, ok := c.Expires(e)
	if !ok || !at.Equal(last.Add(30*24*time.Hour)) {
		t.Fatalf("expires %v %v", at, ok)
	}
	for _, tc := range []struct {
		now               time.Time
		expired, expiring bool
		words             string
	}{
		{last.Add(10 * 24 * time.Hour), false, false, "expires in 20 days"},
		{last.Add(26 * 24 * time.Hour), false, true, "expires in 4 days"},
		{last.Add(30*24*time.Hour - time.Hour), false, true, "expires today"},
		{last.Add(31 * 24 * time.Hour), true, false, "expired"},
	} {
		if got := c.Expired(e, tc.now); got != tc.expired {
			t.Errorf("at %v, expired = %v", tc.now, got)
		}
		if got := c.Expiring(e, tc.now); got != tc.expiring {
			t.Errorf("at %v, expiring = %v", tc.now, got)
		}
		if got := ExpiresWords(at, tc.now); got != tc.words {
			t.Errorf("at %v, %q", tc.now, got)
		}
	}
	// Put back after it expired: it counts from then.
	r := e
	r.Restored = &Restoration{At: last.Add(40 * 24 * time.Hour), By: "you"}
	if c.Expired(r, last.Add(41*24*time.Hour)) {
		t.Error("restored, it's expired again at once")
	}
	for name, e := range map[string]Entry{
		"promoted":  {Place: Local, Artifact: Artifact{Status: Promoted, Versions: []Version{{N: 1, At: last}}}},
		"discarded": {Place: Local, Artifact: Artifact{Status: Discarded, Versions: []Version{{N: 1, At: last}}}},
		"gone":      {Place: "records", Gone: true, Artifact: Artifact{Status: Live, Versions: []Version{{N: 1, At: last}}}},
		"in flight": {Place: "records", Artifact: Artifact{Status: Live, Versions: []Version{{N: 1, At: last}, {N: 2, At: last, Capsule: "records/glow"}}}},
	} {
		if _, ok := c.Expires(e); ok {
			t.Errorf("%s expires", name)
		}
	}
	if _, ok := (Config{}).Expires(e); ok {
		t.Error("expire_after = never, and it expires")
	}
}

// Promoted on this machine, its record says so and which version; put
// back from .discarded, it's as it was, with when; and 30 days discarded,
// its folder's deleted for good, but its row stays.
func TestPromoteRestoreErase(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	add(t, s, "Horizon glow", "Accent at 8%.")
	if _, _, err := s.AddVersion(ctx, "a1", Revision{HTML: []byte(page), At: day.Add(time.Hour), Version: Version{By: "you", Note: "Softer."}}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Promote(ctx, "a1", Promotion{At: day, By: "you", Version: 3}); err == nil || !strings.Contains(err.Error(), "no v3") {
		t.Errorf("promoting v3: %v", err)
	}
	e, err := s.Promote(ctx, "a1", Promotion{At: day, By: "you", Into: "glow"})
	if err != nil || e.Status != Promoted || e.Promoted.Version != 2 || e.Promoted.Into != "glow" {
		t.Fatalf("promoted: %+v %v", e.Promoted, err)
	}
	if data, _ := os.ReadFile(filepath.Join(s.Path(e), RecordFile)); !strings.Contains(string(data), "[promoted]") || !strings.Contains(string(data), `status = "promoted"`) {
		t.Errorf("its record:\n%s", data)
	}

	if _, err := s.Discard(ctx, "a1", Discard{At: day.Add(2 * time.Hour), By: "you", Reason: "Not now."}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Promote(ctx, "a1", Promotion{At: day, By: "you"}); err == nil {
		t.Error("promoted, discarded")
	}
	back := day.Add(3 * time.Hour)
	e, err = s.Restore(ctx, "a1", Restoration{At: back, By: "you"})
	if err != nil {
		t.Fatal(err)
	}
	if e.Status != Promoted || e.Discarded != nil || e.Restored == nil || !e.Restored.At.Equal(back.UTC()) {
		t.Errorf("restored: %+v", e.Artifact)
	}
	if _, err := os.Stat(filepath.Join(s.Dir(), e.Folder, "v2.html")); err != nil {
		t.Errorf("its folder isn't back: %v", err)
	}
	if _, err := s.Restore(ctx, "a1", Restoration{At: back, By: "you"}); err == nil {
		t.Error("restored twice")
	}

	// Discarded 31 days ago, and swept.
	add(t, s, "Another", "One.")
	if _, err := s.Discard(ctx, "a2", Discard{At: day.Add(-31 * 24 * time.Hour), By: "ws", Reason: "expired"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Discard(ctx, "a1", Discard{At: day, By: "you"}); err != nil {
		t.Fatal(err)
	}
	erased, err := s.Erase(ctx, day.Add(-KeepDiscarded))
	if err != nil || len(erased) != 1 || erased[0].ID != "a2" {
		t.Fatalf("erased %+v, %v", erased, err)
	}
	e, _ = s.Get(ctx, "a2")
	if !e.Erased || e.Gone || e.Restorable() || e.Title != "Another" {
		t.Errorf("a2 after: %+v", e)
	}
	if _, err := os.Stat(filepath.Join(s.DiscardedDir(), e.Folder)); !os.IsNotExist(err) {
		t.Errorf("its folder's still there: %v", err)
	}
	if _, err := s.Restore(ctx, "a2", Restoration{At: day, By: "you"}); err == nil || !strings.Contains(err.Error(), "gone for good") {
		t.Errorf("restoring a2: %v", err)
	}
	if a1, _ := s.Get(ctx, "a1"); !a1.Restorable() {
		t.Error("a1, discarded today, isn't restorable")
	}
}

// What took an artifact's folder away is read back from its commit's
// body: Clear expired's list, or what the discard said.
func TestRemovedWhy(t *testing.T) {
	a := &Artifact{Title: "Horizon glow", Versions: []Version{{N: 1, At: day}, {N: 2, At: day}}}
	if got := RemovedWhy("artifacts/2026-11-12-horizon-glow", DiscardBody("you", "the second one's better: softer", a)); got != "the second one's better: softer" {
		t.Errorf("discarded: %q", got)
	}
	if got := RemovedWhy("artifacts/x", DiscardBody("you", "", a)); got != "" {
		t.Errorf("discarded with no reason: %q", got)
	}
	body := ClearBody("you", "30 days", []Entry{{Folder: "artifacts/2026-10-01-glow", Artifact: *a}, {Folder: "artifacts/2026-10-02-other", Artifact: *a}})
	if got := RemovedWhy("artifacts/2026-10-02-other", body); got != "expired" {
		t.Errorf("cleared: %q\n%s", got, body)
	}
	if got := RemovedWhy("artifacts/2026-10-02", body); got != "" {
		t.Errorf("not cleared: %q", got)
	}
}

// Moving one from this machine to a repo: while it lands, it's listed
// once, as the repo's, and not as gone; put back, it's as it was; landed,
// it's read from the ground under the same ref.
func TestMove(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	ground := filepath.Join(t.TempDir(), ".ground")
	s.Repo = &Repo{Name: "records", Ground: ground, Dir: "artifacts"}
	e := add(t, s, "Horizon glow", "Accent at 8%.")
	local := filepath.Join(s.Dir(), e.Folder)

	m, err := s.StartMove(ctx, "a1", "records", "artifacts/"+e.Folder)
	if err != nil {
		t.Fatal(err)
	}
	list, _ := s.List(ctx)
	if len(list) != 1 || list[0].Place != "records" || list[0].Gone {
		t.Fatalf("while it moves: %+v", list)
	}
	if err := m.Undo(ctx); err != nil {
		t.Fatal(err)
	}
	list, _ = s.List(ctx)
	if len(list) != 1 || list[0].ID != "a1" || list[0].Place != Local || list[0].Gone {
		t.Fatalf("put back: %+v", list)
	}
	if _, err := os.Stat(filepath.Join(local, RecordFile)); err != nil {
		t.Fatalf("its folder isn't back: %v", err)
	}

	// Again, and it lands: the ground has it.
	if m, err = s.StartMove(ctx, "a1", "records", "artifacts/"+e.Folder); err != nil {
		t.Fatal(err)
	}
	copyTree(t, filepath.Join(s.Dir(), ".moving-"+e.Folder), filepath.Join(ground, "artifacts", e.Folder))
	if err := m.Done(); err != nil {
		t.Fatal(err)
	}
	list, _ = s.List(ctx)
	if len(list) != 1 || list[0].ID != "a1" || list[0].Place != "records" || list[0].Gone || list[0].Title != "Horizon glow" {
		t.Fatalf("landed: %+v", list)
	}
	if entries, _ := os.ReadDir(s.Dir()); len(entries) != 0 {
		t.Errorf(".ws/artifacts still has %v", entries)
	}
}
