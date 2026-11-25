package review

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/store"
)

// A review is one pass over a mission capsule's changes: comments anchored
// to lines, then a verdict. It's a draft (yours alone) until submitted;
// then crew read it (ws review show) and the orchestrator decides what
// follows. Reviews are kept in the workspace's records (records.go); a
// submitted one of a mission's is also rendered for agents as
// mission:reviews/<id>.md.

// Review states, verdicts and comment kinds.
const (
	Draft     = "draft"
	Submitted = "submitted"

	Approve = "approve"
	Changes = "changes"
	Comment = "comment"
)

// Kinds a comment can be, in the order they matter.
var Kinds = []string{"blocking", "suggestion", "question", "nit"}

// Verdicts a review can end with.
var Verdicts = []string{Approve, Changes, Comment}

// Review is one review of a capsule.
type Review struct {
	ID        string     `json:"id"`      // r1, r2, … per mission
	Capsule   string     `json:"capsule"` // repo/name
	BaseRef   string     `json:"baseRef"`
	Base      string     `json:"base"`
	Head      string     `json:"head"` // the commit reviewed
	State     string     `json:"state"`
	Verdict   string     `json:"verdict,omitempty"`
	Summary   string     `json:"summary,omitempty"`
	Created   time.Time  `json:"created"`
	Submitted *time.Time `json:"submitted,omitempty"`
	Comments  []Note     `json:"comments"`
}

// Note is a comment on lines of the diff. (Comment is the verdict.)
type Note struct {
	ID   string `json:"id"` // r3.c1
	Path string `json:"path"`
	// Side is "new" for lines as they are at the reviewed commit, "old" for
	// lines as they were (deleted ones).
	Side    string `json:"side"`
	Line    int    `json:"line"`
	EndLine int    `json:"endLine"`
	Commit  string `json:"commit"` // the commit Side's lines are from
	// Excerpt is the lines as they were when commented on, so the comment
	// reads the same however the code moves after.
	Excerpt []string `json:"excerpt"`
	Kind    string   `json:"kind"`
	Body    string   `json:"body"`
	// By is who wrote it, when that isn't you: "agent" for a comment an
	// agent proposed that you added to your draft, a login for someone
	// else's comment on a pull request. URL is where it is on GitHub.
	By  string `json:"by,omitempty"`
	URL string `json:"url,omitempty"`
	// Suggestion replaces the lines, when set ("" suggests deleting them).
	Suggestion *string `json:"suggestion,omitempty"`
	Status     string  `json:"status"` // open, addressed, wontfix, resolved
	Thread     []Reply `json:"thread"`
	// Sending: its status was changed here (a pull request's thread,
	// resolved or opened again) and GitHub hasn't been told yet.
	Sending bool `json:"sending,omitempty"`
}

// Reply is a message in a comment's thread.
type Reply struct {
	By     string    `json:"by"`
	At     time.Time `json:"at"`
	Body   string    `json:"body"`
	Commit string    `json:"commit,omitempty"`
	Status string    `json:"status,omitempty"`
	// Sending: it was said here, in a pull request's thread, and is on its
	// way to GitHub.
	Sending bool `json:"sending,omitempty"`
}

// Counts says how many comments of each kind a review has.
func (r *Review) Counts() map[string]int {
	c := map[string]int{}
	for _, n := range r.Comments {
		c[n.Kind]++
	}
	return c
}

// Store keeps reviews: one mission's, of its capsules, or, rooted in a
// folder of its own (Dir), those of something outside a mission: a pull
// request.
type Store struct {
	Missions mission.Store
	Slug     string
	// Dir, when set, is the folder of what's kept of what the reviews are
	// of, instead of a mission: there's none to render them for or log them
	// to. Name is what they're of (api#89): their scope in the records.
	Dir  string
	Name string
}

// For is the review store of a mission.
func For(root, slug string) Store { return Store{Missions: mission.Store{Root: root}, Slug: slug} }

// At is the review store of name (a pull request, api#89) in the workspace
// at root, with no mission. dir is the folder of what's kept of it.
func At(root, dir, name string) Store {
	return Store{Missions: mission.Store{Root: root}, Dir: dir, Name: name}
}

