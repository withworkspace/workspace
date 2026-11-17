package workspace

import (
	"cmp"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/withworkspace/workspace/internal/git"
	"github.com/withworkspace/workspace/internal/slug"
	"github.com/withworkspace/workspace/internal/words"
)

// Kinds of document: what a workspace says of its documents
// ([documents.<name>] in ws.toml). Each kind is a place and a name (where
// one goes, and what it's called there), the first lines of the commits
// that add and change one, and the file that says how one's written. A
// document is of a kind when its path fits the kind's.

// Kind is a kind of document the workspace has.
type Kind struct {
	Name    string   // its name in ws.toml: bp
	Title   string   // what one's called: Build proposal
	Repo    string   // the repo they're in, by its canonical name
	Pattern string   // where in it, with {date}, {slug} and {number}: bp/proposed/{date}-{slug}.md
	Add     string   // the first line of the commit that adds one, if it says
	Amend   string   // the first line of one that changes one, if it says
	Guide   string   // the file that says how one's written, as configured (see KindGuide)
	Passes  []string // the workspace's own passes that are this kind's: tabs on its pages only
	Moves   []Move   // the steps in one's life, by name: each to another folder

	re     *regexp.Regexp
	fields []string // what each of re's groups is
	places []place  // where one can be: its path, then each move's folder with the same name
}

// Move is a step in a kind of document's life: the folder one goes to (To,
// in the kind's repo), the first line of the commit that moves it, and
// what that commit asks for its body.
type Move struct {
	Name, To, Commit, Asks string
}

// place is somewhere a kind's documents can be, and where one there
// stands: its path's folder's name (proposed), or a move's (accepted).
type place struct {
	state  string
	re     *regexp.Regexp
	fields []string
}

// DocFields is what a document's path and text say of it: what a kind's
// path and messages are filled in from.
type DocFields struct {
	Date   string // 2026-11-11
	Slug   string // from its title
	Number string // as it's written in its path: 0038
	Title  string // its first heading
}

var placeholder = regexp.MustCompile(`\{(date|slug|number|title)\}`)

// NewKind checks a kind as the workspace says it, and readies it to tell
// its documents by their paths.
func NewKind(k Kind) (*Kind, error) {
	if k.Title == "" {
		k.Title = k.Name
	}
	p := k.Pattern
	switch {
	case p == "":
		return nil, fmt.Errorf("it has no path: path = \"%s:folder/{date}-{slug}.md\", say", k.Repo)
	case strings.HasPrefix(p, "/") || slices.Contains(strings.Split(p, "/"), ".."):
		return nil, fmt.Errorf("its path %q leads out of %s", p, k.Repo)
	case !isMarkdownPath(p):
		return nil, fmt.Errorf("its path %q isn't a markdown file's: a document's ends .md", p)
	}
	re, fields, err := compilePath(p)
	if err != nil {
		return nil, err
	}
	if !slices.Contains(fields, "slug") && !slices.Contains(fields, "number") {
		return nil, fmt.Errorf("its path %q has neither {slug} nor {number}: every one would be the same file", p)
	}
	k.re, k.fields = re, fields
	k.places = []place{{state: folderName(path.Dir(p)), re: re, fields: fields}}
	// Each move is the same name in another folder.
	for _, m := range k.Moves {
		to := strings.Trim(m.To, "/")
		switch {
		case to == "":
			return nil, fmt.Errorf("its move %s has no folder: to = \"%s/\", say", m.Name, path.Dir(p))
		case strings.HasPrefix(m.To, "/") || slices.Contains(strings.Split(to, "/"), ".."):
			return nil, fmt.Errorf("its move %s's folder %q leads out of %s", m.Name, m.To, k.Repo)
		}
		mre, mfields, err := compilePath(to + "/" + path.Base(p))
		if err != nil {
			return nil, fmt.Errorf("its move %s: %w", m.Name, err)
		}
		k.places = append(k.places, place{state: cmp.Or(folderName(to), m.Name), re: mre, fields: mfields})
	}
	return &k, nil
}

