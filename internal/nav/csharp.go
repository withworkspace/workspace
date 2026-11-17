package nav

import (
	"strings"
)

// csharp is C#. A using names a namespace, not a file: a type is found by
// its name across the repo, and kept if its namespace is in reach.
type csharp struct{}

var csQuery = mustQuery("c_sharp")

func (csharp) Name() string          { return "csharp" }
func (csharp) Suffixes() []string    { return []string{".cs"} }
func (csharp) Grammar(string) string { return "c_sharp" }
func (csharp) Query(string) string   { return csQuery }
func (csharp) BodyOpen() string      { return "{" }
func (csharp) Self() []string        { return []string{"this"} }

// ElemType: see Lang.
func (csharp) ElemType(written string) string {
	return genericArg(written, '<', '>')
}

func (csharp) SeesClass() bool                     { return true }
func (csharp) ImportsExported() bool               { return false }
func (csharp) Namespaces() bool                    { return true }
func (csharp) BareMembers() bool                   { return true }
func (csharp) Shared(Env, string, *Facts) []string { return nil }
func (csharp) ImportName(Env, string, Import) string {
	return ""
}

func (csharp) Submodule(Env, Where, string) (Where, bool) { return Where{}, false }

// Hoists: a local function can be called before it's written.
func (csharp) Hoists(scope string, d Def) bool { return d.Kind == "func" }

// Exported: a type's visibility isn't read; anything at a namespace's
// top can be reached from another file.
func (csharp) Exported(Def) bool { return true }

// Import: a using leads to no file. Its namespace is in reach (Reach).
func (csharp) Import(Env, string, Import) Where { return Where{} }

func (csharp) Literal(text string) string {
	switch {
	case strings.HasPrefix(text, `"`), strings.HasPrefix(text, `@"`), strings.HasPrefix(text, `$"`):
		return "String"
	}
	return ""
}

// Reach: the namespaces around the place, innermost first; the file's
// usings and the project's global ones; then the global namespace.
func (c csharp) Reach(e Env, p string, f *Facts, at Span) []string {
	w, _ := e.(*walker)
	var out []string
	if w != nil {
		out = append(out, w.enclosingNamespaces(f, at)...)
	}
	seen := map[string]bool{}
	for _, ns := range out {
		seen[ns] = true
	}
	add := func(ns string) {
		ns = strings.Join(strings.Fields(ns), "")
		if !seen[ns] {
			seen[ns] = true
			out = append(out, ns)
		}
	}
	for _, im := range f.Imports {
		if im.Kind == "using" || im.Kind == "global" {
			add(im.Path)
		}
	}
	for _, ns := range csGlobalUsings(e) {
		add(ns)
	}
	return append(out, "")
}

// csGlobalUsings are the project's global usings: in any file.
func csGlobalUsings(e Env) []string {
	return e.Memo("cs.global", func() any {
		hits, err := e.Tree().Grep("global", ".cs")
		if err != nil {
			return []string(nil)
		}
		var out []string
		seen := map[string]bool{}
		for _, h := range hits {
			if seen[h.Path] || !strings.Contains(h.Text, "using") {
				continue
			}
			seen[h.Path] = true
			f, err := e.Facts(h.Path)
			if err != nil {
				continue
			}
			for _, im := range f.Imports {
				if im.Kind == "global" {
					out = append(out, im.Path)
				}
			}
		}
		return out
	}).([]string)
}

// csPredefined are C#'s keywords for types, as the types they are.
var csPredefined = map[string]string{
	"string": "String", "object": "", "int": "Int32", "long": "Int64", "short": "Int16", "byte": "Byte", "bool": "Boolean",
	"double": "Double", "float": "Single", "decimal": "Decimal", "char": "Char", "uint": "UInt32", "ulong": "UInt64", "dynamic": "",
}

// TypeName: List<Foo> is List, Foo? is Foo, A.B is A, B; an array or a
// tuple is no single type; string is String.
func (csharp) TypeName(written string) []string {
	t := strings.TrimSpace(written)
	t = strings.TrimSuffix(t, "?")
	t = strings.TrimPrefix(t, "global::")
	if strings.HasSuffix(t, "]") || strings.HasPrefix(t, "(") || t == "var" {
		return nil
	}
	if i := strings.IndexByte(t, '<'); i > 0 {
		t = t[:i]
	}
	if w, ok := csPredefined[t]; ok {
		if w == "" {
			return nil
		}
		return []string{w}
	}
	if t == "" || strings.ContainsAny(t, " ,()[]{}") {
		return nil
	}
	return strings.Split(t, ".")
}