// ofMission is whether the reviews are a mission's.
func (s Store) ofMission() bool { return s.Dir == "" }

// whose names what the reviews are of, for errors.
func (s Store) whose() string {
	if s.ofMission() {
		return "mission " + s.Slug
	}
	return s.Name
}

// RenderedRef is where a submitted review is kept for agents, as an
// artifact reference.
func RenderedRef(id string) string { return "mission:reviews/" + id + ".md" }

func (s Store) renderedPath(id string) string {
	return filepath.Join(s.Missions.FilesDir(s.Slug), "reviews", id+".md")
}

var idShape = regexp.MustCompile(`^r([1-9][0-9]*)$`)

// ErrNotFound is the answer for a review, draft or comment that isn't there.
var ErrNotFound = errors.New("not found")

// Load reads one review.
func (s Store) Load(id string) (*Review, error) {
	if !idShape.MatchString(id) {
		return nil, fmt.Errorf("%w: no review %q (they're named r1, r2, …)", ErrNotFound, id)
	}
	var r *Review
	err := s.read(func(tx *store.Tx) error {
		var err error
		r, err = s.loadIn(tx, id)
		return err
	})
	return r, err
}

// List reads the reviews, oldest first.
func (s Store) List() ([]*Review, error) { return s.list(`1`) }

func num(id string) int {
	n, _ := strconv.Atoi(strings.TrimPrefix(id, "r"))
	return n
}

// Draft is the capsule's draft review, or nil.
func (s Store) Draft(capsule string) (*Review, error) {
	var r *Review
	err := s.read(func(tx *store.Tx) error {
		var err error
		r, err = s.draftIn(tx, capsule)
		return err
	})
	return r, err
}

// Latest is the most recently submitted review, of capsule if it's given,
// or nil.
func (s Store) Latest(capsule string) (*Review, error) {
	q, args := `SELECT id FROM reviews WHERE scope = ? AND state = ?`, []any{s.scope(), Submitted}
	if capsule != "" {
		q, args = q+` AND capsule = ?`, append(args, capsule)
	}
	var r *Review
	err := s.read(func(tx *store.Tx) error {
		var id string
		err := tx.QueryRow(q+` ORDER BY submitted DESC, n DESC LIMIT 1`, args...).Scan(&id)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		r, err = s.loadIn(tx, id)
		return err
	})
	return r, err
}

func writeFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	_, werr := tmp.Write(data)
	if cerr := tmp.Close(); werr == nil {
		werr = cerr
	}
	if werr == nil {
		werr = os.Chmod(tmp.Name(), 0o644)
	}
	if werr == nil {
		werr = os.Rename(tmp.Name(), path)
	}
	if werr != nil {
		os.Remove(tmp.Name())
	}
	return werr
}

// render writes a submitted review of a mission's for agents to read, once
// the transaction that kept it has committed.
func (s Store) render(tx *store.Tx, r *Review) {
	if !s.ofMission() {
		return
	}
	text := Render(r)
	path := s.renderedPath(r.ID)
	tx.OnCommit(func() { writeFile(path, []byte(text)) })
}

// NewNote is a comment to add to a capsule's draft.
type NewNote struct {
	Path       string
	Side       string
	Line       int
	EndLine    int
	Kind       string
	Body       string
	Suggestion *string
	// By is who wrote it, when that isn't you: "agent", for a comment an
	// agent proposed and you added.
	By string
}

// Diffed is the diff the comment was made on: what a new draft is of, and
// what the comment's lines are read from.
type Diffed struct {
	Dir, BaseRef, Base, Head string
}

func validKind(k string) bool {
	for _, v := range Kinds {
		if k == v {
			return true
		}
	}
	return false
}

