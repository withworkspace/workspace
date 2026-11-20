package web

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/config"
	"github.com/withworkspace/workspace/internal/doc"
	gitpkg "github.com/withworkspace/workspace/internal/git"
	"github.com/withworkspace/workspace/internal/ops"
	pages "github.com/withworkspace/workspace/internal/page"
	"github.com/withworkspace/workspace/internal/store/storetest"
	"github.com/withworkspace/workspace/internal/workspace"
)

// A workspace whose records repo is a space: its documents, read from its
// ground into the records this test owns, are listed by space and by
// folder; a repo of code isn't one.
func TestDocuments_SpacesAndTheirDocuments(t *testing.T) {
	root := t.TempDir()
	storetest.Own(t, root)
	ground := filepath.Join(root, "repos", "records", ".ground")
	os.MkdirAll(filepath.Join(ground, "adr"), 0o755)
	os.WriteFile(filepath.Join(ground, "adr", "0001-cache.md"), []byte("# Cache tokens per tenant\n"), 0o644)
	os.WriteFile(filepath.Join(ground, "README.md"), []byte("# Records\n\nSee [the cache](adr/0001-cache.md).\n"), 0o644)
	for _, args := range [][]string{{"init", "-q", "-b", "main"}, {"add", "."}, {"commit", "-q", "-m", "first"}} {
		if out, err := exec.Command("git", append([]string{"-C", ground}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	os.MkdirAll(filepath.Join(root, "repos", "api", ".ground"), 0o755)

	s := startTest(t, root, &fakeWorkspace{}, nil)
	s.App = testOps(s, ops.Options{}, func(c *config.Config, ws *workspace.Workspace) {
		ws.RepoNames = []string{"api", "records"}
		ws.Records = "records"
		ws.DisplayNames = map[string]string{"records": "Records"}
	})

	// The first read answers at once, with what's kept: nothing yet. The
	// ground's read in behind it, and the workspace's spacesSeen moves.
	res := do(t, s, "GET", "/api/documents", true, nil)
	var list struct {
		Spaces  []Space `json:"spaces"`
		Problem string  `json:"problem"`
	}
	json.NewDecoder(res.Body).Decode(&list)
	if res.StatusCode != 200 || len(list.Spaces) != 1 || list.Spaces[0].Count != 0 {
		t.Fatalf("first read: %d %+v", res.StatusCode, list)
	}
	spacesSeen(t, s, "")
	res = do(t, s, "GET", "/api/documents", true, nil)
	json.NewDecoder(res.Body).Decode(&list)
	if res.StatusCode != 200 || len(list.Spaces) != 1 || list.Spaces[0].Repo != "records" || list.Spaces[0].Count != 2 || list.Spaces[0].Label != "Records" || list.Problem != "" {
		t.Fatalf("%d: %+v", res.StatusCode, list)
	}

	res = do(t, s, "GET", "/api/documents/records", true, nil)
	var one struct {
		Space Space      `json:"space"`
		Docs  []SpaceDoc `json:"docs"`
	}
	json.NewDecoder(res.Body).Decode(&one)
	if res.StatusCode != 200 || len(one.Docs) != 2 || one.Docs[1].Path != "adr/0001-cache.md" || one.Docs[1].Title != "Cache tokens per tenant" || one.Docs[1].ChangedBy == "" {
		t.Fatalf("%d: %+v", res.StatusCode, one)
	}
	if res := do(t, s, "GET", "/api/documents/api", true, nil); res.StatusCode != 404 {
		t.Errorf("a repo of code: %d %s", res.StatusCode, body(res))
	}

	// Searched, the ADR's found by its title; what cites it, the README.
	res = do(t, s, "GET", "/api/documents/search?q=tenant", true, nil)
	var found struct {
		Hits []DocHit `json:"hits"`
	}
	json.NewDecoder(res.Body).Decode(&found)
	if res.StatusCode != 200 || len(found.Hits) != 1 || found.Hits[0].Path != "adr/0001-cache.md" || found.Hits[0].Label != "Records" {
		t.Fatalf("search: %d %+v", res.StatusCode, found)
	}
	res = do(t, s, "GET", "/api/documents/records/cites?path=adr/0001-cache.md", true, nil)
	var cites struct {
		CitedBy []Citation `json:"citedBy"`
		Cites   []Citation `json:"cites"`
	}
	json.NewDecoder(res.Body).Decode(&cites)
	if res.StatusCode != 200 || len(cites.CitedBy) != 1 || cites.CitedBy[0].Path != "README.md" || cites.CitedBy[0].Title != "Records" || len(cites.Cites) != 0 {
		t.Fatalf("cites: %d %+v", res.StatusCode, cites)
	}

	// Files says which repos have their home in Documents.
	res = do(t, s, "GET", "/api/repos", true, nil)
	var repos struct {
		Repos []Repo `json:"repos"`
	}
	json.NewDecoder(res.Body).Decode(&repos)
	for _, r := range repos.Repos {
		if r.Documents != (r.Name == "records") {
			t.Errorf("%s: documents = %v", r.Name, r.Documents)
		}
	}
}

// spacesSeen waits for the workspace's spacesSeen to move from was, as a
// page waits for its event, and says what it's moved to.
func spacesSeen(t *testing.T, s *Server, was string) string {
	t.Helper()
	for range 500 {
		res := do(t, s, "GET", "/api/workspace", true, nil)
		var v Workspace
		json.NewDecoder(res.Body).Decode(&v)
		if v.SpacesSeen != was {
			return v.SpacesSeen
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("spacesSeen didn't move from %q", was)
	return ""
}

// A space's activity answers at once: unread at first, read behind it;
// then as kept, without asking git, and a capsule's new document found
// behind a read that's given what was (ops's tests hold the look, to see
// no read starts git), the workspace's spacesSeen moving, and the
// next read has it. The event's due at once: the workspace's topic is
// nudged, not left to its poll.
func TestDocuments_ActivityIsKeptAndPushed(t *testing.T) {
	root := t.TempDir()
	storetest.Own(t, root)
	git := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false"}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	repo := filepath.Join(root, "repos", "records")
	seed := filepath.Join(root, "seed")
	os.MkdirAll(filepath.Join(seed, "adr"), 0o755)
	os.WriteFile(filepath.Join(seed, "adr", "0001-cache.md"), []byte("# Cache tokens per tenant\n"), 0o644)
	git(root, "init", "-q", "--bare", "--initial-branch=main", "origin.git")
	git(seed, "init", "-q", "--initial-branch=main")
	git(seed, "add", ".")
	git(seed, "commit", "-q", "-m", "first")
	git(seed, "push", "-q", filepath.Join(root, "origin.git"), "main")
	os.MkdirAll(repo, 0o755)
	git(repo, "clone", "-q", "--bare", filepath.Join(root, "origin.git"), ".bare")
	git(filepath.Join(repo, ".bare"), "config", "remote.origin.fetch", "+refs/heads/*:refs/remotes/origin/*")
	git(filepath.Join(repo, ".bare"), "fetch", "-q", "origin")
	git(filepath.Join(repo, ".bare"), "worktree", "add", "-q", filepath.Join(repo, ".ground"), "main")
	git(filepath.Join(repo, ".bare"), "worktree", "add", "-q", "-b", "note", filepath.Join(repo, "note"), "origin/main")

	s := startTest(t, root, &fakeWorkspace{}, nil)
	s.App = testOps(s, ops.Options{}, func(c *config.Config, ws *workspace.Workspace) {
		ws.RepoNames = []string{"records"}
		ws.Records = "records"
		ws.DefaultBranch = "main"
	})
	read := func() SpaceActivity {
		t.Helper()
		res := do(t, s, "GET", "/api/documents/records/activity", true, nil)
		var a SpaceActivity
		if err := json.NewDecoder(res.Body).Decode(&a); err != nil || res.StatusCode != 200 {
			t.Fatalf("%d: %v", res.StatusCode, err)
		}
		return a
	}
	seen := s.App.Seen()

	if a := read(); !a.Unread || len(a.Lately) != 0 {
		t.Fatalf("first read: %+v", a)
	}
	was := spacesSeen(t, s, "")
	select {
	case <-seen:
	case <-time.After(5 * time.Second):
		t.Fatal("the look that found it didn't say so")
	}
	a := read()
	if a.Unread || len(a.Lately) != 1 || a.Lately[0].Docs[0].Title != "Cache tokens per tenant" || len(a.InFlight) != 0 {
		t.Fatalf("once read: %+v", a)
	}

	// Read again, nothing's moved: kept, and git isn't asked.
	before := gitpkg.Started()
	for range 5 {
		read()
	}
	if n := gitpkg.Started() - before; n != 0 {
		t.Errorf("5 reads of a space that hasn't moved started git %d times, want none", n)
	}

	// A capsule commits a new document: the read's given what was, and
	// what's in flight is found behind it.
	os.MkdirAll(filepath.Join(repo, "note", "notes"), 0o755)
	os.WriteFile(filepath.Join(repo, "note", "notes", "new.md"), []byte("# A new note\n"), 0o644)
	git(filepath.Join(repo, "note"), "add", ".")
	git(filepath.Join(repo, "note"), "commit", "-q", "-m", "a note")
	if a := read(); len(a.InFlight) != 0 {
		t.Fatalf("read as it was: %+v", a)
	}
	spacesSeen(t, s, was)
	if a := read(); len(a.InFlight) != 1 || a.InFlight[0].Capsule != "note" || a.InFlight[0].Docs[0].Title != "A new note" {
		t.Fatalf("in flight: %+v", a.InFlight)
	}
}

// A page tied to a file says when it has something to save: what it would
// write isn't what it last agreed with the file.
func TestPages_UnsavedWhenATiedPageHasSomethingToSave(t *testing.T) {
	root := t.TempDir()
	storetest.Own(t, root)
	ps := pages.Store{Root: root}
	p, err := ps.New([]byte("# A\n\nWords.\n"))
	if err != nil {
		t.Fatal(err)
	}
	text, _, _ := ps.Read(p.ID)
	ps.SetMeta(p.ID, pages.Meta{Source: &pages.Source{Ref: "records:a.md", Version: doc.Version(pages.AsText(text))}})
	if Pages(root)[0].Unsaved {
		t.Error("as it was taken in, and said to be unsaved")
	}
	if _, err := ps.Write(p.ID, doc.Version(text), []byte("# A\n\nMore words.\n")); err != nil {
		t.Fatal(err)
	}
	if !Pages(root)[0].Unsaved {
		t.Error("edited, and not said to be unsaved")
	}
}