// compilePath is a kind's path as a pattern, and what each of its groups
// is.
func compilePath(p string) (*regexp.Regexp, []string, error) {
	var re strings.Builder
	var fields []string
	re.WriteString("^")
	last := 0
	for _, m := range placeholder.FindAllStringSubmatchIndex(p, -1) {
		re.WriteString(regexp.QuoteMeta(p[last:m[0]]))
		last = m[1]
		name := p[m[2]:m[3]]
		if slices.Contains(fields, name) {
			return nil, nil, fmt.Errorf("its path has {%s} twice", name)
		}
		switch name {
		case "date":
			re.WriteString(`(\d{4}-\d{2}-\d{2})`)
		case "slug":
			re.WriteString(`([^/]+?)`)
		case "number":
			re.WriteString(`(\d+)`)
		case "title":
			return nil, nil, fmt.Errorf("its path has {title}, which is for its messages: a path has {slug}, made from the title")
		}
		fields = append(fields, name)
	}
	re.WriteString(regexp.QuoteMeta(p[last:]) + "$")
	return regexp.MustCompile(re.String()), fields, nil
}

// folderName is a folder's own name: proposed, for bp/proposed.
func folderName(dir string) string {
	if dir == "." || dir == "" {
		return ""
	}
	return path.Base(dir)
}

func isMarkdownPath(p string) bool {
	switch strings.ToLower(filepath.Ext(p)) {
	case ".md", ".markdown", ".mdx":
		return true
	}
	return false
}

// Match reports whether rel, a slash-separated path in the kind's repo, is
// where a document of the kind would be (its path, or where one of its
// moves takes it), and what that path says of it.
func (k *Kind) Match(rel string) (DocFields, bool) {
	f, _, ok := k.MatchState(rel)
	return f, ok
}

// MatchState is Match, and where a document there stands: the name of
// the folder it's in (proposed, accepted).
func (k *Kind) MatchState(rel string) (DocFields, string, bool) {
	var f DocFields
	places := k.places
	if len(places) == 0 && k.re != nil {
		places = []place{{re: k.re, fields: k.fields}}
	}
	for _, pl := range places {
		if m := pl.re.FindStringSubmatch(rel); m != nil {
			return fields(pl.fields, m), pl.state, true
		}
	}
	return f, "", false
}

// States is where a kind's documents can stand, in the order of their
// life: its path's folder's name, then each move's. Empty for a kind with
// no moves, whose documents all stand in one place.
func (k *Kind) States() []string {
	if len(k.places) < 2 {
		return nil
	}
	var out []string
	for _, pl := range k.places {
		out = append(out, pl.state)
	}
	return out
}

func fields(names []string, m []string) DocFields {
	var f DocFields
	for i, name := range names {
		switch name {
		case "date":
			f.Date = m[i+1]
		case "slug":
			f.Slug = m[i+1]
		case "number":
			f.Number = m[i+1]
		}
	}
	return f
}

// Numbered reports whether a kind's documents are numbered ({number}).
func (k *Kind) Numbered() bool { return slices.Contains(k.fields, "number") }

// NextNumber is the number a new document of the kind takes in dir (a
// worktree of its repo): one above the highest there, to four digits.
func (k *Kind) NextNumber(dir string) string {
	matches, _ := filepath.Glob(filepath.Join(dir, filepath.FromSlash(placeholder.ReplaceAllString(k.Pattern, "*"))))
	high := 0
	for _, m := range matches {
		rel, err := filepath.Rel(dir, m)
		if err != nil {
			continue
		}
		if f, ok := k.Match(filepath.ToSlash(rel)); ok {
			if n, err := strconv.Atoi(f.Number); err == nil && n > high {
				high = n
			}
		}
	}
	return fmt.Sprintf("%04d", high+1)
}