// AddNote adds a comment to the capsule's draft, starting one if there's
// none. Its excerpt is read from git: the new side at d.Head, the old at
// d.Base. One with no path is on the change as a whole, and has no lines.
func (s Store) AddNote(capsule string, d Diffed, n NewNote, at time.Time) (*Review, error) {
	if n.Path == "" {
		return s.addGeneral(capsule, d, n, at)
	}
	if n.Side != "new" && n.Side != "old" {
		return nil, fmt.Errorf("side is new or old, not %q", n.Side)
	}
	if n.EndLine == 0 {
		n.EndLine = n.Line
	}
	if n.Line < 1 || n.EndLine < n.Line {
		return nil, fmt.Errorf("lines %d–%d aren't a range", n.Line, n.EndLine)
	}
	if !validKind(n.Kind) {
		return nil, fmt.Errorf("a comment is %s, not %q", strings.Join(Kinds, ", "), n.Kind)
	}
	if strings.TrimSpace(n.Body) == "" && n.Suggestion == nil {
		return nil, errors.New("say something, or suggest a change")
	}
	commit := d.Head
	if n.Side == "old" {
		commit = d.Base
	}
	lines, err := Blob(d.Dir, commit, n.Path)
	if err != nil {
		return nil, err
	}
	if n.EndLine > len(lines) {
		return nil, fmt.Errorf("%s has %d lines at %s, not %d", n.Path, len(lines), short(commit), n.EndLine)
	}
	var r *Review
	err = s.write(func(tx *store.Tx) error {
		if r, err = s.draftIn(tx, capsule); err != nil {
			return err
		}
		if r == nil {
			if r, err = s.newDraft(tx, capsule, d, at); err != nil {
				return err
			}
		}
		r.Base, r.Head, r.BaseRef = d.Base, d.Head, d.BaseRef
		r.Comments = append(r.Comments, Note{
			ID: nextNoteID(r), Path: n.Path, Side: n.Side, Line: n.Line, EndLine: n.EndLine,
			Commit: commit, Excerpt: lines[n.Line-1 : n.EndLine], Kind: n.Kind, Body: strings.TrimSpace(n.Body), By: n.By,
			Suggestion: n.Suggestion, Status: "open", Thread: []Reply{},
		})
		return s.saveIn(tx, r)
	})
	return r, err
}

// addGeneral adds a comment on the change as a whole to the draft: what
// a line couldn't say (it should be two changes, it's the wrong place to
// fix this).
func (s Store) addGeneral(capsule string, d Diffed, n NewNote, at time.Time) (*Review, error) {
	if !validKind(n.Kind) {
		return nil, fmt.Errorf("a comment is %s, not %q", strings.Join(Kinds, ", "), n.Kind)
	}
	if strings.TrimSpace(n.Body) == "" {
		return nil, errors.New("say something")
	}
	var r *Review
	err := s.write(func(tx *store.Tx) (err error) {
		if r, err = s.draftIn(tx, capsule); err != nil {
			return err
		}
		if r == nil {
			if r, err = s.newDraft(tx, capsule, d, at); err != nil {
				return err
			}
		}
		r.Base, r.Head, r.BaseRef = d.Base, d.Head, d.BaseRef
		r.Comments = append(r.Comments, Note{ID: nextNoteID(r), Commit: d.Head, Excerpt: []string{}, Kind: n.Kind, Body: strings.TrimSpace(n.Body), By: n.By, Status: "open", Thread: []Reply{}})
		return s.saveIn(tx, r)
	})
	return r, err
}

// General is whether a comment is on the change as a whole, not on lines.
func (n Note) General() bool { return n.Path == "" }

func (s Store) newDraft(tx *store.Tx, capsule string, d Diffed, at time.Time) (*Review, error) {
	return s.newReview(tx, &Review{Capsule: capsule, BaseRef: d.BaseRef, Base: d.Base, Head: d.Head}, at)
}

// newReview makes r a new draft: the next id, no comments yet.
func (s Store) newReview(tx *store.Tx, r *Review, at time.Time) (*Review, error) {
	next, err := s.nextIn(tx)
	if err != nil {
		return nil, err
	}
	r.ID, r.State, r.Created, r.Comments = "r"+strconv.Itoa(next), Draft, at, []Note{}
	return r, nil
}

// nextNoteID is the id for a new comment on r, not reusing one still in use.
func nextNoteID(r *Review) string {
	next := 1
	for _, c := range r.Comments {
		if k, err := strconv.Atoi(strings.TrimPrefix(c.ID, r.ID+".c")); err == nil && k >= next {
			next = k + 1
		}
	}
	return fmt.Sprintf("%s.c%d", r.ID, next)
}

