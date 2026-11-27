package ref

import (
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"
)

// Tree is a repo's default branch, for telling a file or a folder that's
// there from words that only look like one.
type Tree interface {
	Has(path string) bool
	HasDir(path string) bool
}

// Known is what there is to name: each repo with its default branch (a nil
// Tree while it isn't read: none of its files are found), what else each
// repo is called (alias to repo), each repo's capsules, the workspace's
// folder (a path on disk under it is read as the file it is), the
// missions' slugs, the pages, conversations and artifacts by id, and how
// each plugin's things are named. Where each repo is on GitHub (Remotes,
// or else under Org by its name), the default branch and each repo's
// capsules by branch (repo to branch to capsule) are for reading
// addresses.
type Known struct {
	Repos         map[string]Tree
	Aliases       map[string]string
	Capsules      map[string][]string
	Root          string
	Missions      []string
	Pages         []string
	Talks         []string
	Artifacts     []string
	Things        []Named
	Org           string
	Remotes       map[string]Remote
	DefaultBranch string
	Branches      map[string]map[string]string

	once sync.Once
	pat  patterns
}

// Found is a ref in a run of text, from a byte offset to another.
type Found struct {
	From, To int
	Ref      Ref
}

// The edges a name can't have: what's before or after it that makes it
// part of something else (a longer path, a word, an address).
const (
	notBeforeFile = `\w./@#~-`
	notBeforeRepo = `\w./@#:~-`
	notAfterRepo  = `\w/#:@~-`
	notBeforePR   = `\w./@#-`
	notAfterPR    = `\w#`
	// A commit's sha isn't one followed by more of a word, a path, or a
	// name's dot (demo@4f2a9c1.com is an email's).
	notAfterCommit = `\w#@/~-`
	notBeforeWord  = `\w/.#-`
	notAfterWord   = `\w/#-`
)

const (
	seg       = `[\w.@+~-]+`
	filePath  = seg + `(?:/` + seg + `)*`
	lineRange = `(?::(\d+)(?:-(\d+))?)?`
)

type patterns struct {
	ground, inCapsule, onDisk, bareRepo, prs, commits, missions *regexp.Regexp
	ids, artifacts                                              *regexp.Regexp
	// things are the Known's Things' names, in their order: nil for one
	// whose tail isn't known, none of whose names are found in words.
	things []*regexp.Regexp
}

// alt is names as alternatives, the longest first, so the longest wins.
func alt(names []string) string {
	ns := slices.Clone(names)
	sort.Slice(ns, func(i, j int) bool {
		if len(ns[i]) != len(ns[j]) {
			return len(ns[i]) > len(ns[j])
		}
		return ns[i] < ns[j]
	})
	for i, n := range ns {
		ns[i] = regexp.QuoteMeta(n)
	}
	return strings.Join(ns, "|")
}

func (k *Known) patterns() *patterns {
	k.once.Do(func() {
		repos := make([]string, 0, len(k.Repos))
		for r := range k.Repos {
			repos = append(repos, r)
		}
		names := slices.Clone(repos)
		for a, of := range k.Aliases {
			if _, isRepo := k.Repos[a]; !isRepo {
				if _, ok := k.Repos[of]; ok {
					names = append(names, a)
				}
			}
		}
		p := &k.pat
		if len(repos) > 0 {
			p.ground = regexp.MustCompile(`(` + alt(names) + `)/(` + filePath + `)` + lineRange)
			var capsules []string
			for _, cs := range k.Capsules {
				for _, c := range cs {
					if !slices.Contains(capsules, c) {
						capsules = append(capsules, c)
					}
				}
			}
			if len(capsules) > 0 {
				p.inCapsule = regexp.MustCompile(`(` + alt(names) + `)/(` + alt(capsules) + `):(` + filePath + `)` + lineRange)
			}
			root := ""
			if k.Root != "" {
				root = `(?:` + regexp.QuoteMeta(strings.TrimRight(k.Root, "/")) + `/)?`
			}
			p.onDisk = regexp.MustCompile(root + `repos/(` + alt(repos) + `)/(` + seg + `)/(` + filePath + `)` + lineRange)
			p.bareRepo = regexp.MustCompile(`(` + alt(repos) + `)`)
			p.prs = regexp.MustCompile(`(` + alt(names) + `)#([1-9]\d{0,6})`)
			p.commits = regexp.MustCompile(`(` + alt(names) + `)@([0-9a-f]{7,40})`)
		}
		var slugs []string
		for _, s := range k.Missions {
			if len(s) >= 3 {
				slugs = append(slugs, s)
			}
		}
		if len(slugs) > 0 {
			p.missions = regexp.MustCompile(`(` + alt(slugs) + `)`)
		}
		for _, n := range k.Things {
			p.things = append(p.things, n.pattern())
		}
		p.ids = regexp.MustCompile(`([pw])([1-9]\d{0,4})`)
		p.artifacts = regexp.MustCompile(`(a[1-9]\d{0,6})(?:\.v([1-9]\d{0,4}))?`)
	})
	return &k.pat
}

