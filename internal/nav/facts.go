package nav

import (
	"sort"
	"strings"
)

// Facts is what one file's parse found: its scopes, the names it defines,
// the names it uses, its calls and its imports. It's built from a query's
// captures alone (see queries/), and kept by the file's blob.
type Facts struct {
	Lang      string
	Package   string // Go's package line
	PackageAt Span
	Namespace string // C#'s file-scoped namespace
	// Broken is whether the file didn't parse cleanly: what's found is
	// what the parser made of it.
	Broken  bool
	Lines   []int32 // each line's first byte
	Scopes  []Scope // the file's own first, then by where they start
	Defs    []Def   // by where their names are
	Uses    []Use   // by where they are
	Calls   []Call  // by where they start
	Imports []Import

	byName map[string][]int // definitions by name: made by index
}

// index makes what's looked up often, after the facts are made or read
// back.
func (f *Facts) index() *Facts {
	f.byName = map[string][]int{}
	for i, d := range f.Defs {
		f.byName[d.Name] = append(f.byName[d.Name], i)
	}
	return f
}

// Named is the definitions of a name in the file, in order.
func (f *Facts) Named(name string) []int { return f.byName[name] }

// Span is a stretch of a file, in bytes. A Span that ends at 0 is none.
type Span struct{ Start, End int32 }

// None is whether it's no span at all.
func (s Span) None() bool { return s.End == 0 }

// Contains is whether o is inside s.
func (s Span) Contains(o Span) bool { return s.Start <= o.Start && o.End <= s.End }

// Expr is some source, kept with its words: a type written on a
// definition, a base class.
type Expr struct {
	Span
	Text string
}

// Scope is a stretch in which names are defined.
type Scope struct {
	Kind string // file, namespace, class, function, block
	Span
	Parent int32  // -1 for the file's own
	Name   string // a namespace's
}

// Def is a name defined.
type Def struct {
	Name string
	Kind string // func, method, class, struct, interface, enum, type, member, field, prop, var, const, param, typeparam, namespace, module
	At   Span   // the name
	// Extent is the whole declaration, for its signature, the comment
	// above it, and the scope it opens.
	Extent    Span
	Scope     int32 // the scope it's defined in
	Container int32 // the type it's a member of, or -1
	// Typed is the type written on it (a function's: what it returns);
	// New is the type it's made from (x := Foo{}); From is the callee it's
	// set from (x := NewFoo()).
	Typed    Expr
	New      Expr
	From     Span
	Receiver Expr   // Go: a method's receiver type
	Bases    []Expr // base classes, embedded fields, extends
	Exported bool   // marked so: TypeScript's export (Go and Python tell by its name)
	Default  bool   // TypeScript's export default
	Doc      Span   // Python's docstring
	// Elem is what a loop's variable is taken from: for x in xs.
	Elem Span
}

// Use is a name used.
type Use struct {
	Name string
	At   Span
	Part byte // 'r' a use, 'm' a member, 'i' part of an import
	In   byte // 'v' where a value goes, 't' a type, 'c' a callee
	// Object is what a member is of: the a of a.b.
	Object Span
}

// Expr is a member's whole expression: a.b's from a to b.
func (u Use) Expr() Span {
	if u.Part == 'm' && !u.Object.None() {
		return Span{u.Object.Start, u.At.End}
	}
	return u.At
}

// Call is a call, and its callee.
type Call struct{ At, Callee Span }

// Import is one name an import brings in (or the module itself).
type Import struct {
	Kind    string // module, named, default, namespace, star, using, static, blank
	Path    string // as written, unquoted
	PathAt  Span
	Name    string // the name imported (named, default)
	NameAt  Span
	Alias   string // the name it's known by here, when written
	AliasAt Span
	// Reexport: it's an export … from: it's what this file exports, not
	// what it uses.
	Reexport bool
}

// Local is the name the import is known by here: its alias, or the name
// it imports. "" for one that isn't known by a name (Go's package, before
// its package line is read; a star).
func (im Import) Local() string {
	if im.Alias != "" {
		return im.Alias
	}
	return im.Name
}

