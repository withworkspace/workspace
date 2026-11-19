package space

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/withworkspace/workspace/internal/config"
	"github.com/withworkspace/workspace/internal/ref"
	"github.com/withworkspace/workspace/internal/store"
	"github.com/withworkspace/workspace/internal/testgit"
	"github.com/withworkspace/workspace/internal/workspace"
)

var bg = context.Background()

// Tests make commits: never ask the machine's signing agent for them.
func TestMain(m *testing.M) {
	cleanup := testgit.NoSigning()
	code := m.Run()
	cleanup()
	os.Exit(code)
}

func records(t testing.TB) *store.DB {
	t.Helper()
	db, err := store.Own(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestParse_SectionsAnchorsAndWhatItCites(t *testing.T) {
	text := "---\ntitle: Token cache\nstatus: proposed\n---\n# Token cache\n\nWhy we [cache](../adr/0038-cache.md#why) tokens.\n\n## Keying: per *tenant*\n\nSee `x` and [the README](README.md).\n\n```\n# not a heading\n[nor](a-link.md)\n```\n\n## Keying: per *tenant*\n\nAgain, and [out](https://example.com/x.md).\n"
	r := parse("records", "bp/proposed/token-cache.md", []byte(text), nil)
	if r.title != "Token cache" {
		t.Errorf("title = %q", r.title)
	}
	var got []string
	for _, s := range r.sections {
		got = append(got, fmt.Sprintf("%s|%s|%d", s.Anchor, s.Heading, s.Level))
	}
	want := []string{"||0", "keying-per-tenant|Keying: per *tenant*|2", "keying-per-tenant-1|Keying: per *tenant*|2"}
	if !slices.Equal(got, want) {
		t.Errorf("sections = %q, want %q", got, want)
	}
	if b := r.sections[0].Body; strings.Contains(b, "status:") || !strings.Contains(b, "Why we cache tokens") {
		t.Errorf("the top's words = %q", b)
	}
	if b := r.sections[1].Body; !strings.Contains(b, "# not a heading") {
		t.Errorf("a fence's lines are words, not headings: %q", b)
	}
	var cites []string
	for _, l := range r.links {
		cites = append(cites, fmt.Sprintf("%s:%s@%d(%s)", l.repo, l.path, l.line, l.how))
	}
	wantCites := []string{"records:bp/adr/0038-cache.md@7(link)", "records:bp/proposed/README.md@11(link)"}
	if !slices.Equal(cites, wantCites) {
		t.Errorf("cites %q, want %q", cites, wantCites)
	}
}

// corpus reads testdata/corpus into records as the space "designs".
func corpus(t testing.TB) *store.DB {
	t.Helper()
	db := records(t)
	files, _ := filepath.Glob("testdata/corpus/*.md")
	if err := db.Write(bg, func(tx *store.Tx) error {
		for _, f := range files {
			text, err := os.ReadFile(f)
			if err != nil {
				return err
			}
			p := filepath.Base(f)
			if err := keepDoc(tx, "designs", entry{path: p, blob: p}, len(text), parse("designs", p, text, nil), change{}); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestSearch_Ranking(t *testing.T) {
	db := corpus(t)
	data, err := os.ReadFile("testdata/ranking.json")
	if err != nil {
		t.Fatal(err)
	}
	var file struct {
		Cases []struct {
			Q      string  `json:"q"`
			Want   string  `json:"want"`
			Anchor *string `json:"anchor"`
			Within int     `json:"within"`
			Weak   string  `json:"weak"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(data, &file); err != nil {
		t.Fatal(err)
	}
	for _, c := range file.Cases {
		hits, _, err := Search(bg, db, Query{Words: c.Q, Limit: 5})
		if err != nil {
			t.Errorf("%q: %v", c.Q, err)
			continue
		}
		var got []string
		at := -1
		for i, h := range hits {
			got = append(got, fmt.Sprintf("%s#%s (%.1f)", h.Path, h.Anchor, h.Score))
			if h.Path == c.Want && at < 0 {
				at = i
			}
		}
		switch {
		case c.Weak != "" && (at < 0 || at >= c.Within):
			t.Logf("%q (weak: %s): %s", c.Q, c.Weak, strings.Join(got, ", "))
		case at < 0 || at >= c.Within:
			t.Errorf("%q: %s not within %d: %s", c.Q, c.Want, c.Within, strings.Join(got, ", "))
		case c.Anchor != nil && hits[at].Anchor != *c.Anchor:
			t.Errorf("%q: %s leads to #%s, want #%s", c.Q, c.Want, hits[at].Anchor, *c.Anchor)
		default:
			t.Logf("%q: %s", c.Q, strings.Join(got, ", "))
		}
	}
}

func TestSearch_SomeWordsCountToo_AndSaySo(t *testing.T) {
	db := corpus(t)
	hits, _, err := Search(bg, db, Query{Words: "decisions xyzzy", Limit: 3})
	if err != nil || len(hits) == 0 {
		t.Fatalf("%v %v", hits, err)
	}
	if hits[0].Every || hits[0].Path != "02-decisions.md" {
		t.Errorf("first = %+v", hits[0])
	}
	if hits, _, _ := Search(bg, db, Query{Words: "cold start tests", Limit: 3}); !hits[0].Every {
		t.Errorf("every word, and not said: %+v", hits[0])
	}
	hits, more, _ := Search(bg, db, Query{Words: "the", Limit: 2})
	if len(hits) != 2 || more == 0 {
		t.Errorf("limit: %d hits, %d more", len(hits), more)
	}
	if hits, _, _ := Search(bg, db, Query{Words: "decision", Repos: []string{"elsewhere"}}); len(hits) != 0 {
		t.Errorf("a space not asked for: %v", hits)
	}
	if len(hits[0].Snippet) == 0 {
		t.Errorf("snippet = %+v", hits[0].Snippet)
	}
	hits, _, _ = Search(bg, db, Query{Words: "suggested edits"})
	if !slices.ContainsFunc(hits[0].Snippet, func(p Part) bool { return p.Mark }) {
		t.Errorf("snippet's words unmarked: %+v", hits[0].Snippet)
	}
}

// git runs git in dir, failing the test if it fails.
func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func write(t *testing.T, dir, rel, text string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(rel))
	os.MkdirAll(filepath.Dir(p), 0o755)
	if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRefresh_ReadsWhatACommitChanged(t *testing.T) {
	root := t.TempDir()
	db, err := store.Own(root)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ws := &workspace.Workspace{Root: root, RepoNames: []string{"api", "notes", "records"}, Records: "records",
		DocumentsSaid: map[string]*config.Documents{"api": {On: true, Folder: "docs/"}}}
	for _, repo := range ws.RepoNames {
		g := ws.MainWorktree(repo)
		os.MkdirAll(g, 0o755)
		gitIn(t, g, "init", "-q", "-b", "main")
	}
	rec := ws.MainWorktree("records")
	write(t, rec, "adr/0001-cache.md", "# Cache tokens\n\nPer tenant.\n")
	write(t, rec, "bp/proposed/2026-11-12-retries.md", "# Retries\n\nSee [the ADR](../../adr/0001-cache.md). It's what RAM-12 asks of the api (api#4), and RAM-12 again.\n")
	write(t, rec, "logo.png", "not a document")
	gitIn(t, rec, "add", ".")
	gitIn(t, rec, "commit", "-q", "-m", "first")
	api := ws.MainWorktree("api")
	write(t, api, "docs/guide.md", "# Guide\n")
	write(t, api, "README.md", "# API\n") // not under the folder: code
	gitIn(t, api, "add", ".")
	gitIn(t, api, "commit", "-q", "-m", "first")
	write(t, ws.MainWorktree("notes"), "a.md", "# Not a space\n")

	// What there is to name: the repos, and a plugin's issues.
	known := func() *ref.Known {
		return &ref.Known{Repos: map[string]ref.Tree{"api": nil, "notes": nil, "records": nil},
			Things: []ref.Named{{Plugin: "jira", What: "issue", Prefixes: []string{"RAM-"}}}}
	}
	st, err := Refresh(bg, db, ws, known)
	if err != nil {
		t.Fatal(err)
	}
	if st.Spaces != 2 || st.Read != 3 {
		t.Fatalf("first: %+v", st)
	}
	// Each is a thing in the graph, as the repo's file is named, and what
	// its words name is linked from it: not the repo by its name alone.
	retries, issue := store.Key{Name: "records/bp/proposed/2026-11-12-retries.md"}, store.Key{Name: "RAM-12"}
	if got := naming(t, db, issue); len(got) != 1 || got[0].Src != retries || got[0].Kind != store.Mentions || got[0].From.Kind != ThingKind || got[0].From.Title != "Retries" {
		t.Fatalf("what names RAM-12: %+v", got)
	}
	if got := naming(t, db, store.Key{Name: "api#4"}); len(got) != 1 || got[0].Src != retries {
		t.Fatalf("what names api#4: %+v", got)
	}
	if got := naming(t, db, store.Key{Name: "api"}); len(got) != 0 {
		t.Fatalf("the api repo, named by its name alone, is linked: %+v", got)
	}
	docs, _ := Docs(bg, db, "records")
	if len(docs) != 2 || docs[0].Title != "Cache tokens" || docs[0].ChangedBy == "" || docs[0].Changed == "" {
		t.Fatalf("records' documents: %+v", docs)
	}
	if apiDocs, _ := Docs(bg, db, "api"); len(apiDocs) != 1 || apiDocs[0].Path != "docs/guide.md" {
		t.Fatalf("api's: %+v", apiDocs)
	}
	cited, _ := CitedBy(bg, db, "records", "adr/0001-cache.md")
	if len(cited) != 1 || cited[0].Title != "Retries" {
		t.Fatalf("cited by: %+v", cited)
	}

	// Nothing moved: nothing's read.
	if st, _ = Refresh(bg, db, ws, nil); st.Moved != 0 || st.Read != 0 {
		t.Fatalf("again: %+v", st)
	}

	// A commit changes one and removes another: only those.
	write(t, rec, "adr/0001-cache.md", "# Cache tokens per tenant\n\nMinting is slow.\n")
	gitIn(t, rec, "rm", "-q", "bp/proposed/2026-11-12-retries.md")
	gitIn(t, rec, "commit", "-q", "-am", "second")
	told, stop := db.Watch()
	defer stop()
	if st, err = Refresh(bg, db, ws, nil); err != nil || st.Moved != 1 || st.Read != 1 || st.Removed != 1 {
		t.Fatalf("after a commit: %+v %v", st, err)
	}
	select {
	case <-told:
	default:
		t.Error("the records weren't told")
	}
	if hits, _, _ := Search(bg, db, Query{Words: "minting"}); len(hits) != 1 || hits[0].Title != "Cache tokens per tenant" {
		t.Fatalf("search: %+v", hits)
	}
	if hits, _, _ := Search(bg, db, Query{Words: "retries"}); len(hits) != 0 {
		t.Fatalf("a removed document's words: %+v", hits)
	}
	if cited, _ := CitedBy(bg, db, "records", "adr/0001-cache.md"); len(cited) != 0 {
		t.Fatalf("a removed document's citations: %+v", cited)
	}
	if got := naming(t, db, issue); len(got) != 0 {
		t.Fatalf("a removed document still names RAM-12: %+v", got)
	}

	// The records repo isn't a space once the workspace says so.
	ws.DocumentsSaid["records"] = &config.Documents{On: false}
	if _, err := Refresh(bg, db, ws, nil); err != nil {
		t.Fatal(err)
	}
	if docs, _ := Docs(bg, db, "records"); len(docs) != 0 {
		t.Fatalf("let go of: %+v", docs)
	}
	var things int
	db.Read(bg, func(tx *store.Tx) error {
		return tx.QueryRow(`SELECT count(*) FROM things WHERE kind = ? AND name LIKE 'records/%'`, ThingKind).Scan(&things)
	})
	if things != 0 {
		t.Fatalf("%d of its documents are still things in the graph", things)
	}
}

// naming is what points at a thing in the graph.
func naming(t testing.TB, db *store.DB, k store.Key) []store.Linked {
	t.Helper()
	var out []store.Linked
	if err := db.Read(bg, func(tx *store.Tx) (err error) {
		out, err = tx.LinksTo(k)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return out
}

// Through a command's connection to the daemon, as `ws documents` reads.
func TestRefresh_FromACommand(t *testing.T) {
	root := t.TempDir()
	owner, err := store.Own(root)
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	cmdDB := borrowed(t, root)
	ws := &workspace.Workspace{Root: root, RepoNames: []string{"records"}, Records: "records"}
	g := ws.MainWorktree("records")
	os.MkdirAll(g, 0o755)
	gitIn(t, g, "init", "-q", "-b", "main")
	write(t, g, "a.md", "# Alpha\n\nWords about minting.\n")
	gitIn(t, g, "add", ".")
	gitIn(t, g, "commit", "-q", "-m", "first")
	if st, err := Refresh(bg, cmdDB, ws, nil); err != nil || st.Read != 1 {
		t.Fatalf("%+v %v", st, err)
	}
	if hits, _, err := Search(bg, cmdDB, Query{Words: "minting"}); err != nil || len(hits) != 1 {
		t.Fatalf("%+v %v", hits, err)
	}
}

func TestSearch_TheLastWordAsItsBegun(t *testing.T) {
	db := corpus(t)
	// "decis" is no word, but it's how "decisions" starts.
	hits, _, err := Search(bg, db, Query{Words: "decis", Limit: 3})
	if err != nil || len(hits) == 0 || hits[0].Path != "02-decisions.md" {
		t.Fatalf("begun: %+v %v", hits, err)
	}
	if !slices.ContainsFunc(hits[0].Snippet, func(p Part) bool { return p.Mark }) {
		t.Errorf("what it begins isn't marked: %+v", hits[0].Snippet)
	}
	// Finished (a space after it), it's the word it is: none.
	if hits, _, _ := Search(bg, db, Query{Words: "decis "}); len(hits) != 0 {
		t.Errorf("finished: %+v", hits)
	}
}