// edge is whether the rune before i (or at i, after) is one of set.
func before(text string, i int, set string) bool {
	if i == 0 {
		return false
	}
	r, _ := utf8.DecodeLastRuneInString(text[:i])
	return inSet(r, set)
}

func after(text string, i int, set string) bool {
	if i >= len(text) {
		return false
	}
	r, _ := utf8.DecodeRuneInString(text[i:])
	return inSet(r, set)
}

// inSet is whether r is in a set written as a regexp class's insides
// (\w, and literal runes).
func inSet(r rune, set string) bool {
	if strings.Contains(set, `\w`) && (r == '_' || r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z') {
		return true
	}
	return strings.ContainsRune(strings.ReplaceAll(set, `\w`, ""), r)
}

// scan is each match of re in text that ok takes, as a lookbehind would
// have it: a match that isn't taken is tried again a rune later.
func scan(re *regexp.Regexp, text string, ok func(m []int) bool) {
	for i := 0; i < len(text); {
		loc := re.FindStringSubmatchIndex(text[i:])
		if loc == nil {
			return
		}
		for j := range loc {
			if loc[j] >= 0 {
				loc[j] += i
			}
		}
		if ok(loc) {
			i = loc[1]
			if loc[1] == loc[0] {
				i++
			}
			continue
		}
		_, w := utf8.DecodeRuneInString(text[loc[0]:])
		i = loc[0] + w
	}
}

func group(text string, m []int, n int) string {
	if m[2*n] < 0 {
		return ""
	}
	return text[m[2*n]:m[2*n+1]]
}

func linesOf(from, to string) (int, int) {
	if from == "" {
		return 0, 0
	}
	l, _ := strconv.Atoi(from)
	e, _ := strconv.Atoi(to)
	return l, e
}

// looksLikeFile is a path in a capsule, which isn't checked against a
// tree: it has to look like a file, or name lines. A sentence's full stop
// is taken off it.
var fileShape = regexp.MustCompile(`\.[\w+-]+$`)

func looksLikeFile(p string, line int) (string, bool) {
	if line > 0 {
		return p, true
	}
	bare := strings.TrimRight(p, ".")
	return bare, fileShape.MatchString(bare)
}

func (k *Known) repoOf(name string) string {
	if _, ok := k.Repos[name]; ok {
		return name
	}
	if r, ok := k.Aliases[name]; ok {
		return r
	}
	return name
}

