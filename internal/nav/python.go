package nav

import (
	"path"
	"strings"
)

// python is Python.
type python struct{ noNamespaces }

var pyQuery = mustQuery("python")

func (python) Name() string          { return "python" }
func (python) Suffixes() []string    { return []string{".py", ".pyi"} }
func (python) Grammar(string) string { return "python" }
func (python) Query(string) string   { return pyQuery }
func (python) BodyOpen() string      { return ":" }
func (python) Self() []string        { return []string{"self", "cls"} }
func (python) ImportsExported() bool { return true }

// Literal: see Lang.
func (python) Literal(text string) string {
	t := strings.TrimLeft(text, "rRbBuUfF")
	switch {
	case strings.HasPrefix(t, `"`), strings.HasPrefix(t, "'"):
		if strings.HasPrefix(strings.ToLower(text), "b") {
			return "bytes"
		}
		return "str"
	case strings.HasPrefix(text, "["):
		return "list"
	case strings.HasPrefix(text, "{"):
		return "dict"
	}
	return ""
}

// ElemType: see Lang.
func (python) ElemType(written string) string {
	return genericArg(strings.Trim(written, `"'`), '[', ']')
}

func (python) SeesClass() bool                     { return false }
func (python) Exported(Def) bool                   { return true }
func (python) Shared(Env, string, *Facts) []string { return nil }

// Hoists: a name assigned anywhere in a function is the function's
// throughout it.
func (python) Hoists(scope string, d Def) bool { return scope == "function" }

var pyBuiltins = map[string]string{} // name: the page it's documented on

func init() {
	for n := range strings.FieldsSeq(`abs aiter all anext any ascii bin breakpoint callable chr compile delattr dir divmod enumerate eval exec
		filter format getattr globals hasattr hash help hex id input isinstance issubclass iter len locals map max min next oct open ord pow print
		repr reversed round setattr sorted sum vars zip __import__ super property staticmethod classmethod`) {
		pyBuiltins[n] = "functions.html#" + n
	}
	for n := range strings.FieldsSeq(`bool bytearray bytes complex dict float frozenset int list memoryview object range set slice str tuple type`) {
		pyBuiltins[n] = "stdtypes.html#" + n
	}
	for n := range strings.FieldsSeq(`BaseException Exception ArithmeticError AssertionError AttributeError EOFError FileNotFoundError ImportError
		ModuleNotFoundError IndexError KeyError KeyboardInterrupt LookupError MemoryError NameError NotImplementedError OSError IOError
		OverflowError PermissionError RecursionError RuntimeError StopIteration StopAsyncIteration SyntaxError SystemExit TimeoutError
		TypeError UnboundLocalError UnicodeDecodeError UnicodeEncodeError ValueError ZeroDivisionError Warning DeprecationWarning UserWarning`) {
		pyBuiltins[n] = "exceptions.html#" + n
	}
	for n := range strings.FieldsSeq(`None True False NotImplemented Ellipsis __debug__`) {
		pyBuiltins[n] = "constants.html#" + n
	}
	for n := range strings.FieldsSeq(`__name__ __file__ __doc__ __package__ __spec__ __path__ __builtins__ __all__`) {
		pyBuiltins[n] = "../reference/import.html"
	}
}

func (python) Builtin(name string) bool { _, ok := pyBuiltins[name]; return ok }

// pyStdlib are the standard library's top-level modules most often
// imported: they're documented at docs.python.org.
var pyStdlib = map[string]bool{}

func init() {
	for n := range strings.FieldsSeq(`abc argparse array ast asyncio base64 bisect builtins calendar collections concurrent contextlib copy csv
		ctypes dataclasses datetime decimal difflib dis email enum errno functools gc getpass glob gzip hashlib heapq hmac html http importlib
		inspect io ipaddress itertools json logging math mimetypes multiprocessing operator os pathlib pickle platform pprint queue random re
		secrets select shlex shutil signal socket sqlite3 ssl stat statistics string struct subprocess sys tarfile tempfile textwrap threading
		time timeit tomllib traceback types typing unicodedata unittest urllib uuid warnings weakref xml zipfile zlib zoneinfo`) {
		pyStdlib[n] = true
	}
}

func (python) Docs(module, name string) string {
	if module == "" {
		parts := strings.SplitN(name, ".", 2)
		page, ok := pyBuiltins[parts[0]]
		if !ok {
			return ""
		}
		if len(parts) == 2 && strings.HasPrefix(page, "stdtypes.html") {
			return "https://docs.python.org/3/library/stdtypes.html#" + name
		}
		return "https://docs.python.org/3/library/" + page
	}
	top, _, _ := strings.Cut(module, ".")
	if !pyStdlib[top] {
		return "" // a package's name isn't its module's
	}
	u := "https://docs.python.org/3/library/" + module + ".html"
	if name != "" {
		u += "#" + module + "." + name
	}
	return u
}

