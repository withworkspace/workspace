// Package team is the Team page's side of ws: what the people working in
// the workspace's repos have been doing, worked out from what lands on
// default branches (git) and from pull requests (GitHub). Everything read
// becomes events, plain facts; people are paired across their names in git
// and on GitHub; and the timeline is the events by day, each pull request
// or run of commits as one thing. It has no UI: the web UI and ws team
// show what it returns.
package team

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/github"
)

// Kinds of event.
const (
	Opened   = "opened"   // a pull request opened
	Ready    = "ready"    // a pull request taken out of draft
	Reviewed = "reviewed" // someone approved, asked for changes or commented
	Merged   = "merged"   // a pull request merged
	Closed   = "closed"   // a pull request closed without merging
	Landed   = "landed"   // a commit on a default branch with no pull request
)

// Verdicts of a review.
const (
	Approved  = "approved"
	Changes   = "changes"
	Commented = "commented"
)

// Event is one thing that happened: a plain fact, from git or GitHub.
type Event struct {
	// ID names it, and is the same however often it's read.
	ID   string    `json:"id"`
	At   time.Time `json:"at"`
	Kind string    `json:"kind"`
	Repo string    `json:"repo"`
	// PR is the pull request it's about, and Commit the commit: a landed
	// commit, or the one a pull request became.
	PR     int    `json:"pr,omitempty"`
	Commit string `json:"commit,omitempty"`
	// Title is the pull request's, or the commit's subject.
	Title string `json:"title"`
	URL   string `json:"url,omitempty"`
	// Branch is a pull request's branch, or the branch a commit landed on.
	Branch string `json:"branch,omitempty"`

	// Who did it: their login on GitHub, their name and email in git, or
	// both where one event has both (a squash merge's commit is its pull
	// request's author's).
	Login string `json:"login,omitempty"`
	Name  string `json:"name,omitempty"`
	Email string `json:"email,omitempty"`
	Bot   bool   `json:"bot,omitempty"`
	// Who is the key of the person it's by, once they're worked out: in a
	// timeline, not in what's kept.
	Who string `json:"who,omitempty"`

	// Verdict is a review's: approved, changes or commented.
	Verdict string `json:"verdict,omitempty"`
	// Draft: the pull request is a draft now (on its opened event).
	Draft bool `json:"draft,omitempty"`

	Additions int `json:"additions,omitempty"`
	Deletions int `json:"deletions,omitempty"`
	Files     int `json:"files,omitempty"`
	// Paths are the files a commit touched, the first of them.
	Paths []string `json:"paths,omitempty"`
}

// PR is a pull request as GitHub last said it, kept between looks so only
// what's changed is asked for again.
type PR struct {
	Repo string `json:"repo"`
	github.PRActivity
}

// Key is how a pull request is named: api#89.
func (p PR) Key() string { return prKey(p.Repo, p.Number) }

func prKey(repo string, number int) string { return repo + "#" + strconv.Itoa(number) }

var (
	// squashed is a squash merge's subject: its title, then its number.
	squashed = regexp.MustCompile(`\s*\(#(\d+)\)\s*$`)
	// mergedPR is a merge commit's subject.
	mergedPR = regexp.MustCompile(`^Merge pull request #(\d+) from `)
)

// prOfSubject is the pull request a commit's subject names, 0 for none,
// and the subject without it.
func prOfSubject(subject string) (int, string) {
	if m := mergedPR.FindStringSubmatch(subject); m != nil {
		n, _ := strconv.Atoi(m[1])
		return n, subject
	}
	if m := squashed.FindStringSubmatchIndex(subject); m != nil {
		n, _ := strconv.Atoi(subject[m[2]:m[3]])
		return n, strings.TrimSpace(subject[:m[0]])
	}
	return 0, subject
}

func parseTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

