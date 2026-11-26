package nav

import (
	"fmt"
	"path"
	"sort"
	"strings"
	"time"
)

// The walk: from a name at a place to where it's defined. Scopes, then
// imports, then what the file shares without importing, then builtins,
// then by name; a member is looked up in what it's of, by the type
// written on it.

// site is a definition in a file.
type site struct {
	path string
	f    *Facts
	i    int
}

func (s site) def() Def { return s.f.Defs[s.i] }

// res is what a name was found to be.
type res struct {
	kind string // def, module, external, builtin, candidates, none
	defs []site
	how  string
	// A module: an import's files in the repo, or a dependency.
	mod      Where
	modPath  string
	external string // a dependency's name for the thing: Println of fmt
	// value: an external is something of the dependency's, not the
	// module itself; typeName is its type, if that's known.
	value    bool
	typeName string
	// likely: an external guessed from what's in reach (C#'s usings), its
	// namespace one of several or unknown.
	likely  bool
	partial bool
}

func (r res) site() (site, bool) {
	if r.kind == "def" && len(r.defs) > 0 {
		return r.defs[0], true
	}
	return site{}, false
}

var none = res{kind: "none"}

// Limits on one question.
const (
	maxDepth      = 8
	maxGrepFiles  = 400
	maxCandidates = 20
	questionTime  = 2 * time.Second
)

// walker answers one question in one tree.
type walker struct {
	s        *Service
	t        Tree
	lang     Lang
	deadline time.Time
	partial  bool
	memo     map[string]any
	srcs     map[string][]byte
	nest     int // how deep name has called itself
}

func (s *Service) walker(t Tree, l Lang) *walker {
	return &walker{s: s, t: t, lang: l, deadline: time.Now().Add(questionTime), memo: map[string]any{}, srcs: map[string][]byte{}}
}

func (w *walker) Tree() Tree { return w.t }

func (w *walker) Facts(p string) (*Facts, error) { return w.s.Facts(w.t, p) }

// Memo is something worked out about the commit: kept with its tree.
func (w *walker) Memo(key string, fn func() any) any {
	if t, ok := w.t.(*gitTree); ok {
		return t.remember(key, fn)
	}
	if v, ok := w.memo[key]; ok {
		return v
	}
	v := fn()
	w.memo[key] = v
	return v
}

func (w *walker) late() bool {
	if time.Now().After(w.deadline) {
		w.partial = true
		return true
	}
	return false
}

func (w *walker) src(p string) []byte {
	if b, ok := w.srcs[p]; ok {
		return b
	}
	b, _ := w.t.Read(p)
	w.srcs[p] = b
	return b
}

func (w *walker) text(p string, s Span) string {
	b := w.src(p)
	if int(s.End) > len(b) || s.Start > s.End {
		return ""
	}
	return string(b[s.Start:s.End])
}

// at is what's at a byte of a file: a definition, an import, or a use.
func (w *walker) at(p string, f *Facts, pos int32) (res, string) {
	if i, ok := f.DefAt(pos); ok {
		return res{kind: "def", defs: []site{{p, f, i}}, how: "defined here"}, f.Defs[i].Name
	}
	if i, ok := f.ImportAt(pos); ok {
		im := f.Imports[i]
		return w.importRes(p, im), im.Path
	}
	if u, ok := f.UseAt(pos); ok {
		return w.use(p, f, u, 0, false), u.Name
	}
	return none, ""
}

// use is where a use leads. exact: no guessing by name.
func (w *walker) use(p string, f *Facts, u Use, depth int, exact bool) res {
	if depth > maxDepth {
		return none
	}
	if u.Part == 'm' {
		return w.member(p, f, u, depth, exact)
	}
	return w.name(p, f, u.Name, u.At, u.In, exact)
}

// maxNest is how deep the walk calls itself before it gives up on a
// question: a guard against a loop no other guard catches.
const maxNest = 64

