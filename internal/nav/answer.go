package nav

import (
	"errors"
	"fmt"
	"path"
	"strings"
	"unicode/utf8"
)

// Answer is GET …/nav/definition: where a name leads, and how that was
// found.
type Answer struct {
	Name string `json:"name"`
	// Answer is resolved (followed to its definition, or several where
	// several are right), candidates (definitions of that name, likeliest
	// first: not followed there), external (a dependency's), builtin (the
	// language's own) or unresolved.
	Answer   string    `json:"answer"`
	How      string    `json:"how,omitempty"`
	Targets  []Target  `json:"targets"`
	External *External `json:"external,omitempty"`
	// Partial: it ran out of time or files to read, and may have missed
	// some.
	Partial bool `json:"partial,omitempty"`
}

// Target is a definition: where, what kind, and what it says.
type Target struct {
	Path      string `json:"path"`
	Line      int    `json:"line"` // from 1
	Col       int    `json:"col"`  // UTF-16 units from 0
	EndLine   int    `json:"endLine"`
	Kind      string `json:"kind"`
	Container string `json:"container,omitempty"`
	Signature string `json:"signature,omitempty"`
	Doc       string `json:"doc,omitempty"`
}

// External is a dependency's name.
type External struct {
	Module string `json:"module,omitempty"`
	Name   string `json:"name,omitempty"`
	URL    string `json:"url,omitempty"`
}

// ErrNoName is the answer for a place there's no name at.
var ErrNoName = errors.New("there's no name there")

// Definition is where the name at a line and column (UTF-16 units, as a
// browser counts) of a file at a commit leads.
func (s *Service) Definition(dir, rev, p string, line, col int) (*Answer, error) {
	key := strings.Join([]string{"def", dir, rev, p, fmt.Sprint(line), fmt.Sprint(col)}, "\x00")
	if a, ok := s.answers.get(key); ok {
		return a, nil
	}
	t, err := s.tree(dir, rev)
	if err != nil {
		return nil, err
	}
	f, err := s.Facts(t, p)
	if err != nil {
		return nil, err
	}
	src, err := t.Read(p)
	if err != nil {
		return nil, err
	}
	pos, ok := byteAt(src, f, line, col)
	if !ok {
		return nil, ErrNoName
	}
	w := s.walker(t, LangOf(p))
	r, name := w.at(p, f, pos)
	if name == "" {
		return nil, ErrNoName
	}
	a := w.answer(name, r)
	if !a.Partial {
		s.answers.put(key, a)
	}
	return a, nil
}

// answer is what's said of a name found to be r.
func (w *walker) answer(name string, r res) *Answer {
	a := &Answer{Name: name, Targets: []Target{}, How: r.how, Partial: w.partial || r.partial}
	switch r.kind {
	case "def":
		a.Answer = "resolved"
		for _, s := range r.defs {
			a.Targets = append(a.Targets, w.target(s))
		}
	case "candidates":
		a.Answer = "candidates"
		for _, s := range r.defs {
			a.Targets = append(a.Targets, w.target(s))
		}
	case "module":
		a.Answer = "resolved"
		if t, ok := w.moduleTarget(r.mod); ok {
			a.Targets = append(a.Targets, t)
		}
	case "external":
		a.Answer = "external"
		if r.modPath == "" && !r.likely {
			a.Answer = "builtin" // a member of the language's own type
		}
		a.External = &External{Module: r.modPath, Name: r.external, URL: w.lang.Docs(r.modPath, r.external)}
	case "builtin":
		a.Answer = "builtin"
		a.How = "built in"
		a.External = &External{Name: r.external, URL: w.lang.Docs("", r.external)}
	case "namespace":
		// A namespace: where it's declared.
		a.Answer, a.How = "unresolved", "the namespace "+r.modPath
		if t, ok := w.namespaceTarget(r.modPath); ok {
			a.Answer = "resolved"
			a.Targets = append(a.Targets, t)
		}
	default:
		a.Answer = "unresolved"
		if a.How == "" {
			a.How = "no definition of it here"
		}
	}
	return a
}

// namespaceTarget is where a namespace is declared: the first file that
// does, at the declaration.
func (w *walker) namespaceTarget(ns string) (Target, bool) {
	hits, err := w.t.Grep("namespace "+ns, w.lang.Suffixes()...)
	if err != nil || len(hits) == 0 {
		return Target{}, false
	}
	h := hits[0]
	return Target{Path: h.Path, Line: h.Line, Col: strings.Index(h.Text, ns), EndLine: h.Line, Kind: "namespace", Signature: "namespace " + ns}, true
}

