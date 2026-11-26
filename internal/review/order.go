package review

import (
	"context"
	"path"
	"sort"
	"strings"

	"github.com/withworkspace/workspace/internal/git"
)

// The order a diff's files are read in, with no agent to say: a folder's
// files together, a file then its tests, and last the files that needn't
// be read closely (lockfiles, generated code, snapshots, files renamed and
// nothing else).

// Why a file is one to skim.
const (
	SkimLockfile  = "lockfile"
	SkimGenerated = "generated"
	SkimSnapshot  = "snapshot"
	SkimRenamed   = "renamed"
)

var lockfiles = map[string]bool{
	"go.sum": true, "go.work.sum": true, "package-lock.json": true, "npm-shrinkwrap.json": true, "yarn.lock": true,
	"pnpm-lock.yaml": true, "bun.lockb": true, "bun.lock": true, "Cargo.lock": true, "Gemfile.lock": true,
	"poetry.lock": true, "uv.lock": true, "Pipfile.lock": true, "composer.lock": true, "flake.lock": true,
	"mix.lock": true, "Podfile.lock": true, "pubspec.lock": true, "packages.lock.json": true,
}

var generatedSuffixes = []string{
	".pb.go", ".pb.gw.go", "_pb2.py", "_pb2_grpc.py", "_pb2.pyi", ".pb.cc", ".pb.h", "_pb.js", "_pb.d.ts", "_pb.ts",
	".gen.go", "_gen.go", "_generated.go", ".g.dart", ".freezed.dart",
	".min.js", ".min.css", ".js.map", ".css.map",
}

// skimOf is why a file is one to skim, or "" when it's one to read.
func skimOf(f File, generated map[string]bool) string {
	base := path.Base(f.Path)
	switch {
	case lockfiles[base]:
		return SkimLockfile
	case strings.HasSuffix(base, ".snap") || strings.Contains("/"+f.Path, "/__snapshots__/"):
		return SkimSnapshot
	case generated[f.Path] || strings.HasPrefix(base, "zz_generated.") || strings.Contains(base, ".generated."):
		return SkimGenerated
	case f.Status == "renamed" && len(f.Hunks) == 0 && !f.Binary && !f.Large:
		return SkimRenamed
	}
	for _, s := range generatedSuffixes {
		if strings.HasSuffix(base, s) {
			return SkimGenerated
		}
	}
	return ""
}

// attrChunk is how many paths git is asked about at once.
const attrChunk = 400

// generatedByAttr are the files the repo itself says are generated
// (.gitattributes' linguist-generated), as its attributes stand at head.
// A git too old to read attributes from a commit says none.
func generatedByAttr(dir, head string, files []File) map[string]bool {
	out := map[string]bool{}
	for i := 0; i < len(files); i += attrChunk {
		args := []string{"check-attr", "--source=" + head, "-z", "linguist-generated", "--"}
		for _, f := range files[i:min(i+attrChunk, len(files))] {
			args = append(args, f.Path)
		}
		data, err := git.Read(context.Background(), dir, args...)
		if err != nil {
			return out
		}
		// path NUL attribute NUL value NUL, for each.
		fields := strings.Split(string(data), "\x00")
		for j := 0; j+2 < len(fields); j += 3 {
			if v := fields[j+2]; v == "set" || v == "true" {
				out[fields[j]] = true
			}
		}
	}
	return out
}

// Generated is what of a change the repo says is generated
// (.gitattributes' linguist-generated). An agent that reads the change
// itself is pointed past it (prompt.ReviewPass.Generated): it's made from
// the rest, and can be most of a diff's size.
type Generated struct {
	// Files are the generated files the change touches, in path order.
	Files []string
	// Pathspecs leave them out of a git diff of the change, and nothing
	// else: a folder whose every changed file is generated is one
	// (":(exclude)client/"), else a file. Paths are named, rather than the
	// attribute (":(exclude,attr:linguist-generated)"), because git reads
	// attributes from a working tree, which a pull request's clone hasn't,
	// and taking them from a commit (--attr-source) is an option before
	// the subcommand, which an agent's read-only git isn't allowed.
	Pathspecs []string
}

// maxPathspecs is how many a diff is given at most: past that, the rest
// stay in it.
const maxPathspecs = 40