// name is where a name used at a place leads.
func (w *walker) name(p string, f *Facts, name string, at Span, in byte, exact bool) res {
	if w.nest++; w.nest > maxNest {
		w.nest--
		w.partial = true
		return none
	}
	defer func() { w.nest-- }()
	// The file's scopes, innermost out.
	if i, ok := w.inScopes(f, name, at); ok {
		how := "local"
		if f.Scopes[f.Defs[i].Scope].Kind == "file" {
			how = "in this file"
		}
		return res{kind: "def", defs: []site{{p, f, i}}, how: how}
	}
	// A member of the class around it, by its name alone (C#). Not while
	// that class's own bases are being read for it.
	if w.lang.BareMembers() {
		if class, ok := w.classAround(f, at); ok {
			key := fmt.Sprintf("bare:%s:%d", p, class)
			if w.memo[key] == nil {
				w.memo[key] = true
				found := w.members(site{p, f, class}, name, 0)
				delete(w.memo, key)
				if len(found) > 0 {
					return res{kind: "def", defs: found, how: "a member of " + f.Defs[class].Name}
				}
			}
		}
	}
	// The file's imports.
	for _, im := range f.Imports {
		if im.Reexport || im.Kind == "blank" || im.Kind == "star" || im.Kind == "using" || im.Kind == "global" {
			continue
		}
		if im.Kind == "static" || im.Kind == "typealias" {
			// Resolving the using's own path doesn't go through it again.
			key := fmt.Sprintf("using:%s:%d", p, im.PathAt.Start)
			if w.memo[key] != nil {
				continue
			}
			w.memo[key] = true
			r, ok := w.using(p, f, im, name, at)
			delete(w.memo, key)
			if ok {
				return r
			}
			continue
		}
		local := im.Local()
		if local == "" && im.Kind == "module" {
			local = w.lang.ImportName(w, p, im)
		}
		if local == name {
			return w.importRes(p, im)
		}
	}
	// What it shares without importing: a package's other files.
	var shared []site
	for _, other := range w.lang.Shared(w, p, f) {
		of, err := w.Facts(other)
		if err != nil {
			continue
		}
		for _, i := range of.Named(name) {
			if d := of.Defs[i]; d.Scope == 0 && !memberKinds[d.Kind] {
				shared = append(shared, site{other, of, i})
			}
		}
	}
	if len(shared) > 0 {
		return res{kind: "def", defs: shared, how: "in this package"}
	}
	// A type in a namespace in reach (C#, TypeSpec).
	if w.lang.Namespaces() {
		if r := w.inReach(p, f, name, at, false); r.kind != "none" {
			return r
		}
	}
	// What a star import brings in.
	for _, im := range f.Imports {
		if im.Kind != "star" {
			continue
		}
		if r := w.inModule(w.lang.Import(w, p, im), im.Path, name); r.kind == "def" {
			r.how = "imported from " + im.Path
			return r
		}
	}
	if w.lang.Builtin(name) {
		return res{kind: "builtin", external: name}
	}
	// Not the repo's, nor known: a guess, said as one. A type (C#'s
	// usings) from a namespace in reach the repo doesn't declare, or a
	// member the class around it has from a package's base class
	// (Response, in an ASP.NET controller); both, when both may be.
	if w.lang.Namespaces() && (!exact || in == 't') && (startsUpper(name) || !w.lang.BareMembers()) {
		ns := w.inReach(p, f, name, at, true)
		var base typ
		if in != 't' && w.lang.BareMembers() {
			if class, ok := w.classAround(f, at); ok {
				key := fmt.Sprintf("bare:%s:%d", p, class)
				if w.memo[key] == nil {
					w.memo[key] = true
					base, _ = w.externalBase(site{p, f, class}, 0)
					delete(w.memo, key)
				}
			}
		}
		switch {
		case ns.kind != "none" && base.name != "":
			return res{kind: "external", external: name, likely: true, how: "not the repo's: " + strings.TrimPrefix(ns.how, "likely ") + ", or a member of " + base.name}
		case ns.kind != "none":
			return ns
		case base.name != "":
			r := w.external(base.mod, base.name, name, "a member of "+base.name+", likely")
			r.likely = true
			return r
		}
	}
	if exact {
		return none
	}
	return w.byName(p, f, name, in, false)
}

// using is what a C# using static or using alias makes of a name: a
// member of the static's type, or the alias's type or namespace.
func (w *walker) using(p string, f *Facts, im Import, name string, at Span) (res, bool) {
	if im.Kind == "static" {
		if t, ok := w.qualifiedType(p, f, im.Path, at); ok {
			if found := w.members(t, name, 0); len(found) > 0 {
				return res{kind: "def", defs: found, how: "using static " + im.Path}, true
			}
		}
		return none, false
	}
	if im.Alias != name {
		return none, false
	}
	how := "using " + im.Alias + " = " + im.Path
	if t, ok := w.typeText(p, f, im.Path, at, 0); ok {
		if t.in {
			return res{kind: "def", defs: w.partsOf(t.site), how: how}, true
		}
		return res{kind: "external", modPath: t.mod, mod: Where{External: t.mod}, external: t.name, how: how}, true
	}
	if !w.declaresNamespace(im.Path) {
		// A package's namespace.
		return res{kind: "external", modPath: im.Path, mod: Where{External: im.Path}, how: how}, true
	}
	return res{kind: "namespace", modPath: im.Path, how: how}, true
}

