// Package nav is code navigation for reviews and the explorer: which
// words in a file are names, where a name is defined, and where a
// definition is used. Files are read from git at a commit (a worktree's
// files as they are now are made one: Now), parsed with tree-sitter when
// first asked about, and what a parse finds is kept by the file's blob.
package nav

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"unicode/utf8"
)

// CacheMax is the most the facts kept on disk take up.
const CacheMax = 200 << 20

// Service answers questions about names, for any repo at any commit.
type Service struct {
	Runtime Runtime
	disk    disk
	facts   *lru[*Facts]   // by blob and grammar
	trees   *lru[*gitTree] // by git directory and commit
	answers *lru[*Answer]  // by question: a commit's don't change
	refs    *lru[*References]
	repos   map[string]*repo
	parsing group[*Facts]
	listing group[*gitTree]
	now     nowCache
	reposMu chan struct{}
}

// New is a service that keeps what it parses in cacheDir ("" for memory
// only), and prunes it there now and then.
func New(cacheDir string) *Service {
	s := &Service{
		Runtime: Builtin(),
		disk:    disk{dir: cacheDir},
		facts:   newLRU[*Facts](2000),
		trees:   newLRU[*gitTree](8),
		answers: newLRU[*Answer](5000),
		refs:    newLRU[*References](200),
		repos:   map[string]*repo{},
		reposMu: make(chan struct{}, 1),
	}
	if cacheDir != "" {
		go s.disk.prune(CacheMax)
	}
	return s
}

// Tree is a git directory's files at a commit.
func (s *Service) Tree(dir, rev string) (Tree, error) {
	return s.tree(dir, rev)
}

func (s *Service) tree(dir, rev string) (*gitTree, error) {
	dir = filepath.Clean(dir)
	key := dir + "\x00" + rev
	if t, ok := s.trees.get(key); ok {
		return t, nil
	}
	return s.listing.do(key, func() (*gitTree, error) {
		t, err := loadTree(s.repo(dir), rev)
		if err != nil {
			return nil, err
		}
		s.trees.put(key, t)
		return t, nil
	})
}

func (s *Service) repo(dir string) *repo {
	s.reposMu <- struct{}{}
	defer func() { <-s.reposMu }()
	r := s.repos[dir]
	if r == nil {
		r = &repo{dir: dir}
		s.repos[dir] = r
	}
	return r
}

// ErrNoLang is the answer for a file of a language there's no navigation
// for.
var ErrNoLang = errors.New("no navigation for this kind of file")

// Facts is what a file's parse found, parsing it if it hasn't been.
func (s *Service) Facts(t Tree, p string) (*Facts, error) {
	l := LangOf(p)
	if l == nil {
		return nil, ErrNoLang
	}
	grammar := l.Grammar(p)
	if !s.Runtime.Has(grammar) {
		return nil, ErrNoParser
	}
	blob, ok := t.Blob(p)
	if !ok {
		return nil, fmt.Errorf("no %s at %s", p, short(t.Rev()))
	}
	key := blob + "-" + grammar
	if f, ok := s.facts.get(key); ok {
		return f, nil
	}
	return s.parsing.do(key, func() (*Facts, error) {
		if f, ok := s.disk.get(blob, grammar); ok {
			s.facts.put(key, f)
			return f, nil
		}
		src, err := t.Read(p)
		if err != nil {
			return nil, err
		}
		matches, broken, err := s.Runtime.Captures(grammar, l.Query(grammar), src)
		if err != nil {
			return nil, err
		}
		f := Build(l.Name(), src, matches, broken)
		s.facts.put(key, f)
		s.disk.put(blob, grammar, f)
		return f, nil
	})
}

// Names is GET …/nav/names: every name in a file at a commit, by line,
// with what part each plays.
type Names struct {
	Lang string `json:"lang"`
	// Why there are none: a kind of file there's no navigation for, too
	// large, a build without a parser.
	Why    string          `json:"why,omitempty"`
	Broken bool            `json:"broken,omitempty"`
	Lines  map[int][]Named `json:"lines"`
}

