package nav

import (
	"path"
	"strings"
	"unicode"
	"unicode/utf8"
)

// golang is Go.
type golang struct{ noNamespaces }

var goQuery = mustQuery("go")

func (golang) Name() string                { return "go" }
func (golang) Suffixes() []string          { return []string{".go"} }
func (golang) Grammar(string) string       { return "go" }
func (golang) Query(grammar string) string { return goQuery }
func (golang) BodyOpen() string            { return "{" }
func (golang) ImportsExported() bool       { return false }

// Literal: see Lang.
func (golang) Literal(text string) string {
	return ""
}

// ElemType: see Lang.
func (golang) ElemType(written string) string {
	t := strings.TrimSpace(written)
	if strings.HasPrefix(t, "[]") {
		return t[2:]
	}
	return ""
}

func (golang) SeesClass() bool { return true }

func (golang) Submodule(Env, Where, string) (Where, bool) { return Where{}, false }

func (golang) Self() []string { return nil }

func (golang) Hoists(scope string, d Def) bool { return false }

func (golang) Exported(d Def) bool {
	r, _ := utf8.DecodeRuneInString(d.Name)
	return unicode.IsUpper(r)
}

var goBuiltins = map[string]bool{}

func init() {
	for _, n := range strings.Fields(`any bool byte comparable complex64 complex128 error float32 float64
		int int8 int16 int32 int64 rune string uint uint8 uint16 uint32 uint64 uintptr
		true false iota nil
		append cap clear close complex copy delete imag len make max min new panic print println real recover`) {
		goBuiltins[n] = true
	}
}

func (golang) Builtin(name string) bool { return goBuiltins[name] }

func (golang) Docs(module, name string) string {
	if module == "" {
		module = "builtin"
	}
	u := "https://pkg.go.dev/" + module
	if name != "" {
		u += "#" + name
	}
	return u
}

// TypeName: *Foo, []Foo's element aside, pkg.Foo, Foo[T], a function's
// results ((Foo, error) is Foo).
func (golang) TypeName(written string) []string {
	t := strings.TrimSpace(written)
	if strings.HasPrefix(t, "(") {
		// A function's results: the first, less its name.
		inner := strings.TrimSuffix(strings.TrimPrefix(t, "("), ")")
		t = strings.TrimSpace(splitTop(inner, ',')[0])
		if f := strings.Fields(t); len(f) == 2 && !strings.ContainsAny(f[0], "*.[") {
			t = f[1]
		}
	}
	for strings.HasPrefix(t, "*") || strings.HasPrefix(t, "&") {
		t = t[1:]
	}
	if i := strings.IndexByte(t, '['); i > 0 {
		t = t[:i] // Foo[T]
	}
	if t == "" || t == "any" || strings.ContainsAny(t, "[]{}() ,") || strings.HasPrefix(t, "map") || strings.HasPrefix(t, "chan") || strings.HasPrefix(t, "func") || strings.HasPrefix(t, "interface") {
		return nil
	}
	return strings.Split(t, ".")
}

// splitTop splits s at sep where it isn't inside brackets.
func splitTop(s string, sep byte) []string {
	var out []string
	depth, start := 0, 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
		case sep:
			if depth == 0 {
				out = append(out, s[start:i])
				start = i + 1
			}
		}
	}
	return append(out, s[start:])
}

// goModule is a module of the repo: its path, and its folder.
type goModule struct{ path, dir string }

// goModules are the modules a file's import paths are read against: its
// own (the nearest go.mod), those of a go.work at the top, and any
// replace that points at a folder of the repo. Longest path first.
func goModules(e Env, from string) []goModule {
	t := e.Tree()
	own := ""
	for dir := path.Dir(from); ; dir = path.Dir(dir) {
		p := path.Join(dir, "go.mod")
		if dir == "." {
			p = "go.mod"
		}
		if _, ok := t.Blob(p); ok {
			own = p
			break
		}
		if dir == "." || dir == "/" {
			break
		}
	}
	return e.Memo("go.modules:"+own, func() any {
		var mods []goModule
		seen := map[string]bool{}
		var read func(gomod string, depth int)
		read = func(gomod string, depth int) {
			if seen[gomod] || depth > 4 {
				return
			}
			seen[gomod] = true
			src, err := t.Read(gomod)
			if err != nil {
				return
			}
			dir := path.Dir(gomod)
			for _, line := range goModLines(string(src)) {
				f := strings.Fields(line)
				switch {
				case len(f) >= 2 && f[0] == "module":
					mods = append(mods, goModule{path: unquote(f[1]), dir: dir})
				case len(f) >= 1 && f[0] == "replace":
					// replace a [v] => ./local [v]
					arrow := indexOf(f, "=>")
					if arrow > 1 && arrow+1 < len(f) && strings.HasPrefix(f[arrow+1], ".") {
						local := path.Clean(path.Join(dir, f[arrow+1]))
						mods = append(mods, goModule{path: unquote(f[1]), dir: local})
						read(path.Join(local, "go.mod"), depth+1)
					}
				}
			}
		}
		if own != "" {
			read(own, 0)
		}
		if src, err := t.Read("go.work"); err == nil {
			for _, line := range goModLines(string(src)) {
				if f := strings.Fields(line); len(f) >= 2 && f[0] == "use" {
					read(path.Join(path.Clean(unquote(f[1])), "go.mod"), 0)
				}
			}
		}
		for i := range mods {
			if mods[i].dir == "." {
				mods[i].dir = ""
			}
		}
		// Longest first, so a nested module wins.
		for i := 1; i < len(mods); i++ {
			for j := i; j > 0 && len(mods[j].path) > len(mods[j-1].path); j-- {
				mods[j], mods[j-1] = mods[j-1], mods[j]
			}
		}
		return mods
	}).([]goModule)
}

