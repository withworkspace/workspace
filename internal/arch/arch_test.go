// Package arch checks which of ws's packages import which: the layering
// the table below sets out. A package may import what its row allows.
// What it imports beyond that today is listed under exceptions: an
// exception that's no longer needed fails the test too, so the list only
// shrinks.
package arch

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

const module = "github.com/withworkspace/workspace/internal/"

// anything is for a package that puts the others together, and may import
// all of them.
var anything = []string{"*"}

// domain is the packages that each keep one store, for the rows that may
// read any of them.
var domain = []string{"mission", "conversation", "review", "page", "doc", "pulls", "team", "find", "space", "process", "artifact"}

// leaves may be imported by any package: they import nothing of ws's (but
// store, which ranks what's said by words) and hold no rule of anyone's.
var leaves = []string{"git", "slug", "execlog", "job", "ref", "release", "store", "words"}

// allowed is what each package may import, of ws's own, beyond the leaves.
// A package that isn't listed may import only the leaves.
var allowed = map[string][]string{
	"store":        {"words"},
	"workspace":    {"config", "git", "slug"},
	"mission":      {"slug", "git"},
	"conversation": {"mission", "doc"},
	"page":         {"doc", "conversation"},
	"review":       {"mission", "git"},
	"pulls":        {"github", "review", "git"},
	"team":         {"github", "pulls", "git"},
	"find":         {"git"},
	"space":        {"workspace"},
	"nav":          {"nav/grammars/typespec"},
	"harness":      {"acp"},
	"prompt":       domain,
	"turn":         {"harness", "acp", "execlog"},
	"crew":         {"harness", "mission", "termpane", "execlog", "acp"},
	"orchestrator": {"mission", "harness", "prompt", "turn", "crew", "execlog", "git"},
	"daemon":       {"crew", "harness", "acp", "termpane", "mission", "execlog"},
	// Plugins: each a process the daemon holds, spoken to as an agent over
	// ACP is. Its tests' stand-in speaks the same way.
	"plugin":            {"config", "acp"},
	"plugin/fakeplugin": {"acp"},
	"ops": append([]string{
		"config", "workspace", "git", "slug", "execlog", "github", "ide",
		"harness", "acp", "prompt", "turn", "crew", "orchestrator", "termpane", "plugin",
	}, domain...),
	"web":      append([]string{"ops", "crew", "harness", "nav", "plugin"}, domain...),
	"cli":      anything,
	"testutil": anything,
}

// exceptions are imports the table doesn't allow and the code has today.
var exceptions = map[string][]string{}

func TestImports(t *testing.T) {
	got := imports(t)
	for pkg, imps := range got {
		allow := append(append([]string(nil), leaves...), allowed[pkg]...)
		if has(leaves, pkg) {
			allow = allowed[pkg] // a leaf imports nothing of ws's, but what its row says
		}
		for _, imp := range imps {
			if has(allow, "*") || has(allow, imp) {
				continue
			}
			if has(exceptions[pkg], imp) {
				continue
			}
			t.Errorf("%s imports %s, which its row in the table doesn't allow", pkg, imp)
		}
	}
	for pkg, excs := range exceptions {
		for _, imp := range excs {
			if !has(got[pkg], imp) {
				t.Errorf("%s no longer imports %s: remove the exception", pkg, imp)
			}
		}
	}
}

// imports reads which of ws's packages each of its packages imports,
// outside its tests.
func imports(t *testing.T) map[string][]string {
	t.Helper()
	root := ".." // internal/
	out := map[string][]string{}
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "dist" || d.Name() == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, _ := filepath.Rel(root, filepath.Dir(path))
		pkg := filepath.ToSlash(rel)
		f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		if _, ok := out[pkg]; !ok {
			out[pkg] = nil
		}
		for _, spec := range f.Imports {
			imp := strings.Trim(spec.Path.Value, `"`)
			if !strings.HasPrefix(imp, module) {
				continue
			}
			imp = strings.TrimPrefix(imp, module)
			if imp != pkg && !has(out[pkg], imp) {
				out[pkg] = append(out[pkg], imp)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, imps := range out {
		sort.Strings(imps)
	}
	if len(out) < 10 {
		t.Fatalf("found %d packages under internal/: is the test run from internal/arch?", len(out))
	}
	return out
}

func has(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