// csKnown are the .NET types most often used, by where they're documented
// (learn.microsoft.com/dotnet/api/<page>).
var csKnown = map[string]string{}

func init() {
	for ns, names := range map[string]string{
		"system":                                   "Console Math String Object Exception ArgumentException ArgumentNullException InvalidOperationException NotImplementedException NotSupportedException DateTime DateTimeOffset TimeSpan Guid Int32 Int64 Double Decimal Boolean Char Byte Convert Enum Array Nullable Lazy Uri Environment Random Tuple ValueTuple Action Func EventHandler EventArgs IDisposable IAsyncDisposable IComparable IEquatable Attribute Type GC StringComparison",
		"system.collections.generic":               "List Dictionary HashSet Queue Stack IEnumerable IList ICollection IDictionary IReadOnlyList IReadOnlyCollection IReadOnlyDictionary KeyValuePair SortedDictionary LinkedList",
		"system.linq":                              "Enumerable Queryable IQueryable IGrouping",
		"system.threading":                         "CancellationToken CancellationTokenSource Thread Interlocked Monitor SemaphoreSlim",
		"system.threading.tasks":                   "Task ValueTask TaskCompletionSource Parallel",
		"system.text":                              "StringBuilder Encoding",
		"system.text.json":                         "JsonSerializer JsonSerializerOptions JsonDocument JsonElement",
		"system.text.regularexpressions":           "Regex Match",
		"system.io":                                "File Path Directory Stream MemoryStream StreamReader StreamWriter FileInfo DirectoryInfo",
		"system.net.http":                          "HttpClient HttpResponseMessage HttpRequestMessage HttpMethod",
		"microsoft.extensions.logging":             "ILogger ILoggerFactory LogLevel",
		"microsoft.extensions.dependencyinjection": "IServiceCollection IServiceProvider ServiceCollection",
	} {
		for _, n := range strings.Fields(names) {
			csKnown[n] = ns + "." + strings.ToLower(n)
		}
	}
	// Generic ones' pages carry their arity.
	for n, arity := range map[string]string{"List": "1", "Dictionary": "2", "HashSet": "1", "Queue": "1", "Stack": "1", "IEnumerable": "1", "IList": "1",
		"ICollection": "1", "IDictionary": "2", "IReadOnlyList": "1", "IReadOnlyCollection": "1", "IReadOnlyDictionary": "2", "KeyValuePair": "2",
		"Nullable": "1", "Lazy": "1", "Task": "1", "ValueTask": "1", "TaskCompletionSource": "1", "IQueryable": "1", "IEquatable": "1",
		"IComparable": "1", "SortedDictionary": "2", "LinkedList": "1", "ILogger": "1", "IGrouping": "2", "Action": "1", "Func": "1"} {
		if p, ok := csKnown[n]; ok && n != "Task" && n != "ValueTask" && n != "ILogger" && n != "Action" && n != "IComparable" {
			csKnown[n] = p + "-" + arity
		}
	}
}

func (csharp) Builtin(name string) bool { _, ok := csKnown[name]; return ok }

func (csharp) Docs(module, name string) string {
	if module == "" {
		parts := strings.SplitN(name, ".", 2)
		page, ok := csKnown[parts[0]]
		if !ok {
			return ""
		}
		u := "https://learn.microsoft.com/dotnet/api/" + page
		if len(parts) == 2 {
			u += "." + strings.ToLower(parts[1])
		}
		return u
	}
	// .NET's own namespaces are documented there; a package's isn't.
	if module != "System" && module != "Microsoft" && !strings.HasPrefix(module, "System.") && !strings.HasPrefix(module, "Microsoft.") {
		return ""
	}
	u := "https://learn.microsoft.com/dotnet/api/" + strings.ToLower(module)
	if name != "" {
		u += "." + strings.ToLower(name)
	}
	return u
}
