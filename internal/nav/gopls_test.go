package nav

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// TestAgainstGopls asks gopls and the walk about the same names in this
// repo at HEAD, and says how often they agree. It runs when NAV_GOPLS is
// gopls's path; NAV_GOPLS_EVERY asks about every nth name (default 7).
//
//	NAV_GOPLS=$(which gopls) go test ./internal/nav -run TestAgainstGopls -v
func TestAgainstGopls(t *testing.T) {
	gopls := os.Getenv("NAV_GOPLS")
	if gopls == "" {
		t.Skip("NAV_GOPLS isn't set")
	}
	againstLSP(t, []string{gopls, "serve"}, "internal", []string{".go"}, "go", nil)
}

// TestAgainstTypeScript is the same for web/src, against a TypeScript
// language server: NAV_TSLS is typescript-language-server's path.
//
//	NAV_TSLS=…/node_modules/.bin/typescript-language-server go test ./internal/nav -run TestAgainstTypeScript -v
func TestAgainstTypeScript(t *testing.T) {
	tsls := os.Getenv("NAV_TSLS")
	if tsls == "" {
		t.Skip("NAV_TSLS isn't set")
	}
	repo, _ := filepath.Abs("../..")
	// It reads types from web/node_modules, which isn't in the commit.
	link := func(tree string) {
		os.Symlink(filepath.Join(repo, "web", "node_modules"), filepath.Join(tree, "web", "node_modules"))
	}
	againstLSP(t, []string{tsls, "--stdio"}, "web/src", []string{".ts", ".tsx"}, "typescriptreact", link)
}

// TestAgainstPyright is the same for a Python repo, against pyright:
// NAV_PYRIGHT is pyright-langserver's path, NAV_REPO the repo and
// NAV_UNDER the folder in it to ask about.
//
//	NAV_PYRIGHT=…/node_modules/.bin/pyright-langserver NAV_REPO=~/src/app NAV_UNDER=src go test ./internal/nav -run TestAgainstPyright -v
func TestAgainstPyright(t *testing.T) {
	pyright := os.Getenv("NAV_PYRIGHT")
	if pyright == "" || os.Getenv("NAV_REPO") == "" {
		t.Skip("NAV_PYRIGHT and NAV_REPO aren't set")
	}
	againstLSP(t, []string{pyright, "--stdio"}, os.Getenv("NAV_UNDER"), []string{".py"}, "python", nil)
}

