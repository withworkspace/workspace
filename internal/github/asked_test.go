package github

import (
	"os"
	"path/filepath"
	"testing"
)

// The fixture says who asks you by name as GitHub's search would: the
// open PRs whose own requests name you, not those asked of a team of
// yours and not a draft; and taking your name off one is remembered.
func TestFixtureAskedByNameAndDropped(t *testing.T) {
	path := filepath.Join(t.TempDir(), "github.json")
	os.WriteFile(path, []byte(`{
		"viewer": "me",
		"requested": [{"repo": "api", "number": 2}],
		"repos": {"api": {"prs": [
			{"number": 1, "title": "One", "authorLogin": "alice", "headRefName": "one", "requested": [{"login": "me"}, {"login": "carol"}]},
			{"number": 2, "title": "Two", "authorLogin": "bob", "headRefName": "two"},
			{"number": 3, "title": "Draft", "authorLogin": "bob", "headRefName": "three", "isDraft": true, "requested": [{"login": "me"}]}]}}
	}`), 0o644)
	c := FixtureClient{Path: path}

	asked, err := c.AskedByName("acme")
	if err != nil || len(asked) != 1 || asked[0] != (PRRef{Owner: "acme", Repo: "api", Number: 1}) {
		t.Fatalf("asked by name = %+v, %v", asked, err)
	}
	if err := c.DropReviewRequest("acme", "api", 1, "me"); err != nil {
		t.Fatal(err)
	}
	if asked, _ := c.AskedByName("acme"); len(asked) != 0 {
		t.Errorf("after your name was taken off, asked by name = %+v", asked)
	}
	prs, _ := c.PRsForRepo("acme", "api")
	if len(prs[0].Requested) != 1 || prs[0].Requested[0].Login != "carol" {
		t.Errorf("the others asked = %+v, want carol still", prs[0].Requested)
	}
	if err := c.DropReviewRequest("acme", "api", 9, "me"); err == nil {
		t.Error("a PR that isn't open should be refused")
	}
}
