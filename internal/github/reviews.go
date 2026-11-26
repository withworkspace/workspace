package github

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
)

// What the Reviews page asks of GitHub besides a repo's open PRs: who you
// are, which PRs ask for your review, a repo's stacks, posting a review,
// and the review threads on a PR.

// openPRsQuery asks for a repo's open PRs with what the Reviews page
// sorts them by, and nothing heavier: a check rollup's state rather than
// every check, and reviews without their bodies.
const openPRsQuery = `query($owner: String!, $repo: String!, $after: String) {
  repository(owner: $owner, name: $repo) {
    pullRequests(states: OPEN, first: 50, after: $after, orderBy: {field: UPDATED_AT, direction: DESC}) {
      pageInfo { hasNextPage endCursor }
      nodes {
        number title url state isDraft reviewDecision mergeable mergeStateStatus
        headRefName headRefOid baseRefName isCrossRepository
        additions deletions changedFiles createdAt updatedAt
        author { __typename login }
        reviewRequests(first: 20) { nodes { requestedReviewer { __typename ... on User { login } ... on Team { slug } } } }
        latestReviews(first: 20) { nodes { author { login } state submittedAt commit { oid } } }
        commits(last: 1) { nodes { commit { statusCheckRollup { state } } } }
      }
    }
  }
}`

// openPRPages is the most pages of open PRs read from a repo (50 a page).
const openPRPages = 10

// OpenPRs is a repo's open PRs for the Reviews page: who opened each (a
// bot?), who's asked to review it, the latest review by each person and
// the commit it was of, its size, its checks, and whether it merges
// cleanly.
//
// VERIFIED (github.com, 2026-11-10): gh pr list --json latestReviews
// leaves each review's commit empty, and asks for every check and every
// review's body, which is slow on a busy repo. So the list is asked for
// through GraphQL, where a review's commit is there.
func OpenPRs(org, repo string) ([]PR, error) {
	var out []PR
	after := ""
	for range openPRPages {
		args := []string{"api", "graphql", "-f", "query=" + openPRsQuery, "-f", "owner=" + org, "-f", "repo=" + repo}
		if after != "" {
			args = append(args, "-f", "after="+after)
		}
		stdOut, stdErr, err := ghExec(args...)
		if err != nil {
			return nil, fmt.Errorf("open PRs of %s: %s", repoSlug(org, repo), ghError(stdOut.Bytes(), stdErr.String(), err))
		}
		page, next, err := parseOpenPRs(stdOut.Bytes())
		if err != nil {
			return nil, err
		}
		out = append(out, page...)
		if next == "" {
			break
		}
		after = next
	}
	return out, nil
}

