package review

import "testing"

func TestSummarize(t *testing.T) {
	dir := capsule(t)
	s, err := Summarize(dir, "origin/main")
	if err != nil {
		t.Fatal(err)
	}
	if s.CommitCount != 2 || len(s.Commits) != 2 || s.Commits[0].Subject != "Fix the var" {
		t.Errorf("commits %d %+v", s.CommitCount, s.Commits)
	}
	if s.Files != 6 || s.Additions == 0 || s.Deletions == 0 {
		t.Errorf("files %d +%d -%d", s.Files, s.Additions, s.Deletions)
	}
	if s.Uncommitted != 1 {
		t.Errorf("uncommitted %d, want 1 (scratch.txt)", s.Uncommitted)
	}

	// What differs from the branch point as the capsule is now: the six
	// committed files, and the one that isn't.
	got := map[string]Changed{}
	for _, f := range s.Changed {
		got[f.Path] = f
	}
	if len(s.Changed) != 7 || s.ChangedMore != 0 {
		t.Fatalf("changed %d (+%d more): %+v", len(s.Changed), s.ChangedMore, s.Changed)
	}
	if f := got["cache.go"]; f.Status != "modified" || f.Additions != 2 || f.Deletions != 2 || f.Uncommitted {
		t.Errorf("cache.go %+v", f)
	}
	if f := got["new name.md"]; f.Status != "renamed" || f.OldPath != "old name.md" || f.Skim != SkimRenamed {
		t.Errorf("new name.md %+v", f)
	}
	if f := got["gone.txt"]; f.Status != "deleted" || f.Deletions != 1 {
		t.Errorf("gone.txt %+v", f)
	}
	if f := got["scratch.txt"]; f.Status != "added" || !f.Uncommitted || f.Additions != 1 {
		t.Errorf("scratch.txt %+v", f)
	}
}

// A tracked file changed and not committed is among what's changed, with
// the lines that aren't committed counted.
func TestSummarizeUncommittedTracked(t *testing.T) {
	dir := capsule(t)
	write(t, dir, "cache.go", "package cache\n\nfunc key(tenant, id string) string {\n\treturn tenant + \":\" + id\n}\n\n// more\n")
	s, err := Summarize(dir, "origin/main")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range s.Changed {
		if f.Path == "cache.go" {
			if !f.Uncommitted || f.Additions != 4 || f.Deletions != 2 {
				t.Errorf("cache.go %+v", f)
			}
			return
		}
	}
	t.Errorf("cache.go isn't among %+v", s.Changed)
}