// inScopes is the definition of a name in reach of a place in its file:
// the innermost scope's that has one, and the first of those there.
func (w *walker) inScopes(f *Facts, name string, at Span) (int, bool) {
	named := f.Named(name)
	if len(named) == 0 {
		return -1, false
	}
	s := f.innermost(at)
	inFunction := false
	for {
		sc := f.Scopes[s]
		if sc.Kind == "class" && inFunction && !w.lang.SeesClass() {
			// A method doesn't see its class's body (Python).
			s = int(sc.Parent)
			continue
		}
		inFunction = inFunction || sc.Kind == "function"
		ordered := sc.Kind == "function" || sc.Kind == "block"
		for _, i := range named {
			d := f.Defs[i]
			if int(d.Scope) != s || memberKinds[d.Kind] {
				continue // a member is reached through what it's of
			}
			if ordered && !w.lang.Hoists(sc.Kind, d) {
				// Not before it's written; and a variable not inside its own
				// statement (n := n + 1 is the n before), where a function or
				// a type can name itself.
				opens := d.Kind == "func" || d.Kind == "method" || typeKinds[d.Kind]
				if d.At.Start > at.Start || !opens && d.Extent != d.At && d.Extent.End > at.Start {
					continue
				}
			}
			return i, true
		}
		if s == 0 {
			return -1, false
		}
		s = int(sc.Parent)
	}
}

// importRes is what an import is: the module it brings in.
func (w *walker) importRes(p string, im Import) res {
	where := w.lang.Import(w, p, im)
	if where.External != "" {
		return res{kind: "external", mod: where, modPath: im.Path, how: "a dependency"}
	}
	if len(where.Files) == 0 {
		return res{kind: "none", how: "imports " + im.Path + ", which isn't here"}
	}
	if im.Kind == "named" || im.Kind == "default" {
		name := im.Name
		if im.Kind == "default" {
			name = "default"
		}
		r := w.inModule(where, im.Path, name)
		r.how = "imported from " + im.Path
		return r
	}
	return res{kind: "module", mod: where, modPath: im.Path, how: "imports " + im.Path}
}

// inModule is a name a module exports: defined there, or re-exported
// (export { a as b } from, export * from, export default x).
func (w *walker) inModule(where Where, modPath, name string) res {
	return w.inModuleAt(where, modPath, name, 0)
}

func (w *walker) inModuleAt(where Where, modPath, name string, depth int) res {
	if where.External != "" {
		return res{kind: "external", mod: where, modPath: modPath, external: name, how: "from " + modPath}
	}
	if depth > maxDepth || w.late() {
		return none
	}
	var found []site
	for _, p := range where.Files {
		f, err := w.Facts(p)
		if err != nil {
			continue
		}
		for i, d := range f.Defs {
			if d.Scope != 0 || memberKinds[d.Kind] {
				continue
			}
			if name == "default" && d.Default || d.Name == name && w.lang.Exported(d) {
				found = append(found, site{p, f, i})
			}
		}
	}
	if len(found) > 0 {
		return res{kind: "def", defs: found, how: "from " + modPath}
	}
	if sub, ok := w.lang.Submodule(w, where, name); ok {
		return res{kind: "module", mod: sub, modPath: modPath + "." + name, how: "imports " + modPath + "." + name}
	}
	// Re-exported: followed to where it's defined.
	for _, p := range where.Files {
		f, err := w.Facts(p)
		if err != nil {
			continue
		}
		for _, im := range f.Imports {
			if !im.Reexport && w.lang.ImportsExported() && im.Kind != "star" && im.Kind != "blank" {
				// What it imports, it exports (Python's __init__.py).
				local := im.Local()
				if local == "" {
					local = w.lang.ImportName(w, p, im)
				}
				if local == name {
					return w.importRes(p, im)
				}
				continue
			}
			if !im.Reexport && !(w.lang.ImportsExported() && im.Kind == "star") {
				continue
			}
			switch {
			case im.Kind == "star" && im.Path != "":
				if r := w.inModuleAt(w.lang.Import(w, p, im), im.Path, name, depth+1); r.kind == "def" || r.kind == "external" {
					return r
				}
			case im.Kind == "namespace" && im.Local() == name:
				return w.importRes(p, Import{Kind: "namespace", Path: im.Path})
			case im.Kind == "default" && name == "default", im.Kind == "named" && im.Local() == name:
				if im.Path == "" {
					// export { a as b }, export default a: a, here.
					if i, ok := w.inScopes(f, im.Name, Span{int32(len(w.src(p))), int32(len(w.src(p)))}); ok {
						return res{kind: "def", defs: []site{{p, f, i}}, how: "from " + modPath}
					}
					continue
				}
				if r := w.inModuleAt(w.lang.Import(w, p, im), im.Path, im.Name, depth+1); r.kind != "none" {
					return r
				}
			}
		}
	}
	return none
}