// Build is the events of some pull requests and of the commits on each
// repo's default branch (by repo, with the branch each was read from),
// oldest first. A commit that's a pull request's merge is that pull
// request's merged event, whichever of git and GitHub knows of it.
func Build(prs []PR, commits map[string][]Commit, branches map[string]string) []Event {
	byID := map[string]*Event{}
	var order []string
	put := func(e Event) *Event {
		if have, ok := byID[e.ID]; ok {
			return have
		}
		byID[e.ID] = &e
		order = append(order, e.ID)
		return byID[e.ID]
	}
	// A pull request's merge, by the commit it became.
	mergeOf := map[string]*PR{}
	byKey := map[string]*PR{}
	for i := range prs {
		pr := &prs[i]
		byKey[pr.Key()] = pr
		if pr.MergeCommit != "" {
			mergeOf[pr.Repo+":"+pr.MergeCommit] = pr
		}
		for _, e := range prEvents(*pr) {
			put(e)
		}
	}
	for repo, list := range commits {
		for _, c := range list {
			number, title := prOfSubject(c.Subject)
			// A merge commit's subject says where it came from; what it's
			// called is the line after.
			if number != 0 && c.Parents > 1 && mergedPR.MatchString(c.Subject) && strings.TrimSpace(c.Lead) != "" {
				title = strings.TrimSpace(c.Lead)
			}
			pr := mergeOf[repo+":"+c.SHA]
			if pr == nil && number != 0 {
				pr = byKey[prKey(repo, number)]
			}
			if pr != nil {
				number = pr.Number
			}
			if number == 0 {
				put(Event{
					ID: "c:" + repo + ":" + c.SHA, At: c.At, Kind: Landed, Repo: repo, Commit: c.SHA, Title: c.Subject, Branch: branches[repo],
					Name: c.Name, Email: c.Email, Bot: botName(c.Name, c.Email),
					Additions: c.Additions, Deletions: c.Deletions, Files: c.Files, Paths: c.Paths,
				})
				continue
			}
			e := put(Event{ID: mergedID(repo, number), At: c.At, Kind: Merged, Repo: repo, PR: number, Title: title, Bot: botName(c.Name, c.Email)})
			e.Commit, e.Paths = c.SHA, c.Paths
			if e.Files == 0 {
				e.Additions, e.Deletions, e.Files = c.Additions, c.Deletions, c.Files
			}
			// Whose commit it is: a squash's is its pull request's author's.
			// A merge commit's is whoever merged, who may be someone else.
			if e.Login == "" || c.Parents < 2 || (pr != nil && pr.MergedBy == pr.Author) {
				e.Name, e.Email = c.Name, c.Email
			}
		}
	}
	out := make([]Event, 0, len(order))
	for _, id := range order {
		out = append(out, *byID[id])
	}
	sort.SliceStable(out, func(i, j int) bool {
		if !out[i].At.Equal(out[j].At) {
			return out[i].At.Before(out[j].At)
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func mergedID(repo string, number int) string { return "pr:" + prKey(repo, number) + ":merged" }

// prEvents is what's happened to a pull request, as GitHub tells it.
func prEvents(pr PR) []Event {
	base := Event{
		Repo: pr.Repo, PR: pr.Number, Title: pr.Title, URL: pr.URL, Branch: pr.HeadRefName,
		Additions: pr.Additions, Deletions: pr.Deletions, Files: pr.ChangedFiles,
	}
	id := "pr:" + pr.Key() + ":"
	var out []Event
	add := func(kind, suffix, at, login string, bot bool, set func(*Event)) {
		t := parseTime(at)
		if t.IsZero() {
			return
		}
		e := base
		e.ID, e.At, e.Kind, e.Login, e.Bot = id+suffix, t, kind, login, bot
		if set != nil {
			set(&e)
		}
		out = append(out, e)
	}
	merged := pr.State == "MERGED" || pr.MergedAt != ""
	// Its author's name, as GitHub has it, is on what they did: a login
	// alone is a poor thing to call someone.
	authored := func(e *Event) { e.Name = pr.AuthorName }
	add(Opened, "opened", pr.CreatedAt, pr.Author, pr.Bot, func(e *Event) {
		authored(e)
		e.Draft = pr.IsDraft && !merged && pr.State != "CLOSED"
	})
	if pr.ReadyAt != "" && pr.ReadyAt != pr.CreatedAt {
		by := pr.ReadyBy
		if by == "" {
			by = pr.Author
		}
		add(Ready, "ready", pr.ReadyAt, by, pr.Bot && by == pr.Author, func(e *Event) {
			if by == pr.Author {
				authored(e)
			}
		})
	}
	for _, r := range pr.Reviews {
		// Replying on your own pull request is a review to GitHub, and
		// isn't one here.
		if r.Author == "" || r.Author == pr.Author {
			continue
		}
		verdict := ""
		switch r.State {
		case "APPROVED":
			verdict = Approved
		case "CHANGES_REQUESTED":
			verdict = Changes
		case "COMMENTED":
			verdict = Commented
		default: // pending, dismissed
			continue
		}
		add(Reviewed, fmt.Sprintf("reviewed:%s:%s", r.Author, r.SubmittedAt), r.SubmittedAt, r.Author, r.Bot, func(e *Event) { e.Verdict, e.Name = verdict, r.Name })
	}
	switch {
	case merged:
		add(Merged, "merged", pr.MergedAt, pr.Author, pr.Bot, func(e *Event) {
			authored(e)
			e.Commit = pr.MergeCommit
		})
	case pr.State == "CLOSED":
		add(Closed, "closed", pr.ClosedAt, pr.Author, pr.Bot, authored)
	}
	return out
}

// botName reports whether a commit's author is a bot, as far as its name
// and email say.
func botName(name, email string) bool {
	n, e := strings.ToLower(name), strings.ToLower(email)
	if strings.Contains(n, "[bot]") || strings.Contains(e, "[bot]") {
		return true
	}
	switch n {
	case "dependabot", "renovate", "renovate bot", "github-actions", "github actions", "github":
		return true
	}
	// Release tools commit under names of their own.
	return strings.HasSuffix(n, "-bot") || strings.HasSuffix(n, " bot") || strings.Contains(n, "semantic-release") || strings.Contains(n, "release-please")
}
