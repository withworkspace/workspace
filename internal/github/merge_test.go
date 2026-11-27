package github

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestMergeSettings(t *testing.T) {
	s, err := parseMergeSettings([]byte(`{"allow_squash_merge": false, "allow_merge_commit": true, "allow_rebase_merge": true, "allow_auto_merge": true, "delete_branch_on_merge": true}`))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(s.Methods, []string{MethodMerge, MethodRebase}) || !s.AutoMerge || !s.DeleteBranch {
		t.Fatalf("%+v", s)
	}
	for _, c := range []struct{ want, got string }{{"", MethodMerge}, {MethodRebase, MethodRebase}, {MethodSquash, MethodMerge}} {
		if m := s.Method(c.want); m != c.got {
			t.Errorf("Method(%q) = %q, want %q", c.want, m, c.got)
		}
	}
	if m := (MergeSettings{Methods: []string{MethodSquash, MethodMerge}}).Method(""); m != MethodSquash {
		t.Errorf("squash first: %q", m)
	}
}

func TestParseMerging(t *testing.T) {
	m, err := parseMerging([]byte(`{"id": "abc", "status": "pending", "details": {}}`))
	if err != nil || m.ID != "abc" || m.Status != MergePending || m.Why != "" {
		t.Fatalf("%+v %v", m, err)
	}
	m, err = parseMerging([]byte(`{"id": "abc", "status": "FAILED", "details": {"message": "Required status check \"test\" is failing."}}`))
	if err != nil || m.Status != MergeFailed || m.Why != `Required status check "test" is failing.` {
		t.Fatalf("%+v %v", m, err)
	}
	if _, err := parseMerging([]byte(`{}`)); err == nil {
		t.Fatal("no status read as one")
	}
}

// The fixture merges a stack's layer with the layers below it, and
// refuses a head that's moved.
func TestFixtureMergeStack(t *testing.T) {
	path := filepath.Join(t.TempDir(), "github.json")
	os.WriteFile(path, []byte(`{"repos": {"demo": {
	  "prs": [{"number": 1, "headRefName": "a", "headRefOid": "h1"}, {"number": 2, "headRefName": "b", "headRefOid": "h2"}, {"number": 3, "headRefName": "c"}],
	  "stacks": [{"number": 9, "base": "main", "open": true, "prs": [{"number": 1, "state": "open"}, {"number": 2, "state": "open"}]}]}}}`), 0o644)
	c := FixtureClient{Path: path}
	if _, err := c.MergePR("o", "demo", 2, MergeAsk{Head: "old"}); err == nil {
		t.Fatal("merged a head that moved")
	}
	m, err := c.MergePR("o", "demo", 2, MergeAsk{Head: "h2", Method: MethodSquash})
	if err != nil || m.Status != MergeMerged {
		t.Fatalf("%+v %v", m, err)
	}
	open, _ := c.PRsForRepo("o", "demo")
	merged, _ := c.MergedPRsForRepo("o", "demo")
	stacks, _ := c.StacksForRepo("o", "demo")
	if len(open) != 1 || open[0].Number != 3 || len(merged) != 2 || stacks[0].Open {
		t.Fatalf("open %v, merged %v, stacks %+v", open, merged, stacks)
	}
}