// member is a.b: b looked up in what a is.
func (w *walker) member(p string, f *Facts, u Use, depth int, exact bool) res {
	objText := w.text(p, u.Object)
	var obj res
	if strings.HasPrefix(objText, "global::") {
		obj = none // C#'s global::System: a namespace's path, read below
	} else {
		obj = w.expr(p, f, u.Object, depth+1)
	}
	switch {
	case obj.kind == "builtin":
		// A member of the language's own type: Task.CompletedTask.
		return w.external("", obj.external, u.Name, "a member of "+obj.external)
	case obj.kind == "external" && !obj.value && obj.modPath == "" && obj.external != "":
		// A member of a type guessed to be a package's.
		return res{kind: "external", external: obj.external + "." + u.Name, likely: true, how: obj.how}
	case obj.kind == "module" || obj.kind == "external" && !obj.value:
		r := w.inModule(obj.mod, obj.modPath, u.Name)
		if r.kind == "def" || r.kind == "external" {
			r.how = "imported from " + obj.modPath
		}
		if r.kind != "none" || exact {
			return r
		}
	case obj.kind == "external" && obj.typeName != "":
		// A value of a dependency's type (or the language's own): that
		// type's member.
		return w.external(obj.modPath, obj.typeName, u.Name, "by the type written on "+short40(objText))
	case obj.kind == "external":
		r := w.unknownMember(p, f, u, obj.modPath, exact)
		r.likely = r.likely || obj.likely
		return r
	case obj.kind == "def":
		s, _ := obj.site()
		how := "a member of " + s.def().Name
		ty := typ{site: s, in: true}
		if alias := s.def().Kind == "type" && s.def().Typed.Text != ""; !typeKinds[s.def().Kind] || alias {
			var ok bool
			if ty, ok = w.typeOf(s, depth+1); !ok {
				break
			}
			how = "by the type written on " + short40(objText)
			for _, word := range w.lang.Self() {
				if objText == word && ty.in {
					how = "a member of " + ty.def().Name
				}
			}
		}
		if !ty.in && ty.name == "" {
			return w.unknownMember(p, f, u, ty.mod, exact)
		}
		if !ty.in {
			return w.external(ty.mod, ty.name, u.Name, how)
		}
		if found := w.members(ty.site, u.Name, depth+1); len(found) > 0 {
			return res{kind: "def", defs: found, how: how}
		}
		// Not its own: an embedded dependency's, like a sync.Mutex's Lock.
		if bt, ok := w.externalBase(ty.site, depth+1); ok {
			return w.external(bt.mod, bt.name, u.Name, how)
		}
	}
	if obj.kind == "namespace" || obj.kind == "none" && w.lang.Namespaces() {
		// A type in a namespace: Contoso.Billing.Invoice.
		ns := obj.modPath
		if obj.kind == "none" {
			ns = strings.TrimPrefix(strings.Join(strings.Fields(objText), ""), "global::")
		}
		if r := w.inNamespaceFrom(p, f, ns, u.Name, u.At); r.kind != "none" {
			return r
		}
		// A namespace, one more part of a path: the repo's, or (one it
		// doesn't declare) a package's or .NET's.
		if isQualified(ns) && startsUpper(ns) {
			if w.declaresNamespace(ns + "." + u.Name) {
				return res{kind: "namespace", modPath: ns + "." + u.Name}
			}
			if !w.declaresNamespace(ns) {
				return res{kind: "external", modPath: ns, mod: Where{External: ns}, external: u.Name, how: "in " + ns}
			}
		}
	}
	if exact {
		return none
	}
	return w.byName(p, f, u.Name, u.In, true)
}

// unknownMember is a member of something a dependency (or the language)
// gave, its type unknown: useRef's current, a Map's get. When the repo
// defines one member of that name, it's likely that (a type passed to
// the dependency and handed back); otherwise it's the dependency's.
func (w *walker) unknownMember(p string, f *Facts, u Use, mod string, exact bool) res {
	if !exact {
		if r := w.byName(p, f, u.Name, u.In, true); r.kind == "candidates" && len(r.defs) == 1 {
			return r
		}
	}
	how := "on something from " + mod
	if mod == "" {
		how = "on something built in"
	}
	return res{kind: "external", modPath: mod, mod: Where{External: mod}, value: true, how: how}
}

// external is a member of a dependency's type: Mutex.Lock of sync, or
// of something of it whose type isn't known. What it is, in turn, isn't
// known.
func (w *walker) external(mod, typeName, member, how string) res {
	name := ""
	if typeName != "" {
		name = typeName + "." + member
	}
	return res{kind: "external", modPath: mod, mod: Where{External: mod}, external: name, value: true, how: how}
}

// externalBase is a type's first base that's a dependency's.
func (w *walker) externalBase(t site, depth int) (typ, bool) {
	for _, b := range t.def().Bases {
		if bt, ok := w.typeText(t.path, t.f, b.Text, b.Span, depth); ok && !bt.in {
			return bt, true
		}
	}
	return typ{}, false
}

// partsOf is a type and, where a type can be declared in parts (C#'s
// partial), its other parts.
func (w *walker) partsOf(t site) []site {
	parts := []site{t}
	if !w.lang.Namespaces() || !typeKinds[t.def().Kind] {
		return parts
	}
	ns := namespaceOf(t.f, t.def())
	for _, other := range w.namedTypes(t.def().Name) {
		if (other.path != t.path || other.i != t.i) && other.def().Kind == t.def().Kind && namespaceOf(other.f, other.def()) == ns {
			parts = append(parts, other)
		}
	}
	return parts
}