// TypeName: "Store" is Store, Optional[X] and X | None are X, list[X] is
// list, pkg.X is pkg, X.
func (python) TypeName(written string) []string {
	t := strings.Trim(strings.TrimSpace(written), `"'`)
	if strings.Contains(t, "|") {
		var kept []string
		for _, part := range splitTop(t, '|') {
			if p := strings.TrimSpace(part); p != "None" && p != "" {
				kept = append(kept, p)
			}
		}
		if len(kept) != 1 {
			return nil
		}
		t = kept[0]
	}
	for _, wrap := range []string{"Optional[", "typing.Optional[", "Final[", "ClassVar[", "Annotated["} {
		if strings.HasPrefix(t, wrap) && strings.HasSuffix(t, "]") {
			inner := t[len(wrap) : len(t)-1]
			return python{}.TypeName(splitTop(inner, ',')[0])
		}
	}
	if i := strings.IndexByte(t, '['); i > 0 {
		t = t[:i]
	}
	if t == "" || strings.ContainsAny(t, "[](){} ,") || pyUntyped[t] {
		return nil
	}
	return strings.Split(t, ".")
}

// pyUntyped are written types that say nothing of what's there.
var pyUntyped = map[string]bool{"object": true, "Any": true, "typing.Any": true, "None": true}

// ImportName: import a.b binds a.
func (python) ImportName(_ Env, _ string, im Import) string {
	if im.Kind != "module" {
		return ""
	}
	return strings.SplitN(im.Path, ".", 2)[0]
}

// pyRoots are the folders an absolute import is read from, for a file:
// each folder above it with a pyproject.toml, setup.py or setup.cfg, and
// its src/; then the top of the repo.
func pyRoots(e Env, from string) []string {
	t := e.Tree()
	return e.Memo("py.roots:"+path.Dir(from), func() any {
		var roots []string
		for dir := path.Dir(from); ; dir = path.Dir(dir) {
			for _, marker := range []string{"pyproject.toml", "setup.py", "setup.cfg"} {
				if _, ok := t.Blob(path.Join(dir, marker)); ok {
					roots = append(roots, path.Join(dir, "src"), dir)
					break
				}
			}
			if dir == "." || dir == "/" {
				break
			}
		}
		roots = append(roots, "src", ".")
		return roots
	}).([]string)
}

// pyModule is a dotted module under a folder: a file, or a package (its
// __init__.py, or a namespace package's folder).
func pyModule(t Tree, root string, parts []string) (Where, bool) {
	base := path.Join(append([]string{root}, parts...)...)
	if len(parts) == 0 {
		base = root
	}
	for _, ext := range []string{".py", ".pyi"} {
		if _, ok := t.Blob(base + ext); ok && len(parts) > 0 {
			return Where{Files: []string{base + ext}, Dir: path.Dir(base)}, true
		}
	}
	for _, init := range []string{"__init__.py", "__init__.pyi"} {
		if _, ok := t.Blob(path.Join(base, init)); ok {
			return Where{Files: []string{path.Join(base, init)}, Dir: base}, true
		}
	}
	if len(parts) > 0 && len(t.Dir(base)) > 0 {
		return Where{Dir: base}, true // a namespace package
	}
	return Where{}, false
}

func (py python) Import(e Env, from string, im Import) Where {
	spec := im.Path
	t := e.Tree()
	if strings.HasPrefix(spec, ".") {
		// Relative: a dot for this file's folder, another for each above.
		dots := len(spec) - len(strings.TrimLeft(spec, "."))
		dir := path.Dir(from)
		for i := 1; i < dots; i++ {
			dir = path.Dir(dir)
		}
		rest := strings.TrimLeft(spec, ".")
		var parts []string
		if rest != "" {
			parts = strings.Split(rest, ".")
		}
		if w, ok := pyModule(t, dir, parts); ok {
			return w
		}
		return Where{}
	}
	parts := strings.Split(spec, ".")
	if im.Kind == "module" && im.Alias == "" {
		parts = parts[:1] // import a.b binds a
	}
	for _, root := range pyRoots(e, from) {
		if w, ok := pyModule(t, root, parts); ok {
			return w
		}
	}
	return Where{External: strings.Join(parts, ".")}
}

// Submodule: a package's module of that name.
func (python) Submodule(e Env, where Where, name string) (Where, bool) {
	if where.External != "" || where.Dir == "" {
		return Where{}, false
	}
	// Only a package (a folder) has submodules.
	if len(where.Files) == 1 && path.Base(where.Files[0]) != "__init__.py" && path.Base(where.Files[0]) != "__init__.pyi" {
		return Where{}, false
	}
	return pyModule(e.Tree(), where.Dir, []string{name})
}
