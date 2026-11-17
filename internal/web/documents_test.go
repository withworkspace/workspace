package web

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/withworkspace/workspace/internal/config"
	"github.com/withworkspace/workspace/internal/doc"
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

	res := do(t, s, "GET", "/api/documents", true, nil)
	var list struct {
		Spaces  []Space `json:"spaces"`
		Problem string  `json:"problem"`
	}
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