// draftNote finds a comment in the capsule's draft, to change it.
func (s Store) draftNote(capsule, id string, fn func(r *Review, i int) error) (*Review, error) {
	var r *Review
	err := s.write(func(tx *store.Tx) error {
		var err error
		if r, err = s.draftIn(tx, capsule); err != nil {
			return err
		}
		if r == nil {
			return fmt.Errorf("%w: %s has no draft review", ErrNotFound, capsule)
		}
		for i := range r.Comments {
			if r.Comments[i].ID == id {
				if err := fn(r, i); err != nil {
					return err
				}
				return s.saveIn(tx, r)
			}
		}
		return fmt.Errorf("%w: the draft has no comment %s", ErrNotFound, id)
	})
	return r, err
}

// EditNote changes a draft comment's kind, words and suggestion.
func (s Store) EditNote(capsule, id, kind, body string, suggestion *string) (*Review, error) {
	if !validKind(kind) {
		return nil, fmt.Errorf("a comment is %s, not %q", strings.Join(Kinds, ", "), kind)
	}
	if strings.TrimSpace(body) == "" && suggestion == nil {
		return nil, errors.New("say something, or suggest a change")
	}
	return s.draftNote(capsule, id, func(r *Review, i int) error {
		r.Comments[i].Kind, r.Comments[i].Body, r.Comments[i].Suggestion = kind, strings.TrimSpace(body), suggestion
		return nil
	})
}

// DeleteNote removes a draft comment.
func (s Store) DeleteNote(capsule, id string) (*Review, error) {
	return s.draftNote(capsule, id, func(r *Review, i int) error {
		r.Comments = append(r.Comments[:i], r.Comments[i+1:]...)
		return nil
	})
}

// Discard throws the capsule's draft away.
func (s Store) Discard(capsule string) error {
	return s.write(func(tx *store.Tx) error {
		r, err := s.draftIn(tx, capsule)
		if err != nil || r == nil {
			return err
		}
		if _, err := tx.Exec(`DELETE FROM reviews WHERE scope = ? AND id = ?`, s.scope(), r.ID); err != nil {
			return err
		}
		s.changed(tx)
		return tx.Forget(s.origin(r.ID))
	})
}

// Unsubmit makes a submitted review its draft again, with its comments
// and what was written with it: for one that was to be posted somewhere
// and wasn't (a pull request's, which GitHub didn't take). It's refused
// while there's a newer draft, as there's one at a time, and for a
// mission's, which is logged and rendered once it's submitted.
func (s Store) Unsubmit(id string) (*Review, error) {
	if s.ofMission() {
		return nil, errors.New("a mission's review isn't taken back once it's submitted")
	}
	var r *Review
	err := s.write(func(tx *store.Tx) error {
		var err error
		if r, err = s.loadIn(tx, id); err != nil || r.State == Draft {
			return err
		}
		draft, err := s.draftIn(tx, r.Capsule)
		if err != nil {
			return err
		}
		if draft != nil {
			return fmt.Errorf("there's a newer draft review of %s: post or discard that one first", r.Capsule)
		}
		r.State, r.Verdict, r.Submitted = Draft, "", nil
		return s.saveIn(tx, r)
	})
	return r, err
}

// Submit ends the capsule's draft with a verdict, at the commit reviewed:
// it's kept, rendered for agents as an artifact, and logged. An approval
// needs no comments; anything else does, or a summary. A capsule with no
// draft can be approved: the review is made then.
func (s Store) Submit(capsule string, d Diffed, verdict, summary string, at time.Time) (*Review, error) {
	return s.submit(capsule, verdict, summary, at,
		func(tx *store.Tx) (*Review, error) { return s.newDraft(tx, capsule, d, at) },
		func(r *Review) error {
			r.Base, r.Head, r.BaseRef = d.Base, d.Head, d.BaseRef
			return nil
		})
}