// Find is the refs in a run of text, in order, none over another. Only
// what's there is found: a file in its repo's tree, a capsule the repo
// has, a mission (a page, a conversation, an artifact) the workspace has.
func (k *Known) Find(text string) []Found {
	var out []Found
	add := func(from, to int, r Ref) { out = append(out, Found{from, to, r}) }
	p := k.patterns()
	if p.ground != nil {
		// A file or folder on the default branch: checked against its tree.
		scan(p.ground, text, func(m []int) bool {
			if before(text, m[0], notBeforeFile) {
				return false
			}
			repo := k.repoOf(group(text, m, 1))
			tree := k.Repos[repo]
			if tree == nil {
				return false
			}
			path, to := group(text, m, 2), m[1]
			line, end := linesOf(group(text, m, 3), group(text, m, 4))
			bare := strings.TrimRight(path, ".")
			if !tree.Has(path) {
				// A folder: named with its slash after it, or without.
				if line == 0 {
					for _, d := range []string{path, bare} {
						if tree.HasDir(d) {
							to = m[4] + len(d)
							if after(text, to, "/") {
								to++
							}
							add(m[0], to, Ref{Kind: Dir, Repo: repo, Worktree: Ground, Path: d})
							return true
						}
					}
				}
				// A sentence's full stop, taken for the path's.
				if line > 0 || bare == path || !tree.Has(bare) {
					return false
				}
				to -= len(path) - len(bare)
				path, line, end = bare, 0, 0
			}
			add(m[0], to, Ref{Kind: File, Repo: repo, Worktree: Ground, Path: path, Line: line, EndLine: end})
			return true
		})
		// A file in a capsule, by its name: repo/capsule:path.
		if p.inCapsule != nil {
			scan(p.inCapsule, text, func(m []int) bool {
				if before(text, m[0], notBeforeFile) {
					return false
				}
				repo, capsule := k.repoOf(group(text, m, 1)), group(text, m, 2)
				if !slices.Contains(k.Capsules[repo], capsule) {
					return false
				}
				line, end := linesOf(group(text, m, 4), group(text, m, 5))
				raw := group(text, m, 3)
				path, ok := looksLikeFile(raw, line)
				if !ok {
					return false
				}
				add(m[0], m[1]-(len(raw)-len(path)), Ref{Kind: File, Repo: repo, Worktree: capsule, Path: path, Line: line, EndLine: end})
				return true
			})
		}
		// A file as it is on disk: repos/<repo>/<worktree>/<path>, maybe
		// under the workspace's folder. A path in some other folder isn't one.
		scan(p.onDisk, text, func(m []int) bool {
			if before(text, m[0], notBeforeFile) {
				return false
			}
			repo, worktree, raw := group(text, m, 1), group(text, m, 2), group(text, m, 3)
			line, end := linesOf(group(text, m, 4), group(text, m, 5))
			path := raw
			if worktree == Ground {
				tree := k.Repos[repo]
				switch {
				case tree == nil:
					return false
				case tree.Has(path):
				case line == 0 && tree.Has(strings.TrimRight(path, ".")):
					path = strings.TrimRight(path, ".")
				default:
					return false
				}
			} else {
				var ok bool
				if path, ok = looksLikeFile(path, line); !ok {
					return false
				}
			}
			add(m[0], m[1]-(len(raw)-len(path)), Ref{Kind: File, Repo: repo, Worktree: worktree, Path: path, Line: line, EndLine: end})
			return true
		})
		// A repo by its name alone (not an alias: those are short, and words).
		scan(p.bareRepo, text, func(m []int) bool {
			if before(text, m[0], notBeforeRepo) || after(text, m[1], notAfterRepo) {
				return false
			}
			if after(text, m[1], ".") && after(text, m[1]+1, `\w`) {
				return false
			}
			add(m[0], m[1], Ref{Kind: Repo, Repo: group(text, m, 1)})
			return true
		})
		scan(p.prs, text, func(m []int) bool {
			if before(text, m[0], notBeforePR) || after(text, m[1], notAfterPR) {
				return false
			}
			n, _ := strconv.Atoi(group(text, m, 2))
			add(m[0], m[1], Ref{Kind: PR, Repo: k.repoOf(group(text, m, 1)), Number: n})
			return true
		})
		// A commit, by its sha: demo@4f2a9c1. Not a version (demo@v1.2),
		// nor what goes on past the sha.
		scan(p.commits, text, func(m []int) bool {
			if before(text, m[0], notBeforePR) || after(text, m[1], notAfterCommit) {
				return false
			}
			if after(text, m[1], ".") && after(text, m[1]+1, `\w`) {
				return false
			}
			add(m[0], m[1], Ref{Kind: Commit, Repo: k.repoOf(group(text, m, 1)), Commit: group(text, m, 2)})
			return true
		})
	}
	if p.missions != nil {
		scan(p.missions, text, func(m []int) bool {
			if before(text, m[0], notBeforeWord) || after(text, m[1], notAfterWord) {
				return false
			}
			add(m[0], m[1], Ref{Kind: Mission, Slug: group(text, m, 1)})
			return true
		})
	}
	scan(p.ids, text, func(m []int) bool {
		if before(text, m[0], notBeforeWord) || after(text, m[1], notAfterWord) {
			return false
		}
		id := group(text, m, 0)
		switch {
		case id[0] == 'p' && slices.Contains(k.Pages, id):
			add(m[0], m[1], Ref{Kind: Page, ID: id})
		case id[0] == 'w' && slices.Contains(k.Talks, id):
			add(m[0], m[1], Ref{Kind: Talk, ID: id})
		default:
			return false
		}
		return true
	})
	// An artifact (a3), or a version of one (a3.v2): the artifact has to
	// be there; whether the version is, its card says.
	scan(p.artifacts, text, func(m []int) bool {
		if before(text, m[0], notBeforeWord) || after(text, m[1], notAfterWord) {
			return false
		}
		id := group(text, m, 1)
		if !slices.Contains(k.Artifacts, id) {
			return false
		}
		v, _ := strconv.Atoi(group(text, m, 2))
		add(m[0], m[1], Ref{Kind: Artifact, ID: id, Version: v})
		return true
	})
	// A plugin's thing, by a name its plugin registered: RAM-123. Whether
	// there's one by that name, its plugin says when it's shown.
	for i, re := range p.things {
		if re == nil {
			continue
		}
		n := k.Things[i]
		scan(re, text, func(m []int) bool {
			if before(text, m[0], notBeforeWord) || after(text, m[1], notAfterWord) {
				return false
			}
			add(m[0], m[1], n.ref(text[m[0]:m[1]]))
			return true
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].From != out[j].From {
			return out[i].From < out[j].From
		}
		return out[i].To > out[j].To
	})
	kept := out[:0]
	for _, f := range out {
		if len(kept) == 0 || f.From >= kept[len(kept)-1].To {
			kept = append(kept, f)
		}
	}
	return kept
}

// At is the ref that's the whole of a text (a link's address, a name in
// backticks), if it is one: an address, or a name that's there.
func (k *Known) At(text string) (Ref, bool) {
	t := strings.TrimSpace(text)
	if r, ok := ParseAddress(t, k); ok {
		return r, true
	}
	f := k.Find(t)
	if len(f) == 1 && f[0].From == 0 && f[0].To == len(t) {
		return f[0].Ref, true
	}
	return Ref{}, false
}