// GeneratedIn is what a change (base to head, in dir) touches that the
// repo says is generated, as its attributes stand at head.
func GeneratedIn(dir, base, head string) Generated {
	data, err := git.Read(context.Background(), dir, "diff", "--name-only", "-z", "--no-renames", base, head)
	if err != nil {
		return Generated{}
	}
	var changed []string
	var files []File
	for p := range strings.SplitSeq(string(data), "\x00") {
		if p != "" {
			changed = append(changed, p)
			files = append(files, File{Path: p})
		}
	}
	gen := generatedByAttr(dir, head, files)
	g := Generated{}
	for p := range gen {
		g.Files = append(g.Files, p)
	}
	sort.Strings(g.Files)
	g.Pathspecs = leaveOut(changed, gen)
	return g
}

// leaveOut is the fewest pathspecs that exclude the generated files of
// changed, and nothing else: for each, the highest folder whose changed
// files are all generated, else the file. One that can't be quoted for a
// shell as it is stays in.
func leaveOut(changed []string, gen map[string]bool) []string {
	whole := map[string]bool{} // a folder: every changed file under it generated
	for _, p := range changed {
		for d := path.Dir(p); d != "." && d != "/"; d = path.Dir(d) {
			if was, seen := whole[d]; seen {
				whole[d] = was && gen[p]
			} else {
				whole[d] = gen[p]
			}
		}
	}
	seen := map[string]bool{}
	var out []string
	for _, p := range changed {
		if !gen[p] || strings.ContainsAny(p, "'\n") {
			continue
		}
		top := p
		for d := path.Dir(p); d != "." && d != "/" && whole[d]; d = path.Dir(d) {
			top = d + "/"
		}
		if !seen[top] {
			seen[top] = true
			out = append(out, ":(exclude)"+top)
		}
	}
	sort.Strings(out)
	if len(out) > maxPathspecs {
		out = out[:maxPathspecs]
	}
	return out
}

// testOf splits a file's name into the name of the file it tests and
// whether it's a test: cache_test.go is cache.go's, x.test.ts is x.ts's,
// test_x.py is x.py's. A file that isn't a test is its own.
func testOf(base string) (of string, test bool) {
	ext := path.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	for _, s := range []string{".test", ".spec", "_test", "_spec"} {
		if strings.HasSuffix(stem, s) && len(stem) > len(s) {
			return strings.TrimSuffix(stem, s) + ext, true
		}
	}
	if ext == ".py" && strings.HasPrefix(stem, "test_") && len(stem) > len("test_") {
		return strings.TrimPrefix(stem, "test_") + ext, true
	}
	switch ext {
	case ".java", ".kt", ".cs", ".swift", ".scala":
		for _, s := range []string{"Tests", "Test"} {
			if strings.HasSuffix(stem, s) && len(stem) > len(s) {
				return strings.TrimSuffix(stem, s) + ext, true
			}
		}
	}
	return base, false
}

// dirLess orders folders so a folder's own files come before its
// subfolders', and a folder's subfolders stay beside it (src/a, src/a/b,
// src/a-b: not src/a, src/a-b, src/a/b, as plain text sorts them).
func dirLess(a, b string) bool {
	as, bs := strings.Split(a, "/"), strings.Split(b, "/")
	if a == "." {
		as = nil
	}
	if b == "." {
		bs = nil
	}
	for i := 0; i < len(as) && i < len(bs); i++ {
		if as[i] != bs[i] {
			return as[i] < bs[i]
		}
	}
	return len(as) < len(bs)
}

// order sorts a diff's files into the order they're read in, and marks
// those to skim.
func order(dir, head string, files []File) {
	if len(files) == 0 {
		return
	}
	generated := generatedByAttr(dir, head, files)
	type key struct {
		dir, of, base string
		test          bool
	}
	keys := map[string]key{}
	for i := range files {
		f := &files[i]
		f.Skim = skimOf(*f, generated)
		base := path.Base(f.Path)
		of, test := testOf(base)
		keys[f.Path] = key{dir: path.Dir(f.Path), of: of, base: base, test: test}
	}
	sort.SliceStable(files, func(i, j int) bool {
		a, b := files[i], files[j]
		if (a.Skim != "") != (b.Skim != "") {
			return a.Skim == ""
		}
		ka, kb := keys[a.Path], keys[b.Path]
		switch {
		case ka.dir != kb.dir:
			return dirLess(ka.dir, kb.dir)
		case ka.of != kb.of:
			return ka.of < kb.of
		case ka.test != kb.test:
			return !ka.test
		}
		return ka.base < kb.base
	})
}