func againstLSP(t *testing.T, server []string, under string, suffixes []string, languageID string, prepare func(tree string)) {
	every := 7
	if n, err := strconv.Atoi(os.Getenv("NAV_GOPLS_EVERY")); err == nil && n > 0 {
		every = n
	}
	repo, _ := filepath.Abs("../..")
	if r := os.Getenv("NAV_REPO"); r != "" {
		repo = r
	}
	rev := run(t, repo, "rev-parse", "HEAD")
	// gopls reads files on disk: HEAD's, apart from what's being changed.
	tree := t.TempDir()
	archive := exec.Command("sh", "-c", "git archive "+rev+" | tar -x -C "+tree)
	archive.Dir = repo
	if out, err := archive.CombinedOutput(); err != nil {
		t.Fatalf("archive: %v %s", err, out)
	}

	if prepare != nil {
		prepare(tree)
	}
	lsp := startLSP(t, server, tree)
	defer lsp.close()

	s := New("")
	tr, err := s.Tree(repo, rev)
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	examples := map[string][]string{}
	note := func(kind, ex string) {
		counts[kind]++
		if len(examples[kind]) < 12 {
			examples[kind] = append(examples[kind], ex)
		}
	}
	n := 0
	for _, p := range tr.Under(under, suffixes...) {
		if strings.Contains(p, "testdata/") || strings.Contains(p, "/dist/") {
			continue
		}
		f, err := s.Facts(tr, p)
		if err != nil {
			continue
		}
		src, _ := tr.Read(p)
		lsp.open(filepath.Join(tree, p), languageID, string(src))
		for _, u := range f.Uses {
			if n++; n%every != 0 {
				continue
			}
			line, start := f.Line(u.At.Start)
			col := utf16Len(src[start:u.At.Start])
			a, err := s.Definition(repo, rev, p, line, col)
			if err != nil {
				note("ours failed", fmt.Sprintf("%s:%d:%d %v", p, line, col+1, err))
				continue
			}
			theirs := lsp.definition(filepath.Join(tree, p), line-1, col)
			at := fmt.Sprintf("%s:%d:%d %s", p, line, col+1, u.Name)
			inRepo := theirs.path != "" && strings.HasPrefix(theirs.path, tree+"/")
			rel := strings.TrimPrefix(theirs.path, tree+"/")
			same := func(t Target) bool { return t.Path == rel && t.Line == theirs.line+1 }
			switch {
			case (a.Answer == "external" || a.Answer == "resolved") && rel == p && importLine(f, theirs.line+1):
				// An imported name: we go to what it imports, the server to
				// the import.
				note("agree", at)
			case (a.Answer == "external" || a.Answer == "builtin") && theirs.path == "":
				note("ours only", at+" ("+a.Answer+")")
			case a.Answer == "resolved" && inRepo:
				ok := false
				for _, tg := range a.Targets {
					ok = ok || same(tg)
				}
				if ok {
					note("agree", at)
				} else {
					note("disagree", fmt.Sprintf("%s: ours %s:%d (%s), gopls %s:%d", at, a.Targets[0].Path, a.Targets[0].Line, a.How, rel, theirs.line+1))
				}
			case a.Answer == "resolved" && theirs.path == "":
				note("ours only", at+" ("+a.How+")")
			case a.Answer == "resolved":
				note("disagree", fmt.Sprintf("%s: ours %s:%d (%s), gopls outside: %s", at, a.Targets[0].Path, a.Targets[0].Line, a.How, theirs.path))
			case (a.Answer == "external" || a.Answer == "builtin") && theirs.path != "" && !inRepo:
				note("agree", at)
			case a.Answer == "external" || a.Answer == "builtin":
				note("disagree", fmt.Sprintf("%s: ours %s, gopls %s:%d", at, a.Answer, rel, theirs.line+1))
			case a.Answer == "candidates" && inRepo:
				hit := false
				for _, tg := range a.Targets {
					hit = hit || same(tg)
				}
				if hit {
					note("candidates, gopls's among them", at)
				} else {
					note("candidates, gopls's not among them", fmt.Sprintf("%s: gopls %s:%d", at, rel, theirs.line+1))
				}
			case a.Answer == "candidates":
				note("candidates, gopls outside", at)
			case a.Answer == "unresolved" && theirs.path == "":
				note("agree", at)
			default:
				note("unresolved, gopls has it", fmt.Sprintf("%s: gopls %s:%d", at, theirs.path, theirs.line+1))
			}
		}
	}
	var kinds []string
	total := 0
	for k, c := range counts {
		kinds = append(kinds, k)
		total += c
	}
	sort.Strings(kinds)
	for _, k := range kinds {
		t.Logf("%-36s %5d  (%.1f%%)", k, counts[k], 100*float64(counts[k])/float64(total))
	}
	both := counts["agree"] + counts["disagree"]
	if both > 0 {
		t.Logf("where both answer, they agree on %.1f in 100", 100*float64(counts["agree"])/float64(both))
	}
	for _, k := range kinds {
		if k == "agree" || k == "candidates, gopls's among them" {
			continue
		}
		for _, ex := range examples[k] {
			t.Logf("  %s: %s", k, ex)
		}
	}
}

// importLine is whether a line of a file is one of its imports.
func importLine(f *Facts, line int) bool {
	for _, im := range f.Imports {
		for _, at := range []Span{im.PathAt, im.NameAt, im.AliasAt} {
			if l, _ := f.Line(at.Start); !at.None() && l == line {
				return true
			}
		}
	}
	return false
}

type lspClient struct {
	cmd *exec.Cmd
	in  io.WriteCloser
	out *bufio.Reader
	id  int
}

type lspPlace struct {
	path string
	line int
}

func startLSP(t *testing.T, server []string, root string) *lspClient {
	t.Helper()
	cmd := exec.Command(server[0], server[1:]...)
	cmd.Dir = root
	in, _ := cmd.StdinPipe()
	out, _ := cmd.StdoutPipe()
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	c := &lspClient{cmd: cmd, in: in, out: bufio.NewReaderSize(out, 1<<20)}
	c.call("initialize", map[string]any{"processId": os.Getpid(), "rootUri": "file://" + root, "capabilities": map[string]any{}})
	c.notify("initialized", map[string]any{})
	return c
}

func (c *lspClient) close() {
	c.call("shutdown", nil)
	c.notify("exit", nil)
	c.in.Close()
	c.cmd.Wait()
}

func (c *lspClient) send(msg map[string]any) {
	msg["jsonrpc"] = "2.0"
	b, _ := json.Marshal(msg)
	fmt.Fprintf(c.in, "Content-Length: %d\r\n\r\n%s", len(b), b)
}

func (c *lspClient) notify(method string, params any) {
	c.send(map[string]any{"method": method, "params": params})
}