// numberTaken says when a document of a numbered kind that a capsule adds
// (in its commits since base) has a number another of the kind has on the
// branch: two people can take the same one. "" when none has.
func (w *Workspace) numberTaken(repo, dir, base string) string {
	out, err := git.Output(dir, "diff", "--name-only", "--diff-filter=A", base, "HEAD")
	if err != nil {
		return ""
	}
	for _, rel := range strings.Fields(out) {
		k, f := w.KindOf(repo, rel)
		if k == nil || !k.Numbered() {
			continue
		}
		n, _ := strconv.Atoi(f.Number)
		matches, _ := filepath.Glob(filepath.Join(dir, filepath.FromSlash(placeholder.ReplaceAllString(k.Pattern, "*"))))
		for _, m := range matches {
			other, err := filepath.Rel(dir, m)
			if other = filepath.ToSlash(other); err != nil || other == rel {
				continue
			}
			if g, ok := k.Match(other); ok {
				if o, _ := strconv.Atoi(g.Number); o == n {
					next := DocFields{Date: f.Date, Slug: f.Slug, Number: k.NextNumber(dir)}
					return fmt.Sprintf("%s takes number %s, which %s has on %s: two %s can't share one, and nothing was pushed. Renumber it %s (git mv %s %s, and anywhere it says its number), commit, and land again", rel, f.Number, other, w.DefaultBranch, k.Title, next.Number, rel, next.Fill(k.Pattern))
				}
			}
		}
	}
	return ""
}

// Fill is one of a kind's messages (or its path) with what's known of a
// document filled in. What isn't known is left as it's written.
func (f DocFields) Fill(s string) string {
	return placeholder.ReplaceAllStringFunc(s, func(p string) string {
		v := map[string]string{"{date}": f.Date, "{slug}": f.Slug, "{number}": f.Number, "{title}": f.Title}[p]
		if v == "" {
			return p
		}
		return v
	})
}

// Subject is the first line of the commit that adds a document of the kind
// (adding) or changes one: its message, filled in. "" if it has none.
func (k *Kind) Subject(adding bool, f DocFields) string {
	msg := k.Amend
	if adding {
		msg = k.Add
	}
	if msg == "" {
		return ""
	}
	return strings.Join(strings.Fields(f.Fill(msg)), " ")
}

// KindOf is the kind of document a file is, by its repo and its path there
// (slash-separated), and what its path says of it; nil if it's none of the
// workspace's kinds. Where two kinds' paths fit, it's the first by name.
func (w *Workspace) KindOf(repo, rel string) (*Kind, DocFields) {
	for _, k := range w.Kinds {
		if k.Repo != repo {
			continue
		}
		if f, ok := k.Match(rel); ok {
			return k, f
		}
	}
	return nil, DocFields{}
}

// KindNamed is the kind called name, if the workspace has it.
func (w *Workspace) KindNamed(name string) (*Kind, bool) {
	for _, k := range w.Kinds {
		if k.Name == name {
			return k, true
		}
	}
	return nil, false
}

// KindGuide is the file a kind's guide is: relative to the workspace (or
// absolute, or from ~), or repo:path for a file on a repo's default branch
// (as its ground has it). "" for a kind with none.
func (w *Workspace) KindGuide(k *Kind) string {
	if k.Guide == "" {
		return ""
	}
	if repo, path, ok := strings.Cut(k.Guide, ":"); ok && !filepath.IsAbs(k.Guide) {
		if name, ok := w.ResolveAlias(repo); ok {
			return filepath.Join(w.MainWorktree(name), filepath.FromSlash(path))
		}
	}
	return w.inWorkspace(k.Guide)
}

// checkKinds is ws doctor's word on the kinds of document: each, and
// whether its guide is there.
func (w *Workspace) checkKinds() CheckCategory {
	cat := CheckCategory{Name: "Kinds of document"}
	for _, k := range w.Kinds {
		c := CheckResult{Name: k.Name, Status: CheckOK, Detail: k.Where()}
		if guide := w.KindGuide(k); guide != "" {
			if _, err := os.Stat(guide); err != nil {
				c.Status = CheckWarn
				c.Detail = fmt.Sprintf("its guide is %s, which isn't there: agents writing one aren't told how", w.rel(guide))
			}
		}
		cat.Checks = append(cat.Checks, c)
	}
	return cat
}

// Where is a kind's place, as the workspace says it: repo:path.
func (k *Kind) Where() string { return k.Repo + ":" + k.Pattern }

// DocTitle is a document's title: its front matter's title, else its first
// heading's words, without their markdown. "" if it has neither.
func DocTitle(text []byte) string { return words.Title(text) }

// slugRoom is how long a slug made from a title may be.
const slugRoom = 60

// Slug is a title made into a file's name: lowercase words and digits,
// joined by hyphens, cut at a word to slugRoom. "untitled" for a title
// with neither.
func Slug(title string) string { return slug.Make(title, slugRoom, "untitled") }
