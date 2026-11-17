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
}
