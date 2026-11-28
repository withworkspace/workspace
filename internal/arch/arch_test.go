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
	"slices"
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
var domain = []string{"mission", "conversation", "review", "page", "doc", "pulls", "team", "find", "space", "process", "artifact", "notification", "spend", "lessons"}

// leaves may be imported by any package: they import nothing of ws's (but
// store, which ranks what's said by words, and keys what's named as ref
// names it; events, which keeps what ws did in the store; and outbox,
// which keeps there what's still to be sent somewhere slow) and hold no
// rule of anyone's. cost is what measures them, when the daemon's asked to.
// report reads what a task's report files say, as numbers.
var leaves = []string{"cost", "git", "slug", "events", "execlog", "images", "job", "outbox", "ref", "release", "report", "store", "words"}

// allowed is what each package may import, of ws's own, beyond the leaves.
// A package that isn't listed may import only the leaves.
var allowed = map[string][]string{
	"store":        {"words", "ref", "cost"},
	"git":          {"cost"}, // counted, when the daemon's measuring
	"outbox":       {"store"},
	"events":       {"store"},
	"notification": {"events"},
	"workspace":    {"config", "git", "slug"},
	"mission":      {"slug", "git"},
	"conversation": {"mission", "doc", "automation"}, // a proposed automation, as its card shows it
	"page":         {"doc", "conversation"},
	"review":       {"mission", "git"},
	"pulls":        {"github", "review", "git"},
	"team":         {"github", "pulls", "git"},
	"find":         {"git"},
	"space":        {"workspace"},
	"nav":          {"nav/grammars/typespec"},
	"harness":      {"acp"},
	"spend":        {"harness"}, // what agents spent, as their harness reports it
	"prompt":       domain,
	"turn":         {"harness", "acp", "execlog", "spend"},
	"crew":         {"harness", "mission", "termpane", "execlog", "acp"},
	"orchestrator": {"mission", "harness", "prompt", "turn", "crew", "execlog", "git", "spend"},
	// The daemon: the crew it holds and the socket commands reach it on,
	// and, put together around one ops.Workspace for its life, everything
	// it runs (jobs, its loops, the web UI, plugins). Only cli starts it.
	"daemon": append([]string{
		"crew", "harness", "acp", "termpane", "execlog", "config", "workspace",
		"ops", "orchestrator", "web", "plugin", "nav", "runs", "router", "automation",
	}, domain...),
	// Runs: what ws starts for someone and ends for them. It reads the
	// user's processes as the process check does; whose records say an
	// owner has ended is its caller's to say.
	"runs": {"harness"},
	// The router: names under .localhost for workspaces and their runs.
	"router": {"runs", "harness"},
	// Plugins: each a process the daemon holds, spoken to as an agent over
	// ACP is. Its tests' stand-in speaks the same way.
	"plugin":            {"config", "acp"},
	"plugin/fakeplugin": {"acp"},
	"ops": append([]string{
		"config", "workspace", "git", "slug", "execlog", "github", "ide",
		"harness", "acp", "prompt", "turn", "crew", "orchestrator", "termpane", "plugin", "runs", "router", "whatsnew", "automation",
	}, domain...),
	"web":      append([]string{"ops", "crew", "harness", "nav", "plugin", "runs", "router", "whatsnew", "automation"}, domain...),
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
	return slices.Contains(list, s)
}
