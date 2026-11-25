package pulls

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"

	"github.com/withworkspace/workspace/internal/github"
)

// What's been said on a PR itself, on GitHub, as the page's overview shows
// it: who approved, asked for changes or commented and what they wrote
// with it, and the comments that aren't on lines. Comments on lines are
// threads (threads.go). A comment made from here is shown at once, and
// sent behind (unsent.go).

// What a thing said is.
const (
	SaidComment   = "comment"
	SaidApproved  = "approved"
	SaidChanges   = "changes"
	SaidCommented = "commented" // a review with words and no verdict
	SaidDismissed = "dismissed"
)

// Said is one thing said on a PR: a review or a comment.
type Said struct {
	ID   string `json:"id"`
	By   string `json:"by"`
	Mine bool   `json:"mine,omitempty"`
	Bot  bool   `json:"bot,omitempty"`
	Kind string `json:"kind"`
	Body string `json:"body,omitempty"`
	At   string `json:"at"`
	URL  string `json:"url,omitempty"`
	// Stale: a review of commits the PR has moved on from.
	Stale bool `json:"stale,omitempty"`
	// Sending: a comment made from here that's on its way to GitHub.
	Sending bool `json:"sending,omitempty"`
}

// Reviewer is someone else's latest verdict on a PR.
type Reviewer struct {
	Login string `json:"login"`
	Kind  string `json:"kind"` // approved, changes, dismissed
	At    string `json:"at"`
	Stale bool   `json:"stale,omitempty"`
}

// Discussion is what's been said on a PR, oldest first, and where each
// reviewer other than you stands.
type Discussion struct {
	Items     []Said     `json:"items"`
	Reviewers []Reviewer `json:"reviewers"`
}

var saidKinds = map[string]string{"APPROVED": SaidApproved, "CHANGES_REQUESTED": SaidChanges, "COMMENTED": SaidCommented, "DISMISSED": SaidDismissed}

var htmlComment = regexp.MustCompile(`(?s)<!--.*?-->[ \t]*\n?`)

// Readable is a description or a comment as it's read: without the HTML
// comments a template leaves in it.
func Readable(body string) string {
	return strings.TrimSpace(htmlComment.ReplaceAllString(body, ""))
}

// BodyID names a description by what it says, so the page knows when one
// you've read has been edited.
func BodyID(body string) string {
	if body == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(body))
	return hex.EncodeToString(sum[:])[:12]
}

// Discussion reads what's been said on a PR from GitHub.
func (s *Service) Discussion(repo string, number int) (*Discussion, error) {
	pr, _, err := s.Open(repo, number)
	if err != nil {
		return nil, err
	}
	said, err := s.GitHub.Discussion(s.owner(repo), s.name(repo), number)
	if err != nil {
		return nil, err
	}
	return discussion(said, s.me(), pr.Head), nil
}

func discussion(said []github.Said, me, head string) *Discussion {
	out := &Discussion{Items: []Said{}, Reviewers: []Reviewer{}}
	latest := map[string]int{} // a reviewer's place in out.Reviewers
	for _, g := range said {
		it := Said{ID: g.ID, By: g.Author, Mine: sameLogin(g.Author, me), Bot: g.Bot, Kind: SaidComment, Body: Readable(g.Body), At: g.At, URL: g.URL}
		if g.State != "" {
			it.Kind = saidKinds[g.State]
			if it.Kind == "" {
				continue
			}
			it.Stale = g.Commit != "" && head != "" && g.Commit != head
			// A review with no verdict and no words is only what holds its
			// comments on lines, which are threads.
			if it.Kind == SaidCommented && it.Body == "" {
				continue
			}
			if it.Kind != SaidCommented && !it.Mine && g.Author != "" {
				r := Reviewer{Login: g.Author, Kind: it.Kind, At: g.At, Stale: it.Stale}
				if i, ok := latest[strings.ToLower(g.Author)]; ok {
					out.Reviewers[i] = r
				} else {
					latest[strings.ToLower(g.Author)] = len(out.Reviewers)
					out.Reviewers = append(out.Reviewers, r)
				}
			}
		} else if it.Body == "" {
			continue
		}
		out.Items = append(out.Items, it)
	}
	return out
}

// Comment says something on a PR itself, as you, and is what's been said
// on it with that: shown at once and sent behind, or, with nowhere to
// send from, posted and read again.
func (s *Service) Comment(repo string, number int, body string) (*Discussion, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return nil, fmt.Errorf("say something")
	}
	behind, err := s.say(sendComment, "your comment on "+Key(repo, number), outgoing{Repo: repo, Number: number, Body: body})
	if err != nil {
		return nil, err
	}
	if behind {
		return s.KeptDiscussion(repo, number)
	}
	return s.DiscussionNow(repo, number)
}