// Named is a name on a line: its columns in UTF-16 units from 0 (as a
// browser counts), and its part: r a use, m a member, d a definition, i
// part of an import.
type Named struct {
	From, To int
	Part     byte
}

func (n Named) MarshalJSON() ([]byte, error) {
	return json.Marshal([]any{n.From, n.To, string(n.Part)})
}

func (n *Named) UnmarshalJSON(b []byte) error {
	var v struct {
		From, To int
		Part     string
	}
	var arr []json.RawMessage
	if err := json.Unmarshal(b, &arr); err != nil || len(arr) != 3 {
		return fmt.Errorf("a name is [from, to, part]: %s", b)
	}
	if err := json.Unmarshal(arr[0], &v.From); err != nil {
		return err
	}
	if err := json.Unmarshal(arr[1], &v.To); err != nil {
		return err
	}
	if err := json.Unmarshal(arr[2], &v.Part); err != nil || len(v.Part) != 1 {
		return fmt.Errorf("a name's part is one letter: %s", b)
	}
	n.From, n.To, n.Part = v.From, v.To, v.Part[0]
	return nil
}

// Names is every name in a file at a commit.
func (s *Service) Names(dir, rev, p string) (*Names, error) {
	t, err := s.tree(dir, rev)
	if err != nil {
		return nil, err
	}
	out := &Names{Lines: map[int][]Named{}}
	f, err := s.Facts(t, p)
	switch {
	case errors.Is(err, ErrNoLang):
		out.Why = "No navigation for " + describe(p) + " files"
		return out, nil
	case errors.Is(err, ErrNoParser):
		out.Why = "This build has no code navigation: it was built without cgo"
		return out, nil
	case errors.Is(err, ErrTooLarge):
		out.Why = "Too large to navigate"
		return out, nil
	case errors.Is(err, ErrBinary):
		out.Why = "Not text"
		return out, nil
	case err != nil:
		return nil, err
	}
	src, err := t.Read(p)
	if err != nil {
		return nil, err
	}
	out.Lang, out.Broken = f.Lang, f.Broken
	add := func(at Span, part byte) {
		line, start := f.Line(at.Start)
		from := utf16Col(src, start, at.Start)
		out.Lines[line] = append(out.Lines[line], Named{From: from, To: from + utf16Len(src[at.Start:at.End]), Part: part})
	}
	for _, d := range f.Defs {
		add(d.At, 'd')
	}
	for _, u := range f.Uses {
		add(u.At, u.Part)
	}
	for _, im := range f.Imports {
		if !im.PathAt.None() {
			add(im.PathAt, 'i')
		}
		for _, at := range []Span{im.NameAt, im.AliasAt} {
			if !at.None() && at != im.PathAt {
				add(at, 'i')
			}
		}
	}
	for line, names := range out.Lines {
		sortNamed(names)
		out.Lines[line] = names
	}
	return out, nil
}

func sortNamed(n []Named) {
	for i := 1; i < len(n); i++ {
		for j := i; j > 0 && n[j].From < n[j-1].From; j-- {
			n[j], n[j-1] = n[j-1], n[j]
		}
	}
}

// describe is how a file's kind is said: its suffix, or its name.
func describe(p string) string {
	if ext := filepath.Ext(p); ext != "" {
		return ext
	}
	return filepath.Base(p)
}

// utf16Col is the column of a byte on a line, in UTF-16 units.
func utf16Col(src []byte, lineStart, pos int32) int {
	return utf16Len(src[lineStart:pos])
}

func utf16Len(b []byte) int {
	n := 0
	for len(b) > 0 {
		if b[0] < utf8.RuneSelf {
			n++
			b = b[1:]
			continue
		}
		r, size := utf8.DecodeRune(b)
		if r >= 0x10000 {
			n += 2
		} else {
			n++
		}
		b = b[size:]
	}
	return n
}