// parseOpenPRs reads a page of openPRsQuery, and the cursor of the next
// ("" for none).
func parseOpenPRs(data []byte) ([]PR, string, error) {
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
						Author struct {
							Type  string `json:"__typename"`
							Login string `json:"login"`
						} `json:"author"`
						ReviewRequests struct {
							Nodes []struct {
								RequestedReviewer struct {
									Type  string `json:"__typename"`
									Login string `json:"login"`
									Slug  string `json:"slug"`
								} `json:"requestedReviewer"`
							} `json:"nodes"`
						} `json:"reviewRequests"`
						LatestReviews struct {
							Nodes []struct {
								Author struct {
									Login string `json:"login"`
								} `json:"author"`
								State       string `json:"state"`
								SubmittedAt string `json:"submittedAt"`
								Commit      struct {
									OID string `json:"oid"`
								} `json:"commit"`
							} `json:"nodes"`
						} `json:"latestReviews"`
						Commits struct {
							Nodes []struct {
								Commit struct {
									StatusCheckRollup *struct {
										State string `json:"state"`
									} `json:"statusCheckRollup"`
								} `json:"commit"`
							} `json:"nodes"`
						} `json:"commits"`
					} `json:"nodes"`
				} `json:"pullRequests"`
			} `json:"repository"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(data, &res); err != nil {
		return nil, "", fmt.Errorf("parsing open PRs: %w", err)
	}
	if len(res.Errors) > 0 {
		return nil, "", fmt.Errorf("open PRs: %s", res.Errors[0].Message)
	}
	prs := res.Data.Repository.PullRequests
	var out []PR
	for _, n := range prs.Nodes {
		pr := n.PR
		pr.Author, pr.Bot = n.Author.Login, n.Author.Type == "Bot"
		for _, r := range n.ReviewRequests.Nodes {
			switch rr := r.RequestedReviewer; {
			case rr.Slug != "":
				pr.Requested = append(pr.Requested, ReviewRequest{Team: rr.Slug})
			case rr.Login != "":
				pr.Requested = append(pr.Requested, ReviewRequest{Login: rr.Login})
			}
		}
		for _, r := range n.LatestReviews.Nodes {
			pr.Latest = append(pr.Latest, LatestReview{Author: r.Author.Login, State: r.State, SubmittedAt: r.SubmittedAt, Commit: r.Commit.OID})
		}
		// The checks of its latest commit, as one state.
		if len(n.Commits.Nodes) > 0 {
			if roll := n.Commits.Nodes[0].Commit.StatusCheckRollup; roll != nil {
				switch roll.State {
				case "SUCCESS":
					pr.StatusRollup = "success"
				case "FAILURE", "ERROR":
					pr.StatusRollup = "failure"
				case "PENDING", "EXPECTED":
					pr.StatusRollup = "pending"
				}
			}
		}
		out = append(out, pr)
	}
	next := ""
	if prs.PageInfo.HasNextPage {
		next = prs.PageInfo.EndCursor
	}
	return out, next, nil
}

// Viewer is the login gh is signed in as.
func Viewer() (string, error) {
	stdOut, _, err := ghExec("api", "user", "-q", ".login")
	if err != nil {
		return "", fmt.Errorf("gh api user: %w", err)
	}
	login := strings.TrimSpace(stdOut.String())
	if login == "" {
		return "", fmt.Errorf("gh api user: no login")
	}
	return login, nil
}

// PRRef names a PR in one of the org's repos.
type PRRef struct {
	// Owner is whose the repo is on GitHub, and Repo its name there.
	Owner  string `json:"owner,omitempty"`
	Repo   string `json:"repo"`
	Number int    `json:"number"`
}

// ReviewRequested is the org's open PRs that ask for your review: by name,
// or of a team you're in.
//
// VERIFIED (GitHub's search, 2026-11-10): review-requested:@me "returns
// pull requests that have direct requests and team requests", so ws
// needn't know your teams. A request by name is told apart by the PR's own
// review requests (PR.Requested has your login); user-review-requested:@me
// finds the same.
func ReviewRequested(org string) ([]PRRef, error) {
	stdOut, _, err := ghExec(
		"search", "prs",
		"--review-requested=@me",
		"--state=open",
		"--owner="+org,
		"--limit", "200",
		"--json", "number,repository",
	)
	if err != nil {
		return nil, fmt.Errorf("gh search prs --review-requested=@me: %w", err)
	}
	var found []struct {
		Number     int `json:"number"`
		Repository struct {
			Name          string `json:"name"`
			NameWithOwner string `json:"nameWithOwner"`
		} `json:"repository"`
	}
	if err := json.Unmarshal(stdOut.Bytes(), &found); err != nil {
		return nil, fmt.Errorf("parsing gh output: %w", err)
	}
	out := make([]PRRef, 0, len(found))
	for _, f := range found {
		owner, _, _ := strings.Cut(f.Repository.NameWithOwner, "/")
		out = append(out, PRRef{Owner: owner, Repo: f.Repository.Name, Number: f.Number})
	}
	return out, nil
}

// Stack is a stack of pull requests, as GitHub knows it: PRs that build on
// one another, bottom first, merged from the bottom up.
type Stack struct {
	Number int       `json:"number"`
	Base   string    `json:"base"` // the branch the whole stack is for
	Open   bool      `json:"open"`
	PRs    []StackPR `json:"prs"`
}

// StackPR is one layer of a stack.
type StackPR struct {
	Number  int    `json:"number"`
	State   string `json:"state"` // open, closed
	Draft   bool   `json:"draft,omitempty"`
	Merged  bool   `json:"merged,omitempty"`
	HeadRef string `json:"headRef,omitempty"`
	HeadSHA string `json:"headSha,omitempty"`
}

// StacksAPIVersion is the REST API version the stacks calls were read
// against.
const StacksAPIVersion = "2026-03-10"

// stackPages is the most pages of a repo's stacks read (100 a page).
const stackPages = 5

// StacksForRepo lists a repo's stacks: those with a pull request still
// open. A repo where stacks aren't turned on has none.
//
// VERIFIED (docs.github.com/en/rest/pulls/stacks, and a public repo with
// stacks, 2026-11-10; public preview, so it may change): GET
// /repos/{owner}/{repo}/stacks answers an array of {id, number, base:
// {ref}, open, pull_requests: [{number, state, draft, merged_at, head:
// {ref, sha}}]}, paged (per_page up to 100), and 404 where there's nothing
// to list. pull_requests is bottom first, and each layer's PR is based on
// the branch of the one below. Stacks whose PRs have all merged are listed
// too (open: false), newest first, with no way to leave them out: so
// reading stops at a page with none open.
// A PR in a stack also carries stack: {number, position, size, base} from
// the REST list of PRs, but that's one more call per repo than this, and
// says less: the stacks are joined to the PRs by number instead.
func StacksForRepo(org, repo string) ([]Stack, error) {
	var out []Stack
	for page := 1; page <= stackPages; page++ {
		stdOut, stdErr, err := ghExec(
			"api", fmt.Sprintf("repos/%s/stacks?per_page=100&page=%d", repoSlug(org, repo), page),
			"-H", "Accept: application/vnd.github+json",
			"-H", "X-GitHub-Api-Version: "+StacksAPIVersion,
		)
		if err != nil {
			if strings.Contains(stdErr.String(), "HTTP 404") || strings.Contains(err.Error(), "HTTP 404") {
				return out, nil
			}
			return nil, fmt.Errorf("gh api stacks for %s: %s", repoSlug(org, repo), ghError(stdOut.Bytes(), stdErr.String(), err))
		}
		stacks, err := parseStacks(stdOut.Bytes())
		if err != nil {
			return nil, err
		}
		open := 0
		for _, s := range stacks {
			if s.Open {
				out = append(out, s)
				open++
			}
		}
		if len(stacks) < 100 || open == 0 {
			break
		}
	}
	return out, nil
}

// parseStacks reads a page of the stacks call.
func parseStacks(data []byte) ([]Stack, error) {
	var page []struct {
		Number int `json:"number"`
		Base   struct {
			Ref string `json:"ref"`
		} `json:"base"`
		Open         bool `json:"open"`
		PullRequests []struct {
			Number   int     `json:"number"`
			State    string  `json:"state"`
			Draft    bool    `json:"draft"`
			MergedAt *string `json:"merged_at"`
			Head     struct {
				Ref string `json:"ref"`
				SHA string `json:"sha"`
			} `json:"head"`
		} `json:"pull_requests"`
	}
	if err := json.Unmarshal(data, &page); err != nil {
		return nil, fmt.Errorf("parsing stacks: %w", err)
	}
	var out []Stack
	for _, r := range page {
		s := Stack{Number: r.Number, Base: r.Base.Ref, Open: r.Open}
		for _, p := range r.PullRequests {
			s.PRs = append(s.PRs, StackPR{Number: p.Number, State: p.State, Draft: p.Draft, Merged: p.MergedAt != nil && *p.MergedAt != "", HeadRef: p.Head.Ref, HeadSHA: p.Head.SHA})
		}
		out = append(out, s)
	}
	return out, nil
}

// Review events, as GitHub names them.
const (
	EventApprove        = "APPROVE"
	EventRequestChanges = "REQUEST_CHANGES"
	EventComment        = "COMMENT"
)

// ReviewSubmission is a review to post on a PR: its verdict, its summary,
// and its comments on lines, at the commit reviewed.
type ReviewSubmission struct {
	Commit   string          `json:"commit_id"`
	Event    string          `json:"event"`
	Body     string          `json:"body,omitempty"`
	Comments []ReviewComment `json:"comments,omitempty"`
}

// ReviewComment is a comment on lines of a PR's diff. Side is RIGHT for
// lines as they are at the commit, LEFT for lines the PR removes; a
// comment on several lines starts at StartLine.
type ReviewComment struct {
	Path      string `json:"path"`
	Body      string `json:"body"`
	Line      int    `json:"line"`
	Side      string `json:"side"`
	StartLine int    `json:"start_line,omitempty"`
	StartSide string `json:"start_side,omitempty"`
}

// SubmitReview posts a review on a PR, as you, and answers its URL.
func SubmitReview(org, repo string, number int, r ReviewSubmission) (string, error) {
	body, err := json.Marshal(r)
	if err != nil {
		return "", err
	}
	// gh reads a request's body from a file: Exec has no stdin.
	f, err := os.CreateTemp("", "ws-review-*.json")
	if err != nil {
		return "", err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(body); err != nil {
		f.Close()
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	stdOut, stdErr, err := ghExec(
		"api", "-X", "POST", "repos/"+repoSlug(org, repo)+"/pulls/"+strconv.Itoa(number)+"/reviews",
		"-H", "Accept: application/vnd.github+json",
		"--input", f.Name(),
	)
	if err != nil {
		return "", fmt.Errorf("posting the review on %s#%d: %s", repoSlug(org, repo), number, ghError(stdOut.Bytes(), stdErr.String(), err))
	}
	var res struct {
		URL string `json:"html_url"`
	}
	json.Unmarshal(stdOut.Bytes(), &res)
	return res.URL, nil
}

// ghError is what GitHub said was wrong with a request, as plainly as it
// can be read: its message and errors, else what gh printed.
func ghError(stdout []byte, stderr string, err error) string {
	var res struct {
		Message string            `json:"message"`
		Errors  []json.RawMessage `json:"errors"`
	}
	if json.Unmarshal(stdout, &res) == nil && res.Message != "" {
		msg := res.Message
		for _, e := range res.Errors {
			var s string
			var o struct {
				Message string `json:"message"`
			}
			switch {
			case json.Unmarshal(e, &s) == nil && s != "":
				msg += ": " + s
			case json.Unmarshal(e, &o) == nil && o.Message != "":
				msg += ": " + o.Message
			}
		}
		return msg
	}
	if s := strings.TrimSpace(stderr); s != "" {
		return s
	}
	return err.Error()
}

// ReviewThread is a thread of review comments on lines of a PR.
type ReviewThread struct {
	ID       string `json:"id"`
	Resolved bool   `json:"resolved,omitempty"`
	// Outdated: the lines it's on have changed since; Line is then 0, and
	// OriginalLine is where it was at its first comment's OriginalCommit.
	Outdated          bool            `json:"outdated,omitempty"`
	Path              string          `json:"path"`
	Line              int             `json:"line,omitempty"`
	StartLine         int             `json:"startLine,omitempty"`
	OriginalLine      int             `json:"originalLine,omitempty"`
	OriginalStartLine int             `json:"originalStartLine,omitempty"`
	Side              string          `json:"side"` // RIGHT or LEFT
	Comments          []ThreadComment `json:"comments"`
}

// ThreadComment is one comment in a review thread.
type ThreadComment struct {
	Author         string `json:"author"`
	Body           string `json:"body"`
	At             string `json:"at"`
	URL            string `json:"url,omitempty"`
	DiffHunk       string `json:"diffHunk,omitempty"`
	Commit         string `json:"commit,omitempty"`
	OriginalCommit string `json:"originalCommit,omitempty"`
}

const reviewThreadsQuery = `query($owner: String!, $repo: String!, $number: Int!, $after: String) {
  repository(owner: $owner, name: $repo) {
    pullRequest(number: $number) {
      reviewThreads(first: 100, after: $after) {
        pageInfo { hasNextPage endCursor }
        nodes {
          id isResolved isOutdated path line startLine originalLine originalStartLine diffSide
          comments(first: 100) {
            nodes { author { login } body createdAt url diffHunk commit { oid } originalCommit { oid } }
          }
        }
      }
    }
  }
}`

// ReviewThreads reads the review threads on a PR: everyone's comments on
// its lines, with whether each is resolved.
func ReviewThreads(org, repo string, number int) ([]ReviewThread, error) {
	var out []ReviewThread
	after := ""
	for range 10 { // a thousand threads is plenty
		args := []string{"api", "graphql", "-f", "query=" + reviewThreadsQuery, "-f", "owner=" + org, "-f", "repo=" + repo, "-F", "number=" + strconv.Itoa(number)}
		if after != "" {
			args = append(args, "-f", "after="+after)
		}
		stdOut, stdErr, err := ghExec(args...)
		if err != nil {
			return nil, fmt.Errorf("reading the threads on %s#%d: %s", repoSlug(org, repo), number, ghError(stdOut.Bytes(), stdErr.String(), err))
		}
		page, next, err := parseReviewThreads(stdOut.Bytes())
		if err != nil {
			return nil, err
		}
		out = append(out, page...)
		if next == "" {
			break
		}
		after = next
	}
	return out, nil
}

// parseReviewThreads reads a page of the threads query, and the cursor of
// the next ("" for none).
func parseReviewThreads(data []byte) ([]ReviewThread, string, error) {
	var res struct {
		Data struct {
			Repository struct {
				PullRequest struct {
					ReviewThreads struct {
						PageInfo struct {
							HasNextPage bool   `json:"hasNextPage"`
							EndCursor   string `json:"endCursor"`
						} `json:"pageInfo"`
						Nodes []struct {
							ID                string `json:"id"`
							IsResolved        bool   `json:"isResolved"`
							IsOutdated        bool   `json:"isOutdated"`
							Path              string `json:"path"`
							Line              int    `json:"line"`
							StartLine         int    `json:"startLine"`
							OriginalLine      int    `json:"originalLine"`
							OriginalStartLine int    `json:"originalStartLine"`
							DiffSide          string `json:"diffSide"`
							Comments          struct {
								Nodes []struct {
									Author struct {
										Login string `json:"login"`
									} `json:"author"`
									Body      string `json:"body"`
									CreatedAt string `json:"createdAt"`
									URL       string `json:"url"`
									DiffHunk  string `json:"diffHunk"`
									Commit    struct {
										OID string `json:"oid"`
									} `json:"commit"`
									OriginalCommit struct {
										OID string `json:"oid"`
									} `json:"originalCommit"`
								} `json:"nodes"`
							} `json:"comments"`
						} `json:"nodes"`
					} `json:"reviewThreads"`
				} `json:"pullRequest"`
			} `json:"repository"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &res); err != nil {
		return nil, "", fmt.Errorf("parsing review threads: %w", err)
	}
	rt := res.Data.Repository.PullRequest.ReviewThreads
	var out []ReviewThread
	for _, n := range rt.Nodes {
		t := ReviewThread{ID: n.ID, Resolved: n.IsResolved, Outdated: n.IsOutdated, Path: n.Path, Line: n.Line, StartLine: n.StartLine,
			OriginalLine: n.OriginalLine, OriginalStartLine: n.OriginalStartLine, Side: n.DiffSide, Comments: []ThreadComment{}}
		for _, c := range n.Comments.Nodes {
			t.Comments = append(t.Comments, ThreadComment{Author: c.Author.Login, Body: c.Body, At: c.CreatedAt, URL: c.URL, DiffHunk: c.DiffHunk,
				Commit: c.Commit.OID, OriginalCommit: c.OriginalCommit.OID})
		}
		out = append(out, t)
	}
	next := ""
	if rt.PageInfo.HasNextPage {
		next = rt.PageInfo.EndCursor
	}
	return out, next, nil
}