// moduleTarget is where an import of a module in the repo leads: its
// package line (Go: the file that documents the package, if one does),
// or the top of its file.
func (w *walker) moduleTarget(where Where) (Target, bool) {
	if len(where.Files) == 0 {
		return Target{}, false
	}
	pick := where.Files[0]
	for _, p := range where.Files {
		if path.Base(p) == "doc.go" {
			pick = p
			break
		}
	}
	f, err := w.Facts(pick)
	if err != nil {
		return Target{Path: pick, Line: 1, EndLine: 1, Kind: "module"}, true
	}
	t := Target{Path: pick, Line: 1, EndLine: len(f.Lines), Kind: "module"}
	if !f.PackageAt.None() {
		t.Line, _ = f.Line(f.PackageAt.Start)
		t.Kind = "package"
		t.Signature = "package " + f.Package
		if where.Dir != "" {
			t.Container = where.Dir
		}
		t.Doc = w.doc(pick, f, f.PackageAt.Start)
	}
	return t, true
}

// target is a definition, as shown.
func (w *walker) target(s site) Target {
	d := s.def()
	src := w.src(s.path)
	line, start := s.f.Line(d.At.Start)
	end, _ := s.f.Line(maxInt32(d.Extent.End-1, d.Extent.Start))
	t := Target{Path: s.path, Line: line, Col: utf16Len(src[start:d.At.Start]), EndLine: end, Kind: d.Kind}
	if d.Container >= 0 {
		t.Container = s.f.Defs[d.Container].Name
	} else if d.Receiver.Text != "" {
		if r := w.lang.TypeName(d.Receiver.Text); len(r) > 0 {
			t.Container = r[len(r)-1]
		}
	}
	t.Signature = w.signature(s.path, s.f, d)
	if !d.Doc.None() {
		t.Doc = docstring(w.text(s.path, d.Doc))
	} else {
		t.Doc = w.doc(s.path, s.f, d.Extent.Start)
	}
	return t
}

func maxInt32(a, b int32) int32 {
	if a > b {
		return a
	}
	return b
}

// signature is a definition up to its body, on one line.
func (w *walker) signature(p string, f *Facts, d Def) string {
	src := w.src(p)
	if int(d.Extent.End) > len(src) {
		return ""
	}
	text := string(src[d.Extent.Start:d.Extent.End])
	switch d.Kind {
	case "func", "method", "class", "struct", "interface", "enum", "namespace":
		if i := strings.Index(text, w.lang.BodyOpen()); i > 0 {
			text = text[:i]
		}
	default:
		if i := strings.IndexByte(text, '\n'); i > 0 {
			text = text[:i]
		}
	}
	// A type_spec starts at its name: say it's a type.
	if w.lang.Name() == "go" && (typeKinds[d.Kind]) && !strings.HasPrefix(text, "type ") {
		text = "type " + text
	}
	text = strings.Join(strings.Fields(text), " ")
	return cut(text, 200)
}

// doc is the comment just above a line: its first paragraph.
func (w *walker) doc(p string, f *Facts, at int32) string {
	src := w.src(p)
	line, _ := f.Line(at)
	var lines []string
	for l := line - 1; l >= 1; l-- {
		start, _ := f.Offset(l)
		end := int32(len(src))
		if next, ok := f.Offset(l + 1); ok {
			end = next
		}
		text := strings.TrimSpace(string(src[start:end]))
		c, ok := commentText(text)
		if !ok {
			break
		}
		lines = append([]string{c}, lines...)
	}
	// The first paragraph.
	var para []string
	for _, l := range lines {
		if l == "" {
			if len(para) > 0 {
				break
			}
			continue
		}
		para = append(para, l)
	}
	return cut(strings.Join(para, " "), 300)
}

// docstring is a Python docstring's first paragraph.
func docstring(s string) string {
	s = strings.TrimLeft(s, "rRbBuUfF")
	for _, q := range []string{`"""`, "'''", `"`, "'"} {
		if strings.HasPrefix(s, q) && strings.HasSuffix(s, q) && len(s) >= 2*len(q) {
			s = s[len(q) : len(s)-len(q)]
			break
		}
	}
	var para []string
	for l := range strings.SplitSeq(strings.TrimSpace(s), "\n") {
		l = strings.TrimSpace(l)
		if l == "" {
			break
		}
		para = append(para, l)
	}
	return cut(strings.Join(para, " "), 300)
}

// commentText is a line of a comment, without its marks.
func commentText(line string) (string, bool) {
	for _, mark := range []string{"///", "//", "#", "/**", "*/", "*"} {
		if strings.HasPrefix(line, mark) {
			if strings.HasPrefix(line, "#!") {
				return "", false
			}
			return strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line, mark), "*/")), true
		}
	}
	return "", false
}

func cut(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n] + "…"
}

// byteAt is the byte at a line (from 1) and a column in UTF-16 units.
func byteAt(src []byte, f *Facts, line, col int) (int32, bool) {
	start, ok := f.Offset(line)
	if !ok || col < 0 {
		return 0, false
	}
	pos, units := int(start), 0
	for pos < len(src) && src[pos] != '\n' && units < col {
		r, size := utf8.DecodeRune(src[pos:])
		if r >= 0x10000 {
			units += 2
		} else {
			units++
		}
		pos += size
	}
	if units != col {
		return 0, false
	}
	return int32(pos), true
}