// namespaceOf is the namespace a definition is declared in: its file's,
// then the namespace blocks around it, outermost first.
func namespaceOf(f *Facts, d Def) string {
	return namespaceAt(f, d.Scope)
}

func namespaceAt(f *Facts, scope int32) string {
	var parts []string
	for s := scope; s > 0; s = f.Scopes[s].Parent {
		if f.Scopes[s].Kind == "namespace" {
			parts = append([]string{strings.Join(strings.Fields(f.Scopes[s].Name), "")}, parts...)
		}
	}
	if f.Namespace != "" {
		parts = append([]string{f.Namespace}, parts...)
	}
	return strings.Join(parts, ".")
}

// namedTypes are the repo's types of a name, at a namespace's top (not
// nested in another type).
func (w *walker) namedTypes(name string) []site {
	return w.Memo("types:"+name, func() any {
		hits, err := w.t.Grep(name, w.lang.Suffixes()...)
		if err != nil {
			return []site(nil)
		}
		var out []site
		seen := map[string]bool{}
		for _, h := range hits {
			if seen[h.Path] || len(seen) >= maxGrepFiles {
				continue
			}
			seen[h.Path] = true
			f, err := w.Facts(h.Path)
			if err != nil {
				continue
			}
			for _, i := range f.Named(name) {
				d := f.Defs[i]
				sc := f.Scopes[d.Scope].Kind
				if (typeKinds[d.Kind] || d.Kind == "namespace" || d.Kind == "func") && (sc == "file" || sc == "namespace") {
					out = append(out, site{h.Path, f, i})
				}
			}
		}
		return out
	}).([]site)
}

// inReach is a type of a name in the nearest namespace in reach that has
// one. One that isn't the repo's is likely from a namespace in reach the
// repo doesn't declare (a package's).
func (w *walker) inReach(p string, f *Facts, name string, at Span, guess bool) res {
	reach := w.lang.Reach(w, p, f, at)
	if r := w.typeInReach(name, reach); r.kind != "none" {
		return r
	}
	// C#'s attributes are written without their Attribute.
	if r := w.typeInReach(name+"Attribute", reach); r.kind != "none" {
		return r
	}
	if !guess || w.lang.BareMembers() && !startsUpper(name) {
		return none // C#'s lowercase names are locals, not types
	}
	var outside []string
	for _, ns := range reach {
		if ns != "" && !w.declaresNamespace(ns) {
			outside = append(outside, ns)
		}
	}
	if len(outside) == 0 {
		return none
	}
	from := strings.Join(outside, " or ")
	if len(outside) > 2 {
		from = fmt.Sprintf("%s, %s or %d more of its usings", outside[0], outside[1], len(outside)-2)
	}
	r := res{kind: "external", external: name, likely: true, how: "likely from " + from}
	if len(outside) == 1 {
		r.modPath, r.mod = outside[0], Where{External: outside[0]}
	}
	return r
}

// declaresNamespace is whether the repo declares a namespace (or one in
// it).
func (w *walker) declaresNamespace(ns string) bool {
	return w.Memo("ns.declared:"+ns, func() any {
		hits, err := w.t.Grep("namespace "+ns, w.lang.Suffixes()...)
		return err == nil && len(hits) > 0
	}).(bool)
}

func (w *walker) typeInReach(name string, reach []string) res {
	types := w.namedTypes(name)
	if len(types) == 0 {
		return none
	}
	for _, ns := range reach {
		var found []site
		for _, t := range types {
			if namespaceOf(t.f, t.def()) == ns {
				found = append(found, t)
			}
		}
		if len(found) > 0 {
			how := "in " + ns
			if ns == "" {
				how = "in the global namespace"
			}
			return res{kind: "def", defs: found, how: how}
		}
	}
	return none
}

// inNamespaceFrom is a type in a namespace written at a place: as it's
// written, or inside one of the namespaces around it.
func (w *walker) inNamespaceFrom(p string, f *Facts, ns, name string, at Span) res {
	var found []site
	types := w.namedTypes(name)
	for _, outer := range append(w.enclosingNamespaces(f, at), "") {
		full := ns
		if outer != "" {
			full = outer + "." + ns
		}
		for _, t := range types {
			if namespaceOf(t.f, t.def()) == full {
				found = append(found, t)
			}
		}
		if len(found) > 0 {
			return res{kind: "def", defs: found, how: "in " + full}
		}
	}
	return none
}

// enclosingNamespaces are the namespace a place is in and those it's in,
// innermost first: A.B.C, A.B, A.
func (w *walker) enclosingNamespaces(f *Facts, at Span) []string {
	full := namespaceAt(f, int32(f.innermost(at)))
	var out []string
	for full != "" {
		out = append(out, full)
		i := strings.LastIndexByte(full, '.')
		if i < 0 {
			break
		}
		full = full[:i]
	}
	return out
}

// qualifiedType is the type a qualified name means, written at a place.
func (w *walker) qualifiedType(p string, f *Facts, qualified string, at Span) (site, bool) {
	t, ok := w.typeText(p, f, qualified, at, 0)
	if !ok || !t.in {
		return site{}, false
	}
	return t.site, true
}