// goModLines are go.mod's directives one a line, with blocks opened
// out: "require (\n a v1\n)" is "require a v1".
func goModLines(src string) []string {
	var out []string
	block := ""
	for _, line := range strings.Split(src, "\n") {
		if i := strings.Index(line, "//"); i >= 0 {
			line = line[:i]
		}
		line = strings.TrimSpace(line)
		switch {
		case line == "":
		case block != "" && line == ")":
			block = ""
		case block != "":
			out = append(out, block+" "+line)
		case strings.HasSuffix(line, "("):
			block = strings.TrimSpace(strings.TrimSuffix(line, "("))
		default:
			out = append(out, line)
		}
	}
	return out
}

func indexOf(f []string, s string) int {
	for i, x := range f {
		if x == s {
			return i
		}
	}
	return -1
}

// goPackage is an import path's folder in the repo, if it's there.
func goPackage(e Env, from, importPath string) (string, bool) {
	for _, m := range goModules(e, from) {
		if importPath == m.path {
			return m.dir, true
		}
		if strings.HasPrefix(importPath, m.path+"/") {
			return path.Join(m.dir, strings.TrimPrefix(importPath, m.path+"/")), true
		}
	}
	// Vendored, beside the nearest go.mod or at the top.
	for _, m := range goModules(e, from) {
		v := path.Join(m.dir, "vendor", importPath)
		if len(e.Tree().Dir(v)) > 0 {
			return v, true
		}
	}
	return "", false
}

// goFiles are a package folder's files: tests only for a test.
func goFiles(t Tree, dir string, tests bool) []string {
	var out []string
	for _, p := range t.Dir(dir) {
		if !strings.HasSuffix(p, ".go") {
			continue
		}
		if strings.HasSuffix(p, "_test.go") && !tests {
			continue
		}
		out = append(out, p)
	}
	return out
}

func (golang) Import(e Env, from string, im Import) Where {
	dir, ok := goPackage(e, from, im.Path)
	if !ok {
		return Where{External: im.Path}
	}
	files := goFiles(e.Tree(), dir, false)
	if len(files) == 0 {
		files = goFiles(e.Tree(), dir, true)
	}
	return Where{Files: files, Dir: dir}
}

func (g golang) ImportName(e Env, from string, im Import) string {
	if w := g.Import(e, from, im); len(w.Files) > 0 {
		for _, p := range w.Files {
			if f, err := e.Facts(p); err == nil && f.Package != "" && !strings.HasSuffix(f.Package, "_test") {
				return f.Package
			}
		}
	}
	// Not here: the path's last part, as most are named.
	parts := strings.Split(im.Path, "/")
	last := parts[len(parts)-1]
	if len(parts) > 1 && len(last) > 1 && last[0] == 'v' && strings.Trim(last[1:], "0123456789") == "" {
		last = parts[len(parts)-2] // example.com/x/v2 is x
	}
	if i := strings.IndexByte(last, '.'); i > 0 {
		last = last[:i] // gopkg.in/yaml.v3 is yaml
	}
	last = strings.TrimPrefix(last, "go-")
	last = strings.TrimSuffix(last, "-go")
	return strings.ReplaceAll(last, "-", "")
}

// Shared: the folder's other files of the same package. A test sees the
// package's files; the package doesn't see its tests.
func (golang) Shared(e Env, p string, f *Facts) []string {
	test := strings.HasSuffix(p, "_test.go")
	var out []string
	for _, other := range goFiles(e.Tree(), path.Dir(p), test) {
		if other == p {
			continue
		}
		of, err := e.Facts(other)
		if err != nil || of.Package != f.Package {
			continue
		}
		out = append(out, other)
	}
	return out
}
