package pulls

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/github"
	"github.com/withworkspace/workspace/internal/review"
)

// GitHub's review threads on a PR, read as a review's comments are, so
// the page shows them on the lines they're on, with what's been said
// since, and you reply and resolve from there. Replies and resolving post
// at once: they're not part of a draft.

// ThreadsID is the id of the review the threads are shown as.
const ThreadsID = "github"

var suggestionBlock = regexp.MustCompile("(?s)```suggestion[^\n]*\n(.*?)\n?```\n?")

// Threads reads everyone's comments on a PR's lines from GitHub, as one
// review's comments: each thread's first comment, its replies as the
// comment's thread, resolved or open.
func (s *Service) Threads(repo string, number int) (*review.Review, error) {
	pr, st, err := s.Open(repo, number)
	if err != nil {
		return nil, err
	}
	l, err := s.layer(pr, st)
	if err != nil {
		return nil, err
	}
	threads, err := s.GitHub.ReviewThreads(s.Org, repo, number)
	if err != nil {
		return nil, err
	}
	return threadsReview(repo, number, threads, l, s.me()), nil
}

// me is who you are on GitHub, as the list last said: "" if it's never
// been asked.
func (s *Service) me() string {
	if list := s.Peek(); list != nil {
		return list.Me
	}
	return ""
}

// threadsReview is GitHub's threads on a PR as one review's comments.
func threadsReview(repo string, number int, threads []github.ReviewThread, l *layer, me string) *review.Review {
	out := &review.Review{ID: ThreadsID, Capsule: Key(repo, number), BaseRef: l.baseRef, Base: l.base, Head: l.head, State: review.Submitted, Comments: []review.Note{}}
	for _, t := range threads {
		if len(t.Comments) == 0 {
			continue
		}
		out.Comments = append(out.Comments, threadNote(t, l, me))
	}
	return out
}

// threadNote is a GitHub thread as a comment on lines. One whose lines
// have changed since is on the lines it was made on, at that commit.
func threadNote(t github.ReviewThread, l *layer, me string) review.Note {
	first := t.Comments[0]
	n := review.Note{ID: t.ID, Path: t.Path, Side: "new", Status: review.Open, URL: first.URL, Thread: []review.Reply{}}
	if !sameLogin(first.Author, me) {
		n.By = first.Author
	}
	if t.Side == "LEFT" {
		n.Side = "old"
	}
	if t.Resolved {
		n.Status = review.Resolved
	}
	switch {
	case !t.Outdated && t.Line > 0:
		n.Line, n.EndLine = t.Line, t.Line
		if t.StartLine > 0 && t.StartLine < t.Line {
			n.Line = t.StartLine
		}
		n.Commit = l.head
		if n.Side == "old" {
			n.Commit = l.base
		}
	default:
		n.Line, n.EndLine = t.OriginalLine, t.OriginalLine
		if t.OriginalStartLine > 0 && t.OriginalStartLine < t.OriginalLine {
			n.Line = t.OriginalStartLine
		}
		n.Commit = first.OriginalCommit
	}
	if n.Line < 1 {
		n.Line, n.EndLine = 1, 1
	}
	n.Excerpt = hunkExcerpt(first.DiffHunk, n.Side, n.EndLine-n.Line+1)
	if n.Excerpt == nil {
		// GitHub's hunk wasn't given: read the lines, if they're here.
		if lines, err := review.Blob(l.dir, n.Commit, n.Path); err == nil && n.EndLine <= len(lines) {
			n.Excerpt = lines[n.Line-1 : n.EndLine]
		} else {
			n.Excerpt = []string{}
		}
	}
	body := first.Body
	if m := suggestionBlock.FindStringSubmatch(body); m != nil {
		sug := strings.ReplaceAll(m[1], "\r\n", "\n")
		n.Suggestion = &sug
		body = strings.Replace(body, m[0], "", 1)
	}
	n.Kind, n.Body = ParseKind(strings.TrimSpace(body))
	n.Body = strings.TrimSpace(n.Body)
	for _, c := range t.Comments[1:] {
		at, _ := time.Parse(time.RFC3339, c.At)
		by := c.Author
		if sameLogin(by, me) {
			by = "you"
		}
		n.Thread = append(n.Thread, review.Reply{By: by, At: at, Body: c.Body})
	}
	return n
}

// hunkExcerpt is the last n lines of a side of a diff hunk as GitHub
// gives it with a comment: the lines the comment is on.
func hunkExcerpt(hunk, side string, n int) []string {
	if hunk == "" || n < 1 {
		return nil
	}
	var lines []string
	for _, l := range strings.Split(strings.TrimRight(hunk, "\n"), "\n") {
		switch {
		case strings.HasPrefix(l, "@@"), strings.HasPrefix(l, `\`):
		case strings.HasPrefix(l, "+"):
			if side == "new" {
				lines = append(lines, l[1:])
			}
		case strings.HasPrefix(l, "-"):
			if side == "old" {
				lines = append(lines, l[1:])
			}
		default:
			lines = append(lines, strings.TrimPrefix(l, " "))
		}
	}
	if len(lines) == 0 {
		return nil
	}
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return lines
}

// Respond is you on a GitHub thread: a reply, or resolving or reopening
// it. It posts at once.
func (s *Service) Respond(repo string, number int, threadID string, what review.Response, body string) error {
	if _, _, err := s.Open(repo, number); err != nil {
		return err
	}
	switch what {
	case review.Answer:
		if strings.TrimSpace(body) == "" {
			return fmt.Errorf("say something")
		}
		return s.GitHub.ReplyToThread(s.Org, repo, number, threadID, strings.TrimSpace(body))
	case review.Resolve, review.Reopen:
		return s.GitHub.ResolveThread(s.Org, repo, number, threadID, what == review.Resolve)
	}
	return fmt.Errorf("can't %q a thread", what)
}
