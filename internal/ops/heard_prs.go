package ops

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/withworkspace/workspace/internal/events"
	"github.com/withworkspace/workspace/internal/pulls"
	"github.com/withworkspace/workspace/internal/store"
	"github.com/withworkspace/workspace/internal/team"
)

// Every open pull request in the workspace's repos is listed for the
// Reviews page (pulls.Service), and the list is read again from GitHub
// now and then. What moved between one list and the next is heard as
// events: a pull request opened, your review asked for, approved or
// changes asked, its checks going red or green, and one that's left the
// list merged or closed (GitHub's asked which).
//
// The list is kept in the records, so the next one is compared with the
// last whichever process read it. Those this ws opened for a mission are
// heard only for what the mission's own log doesn't say (it says they
// opened, merged, and failed their checks).
//
// Who did it is the pull request's author as a person, but for a review
// asked of you, approved or changes asked: GitHub's list doesn't say who,
// so those are by GitHub (events.ByGitHub), or you where it was your
// review.

// hearPRs keeps what moved between prev and next, the Reviews list as it
// was and as it is, as heard events.
func (w *Workspace) hearPRs(prev, next *pulls.List) {
	heard := w.prMoves(prev, next)
	if len(heard) == 0 {
		return
	}
	db, err := w.Records()
	if err != nil {
		return
	}
	logFailed(db.Write(context.Background(), func(tx *store.Tx) error {
		for _, e := range heard {
			if _, err := events.Append(tx, e); err != nil {
				return err
			}
		}
		return nil
	}))
}

// prMoves is what moved between prev and next, as heard events.
func (w *Workspace) prMoves(prev, next *pulls.List) []events.Event {
	was := map[string]pulls.PR{}
	for _, pr := range prev.All() {
		was[prKey(pr)] = pr
	}
	now := map[string]pulls.PR{}
	for _, pr := range next.All() {
		now[prKey(pr)] = pr
	}
	people := w.loginPeople(next.Me, append(prev.All(), next.All()...))
	var out []events.Event
	heard := func(kind string, pr pulls.PR, by string) {
		e := events.Event{Kind: kind, Origin: events.Heard, By: by, Repo: pr.Repo, PR: prKey(pr), Title: pr.Title, Branch: pr.Branch}
		if pr.Mission != nil {
			e.Mission = pr.Mission.Slug
		}
		out = append(out, e)
	}
	for _, key := range sortedKeys(now) {
		pr := now[key]
		author := people(pr.Author)
		mission := pr.Mission != nil // its own log says it opened, merged and failed
		old, ok := was[key]
		if !ok {
			if !mission {
				heard(events.PROpened, pr, author)
			}
			if pr.Asked != "" {
				heard(events.PRReviewRequested, pr, events.ByGitHub)
			}
			continue
		}
		if pr.Asked != "" && old.Asked == "" {
			heard(events.PRReviewRequested, pr, events.ByGitHub)
		}
		if pr.Decision != old.Decision {
			by := events.ByGitHub
			if pr.Reviewed != nil && (old.Reviewed == nil || *pr.Reviewed != *old.Reviewed) {
				by = events.ByYou
			}
			switch pr.Decision {
			case "approved":
				heard(events.PRApproved, pr, by)
			case "changes_requested":
				heard(events.PRChangesRequested, pr, by)
			}
		}
		if pr.Checks != old.Checks {
			switch {
			case pr.Checks == "failure" && !mission:
				heard(events.PRChecksFailed, pr, author)
			case pr.Checks == "success" && old.Checks != "":
				heard(events.PRChecksPassed, pr, author)
			}
		}
	}
	for _, key := range sortedKeys(was) {
		if _, ok := now[key]; ok {
			continue
		}
		pr := was[key]
		kind, by := w.prLeft(pr, people)
		if kind == "" || (kind == events.PRMerged && pr.Mission != nil) {
			continue
		}
		heard(kind, pr, by)
	}
	return out
}

// prLeft is how a pull request that's left the list went, as GitHub says:
// merged or closed (by its author, as a person, there being no telling
// who merged it), or "" when GitHub can't say.
func (w *Workspace) prLeft(pr pulls.PR, people func(string) string) (kind, by string) {
	gh := w.GitHub()
	if gh == nil {
		return "", ""
	}
	remote := w.WS().RemoteOf(pr.Repo)
	got, err := gh.PRFromNumber(remote.Owner, remote.Name, pr.Number)
	if err != nil || got == nil {
		return "", ""
	}
	switch {
	case strings.EqualFold(got.State, "MERGED") || got.MergedAt != "":
		return events.PRMerged, people(pr.Author)
	case strings.EqualFold(got.State, "CLOSED"):
		return events.PRClosed, people(pr.Author)
	}
	return "", "" // still open: no longer one of the list's (a repo taken off it)
}

// loginPeople is who a GitHub login is, as By says it: you for me, a
// person the roster knows by it, else the login itself as a person.
func (w *Workspace) loginPeople(me string, prs []pulls.PR) func(login string) string {
	var known []team.Event
	var cfg map[string]team.PersonConfig
	if t := w.Team(); t != nil {
		if snap := t.Peek(); snap != nil {
			known = snap.Events
		}
		if t.People != nil {
			cfg = t.People()
		}
	}
	all := slices.Clone(known)
	for _, pr := range prs {
		if pr.Author != "" {
			all = append(all, team.Event{Login: pr.Author})
		}
	}
	roster := team.Resolve(all, cfg)
	return func(login string) string {
		switch {
		case login == "":
			return events.ByGitHub
		case me != "" && strings.EqualFold(login, me):
			return events.ByYou
		}
		if p := roster.Of(team.Event{Login: login}); p != nil {
			return events.ByPerson(p.Key)
		}
		return events.ByPerson(login)
	}
}

func prKey(pr pulls.PR) string { return fmt.Sprintf("%s#%d", pr.Repo, pr.Number) }

func sortedKeys(m map[string]pulls.PR) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}