// typ is a type: one in the repo, or a dependency's (its module, and its
// name if known).
type typ struct {
	site
	in   bool
	mod  string
	name string
}

// asType is what a name was found to be, as a type.
func (w *walker) asType(r res, depth int) (typ, bool) {
	switch r.kind {
	case "external":
		if r.value {
			return typ{mod: r.modPath, name: r.typeName}, true
		}
		return typ{mod: r.modPath, name: r.external}, true
	case "builtin":
		return typ{name: r.external}, true // the language's own: Go's error, TypeScript's String
	case "def":
		s, _ := r.site()
		if typeKinds[s.def().Kind] {
			return typ{site: s, in: true}, true
		}
		return w.typeOf(s, depth+1)
	}
	return typ{}, false
}

// value is a value of a type, as what a name was found to be.
func (t typ) value() res {
	if t.in {
		return res{kind: "def", defs: []site{t.site}}
	}
	return res{kind: "external", modPath: t.mod, mod: Where{External: t.mod}, external: t.name, value: true, typeName: t.name}
}

// expr is what an expression is: a name, a member, a call, or a type
// written out.
func (w *walker) expr(p string, f *Facts, s Span, depth int) res {
	if depth > maxDepth || w.late() {
		return none
	}
	if u, ok := f.UseOf(s); ok {
		return w.use(p, f, u, depth, true)
	}
	if c, ok := f.CallAt(s); ok {
		if w.text(p, c.Callee) == "super" {
			// Python's super(): the class's first base.
			if t, ok := w.self(p, f, "super", c.At, depth); ok {
				return t.value()
			}
			return none
		}
		callee := w.expr(p, f, c.Callee, depth+1)
		if (callee.kind == "external" || callee.kind == "builtin") && !callee.value && startsUpper(lastPart(callee.external)) {
			// A dependency's class (or the language's own), made: an
			// instance of it. Path(…), new Map().
			return res{kind: "external", modPath: callee.modPath, mod: callee.mod, value: true, typeName: callee.external}
		}
		if callee.kind == "external" {
			// A dependency's function: what it returns isn't known, but
			// it's of that dependency.
			return res{kind: "external", modPath: callee.modPath, mod: callee.mod, value: true}
		}
		cs, ok := callee.site()
		if !ok {
			return none
		}
		if typeKinds[cs.def().Kind] {
			return callee // a constructor: an instance of it
		}
		if ty, ok := w.typeOf(cs, depth+1); ok {
			return ty.value()
		}
		return none
	}
	text := w.text(p, s)
	if t, ok := w.self(p, f, text, s, depth); ok {
		return t.value()
	}
	if lit := w.lang.Literal(text); lit != "" {
		return typ{name: lit}.value() // "x".join: str's
	}
	if t, ok := w.typeText(p, f, text, s, depth); ok {
		return t.value()
	}
	return none
}

// self is this (or super) where it's written: the class around it, or
// that class's first base.
func (w *walker) self(p string, f *Facts, text string, at Span, depth int) (typ, bool) {
	isSelf := false
	for _, s := range w.lang.Self() {
		isSelf = isSelf || text == s
	}
	if text == "base" {
		text = "super" // C#'s
	}
	if !isSelf && text != "super" {
		return typ{}, false
	}
	class, ok := w.classAround(f, at)
	if !ok {
		return typ{}, false
	}
	if text != "super" {
		return typ{site: site{p, f, class}, in: true}, true
	}
	for _, b := range f.Defs[class].Bases {
		if t, ok := w.typeText(p, f, b.Text, b.Span, depth+1); ok {
			return t, true
		}
	}
	return typ{}, false
}

// classAround is the innermost class whose declaration holds a place.
func (w *walker) classAround(f *Facts, at Span) (int, bool) {
	best := -1
	for i, d := range f.Defs {
		if (d.Kind == "class" || d.Kind == "struct" || d.Kind == "interface" || d.Kind == "enum") && d.Extent.Contains(at) && (best < 0 || f.Defs[best].Extent.Contains(d.Extent)) {
			best = i
		}
	}
	return best, best >= 0
}

