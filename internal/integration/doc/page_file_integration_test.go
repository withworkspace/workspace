package doc_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/page"
	"github.com/withworkspace/workspace/internal/testutil"
	"github.com/withworkspace/workspace/internal/web"
)

// kindsWorkspace is docsWorkspace with kinds of document: numbered ADRs in
// records (direct), with one there already, and dated notes in api (which
// takes pull requests).
func kindsWorkspace(t *testing.T) *testutil.Workspace {
	t.Helper()
	w := docsWorkspace(t, "")
	f, err := os.OpenFile(filepath.Join(w.Root, "ws.toml"), os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("\n[documents.adr]\ntitle = \"ADR\"\npath = \"records:adr/{number}-{slug}.md\"\nadd = \"docs: add adr {number} {title}\"\n\n[documents.note]\ntitle = \"Note\"\npath = \"api:notes/{date}-{slug}.md\"\n")
	f.Close()
	testutil.CommitIn(t, w.Sources["records"], "adr/0007-one-binary.md", "# One binary\n", "docs: add adr 0007")
	ground := filepath.Join(w.Root, "repos", "records", ".ground")
	testutil.GitCmd(t, ground, "fetch", "origin")
	testutil.GitCmd(t, ground, "merge", "--ff-only", "origin/main")
	return w
}

// A page filed as a kind of document is a new file where the kind says,
// as the page reads (cells as text, questions gone), in a commit with the
// kind's message, landed in a repo with no pull requests. The page stays,
// and says where it went; the document says what page it came from.
func TestPages_FilingAPageLandsItWhereItsKindSays(t *testing.T) {
	w := kindsWorkspace(t)
	ps := page.Store{Root: w.Root}
	p, _ := ps.New([]byte("# How loud\n\nWhy it matters.\n\n<!-- ws:cell w3 v1 | a table -->\n| a | b |\n|---|---|\n<!-- /ws:cell -->\n\n<!-- ws:cell w4 v0 ask | is it? -->\n<!-- /ws:cell -->\n"))
	file := testutil.FilePage(w.Root, time.Date(2026, 11, 11, 9, 0, 0, 0, time.Local))

	plan, err := file(p.ID, web.PageFile{Kind: "adr", Dry: true})
	if err != nil || plan.Ref != "records:adr/0008-how-loud.md" || plan.Subject != "docs: add adr 0008 How loud" || plan.How != "land" || plan.Taken {
		t.Fatalf("what it would do: %+v, %v", plan, err)
	}
	if _, err := os.Stat(filepath.Join(w.Root, "repos", "records", "how-loud")); err == nil {
		t.Error("asking what it would do lifted a capsule")
	}

	done, err := file(p.ID, web.PageFile{Kind: "adr"})

	if err != nil || done.Ref != "records:adr/0008-how-loud.md" || !strings.HasPrefix(done.Message, "Filed: landed on main") || done.Commit == "" {
		t.Fatalf("filing: %+v, %v", done, err)
	}
	src := w.Sources["records"]
	if got := testutil.Subjects(t, src, "main"); !strings.HasPrefix(got, "docs: add adr 0008 How loud | docs: add adr 0007") {
		t.Errorf("the remote's main has: %s", got)
	}
	if got := testutil.GitOut(t, src, "show", "main:adr/0008-how-loud.md"); got != "# How loud\n\nWhy it matters.\n\n| a | b |\n|---|---|\n" {
		t.Errorf("what was filed: %q", got)
	}
	if body := testutil.GitOut(t, src, "log", "-1", "--format=%b", "main"); !strings.Contains(body, "Filed from page "+p.ID) {
		t.Errorf("the commit's body: %q", body)
	}
	_, after, _ := ps.Read(p.ID)
	if after.Filed == nil || after.Filed.Ref != "records:adr/0008-how-loud.md" || after.Filed.Kind != "adr" || !after.Filed.Landed {
		t.Errorf("the page after: %+v", after.Filed)
	}
	entries, _ := os.ReadDir(filepath.Join(w.Root, "repos", "records"))
	if len(entries) != 2 {
		t.Errorf("a capsule was left: %v", entries)
	}
	if _, err := file(p.ID, web.PageFile{Kind: "adr"}); err == nil || !strings.Contains(err.Error(), "filed already") {
		t.Errorf("filing it again: %v", err)
	}
	a := openDoc(t, w, "records:adr/0008-how-loud.md")
	if a.Page != p.ID || a.DocKind == nil || a.DocKind.Name != "adr" {
		t.Errorf("the document: page %q, kind %+v", a.Page, a.DocKind)
	}
}

// In a repo that takes pull requests, it's left in a capsule lifted for
// it; a title whose file's there already, a kind the workspace hasn't,
// and a page with nothing in it are refused.
func TestPages_FilingIntoAPRRepoLeavesACapsule(t *testing.T) {
	w := kindsWorkspace(t)
	ps := page.Store{Root: w.Root}
	p, _ := ps.New([]byte("Caching, in short.\n"))
	file := testutil.FilePage(w.Root, time.Date(2026, 11, 11, 9, 0, 0, 0, time.Local))

	if _, err := file(p.ID, web.PageFile{Kind: "note"}); err == nil || !strings.Contains(err.Error(), "give it a title") {
		t.Errorf("no heading and no title: %v", err)
	}
	if _, err := file(p.ID, web.PageFile{Kind: "bp", Title: "Caching"}); err == nil || !strings.Contains(err.Error(), "no kind of document named") {
		t.Errorf("a kind it hasn't: %v", err)
	}
	done, err := file(p.ID, web.PageFile{Kind: "note", Title: "Caching requirements", Message: "Note what caching needs"})

	if err != nil || done.Ref != "api/add-caching-requirements:notes/2026-11-11-caching-requirements.md" || done.How != "lift" {
		t.Fatalf("filing: %+v, %v", done, err)
	}
	capsule := testutil.CapsuleDir(w, "api", "add-caching-requirements")
	if got := testutil.Subjects(t, capsule, "HEAD"); !strings.HasPrefix(got, "Note what caching needs | docs: add the plan") {
		t.Errorf("the capsule has: %s", got)
	}
	if got, _ := os.ReadFile(filepath.Join(capsule, "notes", "2026-11-11-caching-requirements.md")); string(got) != "# Caching requirements\n\nCaching, in short.\n" {
		t.Errorf("what was filed: %q", got)
	}
	if got := testutil.Subjects(t, w.Sources["api"], "main"); got != "docs: add the plan | initial" {
		t.Errorf("something was pushed: %s", got)
	}

	// The same title again, filed from another page: it's there already.
	testutil.CommitIn(t, w.Sources["api"], "notes/2026-11-11-taken.md", "# Taken\n", "a note")
	q, _ := ps.New([]byte("# Taken\n"))
	if _, err := file(q.ID, web.PageFile{Kind: "note"}); err == nil || !strings.Contains(err.Error(), "already") {
		t.Errorf("a file there already: %v", err)
	}
	empty, _ := ps.New(nil)
	if _, err := file(empty.ID, web.PageFile{Kind: "note", Title: "x"}); err == nil || !strings.Contains(err.Error(), "nothing in it") {
		t.Errorf("an empty page: %v", err)
	}
}

// A number is checked again when it lands: an ADR numbered in a capsule
// whose number has landed meanwhile stops, saying the next, and nothing's
// pushed.
func TestLand_ANumberTakenMeanwhileStops(t *testing.T) {
	w := kindsWorkspace(t)
	testutil.Run(t, w, nil, "lift", "records", "quiet")
	testutil.CommitIn(t, testutil.CapsuleDir(w, "records", "quiet"), "adr/0008-quiet-hours.md", "# Quiet hours\n", "docs: add adr 0008")
	testutil.CommitIn(t, w.Sources["records"], "adr/0008-loud-hours.md", "# Loud hours\n", "docs: add adr 0008, too")
	before := testutil.Subjects(t, w.Sources["records"], "main")

	r := testutil.RunCommand(t, w.Root, nil, "land", "records", "quiet")

	if r.Err == nil || !strings.Contains(r.Err.Error(), "adr/0008-quiet-hours.md takes number 0008, which adr/0008-loud-hours.md has on main") || !strings.Contains(r.Err.Error(), "Renumber it 0009 (git mv adr/0008-quiet-hours.md adr/0009-quiet-hours.md") {
		t.Errorf("ws land: %v", r.Err)
	}
	if got := testutil.Subjects(t, w.Sources["records"], "main"); got != before {
		t.Errorf("something was pushed: %s", got)
	}
	// Changing one that's there, with its number, is no clash.
	testutil.CommitIn(t, testutil.CapsuleDir(w, "records", "quiet"), "adr/0007-one-binary.md", "# One binary, still\n", "docs: amend adr 0007")
	testutil.GitCmd(t, testutil.CapsuleDir(w, "records", "quiet"), "rm", "-q", "adr/0008-quiet-hours.md")
	testutil.GitCmd(t, testutil.CapsuleDir(w, "records", "quiet"), "-c", "user.name=Test", "-c", "user.email=test@test.com", "commit", "-q", "-m", "drop it")
	testutil.Run(t, w, nil, "land", "records", "quiet")
}

// Filing takes a number, and when someone lands that number first, it
// takes the next and files again: the kind's message says the new one.
func TestPages_FilingTakesTheNextNumberWhenOneIsTakenMeanwhile(t *testing.T) {
	w := kindsWorkspace(t)
	// The moment filing commits, someone else lands an ADR 0008. Once.
	hook := filepath.Join(w.Root, "repos", "records", ".bare", "hooks", "post-commit")
	once := filepath.Join(t.TempDir(), "once")
	os.MkdirAll(filepath.Dir(hook), 0o755)
	script := "#!/bin/sh\n[ -e '" + once + "' ] && exit 0\ntouch '" + once + "'\nunset GIT_DIR GIT_INDEX_FILE GIT_WORK_TREE GIT_PREFIX\ncd '" + w.Sources["records"] + "' && printf '# Loud hours\\n' >adr/0008-loud-hours.md && git add -A && git -c user.name=T -c user.email=t@t.com commit -qm 'docs: add adr 0008, someone else'\n"
	if err := os.WriteFile(hook, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	p, _ := page.Store{Root: w.Root}.New([]byte("# Quiet hours\n\nNo noise.\n"))

	done, err := testutil.FilePage(w.Root, time.Now())(p.ID, web.PageFile{Kind: "adr"})

	if err != nil || done.Ref != "records:adr/0009-quiet-hours.md" || done.Subject != "docs: add adr 0009 Quiet hours" {
		t.Fatalf("filing: %+v, %v", done, err)
	}
	if got := testutil.Subjects(t, w.Sources["records"], "main"); !strings.HasPrefix(got, "docs: add adr 0009 Quiet hours | docs: add adr 0008, someone else | docs: add adr 0007") {
		t.Errorf("the remote's main has: %s", got)
	}
	if _, err := os.Stat(once); err != nil {
		t.Error("the hook never ran: nothing raced it")
	}
}

// A page of no kind is filed where it's said to go: a folder of a repo,
// named from its title, as the page said when it was started (to), or as
// filing says. With nowhere said, it's refused.
func TestPages_FilingAPageOfNoKindToAFolder(t *testing.T) {
	w := kindsWorkspace(t)
	ps := page.Store{Root: w.Root}
	p, _ := ps.New([]byte("# Release checklist\n\nWhat to check.\n"))
	file := testutil.FilePage(w.Root, time.Date(2026, 11, 11, 9, 0, 0, 0, time.Local))
	if _, err := file(p.ID, web.PageFile{Dry: true}); err == nil || !strings.Contains(err.Error(), "say where") {
		t.Fatalf("filed nowhere: %v", err)
	}
	ps.SetMeta(p.ID, page.Meta{To: "records:guides/"})

	plan, err := file(p.ID, web.PageFile{Dry: true})
	if err != nil || plan.Ref != "records:guides/release-checklist.md" || plan.Subject != "Add guides/release-checklist.md" || plan.Kind != "" || plan.How != "land" {
		t.Fatalf("what it would do: %+v, %v", plan, err)
	}
	// Filing can say somewhere else, a file this time.
	plan, err = file(p.ID, web.PageFile{To: "records:checklists/release.md", Dry: true})
	if err != nil || plan.Ref != "records:checklists/release.md" {
		t.Fatalf("to a file: %+v, %v", plan, err)
	}
	if _, err := file(p.ID, web.PageFile{To: "records:../out/", Dry: true}); err == nil {
		t.Error("a folder out of the repo was taken")
	}

	done, err := file(p.ID, web.PageFile{})
	if err != nil || done.Ref != "records:guides/release-checklist.md" || !strings.HasPrefix(done.Message, "Filed: landed on main") {
		t.Fatalf("filing: %+v, %v", done, err)
	}
	if got := testutil.GitOut(t, w.Sources["records"], "show", "main:guides/release-checklist.md"); got != "# Release checklist\n\nWhat to check.\n" {
		t.Errorf("what was filed: %q", got)
	}
	_, after, _ := ps.Read(p.ID)
	if after.Filed == nil || after.Filed.Ref != "records:guides/release-checklist.md" || after.Filed.Kind != "" || after.Kind != "" || after.Source == nil {
		t.Errorf("the page after: %+v", after)
	}
}
