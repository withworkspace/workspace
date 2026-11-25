package github

import (
	"encoding/json"
	"fmt"
	"time"
)

// What the Team page asks of GitHub: a repo's pull requests that have
// changed lately, open or not, each with what's happened to it (who
// reviewed it and how, when it was made ready, when and by whom it merged
// or closed).

// PRActivity is a pull request with what's happened to it.
type PRActivity struct {
	PR
	ClosedAt string `json:"closedAt,omitempty"`
	// MergedBy is who merged it, and MergeCommit the commit it became on
	// the branch it was for.
	MergedBy    string `json:"mergedBy,omitempty"`
	MergeCommit string `json:"mergeCommit,omitempty"`
	// ReadyAt is when it was last taken out of draft, and ReadyBy by whom.
	ReadyAt string `json:"readyAt,omitempty"`
	ReadyBy string `json:"readyBy,omitempty"`
	// Reviews are every review of it, oldest first.
	Reviews []PRReview `json:"reviews,omitempty"`
	// Body is its description.
	Body string `json:"body,omitempty"`
	// AuthorName is its author's name, as their profile has it.
	AuthorName string `json:"authorName,omitempty"`
}

// PRReview is one review of a pull request.
type PRReview struct {
	Author string `json:"author"`
	// Name is the reviewer's name, as their profile has it.
	Name string `json:"name,omitempty"`
	Bot  bool   `json:"bot,omitempty"`
	// State is APPROVED, CHANGES_REQUESTED, COMMENTED, DISMISSED or PENDING.
	State       string `json:"state"`
	SubmittedAt string `json:"submittedAt,omitempty"`
}

// changedPRsQuery asks for a repo's pull requests, latest changed first,
// with what the Team page says of each.
//
// VERIFIED (github.com, 2026-11-10): one page costs 1 of the hour's 5,000
// points, whatever the repo.
const changedPRsQuery = `query($owner: String!, $repo: String!, $after: String) {
  repository(owner: $owner, name: $repo) {
    pullRequests(first: 50, after: $after, orderBy: {field: UPDATED_AT, direction: DESC}) {
      pageInfo { hasNextPage endCursor }
      nodes {
        number title url state isDraft body
        headRefName baseRefName
        additions deletions changedFiles createdAt updatedAt mergedAt closedAt
        author { __typename login ... on User { name } }
        mergedBy { login }
        mergeCommit { oid }
        reviews(first: 50) { nodes { author { __typename login ... on User { name } } state submittedAt } }
        timelineItems(itemTypes: [READY_FOR_REVIEW_EVENT], last: 1) { nodes { ... on ReadyForReviewEvent { createdAt actor { login } } } }
      }
    }
  }
}`

// changedPRPages is the most pages of changed PRs read from a repo (50 a
// page).
const changedPRPages = 10

// ChangedPRs is a repo's pull requests that have changed since a time,
// open, merged or closed, each with what's happened to it.
func ChangedPRs(org, repo string, since time.Time) ([]PRActivity, error) {
	var out []PRActivity
	after := ""
	for range changedPRPages {
		args := []string{"api", "graphql", "-f", "query=" + changedPRsQuery, "-f", "owner=" + org, "-f", "repo=" + repo}
		if after != "" {
			args = append(args, "-f", "after="+after)
		}
		stdOut, stdErr, err := ghExec(args...)
		if err != nil {
			return nil, fmt.Errorf("changed PRs of %s: %s", repoSlug(org, repo), ghError(stdOut.Bytes(), stdErr.String(), err))
		}
		page, next, err := parseChangedPRs(stdOut.Bytes())
		if err != nil {
			return nil, err
		}
		older := false
		for _, pr := range page {
			if changedSince(pr.UpdatedAt, since) {
				out = append(out, pr)
			} else {
				older = true
			}
		}
		// They come latest changed first: one from before means the rest are.
		if next == "" || older {
			break
		}
		after = next
	}
	return out, nil
}

// changedSince reports whether a time GitHub gave is at or after since. One
// that can't be read counts: better shown than dropped.
func changedSince(at string, since time.Time) bool {
	if since.IsZero() || at == "" {
		return true
	}
	t, err := time.Parse(time.RFC3339, at)
	return err != nil || !t.Before(since)
}

// parseChangedPRs reads a page of changedPRsQuery, and the cursor of the
// next ("" for none).
func parseChangedPRs(data []byte) ([]PRActivity, string, error) {
	type who struct {
		Type  string `json:"__typename"`
		Login string `json:"login"`
		Name  string `json:"name"`
	}
	var res struct {
		Data struct {
			Repository struct {
				PullRequests struct {
					PageInfo struct {
						HasNextPage bool   `json:"hasNextPage"`
						EndCursor   string `json:"endCursor"`
					} `json:"pageInfo"`
					Nodes []struct {
						PR
						Body        string `json:"body"`
						ClosedAt    string `json:"closedAt"`
						Author      who    `json:"author"`
						MergedBy    *who   `json:"mergedBy"`
						MergeCommit *struct {
							OID string `json:"oid"`
						} `json:"mergeCommit"`
						Reviews struct {
							Nodes []struct {
								Author      who    `json:"author"`
								State       string `json:"state"`
								SubmittedAt string `json:"submittedAt"`
							} `json:"nodes"`
						} `json:"reviews"`
						TimelineItems struct {
							Nodes []struct {
								CreatedAt string `json:"createdAt"`
								Actor     *who   `json:"actor"`
							} `json:"nodes"`
						} `json:"timelineItems"`
					} `json:"nodes"`
				} `json:"pullRequests"`
			} `json:"repository"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(data, &res); err != nil {
		return nil, "", fmt.Errorf("parsing changed PRs: %w", err)
	}
	if len(res.Errors) > 0 {
		return nil, "", fmt.Errorf("changed PRs: %s", res.Errors[0].Message)
	}
	prs := res.Data.Repository.PullRequests
	var out []PRActivity
	for _, n := range prs.Nodes {
		a := PRActivity{PR: n.PR, ClosedAt: n.ClosedAt, Body: n.Body}
		a.Author, a.Bot, a.AuthorName = n.Author.Login, n.Author.Type == "Bot", n.Author.Name
		if n.MergedBy != nil {
			a.MergedBy = n.MergedBy.Login
		}
		if n.MergeCommit != nil {
			a.MergeCommit = n.MergeCommit.OID
		}
		for _, r := range n.Reviews.Nodes {
			a.Reviews = append(a.Reviews, PRReview{Author: r.Author.Login, Name: r.Author.Name, Bot: r.Author.Type == "Bot", State: r.State, SubmittedAt: r.SubmittedAt})
		}
		if items := n.TimelineItems.Nodes; len(items) > 0 {
			a.ReadyAt = items[len(items)-1].CreatedAt
			if actor := items[len(items)-1].Actor; actor != nil {
				a.ReadyBy = actor.Login
			}
		}
		out = append(out, a)
	}
	next := ""
	if prs.PageInfo.HasNextPage {
		next = prs.PageInfo.EndCursor
	}
	return out, next, nil
}