// LocalAt is where that name is written.
func (im Import) LocalAt() Span {
	if !im.AliasAt.None() {
		return im.AliasAt
	}
	if !im.NameAt.None() {
		return im.NameAt
	}
	return im.PathAt
}

// attachments are the captures that say more of the definition in the
// same match, rather than naming one.
var attachments = map[string]bool{"extent": true, "typed": true, "new": true, "from": true, "receiver": true, "base": true, "exported": true, "doc": true, "elem": true}

// typeKinds are definitions that have members.
var typeKinds = map[string]bool{"class": true, "struct": true, "interface": true, "enum": true, "type": true}

// memberKinds are definitions that are members of a type.
var memberKinds = map[string]bool{"field": true, "method": true, "prop": true, "member": true}

// specific are kinds that say more than another capture of the same
// name: a type_spec matched as a struct and as a type is a struct.
var specific = map[string]bool{"class": true, "struct": true, "interface": true, "enum": true, "method": true, "func": true, "field": true}

// usePriority is which capture of one name wins: a member over a callee
// over a type over a plain use.
func usePriority(u Use) int {
	switch {
	case u.Part == 'm':
		return 4
	case u.In == 'c':
		return 3
	case u.In == 't':
		return 2
	}
	return 1
}

// Build makes a file's facts from its query's matches.
func Build(lang string, src []byte, matches []Match, broken bool) *Facts {
	f := &Facts{Lang: lang, Broken: broken, Lines: lineStarts(src)}
	text := func(c Capture) string { return string(src[c.Start:c.End]) }
	span := func(c Capture) Span { return Span{int32(c.Start), int32(c.End)} }

	type scoped struct {
		Scope
		order int
	}
	var scopes []scoped
	scopeAt := map[Span]int{}
	defAt := map[Span]int{}
	useAt := map[Span]int{}
	var defs []Def
	var uses []Use
	var pkgAt Span
	exported, defaults, skipped := map[Span]bool{}, map[Span]bool{}, map[Span]bool{}
	var globals, within []Span
	var fileNS []Span
	blockNS := map[Span]bool{}

	for _, m := range matches {
		var def *Capture
		var objects []Capture
		var memberNames []Capture
		var callAt, callee *Capture
		var im Import
		imported := false
		for i := range m.Captures {
			c := &m.Captures[i]
			name := c.Name
			switch {
			case strings.HasPrefix(name, "_"):
			case strings.HasPrefix(name, "scope."):
				kind := strings.TrimPrefix(name, "scope.")
				if kind == "file" {
					continue
				}
				s := span(*c)
				if i, ok := scopeAt[s]; ok {
					if scopes[i].Kind == "block" {
						scopes[i].Kind = kind
					}
					continue
				}
				scopeAt[s] = len(scopes)
				scopes = append(scopes, scoped{Kind: kind, Span: s, order: len(scopes)})
			case name == "namespace.name":
				// Set below, once the scope in the same match is known.
			case name == "export.name":
				exported[span(*c)] = true
			case name == "export.default":
				exported[span(*c)], defaults[span(*c)] = true, true
			case name == "skip":
				skipped[span(*c)] = true
			case name == "global.name":
				globals = append(globals, span(*c))
			case name == "skip.within":
				within = append(within, span(*c))
			case name == "namespace.file":
				fileNS = append(fileNS, span(*c))
			case strings.HasPrefix(name, "def."):
				if !attachments[strings.TrimPrefix(name, "def.")] {
					def = c
				}
			case name == "ref" || name == "ref.type" || name == "ref.call":
				in := byte('v')
				if name == "ref.type" {
					in = 't'
				} else if name == "ref.call" {
					in = 'c'
				}
				uses = append(uses, Use{Name: text(*c), At: span(*c), Part: 'r', In: in})
			case name == "member" || name == "member.call" || name == "member.type":
				memberNames = append(memberNames, *c)
			case name == "member.object":
				objects = append(objects, *c)
			case name == "call":
				callAt = c
			case name == "call.callee":
				callee = c
			case name == "package":
				f.Package = text(*c)
				pkgAt = span(*c)
				f.PackageAt = pkgAt
			case name == "import.path":
				im.Path, im.PathAt = unquote(text(*c)), span(*c)
				imported = true
			case name == "import.alias":
				im.Alias, im.AliasAt = text(*c), span(*c)
			case strings.HasPrefix(name, "import.") || strings.HasPrefix(name, "reexport."):
				kind := name[strings.IndexByte(name, '.')+1:]
				im.Kind = kind
				im.Reexport = strings.HasPrefix(name, "reexport.")
				if kind == "named" || kind == "default" {
					im.Name, im.NameAt = text(*c), span(*c)
				}
				imported = true
			}
		}
		// A.B.C as one node (TypeSpec's): each member's object is all
		// that's before it.
		var chainBase *Capture
		var chain []Capture
		for i := range m.Captures {
			switch m.Captures[i].Name {
			case "chain.base":
				chainBase = &m.Captures[i]
			case "chain.member":
				chain = append(chain, m.Captures[i])
			}
		}
		if chainBase != nil {
			sort.Slice(chain, func(i, j int) bool { return chain[i].Start < chain[j].Start })
			prev := span(*chainBase)
			for _, c := range chain {
				uses = append(uses, Use{Name: text(c), At: span(c), Part: 'm', In: 't', Object: Span{int32(chainBase.Start), prev.End}})
				prev = span(c)
			}
		}
		// A namespace's name, beside its scope.
		for _, c := range m.Captures {
			if c.Name != "namespace.name" {
				continue
			}
			blockNS[span(c)] = true
			for _, d := range m.Captures {
				if d.Name == "scope.namespace" {
					if i, ok := scopeAt[span(d)]; ok {
						scopes[i].Name = text(c)
					}
				}
			}
		}
		for i, c := range memberNames {
			u := Use{Name: text(c), At: span(c), Part: 'm', In: 'v'}
			if c.Name == "member.call" {
				u.In = 'c'
			} else if c.Name == "member.type" {
				u.In = 't'
			}
			if i < len(objects) {
				u.Object = span(objects[i])
			} else if len(objects) > 0 {
				u.Object = span(objects[len(objects)-1])
			}
			uses = append(uses, u)
		}
		if callAt != nil && callee != nil {
			f.Calls = append(f.Calls, Call{At: span(*callAt), Callee: span(*callee)})
		}
		if imported && im.Kind != "" {
			f.Imports = append(f.Imports, im)
		}
		if def == nil {
			continue
		}
		at := span(*def)
		kind := strings.TrimPrefix(def.Name, "def.")
		i, seen := defAt[at]
		if !seen {
			i = len(defs)
			defAt[at] = i
			defs = append(defs, Def{Name: text(*def), Kind: kind, At: at, Scope: -1, Container: -1})
		} else if specific[kind] && !specific[defs[i].Kind] {
			defs[i].Kind = kind
		}
		d := &defs[i]
		for _, c := range m.Captures {
			switch c.Name {
			case "def.extent":
				d.Extent = span(c)
			case "def.typed":
				d.Typed = Expr{span(c), text(c)}
			case "def.new":
				d.New = Expr{span(c), text(c)}
			case "def.from":
				d.From = span(c)
			case "def.receiver":
				d.Receiver = Expr{span(c), text(c)}
			case "def.base":
				b := Expr{span(c), text(c)}
				dup := false
				for _, x := range d.Bases {
					dup = dup || x.Span == b.Span
				}
				if !dup {
					d.Bases = append(d.Bases, b)
				}
			case "def.exported":
				d.Exported = true
			case "def.doc":
				d.Doc = span(c)
			case "def.elem":
				d.Elem = span(c)
			}
		}
	}

	// A namespace without a block is the file's.
	for _, sp := range fileNS {
		if !blockNS[sp] {
			f.Namespace = strings.Join(strings.Fields(string(src[sp.Start:sp.End])), "")
		}
	}

	// Scopes: the file's own, then by where they start, the outer first.
	sort.SliceStable(scopes, func(i, j int) bool {
		if scopes[i].Start != scopes[j].Start {
			return scopes[i].Start < scopes[j].Start
		}
		return scopes[i].End > scopes[j].End
	})
	f.Scopes = append(f.Scopes, Scope{Kind: "file", Span: Span{0, int32(len(src))}, Parent: -1})
	for _, s := range scopes {
		s.Parent = 0
		f.Scopes = append(f.Scopes, s.Scope)
	}
	stack := []int32{0}
	for i := 1; i < len(f.Scopes); i++ {
		for len(stack) > 1 && !f.Scopes[stack[len(stack)-1]].Contains(f.Scopes[i].Span) {
			stack = stack[:len(stack)-1]
		}
		f.Scopes[i].Parent = stack[len(stack)-1]
		stack = append(stack, int32(i))
	}

	// An import's path matched alone, beside the same import matched with
	// what it brings in: that's the one kept. (So is C#'s using static over
	// its plain using.)
	weak := map[string]bool{"blank": true, "using": true}
	brings := map[Span]bool{}
	for _, im := range f.Imports {
		if !weak[im.Kind] {
			brings[im.PathAt] = true
		}
	}
	imports := f.Imports[:0]
	for _, im := range f.Imports {
		if !weak[im.Kind] || !brings[im.PathAt] {
			imports = append(imports, im)
		}
	}
	f.Imports = imports

	// Definitions, by where their names are, each in its scope.
	defs = dropBlank(defs)
	for i := range defs {
		defs[i].Exported = defs[i].Exported || exported[defs[i].At]
		defs[i].Default = defaults[defs[i].At]
	}
	sort.SliceStable(defs, func(i, j int) bool { return defs[i].At.Start < defs[j].At.Start })
	owner := map[Span]int32{} // a scope's span: the definition that opens it
	for i := range defs {
		d := &defs[i]
		if d.Extent.None() {
			d.Extent = d.At
		}
		owner[d.Extent] = int32(i)
	}
	for i := range defs {
		d := &defs[i]
		s := f.innermost(d.At)
		if f.Scopes[s].Span == d.Extent && s > 0 {
			s = int(f.Scopes[s].Parent)
		}
		d.Scope = int32(s)
	}
	// global x and nonlocal x: x assigned in that function isn't its own.
	if len(globals) > 0 {
		outside := map[[2]any]bool{}
		for _, g := range globals {
			outside[[2]any{string(src[g.Start:g.End]), int32(f.innermost(g))}] = true
		}
		kept := defs[:0]
		for _, d := range defs {
			if !outside[[2]any{d.Name, d.Scope}] {
				kept = append(kept, d)
			}
		}
		defs = kept
	}
	var types []int
	for i, d := range defs {
		if typeKinds[d.Kind] {
			types = append(types, i)
		}
	}
	for i := range defs {
		d := &defs[i]
		if sc := f.Scopes[d.Scope]; sc.Kind == "class" {
			if o, ok := owner[sc.Span]; ok && int(o) != i {
				d.Container = o
				continue
			}
		}
		if !memberKinds[d.Kind] {
			continue
		}
		best := -1
		for _, t := range types {
			if t != i && defs[t].Extent.Contains(d.At) && (best < 0 || defs[best].Extent.Contains(defs[t].Extent)) {
				best = t
			}
		}
		d.Container = int32(best)
	}
	f.Defs = defs

	// Uses: one per name, the most telling capture of it, less those that
	// are definitions or imports.
	taken := map[Span]bool{pkgAt: true}
	for sp := range skipped {
		taken[sp] = true
	}
	for _, d := range defs {
		taken[d.At] = true
	}
	for _, im := range f.Imports {
		taken[im.AliasAt], taken[im.NameAt] = true, true
	}
	// Names inside an import's path, or a namespace's name, are the path.
	for _, im := range f.Imports {
		within = append(within, im.PathAt)
	}
	inside := func(at Span) bool {
		for _, w := range within {
			if w.Contains(at) && w != at {
				return true
			}
		}
		return false
	}
	var kept []Use
	for _, u := range uses {
		if taken[u.At] || u.Name == "_" || u.Name == "" || inside(u.At) {
			continue
		}
		if i, ok := useAt[u.At]; ok {
			if usePriority(u) > usePriority(kept[i]) {
				kept[i] = u
			}
			continue
		}
		useAt[u.At] = len(kept)
		kept = append(kept, u)
	}
	sort.SliceStable(kept, func(i, j int) bool { return kept[i].At.Start < kept[j].At.Start })
	f.Uses = kept
	sort.SliceStable(f.Calls, func(i, j int) bool { return f.Calls[i].At.Start < f.Calls[j].At.Start })
	return f.index()
}