// ReplyToThread posts a reply in a review thread, as you. (With a review
// of yours pending on GitHub, GitHub adds it to that instead of posting.)
func ReplyToThread(threadID, body string) error {
	const q = `mutation($id: ID!, $body: String!) { addPullRequestReviewThreadReply(input: {pullRequestReviewThreadId: $id, body: $body}) { comment { id } } }`
	stdOut, stdErr, err := ghExec("api", "graphql", "-f", "query="+q, "-f", "id="+threadID, "-f", "body="+body)
	if err != nil {
		return fmt.Errorf("replying on GitHub: %s", ghError(stdOut.Bytes(), stdErr.String(), err))
	}
	return nil
}

// ResolveThread resolves a review thread, or reopens it.
func ResolveThread(threadID string, resolved bool) error {
	q := `mutation($id: ID!) { resolveReviewThread(input: {threadId: $id}) { thread { id } } }`
	if !resolved {
		q = `mutation($id: ID!) { unresolveReviewThread(input: {threadId: $id}) { thread { id } } }`
	}
	stdOut, stdErr, err := ghExec("api", "graphql", "-f", "query="+q, "-f", "id="+threadID)
	if err != nil {
		return fmt.Errorf("resolving on GitHub: %s", ghError(stdOut.Bytes(), stdErr.String(), err))
	}
	return nil
}

