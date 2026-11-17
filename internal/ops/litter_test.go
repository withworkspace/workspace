package ops

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"testing"
)

// What's under .ws that no record has is each store's to say, and ws
// doctor --fix moves it aside, keeping where it was; what has its record
// stays where it is.
func TestLitter_SweptAside(t *testing.T) {
	w := records(t)
	ctx := context.Background()
	ws := filepath.Join(w.Root(), ".ws")
	if got, err := w.Litter(ctx); err != nil || len(got) != 0 {
		t.Fatalf("litter in a workspace with none: %+v %v", got, err)
	}
	put := func(rel string) {
		t.Helper()
		p := filepath.Join(ws, filepath.FromSlash(rel))
		os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	litter := []string{
		"missions/ghost", // a folder no mission's row names
		"missions/house-sorting-cli/mission.toml",           // kept before the records
		"conversations/w99.run",                             // a transcript with no conversation
		"missions/greet-people-by-name/files/reviews/r9.md", // a rendered review with no review
		"pages/p5.md",   // a page's text with no row
		"pages/p2.json", // kept beside a page before the records
		"prs",
		"team",
		"before-db",
	}
	for _, rel := range []string{"missions/ghost/brief.md", "missions/house-sorting-cli/mission.toml", "conversations/w99.run", "missions/greet-people-by-name/files/reviews/r9.md", "pages/p5.md", "pages/p2.json", "prs/api/89/pr.json", "team/look.json", "before-db/conversations/w1.jsonl"} {
		put(rel)
	}
	kept := []string{"conversations/w1.run", "missions/house-sorting-cli/brief.md", "missions/greet-people-by-name/files/reviews/r1.md"}
	put(kept[0])

	got, err := w.Litter(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, l := range got {
		paths = append(paths, l.Path)
	}
	sort.Strings(paths)
	sort.Strings(litter)
	if !slices.Equal(paths, litter) {
		t.Errorf("litter:\n  %v\nwant\n  %v", paths, litter)
	}
	if n, err := w.Sweep(got); err != nil || n != len(litter) {
		t.Fatalf("swept %d: %v", n, err)
	}
	swept := SweptDir(w.Root(), w.now())
	for _, rel := range litter {
		if _, err := os.Stat(filepath.Join(ws, rel)); !os.IsNotExist(err) {
			t.Errorf("%s is still there", rel)
		}
		if _, err := os.Stat(filepath.Join(swept, rel)); err != nil {
			t.Errorf("%s isn't in %s: %v", rel, swept, err)
		}
	}
	for _, rel := range kept {
		if _, err := os.Stat(filepath.Join(ws, rel)); err != nil {
			t.Errorf("%s, which has its record, was moved: %v", rel, err)
		}
	}
	if got, err := w.Litter(ctx); err != nil || len(got) != 0 {
		t.Errorf("litter after sweeping: %+v %v", got, err)
	}
}