func dropBlank(defs []Def) []Def {
	out := defs[:0]
	for _, d := range defs {
		if d.Name != "_" && d.Name != "" {
			out = append(out, d)
		}
	}
	return out
}

// innermost is the innermost scope that holds s.
func (f *Facts) innermost(s Span) int {
	// The last scope starting at or before s; then out, until one holds it.
	i := sort.Search(len(f.Scopes), func(i int) bool { return f.Scopes[i].Start > s.Start }) - 1
	for i > 0 && !f.Scopes[i].Contains(s) {
		i = int(f.Scopes[i].Parent)
	}
	if i < 0 {
		return 0
	}
	return i
}

// UseAt is the use at a byte, if any.
func (f *Facts) UseAt(pos int32) (Use, bool) {
	i := sort.Search(len(f.Uses), func(i int) bool { return f.Uses[i].At.End > pos })
	if i < len(f.Uses) && f.Uses[i].At.Start <= pos {
		return f.Uses[i], true
	}
	return Use{}, false
}

// DefAt is the definition whose name is at a byte, if any.
func (f *Facts) DefAt(pos int32) (int, bool) {
	i := sort.Search(len(f.Defs), func(i int) bool { return f.Defs[i].At.End > pos })
	if i < len(f.Defs) && f.Defs[i].At.Start <= pos {
		return i, true
	}
	return -1, false
}