// Said is something said on a PR itself, not on its lines: a review (who
// approved, asked for changes or commented, and what they wrote with it),
// or a comment.
type Said struct {
	ID     string `json:"id"`
	Author string `json:"author"`
	Bot    bool   `json:"bot,omitempty"`
	// State is a review's: APPROVED, CHANGES_REQUESTED, COMMENTED or
	// DISMISSED. A comment has none.
	State string `json:"state,omitempty"`
	Body  string `json:"body,omitempty"`
	At    string `json:"at"`
	// Commit is the commit a review was of.
	Commit string `json:"commit,omitempty"`
	URL    string `json:"url,omitempty"`
}

const discussionQuery = `query($owner: String!, $repo: String!, $number: Int!) {
  repository(owner: $owner, name: $repo) {
    pullRequest(number: $number) {
      reviews(last: 100) { nodes { id author { __typename login } state body submittedAt url commit { oid } } }
      comments(last: 100) { nodes { id author { __typename login } body createdAt url } }
    }
  }
}`

// Discussion reads what's been said on a PR itself: its reviews and its
// comments, the latest hundred of each, oldest first. (GraphQL
// pullRequest.reviews and .comments; not tried on a PR with more.)
func Discussion(org, repo string, number int) ([]Said, error) {
	stdOut, stdErr, err := ghExec("api", "graphql", "-f", "query="+discussionQuery, "-f", "owner="+org, "-f", "repo="+repo, "-F", "number="+strconv.Itoa(number))
	if err != nil {
		return nil, fmt.Errorf("reading what's been said on %s#%d: %s", repoSlug(org, repo), number, ghError(stdOut.Bytes(), stdErr.String(), err))
	}
	return parseDiscussion(stdOut.Bytes())
}

