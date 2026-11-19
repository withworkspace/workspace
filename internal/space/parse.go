package space

import (
	"net/url"
	"path"
	"regexp"
	"strings"

	"github.com/withworkspace/workspace/internal/ref"
	"github.com/withworkspace/workspace/internal/words"
	"github.com/withworkspace/workspace/internal/workspace"
)

// read is what a document is, read from its text: its title, its sections
// (the places a search leads to), what it cites, and what else its words
// name (a plugin's thing, a pull request, a mission, a file that isn't a
// document), each as it's first named.
type read struct {
	title    string
	sections []section
	links    []link
	names    []ref.Ref
}

// section is a heading and what's under it: a place a search leads to.
type section = words.Section

// link is a document cited: by a link in the markdown, or named as a ref.
type link struct {
	repo, path string
	line       int    // where it's cited, from 1
	how        string // "link" or "ref"
}

var (
	mdLink = regexp.MustCompile(`!?\[([^\]]*)\]\(\s*<?([^)\s>]+)>?(?:\s+["'][^"']*["'])?\s*\)`)
	scheme = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9+.-]*:`)
)

// parse reads a document at rel in repo. known, when there is one, finds
// what its text names as refs (cr-records:bp/…, a repo's address).
func parse(repo, rel string, text []byte, known *ref.Known) read {
	r := read{title: workspace.DocTitle(text)}
	r.sections = words.Sections(text, r.title)
	r.links, r.names = links(repo, rel, strings.Split(strings.ReplaceAll(string(text), "\r\n", "\n"), "\n"), known)
	return r
}

// links is what a document cites: markdown documents its links lead to (in
// its repo, by a relative path), and files its words name as refs (in any
// repo), each once, where it's first cited. names is what else its words
// name, which isn't a document to cite: what the graph links it to.
func links(repo, rel string, lines []string, known *ref.Known) (out []link, names []ref.Ref) {
	seen := map[string]bool{}
	add := func(l link) {
		if l.repo == repo && l.path == rel {
			return // itself
		}
		key := l.repo + ":" + l.path
		if !seen[key] {
			seen[key] = true
			out = append(out, l)
		}
	}
	dir := path.Dir(rel)
	fenced := false
	for i, line := range lines {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~") {
			fenced = !fenced
		}
		if fenced {
			continue
		}
		for _, m := range mdLink.FindAllStringSubmatch(line, -1) {
			target := m[2]
			if scheme.MatchString(target) || strings.HasPrefix(target, "#") || strings.HasPrefix(target, "/") {
				continue // an address: a ref, if it's a repo's (below)
			}
			target, _, _ = strings.Cut(target, "#")
			target, _, _ = strings.Cut(target, "?")
			if u, err := url.PathUnescape(target); err == nil {
				target = u
			}
			p := path.Clean(path.Join(dir, target))
			if p == "." || strings.HasPrefix(p, "../") || p == ".." || !isMarkdown(p) {
				continue
			}
			add(link{repo: repo, path: p, line: i + 1, how: "link"})
		}
		if known == nil {
			continue
		}
		var found []ref.Ref
		for _, f := range known.Find(line) {
			found = append(found, f.Ref)
		}
		// And what's named by where it is: an address written out, or a link's.
		for _, r := range append(found, known.Addressed(line)...) {
			if r.Kind == ref.File && (r.Worktree == "" || r.Worktree == ref.Ground) && isMarkdown(r.Path) {
				add(link{repo: r.Repo, path: r.Path, line: i + 1, how: "ref"})
				continue
			}
			names = append(names, r)
		}
	}
	return out, names
}

func isMarkdown(p string) bool {
	switch strings.ToLower(path.Ext(p)) {
	case ".md", ".markdown", ".mdx":
		return true
	}
	return false
}