func (c *lspClient) call(method string, params any) json.RawMessage {
	c.id++
	id := c.id
	c.send(map[string]any{"id": id, "method": method, "params": params})
	for {
		length := 0
		for {
			line, err := c.out.ReadString('\n')
			if err != nil {
				return nil
			}
			line = strings.TrimSpace(line)
			if line == "" {
				break
			}
			if v, ok := strings.CutPrefix(line, "Content-Length: "); ok {
				length, _ = strconv.Atoi(v)
			}
		}
		body := make([]byte, length)
		if _, err := io.ReadFull(c.out, body); err != nil {
			return nil
		}
		var msg struct {
			ID     *int            `json:"id"`
			Method string          `json:"method"`
			Result json.RawMessage `json:"result"`
		}
		json.Unmarshal(body, &msg)
		if msg.ID != nil && msg.Method != "" {
			// A request of gopls's own: answer it with nothing.
			c.send(map[string]any{"id": *msg.ID, "result": nil})
			continue
		}
		if msg.ID != nil && *msg.ID == id {
			return msg.Result
		}
	}
}

// open tells the server a file's open, as a language server for an
// editor wants before it's asked about it.
func (c *lspClient) open(path, languageID, text string) {
	c.notify("textDocument/didOpen", map[string]any{"textDocument": map[string]any{"uri": "file://" + path, "languageId": languageID, "version": 1, "text": text}})
}

func (c *lspClient) definition(path string, line, col int) lspPlace {
	raw := c.call("textDocument/definition", map[string]any{
		"textDocument": map[string]any{"uri": "file://" + path},
		"position":     map[string]any{"line": line, "character": col},
	})
	var locs []struct {
		URI   string `json:"uri"`
		Range struct {
			Start struct{ Line int } `json:"start"`
		} `json:"range"`
	}
	if json.Unmarshal(raw, &locs) != nil || len(locs) == 0 {
		return lspPlace{}
	}
	return lspPlace{path: strings.TrimPrefix(locs[0].URI, "file://"), line: locs[0].Range.Start.Line}
}

// TestCoverage says how a repo's names are answered, without a language
// server to compare with: how many resolve, how many are candidates,
// external, builtin or unresolved, with examples of the last two.
// NAV_REPO is a git directory, NAV_UNDER a folder in it, NAV_SUFFIX the
// files' ending.
//
//	NAV_REPO=~/src/app NAV_UNDER=src NAV_SUFFIX=.cs go test ./internal/nav -run TestCoverage -v
func TestCoverage(t *testing.T) {
	repo, suffix := os.Getenv("NAV_REPO"), os.Getenv("NAV_SUFFIX")
	if repo == "" || suffix == "" {
		t.Skip("NAV_REPO and NAV_SUFFIX aren't set")
	}
	every := 7
	if n, err := strconv.Atoi(os.Getenv("NAV_GOPLS_EVERY")); err == nil && n > 0 {
		every = n
	}
	rev := run(t, repo, "rev-parse", "HEAD")
	s := New("")
	tr, err := s.Tree(repo, rev)
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	examples := map[string][]string{}
	n, files, broken := 0, 0, 0
	for _, p := range tr.Under(os.Getenv("NAV_UNDER"), suffix) {
		f, err := s.Facts(tr, p)
		if err != nil {
			continue
		}
		files++
		if f.Broken {
			broken++
		}
		src, _ := tr.Read(p)
		for _, u := range f.Uses {
			if n++; n%every != 0 {
				continue
			}
			line, start := f.Line(u.At.Start)
			a, err := s.Definition(repo, rev, p, line, utf16Len(src[start:u.At.Start]))
			kind := "failed"
			if err == nil {
				kind = a.Answer
			}
			counts[kind]++
			resolvedEvery := 40
			if len(examples[kind]) < 25 && (kind != "resolved" || counts[kind]%resolvedEvery == 0) {
				text := strings.TrimSpace(string(src[start:min(int(u.At.End)+30, len(src))]))
				if i := strings.IndexByte(text, '\n'); i > 0 {
					text = text[:i]
				}
				if a != nil && a.Answer == "resolved" && len(a.Targets) > 0 {
					text += fmt.Sprintf("  → %s:%d %s (%s)", a.Targets[0].Path, a.Targets[0].Line, a.Targets[0].Signature, a.How)
				}
				if a != nil && a.External != nil {
					text += "  → " + a.External.Module + " " + a.External.Name + " " + a.External.URL + " (" + a.How + ")"
				}
				examples[kind] = append(examples[kind], fmt.Sprintf("%s:%d %s  | %s", p, line, u.Name, text))
			}
		}
	}
	total := 0
	for _, c := range counts {
		total += c
	}
	for _, k := range []string{"resolved", "candidates", "external", "builtin", "unresolved", "failed"} {
		t.Logf("%-12s %6d  (%.1f%%)", k, counts[k], 100*float64(counts[k])/float64(max(total, 1)))
	}
	t.Logf("%d files, %d that didn't parse cleanly", files, broken)
	for _, k := range []string{"resolved", "unresolved", "candidates", "builtin", "external"} {
		for _, ex := range examples[k] {
			t.Logf("  %s: %s", k, ex)
		}
	}
}
