package pulls

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/github"
)

// asking is the world with what its searches say counted: lists is how
// many times GitHub was asked for a repo's pull requests (the list), and
// searches how many times who asks you by name was searched for, which
// says the world's own PRs that do unless found is set.
type asking struct {
	*world
	lists, searches int
	found           []github.PRRef
	foundErr        error
	override        bool
}

func newAsking(t *testing.T) *asking {
	a := &asking{world: newWorld(t)}
	a.gh.PRsForRepoFn = func(org, repo string) ([]github.PR, error) {
		a.lists++
		return a.prs, nil
	}
	a.gh.AskedByNameFn = func(org string) ([]github.PRRef, error) {
		a.searches++
		if a.foundErr != nil {
			return nil, a.foundErr
		}
		if a.override {
			return a.found, nil
		}
		var out []github.PRRef
		for _, p := range a.prs {
			for _, q := range p.Requested {
				if q.Login == "me" && !p.IsDraft {
					out = append(out, github.PRRef{Owner: org, Repo: "api", Number: p.Number})
				}
			}
		}
		return out, nil
	}
	return a
}

// later moves the clock on: past where a list just asked for still
// answers.
func (a *asking) later() { a.now = a.now.Add(time.Minute) }

func refs(asked []Asked) []string {
	var out []string
	for _, a := range asked {
		out = append(out, fmt.Sprintf("%s %s +%d", a.Key, a.PR.Ref(), a.More))
	}
	return out
}

func sameStrings(t *testing.T, what string, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s = %v, want %v", what, got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("%s[%d] = %q, want %q", what, i, got[i], want[i])
		}
	}
}

// What asks for your review by name is the pull requests whose own
// requests name you: not one asked of a team of yours, which someone else
// can answer, nor a draft. A stack is one, at its lowest layer that asks.
func TestAskedOfYouIsByNameAndNotADraft(t *testing.T) {
	a := newAsking(t)
	at := "2026-11-10T11:00:00Z"
	a.prs = append(a.prs,
		github.PR{Number: 93, Title: "Asked of your team", HeadRefName: "team", BaseRefName: "main", Author: "bob", UpdatedAt: at},
		github.PR{Number: 94, Title: "Still a draft", HeadRefName: "draft", BaseRefName: "main", Author: "bob", UpdatedAt: at, IsDraft: true,
			Requested: []github.ReviewRequest{{Login: "me"}}},
		github.PR{Number: 95, Title: "Asked of you", HeadRefName: "named", BaseRefName: "main", Author: "carol", UpdatedAt: at,
			Requested: []github.ReviewRequest{{Login: "me"}}},
	)
	// The top of the stack asks you too: still one, at the lowest.
	a.prs[2].Requested = []github.ReviewRequest{{Login: "me"}}
	a.gh.ReviewRequestedFn = func(org string) ([]github.PRRef, error) {
		return []github.PRRef{{Repo: "api", Number: 93}}, nil
	}
	if got := a.svc.AskedOfYou(); got != nil {
		t.Fatalf("before GitHub's been asked, asked = %v", got)
	}
	if _, err := a.svc.List(false); err != nil {
		t.Fatal(err)
	}
	got := a.svc.AskedOfYou()
	sameStrings(t, "asked", refs(got), []string{"api#95 api#95 +0", "api/stack-3 api#89 +1"})
	if got[1].Stack == nil || got[1].Stack.Title != "Storage layer" {
		t.Errorf("the stack's = %+v", got[1].Stack)
	}
}

// A look is one search: the list is asked for only when what the search
// found isn't what the list has, so a daemon that looks every few minutes
// asks GitHub about every repo only when something's changed.
func TestLookAsksForTheListOnlyWhenTheSearchDisagrees(t *testing.T) {
	a := newAsking(t)
	// Nothing kept yet, and someone's asking: the list is asked for.
	asked, err := a.svc.Look()
	if err != nil || !asked || a.lists != 1 {
		t.Fatalf("the first look: asked %v, lists %d, err %v", asked, a.lists, err)
	}
	sameStrings(t, "asked", refs(a.svc.AskedOfYou()), []string{"api/stack-3 api#89 +0"})

	// Nothing's changed: a search, and no more.
	a.later()
	if asked, err := a.svc.Look(); err != nil || asked || a.lists != 1 || a.searches != 2 {
		t.Fatalf("a look with nothing changed: asked %v, lists %d, searches %d, err %v", asked, a.lists, a.searches, err)
	}

	// You reviewed it somewhere else: it's gone from the search, so the
	// list is asked for, and it no longer asks.
	a.prs[1].Requested = nil
	a.later()
	if asked, err := a.svc.Look(); err != nil || !asked || a.lists != 2 {
		t.Fatalf("a look after the ask went: asked %v, lists %d, err %v", asked, a.lists, err)
	}
	if got := a.svc.AskedOfYou(); len(got) != 0 {
		t.Errorf("asked = %v, want nothing", refs(got))
	}

	// Someone new asks.
	a.prs[3].Requested = []github.ReviewRequest{{Login: "me"}}
	a.later()
	if asked, _ := a.svc.Look(); !asked || a.lists != 3 {
		t.Fatalf("a look after a new ask: asked %v, lists %d", asked, a.lists)
	}
	sameStrings(t, "asked", refs(a.svc.AskedOfYou()), []string{"api#92 api#92 +0"})
}