// UseOf is the use whose whole expression is a span: a name, or a
// member with what it's of.
func (f *Facts) UseOf(s Span) (Use, bool) {
	i := sort.Search(len(f.Uses), func(i int) bool { return f.Uses[i].At.End >= s.End })
	for ; i < len(f.Uses) && f.Uses[i].At.End == s.End; i++ {
		if f.Uses[i].Expr() == s {
			return f.Uses[i], true
		}
	}
	return Use{}, false
}

// CallAt is the call that's exactly a span, if any.
func (f *Facts) CallAt(s Span) (Call, bool) {
	i := sort.Search(len(f.Calls), func(i int) bool { return f.Calls[i].At.Start >= s.Start })
	for ; i < len(f.Calls) && f.Calls[i].At.Start == s.Start; i++ {
		if f.Calls[i].At == s {
			return f.Calls[i], true
		}
	}
	return Call{}, false
}

// ImportAt is the import whose name or path is at a byte, if any.
func (f *Facts) ImportAt(pos int32) (int, bool) {
	at := Span{pos, pos + 1}
	for i, im := range f.Imports {
		if im.AliasAt.Contains(at) || im.NameAt.Contains(at) || im.PathAt.Contains(at) {
			return i, true
		}
	}
	return -1, false
}

// Line is the line a byte is on, from 1, and the byte its line starts at.
func (f *Facts) Line(pos int32) (line int, start int32) {
	i := max(sort.Search(len(f.Lines), func(i int) bool { return f.Lines[i] > pos })-1, 0)
	return i + 1, f.Lines[i]
}

// Offset is the byte a line (from 1) starts at, and whether there's such
// a line.
func (f *Facts) Offset(line int) (int32, bool) {
	if line < 1 || line > len(f.Lines) {
		return 0, false
	}
	return f.Lines[line-1], true
}

func lineStarts(src []byte) []int32 {
	out := []int32{0}
	for i, b := range src {
		if b == '\n' && i+1 < len(src) {
			out = append(out, int32(i+1))
		}
	}
	return out
}

// unquote is an import's path as meant: without its quotes.
func unquote(s string) string {
	if len(s) >= 2 {
		switch s[0] {
		case '"', '\'', '`':
			if s[len(s)-1] == s[0] {
				return s[1 : len(s)-1]
			}
		}
	}
	return s
}