func parseDiscussion(data []byte) ([]Said, error) {
	type author struct {
		Type  string `json:"__typename"`
		Login string `json:"login"`
	}
	var res struct {
		Data struct {
			Repository struct {
				PullRequest struct {
					Reviews struct {
						Nodes []struct {
							ID          string `json:"id"`
							Author      author `json:"author"`
							State       string `json:"state"`
							Body        string `json:"body"`
							SubmittedAt string `json:"submittedAt"`
							URL         string `json:"url"`
							Commit      struct {
								OID string `json:"oid"`
							} `json:"commit"`
						} `json:"nodes"`
					} `json:"reviews"`
					Comments struct {
						Nodes []struct {
							ID        string `json:"id"`
							Author    author `json:"author"`
							Body      string `json:"body"`
							CreatedAt string `json:"createdAt"`
							URL       string `json:"url"`
						} `json:"nodes"`
					} `json:"comments"`
				} `json:"pullRequest"`
			} `json:"repository"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &res); err != nil {
		return nil, fmt.Errorf("parsing a PR's reviews and comments: %w", err)
	}
	pr := res.Data.Repository.PullRequest
	out := []Said{}
	for _, n := range pr.Reviews.Nodes {
		// One not yet submitted (yours, pending) hasn't been said.
		if n.State == "PENDING" || n.SubmittedAt == "" {
			continue
		}
		out = append(out, Said{ID: n.ID, Author: n.Author.Login, Bot: n.Author.Type == "Bot", State: n.State, Body: n.Body, At: n.SubmittedAt, Commit: n.Commit.OID, URL: n.URL})
	}
	for _, n := range pr.Comments.Nodes {
		out = append(out, Said{ID: n.ID, Author: n.Author.Login, Bot: n.Author.Type == "Bot", Body: n.Body, At: n.CreatedAt, URL: n.URL})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].At < out[j].At })
	return out, nil
}

// Comment posts a comment on a PR itself, as you. (REST: POST
// /repos/{owner}/{repo}/issues/{number}/comments. Not tried on real
// GitHub.)
func Comment(org, repo string, number int, body string) error {
	stdOut, stdErr, err := ghExec("api", "--method", "POST", fmt.Sprintf("repos/%s/%s/issues/%d/comments", org, repo, number), "-f", "body="+body)
	if err != nil {
		return fmt.Errorf("commenting on GitHub: %s", ghError(stdOut.Bytes(), stdErr.String(), err))
	}
	return nil
}