// submit ends the capsule's draft (made by start if there's none) with a
// verdict, once at has set what it's of.
func (s Store) submit(capsule, verdict, summary string, at time.Time, start func(tx *store.Tx) (*Review, error), set func(r *Review) error) (*Review, error) {
	ok := false
	for _, v := range Verdicts {
		ok = ok || v == verdict
	}
	if !ok {
		return nil, fmt.Errorf("a review ends with %s, not %q", strings.Join(Verdicts, ", "), verdict)
	}
	var r *Review
	err := s.write(func(tx *store.Tx) error {
		var err error
		if r, err = s.draftIn(tx, capsule); err != nil {
			return err
		}
		if r == nil {
			if r, err = start(tx); err != nil {
				return err
			}
		}
		if verdict != Approve && len(r.Comments) == 0 && strings.TrimSpace(summary) == "" {
			return errors.New("say what needs changing: comment on the lines, or write a summary")
		}
		if err := set(r); err != nil {
			return err
		}
		r.State, r.Verdict, r.Summary, r.Submitted = Submitted, verdict, strings.TrimSpace(summary), &at
		if err := s.saveIn(tx, r); err != nil {
			return err
		}
		s.render(tx, r)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if !s.ofMission() { // no mission to record it with
		return r, nil
	}
	// The artifact is recorded without its own log line: the review's says it.
	if _, err := s.Missions.Update(s.Slug, func(m *mission.Mission) error {
		m.Artifacts = append(m.Artifacts, mission.Artifact{Ref: RenderedRef(r.ID), Kind: "review", Label: Title(r), Added: at})
		return nil
	}); err != nil {
		return r, err
	}
	repo, name, _ := strings.Cut(r.Capsule, "/")
	return r, s.Missions.AppendLog(s.Slug, mission.Event{Time: at, Kind: mission.EventReview, Repo: repo, Capsule: name, Text: r.ID + ": " + verdictWords[r.Verdict] + countWords(r)})
}

var verdictWords = map[string]string{Approve: "approved", Changes: "changes requested", Comment: "comments"}

// Title is a review's one-line name.
func Title(r *Review) string {
	return fmt.Sprintf("Review %s of %s: %s", r.ID, r.Capsule, verdictWords[r.Verdict])
}

// countWords is " (2 blocking, 1 nit)", or "" for no comments.
func countWords(r *Review) string {
	c := r.Counts()
	var parts []string
	for _, k := range Kinds {
		if c[k] > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", c[k], k))
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return " (" + strings.Join(parts, ", ") + ")"
}

// Viewed is which files of a capsule you've marked viewed, by their blob
// at the time: a file that changes again isn't viewed any more.
func (s Store) Viewed(capsule string) (map[string]string, error) {
	out := map[string]string{}
	err := s.read(func(tx *store.Tx) error {
		var err error
		out, err = s.viewedIn(tx, capsule)
		return err
	})
	return out, err
}

func (s Store) viewedIn(tx *store.Tx, capsule string) (map[string]string, error) {
	out := map[string]string{}
	rows, err := tx.Query(`SELECT path, blob FROM viewed WHERE scope = ? AND capsule = ?`, s.scope(), capsule)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var p, b string
		if err := rows.Scan(&p, &b); err != nil {
			return nil, err
		}
		out[p] = b
	}
	return out, rows.Err()
}

// SetViewed marks a file of the capsule viewed at blob, or not.
func (s Store) SetViewed(capsule, path, blob string, viewed bool) (map[string]string, error) {
	var out map[string]string
	err := s.write(func(tx *store.Tx) error {
		var err error
		if viewed {
			_, err = tx.Exec(`INSERT INTO viewed (scope, capsule, path, blob) VALUES (?, ?, ?, ?)
				ON CONFLICT (scope, capsule, path) DO UPDATE SET blob = excluded.blob`, s.scope(), capsule, path, blob)
		} else {
			_, err = tx.Exec(`DELETE FROM viewed WHERE scope = ? AND capsule = ? AND path = ?`, s.scope(), capsule, path)
		}
		if err != nil {
			return err
		}
		s.changed(tx)
		out, err = s.viewedIn(tx, capsule)
		return err
	})
	return out, err
}
