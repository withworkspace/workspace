package artifact_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/testutil"
)

// An artifact that's more than its page: a folder is added as a version's
// files, each read back by its path; a version's checked out to a folder,
// worked on there, and the folder added as the next; a conversation's
// agent does the same in a folder of its own, proposing what it made; and
// promoted, the version chosen is in the repo with its files.

const sitePage = `<!doctype html>
<title>Site</title>
<link rel="stylesheet" href="css/site.css">
<img src="img/dot.png">
<h1>Site</h1>
`

// writeSite writes a page and the files it loads in a folder, with what a
// folder gathers that isn't the artifact's (a .DS_Store, a .git).
func writeSite(t *testing.T, dir string) string {
	t.Helper()
	for p, data := range map[string]string{
		"index.html":   sitePage,
		"css/site.css": "h1 { color: tomato }\n",
		"img/dot.png":  "\x89PNG\r\n\x1a\n\x00\x01\x02",
		".DS_Store":    "not its",
		".git/HEAD":    "ref: refs/heads/main\n",
	} {
		path := filepath.Join(dir, filepath.FromSlash(p))
		os.MkdirAll(filepath.Dir(path), 0o755)
		if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestArtifactFiles_AFolderIsAVersionCheckedOutAndPromoted(t *testing.T) {
	w := homeWorkspace(t, "")
	run := func(args ...string) testutil.Result { t.Helper(); return testutil.Run(t, w, nil, args...) }
	refused := func(want string, args ...string) {
		t.Helper()
		if r := testutil.RunCommand(t, w.Root, nil, args...); r.Err == nil || !strings.Contains(r.Err.Error(), want) {
			t.Errorf("ws %s: %v, want %q", strings.Join(args, " "), r.Err, want)
		}
	}
	site := writeSite(t, filepath.Join(t.TempDir(), "site"))

	// A folder with no page isn't one; one whose page loads what isn't in
	// it is added, and told.
	empty := t.TempDir()
	os.WriteFile(filepath.Join(empty, "page.html"), []byte(sitePage), 0o644)
	refused("has no index.html", "artifact", "add", empty, "--note", "x")
	r := run("artifact", "add", writeHTML(t, t.TempDir(), "alone.html", sitePage), "--title", "Alone", "--note", "Its page, without its files.")
	if !strings.Contains(r.Stderr, "its <link> loads css/site.css, which isn't among its files") {
		t.Errorf("a page added without its files said:\n%s", r.Stderr)
	}

	r = run("artifact", "add", site, "--note", "A page with a stylesheet and an image.")
	if strings.TrimSpace(r.Stdout) != "a2.v1" || !strings.Contains(r.Stderr, `Added a2.v1, "Site" (3 files, `) || strings.Contains(r.Stderr, "⚠") {
		t.Fatalf("adding the folder said:\n%s%s", r.Stdout, r.Stderr)
	}
	if got := run("artifact", "show", "a2.v1", "--files").Stdout; got != "css/site.css\t21 bytes\nimg/dot.png\t11 bytes\nindex.html\t"+sizeOf(sitePage)+"\n" {
		t.Errorf("its files:\n%s", got)
	}
	if got := run("artifact", "show", "a2.v1").Stdout; got != sitePage {
		t.Errorf("its page: %q", got)
	}
	if got := run("artifact", "show", "a2", "css/site.css").Stdout; got != "h1 { color: tomato }\n" {
		t.Errorf("its stylesheet: %q", got)
	}
	if got := run("artifact", "show", "a2", "img/dot.png").Stdout; got != "\x89PNG\r\n\x1a\n\x00\x01\x02" {
		t.Errorf("its image: %q", got)
	}
	refused("a2.v1 has no file nope.css", "artifact", "show", "a2", "nope.css")
	refused("has no file .DS_Store", "artifact", "show", "a2", ".DS_Store")

	// Checked out, worked on, and added as the next version: what was left
	// alone goes in as it was.
	work := filepath.Join(t.TempDir(), "work")
	r = run("artifact", "checkout", "a2", work)
	if strings.TrimSpace(r.Stdout) != work || !strings.Contains(r.Stderr, "a2.v1's files (3 files, ") || !strings.Contains(r.Stderr, "ws artifact add "+work+" --to a2 --on v1") {
		t.Errorf("checkout said:\n%s%s", r.Stdout, r.Stderr)
	}
	if got, _ := os.ReadFile(filepath.Join(work, "css", "site.css")); string(got) != "h1 { color: tomato }\n" {
		t.Errorf("the stylesheet, checked out: %q", got)
	}
	if _, err := os.Stat(filepath.Join(work, ".git")); err == nil {
		t.Error("the folder checked out has a .git")
	}
	refused("has files in it already", "artifact", "checkout", "a2", work)
	os.WriteFile(filepath.Join(work, "css", "site.css"), []byte("h1 { color: teal }\n"), 0o644)
	os.WriteFile(filepath.Join(work, "data.json"), []byte(`{"n":1}`), 0o644)
	r = run("artifact", "add", work, "--to", "a2", "--note", "Teal, and its data.")
	if strings.TrimSpace(r.Stdout) != "a2.v2" || !strings.Contains(r.Stderr, "(4 files, ") {
		t.Errorf("adding the folder worked on said:\n%s%s", r.Stdout, r.Stderr)
	}
	if got := run("artifact", "show", "a2.v2", "img/dot.png").Stdout; got != "\x89PNG\r\n\x1a\n\x00\x01\x02" {
		t.Errorf("v2's image: %q", got)
	}
	if got := run("artifact", "show", "a2.v1", "css/site.css").Stdout; got != "h1 { color: tomato }\n" {
		t.Errorf("v1's stylesheet changed: %q", got)
	}
	show := run("show", "a2").Stdout
	for _, want := range []string{"v1 ·", "· 3 files, ", "· 4 files, ", "v2's files (index.html is its page):", "  css/site.css · 19 bytes", "ws artifact show a2.v2 <file>", "ws artifact checkout a2.v2", "ws artifact add <file|folder> --to a2"} {
		if !strings.Contains(show, want) {
			t.Errorf("ws show a2 lacks %q:\n%s", want, show)
		}
	}

	// Promoted, the version chosen is in records with its files, at their
	// paths, and its record beside them; promoted again, the folder holds
	// the version chosen then, and nothing of the one before.
	run("artifact", "promote", "a2.v2")
	folder := "artifacts/" + time.Now().Format("2006-01-02") + "-site"
	if got := strings.Fields(testutil.GitOut(t, w.Sources["records"], "ls-tree", "-r", "--name-only", "main", folder+"/")); strings.Join(got, " ") != folder+"/artifact.toml "+folder+"/css/site.css "+folder+"/data.json "+folder+"/img/dot.png "+folder+"/index.html" {
		t.Errorf("records holds %v", got)
	}
	if got := remoteFile(t, w, folder+"/css/site.css"); got != "h1 { color: teal }\n" {
		t.Errorf("the stylesheet in records: %q", got)
	}
	run("artifact", "promote", "a2.v1")
	if got := testutil.GitOut(t, w.Sources["records"], "ls-tree", "-r", "--name-only", "main", folder+"/"); strings.Contains(got, "data.json") || !strings.Contains(got, "css/site.css") {
		t.Errorf("records, with v1 there, holds:\n%s", got)
	}
	if got := remoteFile(t, w, folder+"/css/site.css"); got != "h1 { color: tomato }\n" {
		t.Errorf("v1's stylesheet in records: %q", got)
	}
}

func sizeOf(s string) string { return itoa(len(s)) + " bytes" }

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for ; n > 0; n /= 10 {
		b = append([]byte{byte('0' + n%10)}, b...)
	}
	return string(b)
}

// A conversation's agent revises one with files: it checks the version
// out into the folder that's its own to write in, changes it there, and
// proposes the folder. What it proposed is kept beside the artifacts,
// and added, files and all, when you accept it.
func TestArtifactFiles_ProposedFromAFolder(t *testing.T) {
	w := testutil.MissionWorkspace(t)
	talks := conversation.ForWorkspace(w.Root)
	talks.OpenTalk(conversation.You, "A site", time.Now())
	run := func(args ...string) testutil.Result { t.Helper(); return testutil.Run(t, w, nil, args...) }
	site := writeSite(t, filepath.Join(t.TempDir(), "site"))

	t.Setenv(conversation.EnvConversation, "w1")
	t.Setenv(mission.EnvCrew, conversation.Agent)
	r := run("artifact", "add", "--propose", "--note", "A page with a stylesheet and an image.", site)
	if !strings.Contains(r.Stderr, "proposed in w1 (proposal 1)") || !strings.Contains(r.Stderr, "(3 files, ") {
		t.Errorf("proposing the folder said:\n%s", r.Stderr)
	}
	c, _ := talks.Load("w1")
	p := c.Proposals[0]
	if p.HTML != "" || len(p.Tree) < 40 || len(p.Files) != 3 || p.Files[0].Path != "css/site.css" || p.Title != "Site" || p.Size == 0 {
		t.Fatalf("the proposal: %+v", p)
	}
	// Nothing's an artifact yet.
	if got := run("artifact", "list").Stdout; got != "" {
		t.Errorf("listed before it's accepted:\n%s", got)
	}

	t.Setenv(conversation.EnvConversation, "")
	t.Setenv(mission.EnvCrew, "")
	if r := run("conversation", "accept", "w1", "1"); !strings.Contains(r.Stderr, `Added a1.v1, "Site" (3 files, `) {
		t.Errorf("accepting it said:\n%s", r.Stderr)
	}
	if got := run("artifact", "show", "a1", "css/site.css").Stdout; got != "h1 { color: tomato }\n" {
		t.Errorf("its stylesheet, added: %q", got)
	}

	// Revising it: checked out where the agent writes, and nowhere else.
	t.Setenv(conversation.EnvConversation, "w1")
	t.Setenv(mission.EnvCrew, conversation.Agent)
	// (The test's workspace is in the temp folder, where it does write: a
	// folder outside it stands in for the workspace's own.)
	if r := testutil.RunCommand(t, w.Root, nil, "artifact", "checkout", "a1", "/ws-test-not-yours/here"); r.Err == nil || !strings.Contains(r.Err.Error(), "isn't somewhere you write") {
		t.Errorf("checking out where it doesn't write: %v", r.Err)
	}
	r = run("artifact", "checkout", "a1")
	work := strings.TrimSpace(r.Stdout)
	if !strings.HasSuffix(work, filepath.Join("workspace", "w1", "a1-v1")) || !strings.Contains(r.Stderr, "ws artifact add --propose --to a1 --on v1 --note \"…\" "+work) {
		t.Fatalf("checkout said:\n%s%s", r.Stdout, r.Stderr)
	}
	os.WriteFile(filepath.Join(work, "css", "site.css"), []byte("h1 { color: teal }\n"), 0o644)
	if r := run("artifact", "add", "--propose", "--to", "a1", "--on", "v1", "--note", "Teal.", work); !strings.Contains(r.Stderr, "proposed in w1 (proposal 2)") || !strings.Contains(r.Stderr, "it's added to a1 as v2") {
		t.Errorf("proposing the revision said:\n%s", r.Stderr)
	}
	// A second checkout is a folder of its own.
	if again := strings.TrimSpace(run("artifact", "checkout", "a1").Stdout); again == work || !strings.HasSuffix(again, "a1-v1-2") {
		t.Errorf("checked out again into %s", again)
	}

	t.Setenv(conversation.EnvConversation, "")
	t.Setenv(mission.EnvCrew, "")
	if r := run("conversation", "accept", "w1", "2"); !strings.Contains(r.Stderr, "Added a1.v2") {
		t.Errorf("accepting the revision said:\n%s", r.Stderr)
	}
	if got := run("artifact", "show", "a1.v2", "css/site.css").Stdout; got != "h1 { color: teal }\n" {
		t.Errorf("v2's stylesheet: %q", got)
	}
	if got := run("artifact", "show", "a1.v2", "img/dot.png").Stdout; got != "\x89PNG\r\n\x1a\n\x00\x01\x02" {
		t.Errorf("v2's image, which wasn't touched: %q", got)
	}
}
