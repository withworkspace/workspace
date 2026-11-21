package pulls

import (
	"errors"
	"slices"
	"testing"

	"github.com/withworkspace/workspace/internal/github"
	"github.com/withworkspace/workspace/internal/ref"
)

// The PRs asking for your review are searched for one owner at a time:
// each owner the repos are under is asked, and what each says is named by
// the repo's name here. A PR of a repo the workspace hasn't is left out,
// and an owner that couldn't be asked doesn't lose the others' answers.
func TestReviewRequested_AsksEachOwnerAndNamesReposAsHere(t *testing.T) {
	remotes := map[string]ref.Remote{
		"api":    {Owner: "acme", Name: "api"},
		"web":    {Owner: "acme", Name: "web"},
		"cr-api": {Owner: "cr-team", Name: "api"},
		"gone":   {Owner: "sso-org", Name: "gone"},
	}
	var asked []string
	gh := &github.Stub{ReviewRequestedFn: func(owner string) ([]github.PRRef, error) {
		asked = append(asked, owner)
		switch owner {
		case "acme":
			return []github.PRRef{{Owner: "acme", Repo: "api", Number: 1}, {Owner: "acme", Repo: "elsewhere", Number: 2}}, nil
		case "cr-team":
			return []github.PRRef{{Repo: "api", Number: 3}}, nil // no owner said: the one asked
		}
		return nil, errors.New("SAML enforcement")
	}}
	s := &Service{GitHub: gh, Remote: func(repo string) ref.Remote { return remotes[repo] }}

	got, err := s.reviewRequested([]string{"api", "cr-api", "gone", "web"})
	slices.Sort(asked)
	if !slices.Equal(asked, []string{"acme", "cr-team", "sso-org"}) {
		t.Errorf("asked %v, want each owner once", asked)
	}
	want := []github.PRRef{{Owner: "acme", Repo: "api", Number: 1}, {Owner: "cr-team", Repo: "cr-api", Number: 3}}
	slices.SortFunc(got, func(a, b github.PRRef) int { return a.Number - b.Number })
	if !slices.Equal(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
	if err == nil {
		t.Error("the owner that couldn't be asked should be said")
	}
}