// GitHub's search and its pull requests can disagree for longer than a
// look: the list is asked for once on what the search said, not at every
// look until they agree. A new list is held against the search afresh.
func TestLookDoesntAskAgainOnTheSameDisagreement(t *testing.T) {
	a := newAsking(t)
	if _, err := a.svc.List(false); err != nil {
		t.Fatal(err)
	}
	a.override, a.found = true, []github.PRRef{{Repo: "api", Number: 89}, {Repo: "api", Number: 90}}
	a.later()
	if asked, _ := a.svc.Look(); !asked || a.lists != 2 {
		t.Fatalf("the first look: asked %v, lists %d", asked, a.lists)
	}
	a.later()
	if asked, _ := a.svc.Look(); asked || a.lists != 2 {
		t.Fatalf("the same disagreement again: asked %v, lists %d", asked, a.lists)
	}
	// The list was asked for by something else since (a Reviews page):
	// the search is held against that one.
	a.later()
	a.svc.Stale()
	if _, err := a.svc.List(false); err != nil {
		t.Fatal(err)
	}
	a.later()
	if asked, _ := a.svc.Look(); !asked || a.lists != 4 {
		t.Fatalf("after a new list: asked %v, lists %d", asked, a.lists)
	}
}

// A search that failed says nothing: part of an answer would read as
// pull requests gone.
func TestLookThatCouldntSearchLeavesTheListAlone(t *testing.T) {
	a := newAsking(t)
	if _, err := a.svc.List(false); err != nil {
		t.Fatal(err)
	}
	a.foundErr = errors.New("rate limited")
	a.later()
	if asked, err := a.svc.Look(); asked || err == nil || a.lists != 1 {
		t.Fatalf("asked %v, lists %d, err %v", asked, a.lists, err)
	}
	sameStrings(t, "asked", refs(a.svc.AskedOfYou()), []string{"api/stack-3 api#89 +0"})
}

// Something done here that changes what a PR asks of you (a review
// posted) is said to whoever follows the list, and their next look asks
// for it without searching.
func TestLookAsksForAStaleList(t *testing.T) {
	a := newAsking(t)
	due := a.svc.Due()
	if _, err := a.svc.List(false); err != nil {
		t.Fatal(err)
	}
	a.svc.Stale()
	select {
	case <-due:
	default:
		t.Fatal("going stale wasn't said")
	}
	if asked, err := a.svc.Look(); err != nil || !asked || a.lists != 2 || a.searches != 0 {
		t.Fatalf("asked %v, lists %d, searches %d, err %v", asked, a.lists, a.searches, err)
	}
}

// Taking your name off a pull request is GitHub's to do, as you; and it
// stops asking at once, before GitHub's been asked for the list again.
func TestDeclineTakesYourNameOffAndItStopsAsking(t *testing.T) {
	a := newAsking(t)
	if _, err := a.svc.List(false); err != nil {
		t.Fatal(err)
	}
	var dropped string
	a.gh.DropReviewRequestFn = func(org, repo string, number int, login string) error {
		dropped = fmt.Sprintf("%s/%s#%d %s", org, repo, number, login)
		for i := range a.prs {
			if a.prs[i].Number == number {
				a.prs[i].Requested = nil
			}
		}
		return nil
	}
	seen := a.svc.Seen()
	a.later()
	if err := a.svc.Decline("api", 89); err != nil {
		t.Fatal(err)
	}
	if dropped != "acme/api#89 me" {
		t.Errorf("dropped %q", dropped)
	}
	if got := a.svc.AskedOfYou(); len(got) != 0 {
		t.Errorf("asked = %v, want nothing: it was dealt with here", refs(got))
	}
	if a.lists != 1 {
		t.Errorf("declining asked for the list (%d times)", a.lists-1)
	}
	if a.svc.Seen() == seen {
		t.Error("what's listed changed, and Seen didn't move")
	}
	// The list asked for again agrees.
	if asked, _ := a.svc.Look(); !asked {
		t.Fatal("the stale list wasn't asked for")
	}
	if got := a.svc.AskedOfYou(); len(got) != 0 {
		t.Errorf("asked = %v after the list was asked for again", refs(got))
	}

	// One GitHub wouldn't let go of still asks.
	a.gh.DropReviewRequestFn = func(org, repo string, number int, login string) error { return errors.New("Not Found") }
	a.prs[3].Requested = []github.ReviewRequest{{Login: "me"}}
	a.later()
	a.svc.Stale()
	if _, err := a.svc.List(false); err != nil {
		t.Fatal(err)
	}
	if err := a.svc.Decline("api", 92); err == nil {
		t.Fatal("a refusal from GitHub wasn't said")
	}
	sameStrings(t, "asked", refs(a.svc.AskedOfYou()), []string{"api#92 api#92 +0"})
}