// typeOf is the type a definition holds: written on it, made from a
// literal, or returned by what it's set from. A function's is what it
// returns.
func (w *walker) typeOf(s site, depth int) (typ, bool) {
	if depth > maxDepth {
		return typ{}, false
	}
	d := s.def()
	if d.Kind == "type" && d.Typed.Text != "" {
		// An alias (TypeSpec's alias X = Y) is what it names.
		if t, ok := w.typeText(s.path, s.f, d.Typed.Text, d.Typed.Span, depth+1); ok {
			return t, true
		}
	}
	if typeKinds[d.Kind] {
		return typ{site: s, in: true}, true
	}
	if d.Kind == "param" {
		for _, word := range w.lang.Self() {
			if d.Name == word {
				if class, ok := w.classAround(s.f, d.At); ok {
					return typ{site: site{s.path, s.f, class}, in: true}, true
				}
			}
		}
	}
	for _, e := range []Expr{d.Typed, d.New} {
		if e.Text != "" {
			if t, ok := w.typeText(s.path, s.f, e.Text, e.Span, depth+1); ok {
				return t, true
			}
		}
	}
	if !d.Elem.None() {
		// for x in xs: what xs holds, by the type written on xs.
		r := w.expr(s.path, s.f, d.Elem, depth+1)
		if xs, ok := r.site(); ok && xs.def().Typed.Text != "" {
			if elem := w.lang.ElemType(xs.def().Typed.Text); elem != "" {
				if t, ok := w.typeText(xs.path, xs.f, elem, xs.def().Typed.Span, depth+1); ok {
					return t, true
				}
			}
		}
	}
	if !d.From.None() {
		r := w.expr(s.path, s.f, d.From, depth+1)
		if r.kind == "external" {
			// What a dependency's function returns: of the dependency,
			// its type unknown.
			return typ{mod: r.modPath}, true
		}
		if fs, ok := r.site(); ok {
			if typeKinds[fs.def().Kind] {
				return typ{site: fs, in: true}, true
			}
			if fs.def().Kind == "func" || fs.def().Kind == "method" {
				return w.typeOf(fs, depth+1)
			}
			if fs != s {
				return w.typeOf(fs, depth+1) // x = y: y's type
			}
		}
	}
	return typ{}, false
}

// typeText is the type a written type names, resolved where it's
// written.
func (w *walker) typeText(p string, f *Facts, written string, at Span, depth int) (typ, bool) {
	names := w.lang.TypeName(written)
	if len(names) == 0 {
		return typ{}, false
	}
	var r res
	if len(names) > 1 && w.lang.Namespaces() {
		// Namespace.Type, in the repo or (a namespace it doesn't declare)
		// a package's.
		ns, last := strings.Join(names[:len(names)-1], "."), names[len(names)-1]
		r = w.inNamespaceFrom(p, f, ns, last, at)
		if r.kind == "none" && !w.declaresNamespace(ns) && len(w.namedTypes(names[0])) == 0 {
			if _, local := w.inScopes(f, names[0], at); !local {
				return typ{mod: ns, name: last}, true
			}
		}
		if r.kind != "none" {
			names = names[:1]
		}
	}
	if r.kind == "" || r.kind == "none" {
		r = w.name(p, f, names[0], at, 't', true)
	}
	for _, n := range names[1:] {
		switch r.kind {
		case "module", "external":
			r = w.inModule(r.mod, r.modPath, n)
		case "builtin":
			r.external += "." + n
		case "def":
			s, _ := r.site()
			found := w.members(s, n, depth+1)
			if len(found) == 0 {
				return typ{}, false
			}
			r = res{kind: "def", defs: found}
		default:
			return typ{}, false
		}
	}
	return w.asType(r, depth)
}

// members is a type's members of a name: its own, then its bases',
// breadth first.
func (w *walker) members(ty site, name string, depth int) []site {
	queue := []site{ty}
	seen := map[string]bool{}
	for level := 0; len(queue) > 0 && level < maxDepth && !w.late(); level++ {
		var found []site
		var next []site
		for _, t := range queue {
			key := fmt.Sprintf("%s:%d", t.path, t.i)
			if seen[key] {
				continue
			}
			seen[key] = true
			found = append(found, w.ownMembers(t, name)...)
			for _, part := range w.partsOf(t) {
				for _, b := range part.def().Bases {
					names := w.lang.TypeName(b.Text)
					bt, ok := w.typeText(part.path, part.f, b.Text, b.Span, depth+1)
					if !ok || !bt.in {
						continue
					}
					if w.lang.Name() == "go" && len(names) > 0 && names[len(names)-1] == name {
						found = append(found, bt.site) // an embedded field, by its type's name
						continue
					}
					next = append(next, bt.site)
				}
			}
		}
		if len(found) > 0 {
			return found
		}
		queue = next
	}
	return nil
}

// ownMembers are a type's own members of a name: those inside it, and
// (Go) methods whose receiver is it, in its package.
func (w *walker) ownMembers(t site, name string) []site {
	var out []site
	for _, part := range w.partsOf(t) {
		for _, i := range part.f.Named(name) {
			if int(part.f.Defs[i].Container) == part.i {
				out = append(out, site{part.path, part.f, i})
			}
		}
	}
	files := append([]string{t.path}, w.lang.Shared(w, t.path, t.f)...)
	tname := t.def().Name
	for _, p := range files {
		f, err := w.Facts(p)
		if err != nil {
			continue
		}
		for _, i := range f.Named(name) {
			d := f.Defs[i]
			if d.Receiver.Text == "" {
				continue
			}
			if r := w.lang.TypeName(d.Receiver.Text); len(r) > 0 && r[len(r)-1] == tname {
				out = append(out, site{p, f, i})
			}
		}
	}
	return out
}

