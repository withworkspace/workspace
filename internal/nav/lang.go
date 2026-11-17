package nav

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"path"
	"strings"
)

//go:embed queries/*.scm
var queryFiles embed.FS

// Lang is what's particular to a language: its grammar, its query, and
// how its imports, scopes and builtins work. The walk (walk.go) is the
// same for all of them.
type Lang interface {
	Name() string
	// Suffixes are its files' endings, for grep and listing.
	Suffixes() []string
	// Grammar is the grammar a file of it is parsed with ("typescript"
	// or "tsx"), and Query that grammar's query.
	Grammar(path string) string
	Query(grammar string) string

	// Import is where an import leads, from a file.
	Import(e Env, from string, im Import) Where
	// ImportName is the name an import is known by when none is written
	// (Go: its package's), or "".
	ImportName(e Env, from string, im Import) string
	// Shared is the files that share a file's top scope without
	// importing it: Go's package.
	Shared(e Env, path string, f *Facts) []string
	Builtin(name string) bool
	// Exported is whether a definition can be imported elsewhere.
	Exported(d Def) bool
	// TypeName is a written type as the names to resolve: "*pkg.Foo" is
	// pkg, Foo; nil for one that's no single type.
	TypeName(written string) []string
	// Hoists is whether a definition counts before where it's written,
	// in a scope of that kind.
	Hoists(scope string, d Def) bool
	// Docs is where a dependency's name is documented, or "".
	Docs(module, name string) string
	// BodyOpen is what starts a definition's body, where its signature
	// ends: "{", or Python's ":".
	BodyOpen() string
	// Self is the words for the object a method is of: this, self.
	Self() []string
	// SeesClass is whether a method sees the names its class's body
	// defines, as names (Python's doesn't).
	SeesClass() bool
	// Submodule is a module's submodule of a name (Python's a.b), if
	// there's one.
	Submodule(e Env, where Where, name string) (Where, bool)
	// ImportsExported is whether what a module imports can be imported
	// from it (Python's can).
	ImportsExported() bool
	// Literal is the type of a literal written so ("x" is a string), or
	// "" for none.
	Literal(text string) string
	// Namespaces is whether a type is found by its name and the
	// namespaces in reach (C#, TypeSpec), not by following an import to
	// a file; Reach is those namespaces at a place, nearest first, ""
	// for the global one.
	Namespaces() bool
	Reach(e Env, path string, f *Facts, at Span) []string
	// BareMembers is whether a class's members are names inside it,
	// without this (C#).
	BareMembers() bool
	// ElemType is what a written collection type holds: List<Foo>, Foo[]
	// and list[Foo] hold Foo. "" for none known.
	ElemType(written string) string
}

// Env is what a language asks of the walk: the tree and its files'
// facts, and somewhere to keep what it works out for the question.
type Env interface {
	Tree() Tree
	Facts(path string) (*Facts, error)
	Memo(key string, fn func() any) any
}

// noNamespaces is what a language without them says.
type noNamespaces struct{}

func (noNamespaces) Namespaces() bool                         { return false }
func (noNamespaces) Reach(Env, string, *Facts, Span) []string { return nil }
func (noNamespaces) BareMembers() bool                        { return false }

// Where is where an import leads.
type Where struct {
	Files []string // in the repo: the files it means
	Dir   string   // and the folder they're in, for a package
	// External is the dependency it names, when it isn't in the repo.
	External string
}

// langs are the languages there's navigation for.
var langs = []Lang{golang{}, typescript{}, python{}, csharp{}, tsp{}}

// LangOf is the language a file is, or nil.
func LangOf(p string) Lang {
	base := path.Base(p)
	for _, l := range langs {
		for _, s := range l.Suffixes() {
			if strings.HasSuffix(base, s) {
				return l
			}
		}
	}
	return nil
}

func mustQuery(name string) string {
	b, err := queryFiles.ReadFile("queries/" + name + ".scm")
	if err != nil {
		panic(err)
	}
	return string(b)
}

// factsVersion is changed when what Facts holds, or how it's built,
// changes: facts kept by an older one are made again.
const factsVersion = 7

// version is what facts on disk are kept under: the version, and every
// query, so changing a query makes them again.
var version = func() string {
	h := sha256.New()
	fmt.Fprintf(h, "%d\n", factsVersion)
	entries, _ := queryFiles.ReadDir("queries")
	for _, e := range entries {
		b, _ := queryFiles.ReadFile("queries/" + e.Name())
		fmt.Fprintf(h, "%s\n%s\n", e.Name(), b)
	}
	return hex.EncodeToString(h.Sum(nil))[:12]
}()
