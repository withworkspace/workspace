// Package page keeps pages: documents you write in the web UI, before or
// beside any mission, with cells an agent fills in as you go. A page's
// text is a markdown file in .ws/pages (p1.md, p2.md, …), local to your
// machine as missions are; what it is beside its text (its kind, where
// it's filed, what it's tied to, its version) is in the workspace's
// records (records.go). Its cells are conversations of the workspace's
// (conversation.Cell), named in the file by the comments around what each
// wrote.
package page

import (
	"errors"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/doc"
)

// Page is a page, as a list shows it.
type Page struct {
	ID      string    `json:"id"`
	Title   string    `json:"title"`
	Updated time.Time `json:"updated"`
	// Kind is the kind of document it's written as ([documents.<kind>]),
	// if it was started as one; Filed, where it was filed, once it was;
	// Source, the file in a repo it's tied to, if it is.
	Kind   string  `json:"kind,omitempty"`
	Filed  *Filed  `json:"filed,omitempty"`
	Source *Source `json:"source,omitempty"`
	// To is where it's to be filed, when it's of no kind: a folder of a
	// repo (repo:folder/), its file's name made from its title.
	To string `json:"to,omitempty"`
}

// Store keeps a workspace's pages.
type Store struct {
	Root string
}

// Dir is where pages are kept.
func (s Store) Dir() string { return filepath.Join(s.Root, ".ws", "pages") }

// Path is a page's file.
func (s Store) Path(id string) string { return filepath.Join(s.Dir(), id+".md") }

var idShape = regexp.MustCompile(`^p([1-9][0-9]*)$`)

// IsRef reports whether ref names a page (p3).
func IsRef(ref string) bool { return idShape.MatchString(ref) }

// ErrNotFound is the answer for a page that isn't there.
var ErrNotFound = errors.New("not found")

// ErrChanged is the answer for saving a page that's changed since the
// version the edit started from: something else wrote it.
var ErrChanged = errors.New("the page has changed since you opened it")

// Untitled is what a page with nothing to name it by is called.
const Untitled = "Untitled"

var marks = strings.NewReplacer("**", "", "__", "", "`", "")

// Title is a page's name: its first line of words (a heading's, usually),
// without its markdown.
func Title(text []byte) string {
	s := string(text)
	if rest, ok := strings.CutPrefix(s, "---\n"); ok { // front matter isn't words
		if _, body, ok := strings.Cut(rest, "\n---\n"); ok {
			s = body
		}
	}
	for line := range strings.SplitSeq(s, "\n") {
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, "<!--") {
			continue
		}
		t = strings.TrimSpace(marks.Replace(strings.TrimLeft(t, "#>-*+ ")))
		if t == "" {
			continue
		}
		if r := []rune(t); len(r) > 80 {
			t = string(r[:79]) + "…"
		}
		return t
	}
	return Untitled
}

// Cell is a cell as a page's text has it: its conversation, how many
// versions have been written into it, what was asked for, and what it
// holds, between its two comments (Start to End in the text).
type Cell struct {
	ID     string
	V      int
	Prompt string
	Body   string
	Start  int
	End    int
	// Ask: it's a question (>?), answered beside the page, not in it.
	Ask bool
}

var (
	cellOpen  = regexp.MustCompile(`(?m)^<!--[ \t]*ws:cell[ \t]+([A-Za-z0-9-]+)[ \t]+v(\d+)([ \t]+ask)?[ \t]*(?:\|[ \t]*(.*?))?[ \t]*-->[ \t]*\r?$`)
	cellClose = regexp.MustCompile(`(?m)^<!--[ \t]*/ws:cell[ \t]*-->[ \t]*\r?$`)
)

// Cells are the cells in a page's text, in order, as the editor reads
// them: a comment that opens one, to the next that closes it. One that
// isn't closed before another opens isn't a cell.
func Cells(text []byte) []Cell {
	opens := cellOpen.FindAllSubmatchIndex(text, -1)
	var out []Cell
	for i, m := range opens {
		end := cellClose.FindIndex(text[m[1]:])
		if end == nil {
			break
		}
		closeAt, closeEnd := m[1]+end[0], m[1]+end[1]
		if i+1 < len(opens) && opens[i+1][0] < closeAt {
			continue
		}
		c := Cell{ID: string(text[m[2]:m[3]]), Body: strings.TrimSpace(string(text[m[1]:closeAt])), Start: m[0], End: closeEnd}
		c.V, _ = strconv.Atoi(string(text[m[4]:m[5]]))
		c.Ask = m[6] >= 0
		if m[8] >= 0 {
			c.Prompt = string(text[m[8]:m[9]])
		}
		out = append(out, c)
	}
	return out
}

// Above names what the cells before the cell id hold, and Around the page
// but for that cell: kept when the cell's written, they tell when what it
// was written beside has changed. Both are "" for a cell that isn't in the
// text.
func Above(text []byte, id string) string {
	var held strings.Builder
	for _, c := range Cells(text) {
		if c.ID == id {
			return doc.Version([]byte(held.String()))
		}
		if c.Body == "" {
			continue // nothing there to have been written beside (a question, say)
		}
		held.WriteString(c.ID + "\n" + c.Body + "\n\x00")
	}
	return ""
}

func Around(text []byte, id string) string {
	for _, c := range Cells(text) {
		if c.ID == id {
			return doc.Version(append(append([]byte{}, text[:c.Start]...), text[c.End:]...))
		}
	}
	return ""
}

func num(id string) int {
	n, _ := strconv.Atoi(strings.TrimPrefix(id, "p"))
	return n
}

// Meta is what's kept of a page beside its text, in its row: the kind of
// document it's written as, if it was started as one, where it was filed,
// once it was, the file it's tied to, and where it's to be filed.
type Meta struct {
	Kind   string  `json:"kind,omitempty"`
	Filed  *Filed  `json:"filed,omitempty"`
	Source *Source `json:"source,omitempty"`
	To     string  `json:"to,omitempty"`
}

// Source is the file in a repo a page is tied to: opened from it, or filed
// as it. Saving the page writes it there. Ref names it (repo:path, or
// repo/capsule:path); Version is the version of its text the page last
// agreed with (conversation.Version), kept with the workspace's versions:
// what's changed in the file since is brought into the page from there.
type Source struct {
	Ref     string    `json:"ref"`
	Version string    `json:"version"`
	At      time.Time `json:"at"`
}

// Filed is where a page was filed as a document: its ref (repo:path, or
// repo/capsule:path where it was left in a capsule), of what kind, the
// commit that added it, whether that landed, when, and the version of the
// page it was filed from.
type Filed struct {
	Ref     string    `json:"ref"`
	Kind    string    `json:"kind"`
	Commit  string    `json:"commit,omitempty"`
	Landed  bool      `json:"landed,omitempty"`
	At      time.Time `json:"at"`
	Version string    `json:"version"`
}

// AsText is a page as it reads, with nothing of ws's in it: each cell's
// words kept as text, without the comments around them, and questions
// (answered beside the page, not in it) and cells with nothing in them
// gone.
func AsText(text []byte) []byte {
	out := string(text)
	cells := Cells(text)
	for _, c := range slices.Backward(cells) {

		before, after := out[:c.Start], out[c.End:]
		if !c.Ask && c.Body != "" {
			out = before + c.Body + after
			continue
		}
		// Gone, with the blank lines around it: one between what's left.
		before, after = strings.TrimRight(before, "\n"), strings.TrimLeft(after, "\n")
		switch {
		case before == "":
			out = after
		case after == "":
			out = before + "\n"
		default:
			out = before + "\n\n" + after
		}
	}
	return []byte(out)
}