// byName is the fallback: every definition of a name in the repo,
// likeliest first.
func (w *walker) byName(p string, f *Facts, name string, in byte, member bool) res {
	hits, err := w.t.Grep(name, w.lang.Suffixes()...)
	if err != nil {
		return none
	}
	var files []string
	seen := map[string]bool{}
	for _, h := range hits {
		if !seen[h.Path] {
			seen[h.Path] = true
			files = append(files, h.Path)
		}
	}
	partial := false
	if len(files) > maxGrepFiles {
		sort.Slice(files, func(i, j int) bool { return closeness(p, files[i]) > closeness(p, files[j]) })
		files, partial = files[:maxGrepFiles], true
	}
	var found []site
	for _, other := range files {
		if w.late() {
			partial = true
			break
		}
		of, err := w.Facts(other)
		if err != nil {
			continue
		}
		for _, i := range of.Named(name) {
			d := of.Defs[i]
			sc := of.Scopes[d.Scope].Kind
			if !memberKinds[d.Kind] && sc != "file" && sc != "namespace" && sc != "class" {
				continue // a local, or a parameter
			}
			if member && !memberKinds[d.Kind] && sc != "class" {
				continue
			}
			found = append(found, site{other, of, i})
		}
	}
	if len(found) == 0 {
		return res{kind: "none", partial: partial}
	}
	near := w.importedDirs(p, f)
	sort.SliceStable(found, func(i, j int) bool {
		return rank(p, found[i], in, member, near) > rank(p, found[j], in, member, near)
	})
	if len(found) > maxCandidates {
		found = found[:maxCandidates]
	}
	return res{kind: "candidates", defs: found, how: "by name", partial: partial}
}

// importedDirs are the folders a file's imports lead to, in the repo.
func (w *walker) importedDirs(p string, f *Facts) map[string]bool {
	out := map[string]bool{}
	for _, im := range f.Imports {
		where := w.lang.Import(w, p, im)
		for _, file := range where.Files {
			out[path.Dir(file)] = true
		}
	}
	return out
}

// rank is how likely a definition is the one meant.
func rank(from string, s site, in byte, member bool, imported map[string]bool) int {
	score := closeness(from, s.path) * 10
	if imported[path.Dir(s.path)] {
		score += 500
	}
	d := s.def()
	var fits bool
	switch {
	case member:
		fits = memberKinds[d.Kind] || d.Kind == "func"
	case in == 'c':
		fits = d.Kind == "func" || d.Kind == "method" || typeKinds[d.Kind]
	case in == 't':
		fits = typeKinds[d.Kind] || d.Kind == "typeparam"
	default:
		fits = !typeKinds[d.Kind]
	}
	if fits {
		score += 1000
	}
	if aside(s.path) {
		score -= 2000
	}
	return score
}

// closeness is how much of two paths' folders are shared: the same file
// most, then the same folder.
func closeness(a, b string) int {
	if a == b {
		return 100
	}
	da, db := strings.Split(path.Dir(a), "/"), strings.Split(path.Dir(b), "/")
	n := 0
	for n < len(da) && n < len(db) && da[n] == db[n] {
		n++
	}
	if n == len(da) && n == len(db) {
		return 50
	}
	return n
}

// aside is whether a file is one that's rarely what's meant: a test,
// generated or vendored.
func aside(p string) bool {
	base := path.Base(p)
	return strings.Contains(p, "vendor/") || strings.Contains(p, "node_modules/") ||
		strings.HasSuffix(base, "_test.go") || strings.Contains(base, ".test.") || strings.Contains(base, ".spec.") ||
		strings.HasPrefix(base, "test_") || strings.Contains(base, ".pb.") || strings.Contains(base, "_gen.") || strings.Contains(base, ".generated.")
}

// genericArg is the first type argument of a generic written with open
// and close (List<Foo>, list[Foo]), or an array's element (Foo[]).
func genericArg(written string, open, close byte) string {
	t := strings.TrimSpace(written)
	if strings.HasSuffix(t, "[]") {
		return strings.TrimSuffix(t, "[]")
	}
	i := strings.IndexByte(t, open)
	if i < 0 || !strings.HasSuffix(t, string(close)) {
		return ""
	}
	return strings.TrimSpace(splitTop(t[i+1:len(t)-1], ',')[0])
}

// isQualified is whether words are a dotted name: A, A.B.
func isQualified(s string) bool {
	if s == "" {
		return false
	}
	for _, part := range strings.Split(s, ".") {
		if part == "" || strings.ContainsAny(part, " ()[]<>\"'") {
			return false
		}
	}
	return true
}

func lastPart(s string) string {
	if i := strings.LastIndexByte(s, '.'); i >= 0 {
		return s[i+1:]
	}
	return s
}

func startsUpper(s string) bool { return s != "" && s[0] >= 'A' && s[0] <= 'Z' }

func short40(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 40 {
		return s[:39] + "…"
	}
	return s
}
