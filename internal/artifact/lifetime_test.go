package artifact

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/store"
)

// A live artifact expires expire_after after its last version, or after
// it was put back if that's later; promoted, discarded or gone, it
// doesn't; and the list warns in its last week.
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
		"gone":      {Place: Local, Gone: true, Artifact: Artifact{Status: Live, Versions: []Version{{N: 1, At: last}}}},
		"away":      {Place: "records", Artifact: Artifact{Status: Live, Versions: []Version{{N: 1, At: last}}}},
	} {
		if _, ok := c.Expires(e); ok {
			t.Errorf("%s expires", name)
		}
	}
	if _, ok := (Config{}).Expires(e); ok {
		t.Error("expire_after = never, and it expires")
	}
}

// Promoted, its record says so and which version, and where it lives in
// a repo once it's been put there; put back from .discarded, it's as it
// was, with when; and 30 days discarded, its folder's deleted for good,
// but its row stays.
func TestPromoteRestoreErase(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	add(t, s, "Horizon glow", "Accent at 8%.")
	if _, _, err := s.AddVersion(ctx, "a1", Revision{HTML: []byte(page), At: day.Add(time.Hour), Version: Version{By: "you", Note: "Softer."}}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Promote(ctx, "a1", Promotion{At: day, By: "you", Version: 3}, nil, false); err == nil || !strings.Contains(err.Error(), "no v3") {
		t.Errorf("promoting v3: %v", err)
	}
	e, err := s.Promote(ctx, "a1", Promotion{At: day, By: "you", Into: "glow"}, nil, true)
	if err != nil || e.Status != Promoted || e.Promoted.Version != 2 || e.Promoted.Into != "glow" || !e.Here {
		t.Fatalf("promoted: %+v %v", e.Promoted, err)
	}
	if data, _ := os.ReadFile(filepath.Join(s.Path(e), RecordFile)); !strings.Contains(string(data), "[promoted]") || !strings.Contains(string(data), `status = "promoted"`) || strings.Contains(string(data), "[home]") {
		t.Errorf("its record:\n%s", data)
	}
	if e.Home != nil || e.Lives() != "" || e.Kept() != "this machine" {
		t.Errorf("it lives nowhere but here: %+v %q %q", e.Home, e.Lives(), e.Kept())
	}
	// Put in a repo as it's promoted, its record says where it lives and
	// which version is there; promoted again with nowhere said, that
	// stays.
	home := &Home{Repo: "records", Folder: "artifacts/2026-11-12-horizon-glow", Version: 1, Commit: "4f2a9c1", At: day}
	e, err = s.Promote(ctx, "a1", Promotion{At: day, By: "you", Version: 1}, home, false)
	if err != nil || e.Here || e.Home == nil || *e.Home != (Home{Repo: "records", Folder: "artifacts/2026-11-12-horizon-glow", Version: 1, Commit: "4f2a9c1", At: day.UTC()}) {
		t.Fatalf("its home: %+v %v", e.Home, err)
	}
	if e.Lives() != "records:artifacts/2026-11-12-horizon-glow/" || e.Kept() != "this machine, and records" {
		t.Errorf("where it lives: %q %q", e.Lives(), e.Kept())
	}
	if data, _ := os.ReadFile(filepath.Join(s.Path(e), RecordFile)); !strings.Contains(string(data), "[home]\nrepo = \"records\"\nfolder = \"artifacts/2026-11-12-horizon-glow\"\nversion = 1\ncommit = \"4f2a9c1\"") {
		t.Errorf("its record:\n%s", data)
	}
	if e, err = s.Promote(ctx, "a1", Promotion{At: day, By: "you", Into: "glow"}, nil, false); err != nil || e.Home == nil || e.Home.Version != 1 || e.Promoted.Version != 2 {
		t.Fatalf("promoted again: %+v %+v %v", e.Home, e.Promoted, err)
	}
	if e, err = s.PutHome(ctx, "a1", Home{Repo: "records", Folder: "artifacts/2026-11-12-horizon-glow", Version: 2, Commit: "9b01d22", At: day}); err != nil || e.Home.Version != 2 || e.Home.Commit != "9b01d22" {
		t.Fatalf("put there again: %+v %v", e.Home, err)
	}

	if _, err := s.Discard(ctx, "a1", Discard{At: day.Add(2 * time.Hour), By: "you", Reason: "Not now."}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Promote(ctx, "a1", Promotion{At: day, By: "you"}, nil, false); err == nil {
		t.Error("promoted, discarded")
	}
	// Discarded, it lives in no repo, and still knows where it did.
	if d, _ := s.Get(ctx, "a1"); d.Lives() != "" || d.Home == nil || d.Kept() != "this machine" {
		t.Errorf("discarded: %q %+v", d.Lives(), d.Home)
	}
	back := day.Add(3 * time.Hour)
	e, err = s.Restore(ctx, "a1", Restoration{At: back, By: "you"})
	if err != nil {
		t.Fatal(err)
	}
	if e.Status != Promoted || e.Discarded != nil || e.Restored == nil || !e.Restored.At.Equal(back.UTC()) || e.Lives() == "" {
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

// What an artifact's home in a repo holds: the version chosen, as
// index.html, and a record of it that says what it is, that it was
// promoted, and each version up to that one. Whatever the folder held
// before goes.
func TestWriteHome(t *testing.T) {
	a := &Artifact{Title: "Horizon glow", Status: Live, Made: day, Home: &Home{Repo: "records", Folder: "artifacts/glow", Version: 1},
		Versions: []Version{{N: 1, At: day, By: "you", Note: "One."}, {N: 2, BasedOn: 1, At: day, By: "c1", From: "glow", Note: "Two."}, {N: 3, BasedOn: 2, At: day, By: "you", Note: "Three."}}}
	dir := filepath.Join(t.TempDir(), "artifacts", "glow")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "v1.html"), []byte("old"), 0o644)
	p := Promotion{At: day.UTC(), By: "you", Version: 2, Into: "glow"}
	if err := WriteHome(dir, a, p, []byte(page)); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(dir)
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	if strings.Join(names, " ") != "artifact.toml index.html" {
		t.Errorf("the folder holds %v", names)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, HomeFile)); string(got) != page {
		t.Errorf("index.html: %q", got)
	}
	data, _ := os.ReadFile(filepath.Join(dir, RecordFile))
	rec, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Title != "Horizon glow" || rec.Status != Promoted || rec.Promoted == nil || *rec.Promoted != p || rec.Home != nil || len(rec.Versions) != 2 || rec.Versions[1].Note != "Two." {
		t.Errorf("its record there:\n%s", data)
	}
}

// An artifact that was kept in a repo whole, from before they all lived
// on this machine, is brought here: its files from the repo's ground,
// its ref the same. One that was promoted lives where it was; one that
// wasn't is this machine's only; and one whose folder isn't on the ground
// is here too, with nothing to read.
func TestBringHome(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	ground := t.TempDir()
	// Three in records, as ws kept them: one promoted, one live, one whose
	// folder has gone; and one on this machine all along.
	was := func(n int, folder string, a Artifact, files bool) {
		t.Helper()
		if files {
			dir := filepath.Join(ground, filepath.FromSlash(folder))
			os.MkdirAll(dir, 0o755)
			for _, v := range a.Versions {
				os.WriteFile(filepath.Join(dir, VersionFile(v.N)), []byte(page), 0o644)
			}
			if _, err := writeRecord(dir, &a); err != nil {
				t.Fatal(err)
			}
		}
		db, _ := s.records()
		if err := db.Write(ctx, func(tx *store.Tx) error {
			return saveAs(tx, &kept{Entry: Entry{ID: "a" + string(rune('0'+n)), N: n, Place: "records", Folder: folder, Artifact: a}}, 10, "then")
		}); err != nil {
			t.Fatal(err)
		}
	}
	vs := []Version{{N: 1, At: day, By: "you", Note: "One."}, {N: 2, BasedOn: 1, At: day, By: "you", Note: "Two."}}
	was(1, "artifacts/2026-11-12-horizon-glow", Artifact{Title: "Horizon glow", Status: Promoted, Made: day, Promoted: &Promotion{At: day.UTC(), By: "you", Version: 1}, Versions: vs}, true)
	was(2, "artifacts/2026-11-12-two-takes", Artifact{Title: "Two takes", Status: Live, Made: day, Versions: vs[:1]}, true)
	was(3, "artifacts/2026-11-12-never-landed", Artifact{Title: "Never landed", Status: Live, Made: day, Versions: vs[:1]}, false)
	local := add(t, s, "Here all along", "One.")
	if local.ID != "a4" {
		t.Fatalf("the one here is %s", local.ID)
	}
	if e, _ := s.Get(ctx, "a1"); !e.Away() {
		t.Fatalf("before: %+v", e)
	}
	if _, _, err := s.AddVersion(ctx, "a1", Revision{HTML: []byte(page), At: day, Version: Version{By: "you"}}); err == nil || !strings.Contains(err.Error(), "hasn't been brought onto this machine") {
		t.Errorf("a version of one that's away: %v", err)
	}

	brought, err := s.BringHome(ctx, func(repo string) string {
		if repo == "records" {
			return ground
		}
		return ""
	})
	if err != nil || len(brought) != 3 {
		t.Fatalf("brought %+v, %v", brought, err)
	}
	a1, _ := s.Get(ctx, "a1")
	if a1.Away() || a1.Folder != "2026-11-12-horizon-glow" || a1.Gone || a1.Status != Promoted || a1.Lives() != "records:artifacts/2026-11-12-horizon-glow/" || a1.Home.Version != 1 {
		t.Errorf("a1: %+v, home %+v", a1, a1.Home)
	}
	if text, _, n, err := s.ReadVersion(ctx, "a1", 0); err != nil || n != 2 || string(text) != page {
		t.Errorf("a1's latest: v%d %v", n, err)
	}
	a2, _ := s.Get(ctx, "a2")
	if a2.Away() || a2.Home != nil || a2.Status != Live || a2.Gone {
		t.Errorf("a2: %+v", a2)
	}
	if _, err := os.Stat(filepath.Join(s.Dir(), "2026-11-12-two-takes", "v1.html")); err != nil {
		t.Errorf("a2's file isn't here: %v", err)
	}
	a3, _ := s.Get(ctx, "a3")
	if a3.Away() || !a3.Gone || a3.Title != "Never landed" {
		t.Errorf("a3: %+v", a3)
	}
	// It takes a version now, like any other, and nothing's brought twice.
	if _, n, err := s.AddVersion(ctx, "a1", Revision{HTML: []byte(page), At: day.Add(time.Hour), Version: Version{By: "you", Note: "Three."}}); err != nil || n != 3 {
		t.Errorf("a version of a1: v%d %v", n, err)
	}
	if again, err := s.BringHome(ctx, func(string) string { return ground }); err != nil || len(again) != 0 {
		t.Errorf("brought again: %+v %v", again, err)
	}
	// What's in the repo is left as it was.
	if _, err := os.Stat(filepath.Join(ground, "artifacts", "2026-11-12-two-takes", "v1.html")); err != nil {
		t.Errorf("the repo's folder was touched: %v", err)
	}
}
