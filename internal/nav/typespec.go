package nav

import (
	"path"
	"strings"
)

// tsp is TypeSpec: imports name files, and names are found as C#'s are,
// by the namespaces in reach.
type tsp struct{}

var tspQuery = mustQuery("typespec")

func (tsp) Name() string                               { return "typespec" }
func (tsp) Suffixes() []string                         { return []string{".tsp"} }
func (tsp) Grammar(string) string                      { return "typespec" }
func (tsp) Query(string) string                        { return tspQuery }
func (tsp) BodyOpen() string                           { return "{" }
func (tsp) Self() []string                             { return nil }
func (tsp) SeesClass() bool                            { return true }
func (tsp) ImportsExported() bool                      { return false }
func (tsp) Namespaces() bool                           { return true }
func (tsp) BareMembers() bool                          { return false }
func (tsp) Shared(Env, string, *Facts) []string        { return nil }
func (tsp) ImportName(Env, string, Import) string      { return "" }
func (tsp) Submodule(Env, Where, string) (Where, bool) { return Where{}, false }
func (tsp) Hoists(string, Def) bool                    { return true }
func (tsp) Exported(Def) bool                          { return true }
func (tsp) Literal(string) string                      { return "" }

// ElemType: Foo[] holds Foo.
func (tsp) ElemType(written string) string { return genericArg(written, '<', '>') }

// TypeName: Page<Foo> is Page, A.B is A, B; Foo[] or a union is no
// single type.
func (tsp) TypeName(written string) []string {
	t := strings.TrimSpace(written)
	if i := strings.IndexByte(t, '<'); i > 0 {
		t = t[:i]
	}
	if t == "" || strings.ContainsAny(t, "[]{}()| ,&") {
		return nil
	}
	return strings.Split(t, ".")
}

// Reach: the namespaces around the place, innermost first; the file's
// usings; then the global namespace (where TypeSpec's own are).
func (tsp) Reach(e Env, p string, f *Facts, at Span) []string {
	w, _ := e.(*walker)
	var out []string
	if w != nil {
		out = append(out, w.enclosingNamespaces(f, at)...)
	}
	for _, im := range f.Imports {
		if im.Kind == "using" {
			out = append(out, strings.Join(strings.Fields(im.Path), ""))
		}
	}
	return append(out, "")
}

// Import: a relative path is a file of the repo (or a folder's
// main.tsp); anything else is a library.
func (tsp) Import(e Env, from string, im Import) Where {
	spec := im.Path
	if strings.HasPrefix(spec, ".") || strings.HasPrefix(spec, "/") {
		p := path.Clean(path.Join(path.Dir(from), spec))
		t := e.Tree()
		for _, c := range []string{p, p + ".tsp", path.Join(p, "main.tsp")} {
			if _, ok := t.Blob(c); ok {
				return Where{Files: []string{c}, Dir: path.Dir(c)}
			}
		}
		return Where{}
	}
	return Where{External: spec}
}

var tspBuiltins = map[string]bool{}

func init() {
	for _, n := range strings.Fields(`string bytes boolean null void never unknown numeric integer float int8 int16 int32 int64
		uint8 uint16 uint32 uint64 safeint float32 float64 decimal decimal128 plainDate plainTime utcDateTime offsetDateTime duration url
		Record Array Model Scalar Enum Union Operation Interface Namespace ModelProperty TemplateParameter
		OptionalProperties UpdateableProperties OmitProperties PickProperties OmitDefaults DefaultKeyVisibility
		doc summary key error format pattern minLength maxLength minItems maxItems minValue maxValue minValueExclusive maxValueExclusive
		encode secret visibility removeVisibility withVisibility service friendlyName knownValues deprecated tag discriminator
		overload projectedName returnsDoc errorsDoc example opExample list offset pageIndex pageSize pageItems continuationToken
		nextLink prevLink firstLink lastLink inspectType inspectTypeName parameterVisibility returnTypeVisibility encodedName mediaTypeHint`) {
		tspBuiltins[n] = true
	}
}

func (tsp) Builtin(name string) bool { return tspBuiltins[name] }

func (tsp) Docs(module, name string) string {
	if module == "" {
		first := strings.SplitN(name, ".", 2)[0]
		if !tspBuiltins[first] {
			return "" // a guess: one of several namespaces' (likely)
		}
		if first != "" && first[0] >= 'a' && first[0] <= 'z' && !strings.ContainsAny(first, "0123456789") && first != "string" && first != "bytes" && first != "boolean" && first != "null" && first != "void" && first != "never" && first != "unknown" && first != "numeric" && first != "integer" && first != "float" && first != "decimal" && first != "duration" && first != "url" && first != "safeint" {
			return "https://typespec.io/docs/standard-library/built-in-decorators/#@" + first
		}
		return "https://typespec.io/docs/standard-library/built-in-data-types/"
	}
	// A library: @typespec/http and its namespace TypeSpec.Http are
	// documented at typespec.io; others are named.
	lib := strings.TrimPrefix(module, "@typespec/")
	if strings.HasPrefix(module, "TypeSpec.") {
		lib = strings.ToLower(strings.TrimPrefix(module, "TypeSpec."))
	} else if lib == module {
		return ""
	}
	return "https://typespec.io/docs/libraries/" + lib + "/reference/"
}
