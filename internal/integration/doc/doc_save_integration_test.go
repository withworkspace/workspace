package doc_test

import (
	"github.com/withworkspace/workspace/internal/doc"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/testutil"
	"github.com/withworkspace/workspace/internal/web"
)

// A document edited in the web UI is saved into its capsule, committed as
// you; merged with what crew committed to it while you edited; and, where
// your edit and theirs change the same lines, not saved but handed back
// with the clash marked. A file with crew's uncommitted work in it gets
// the edit alongside, uncommitted.
func TestDocs_Save(t *testing.T) {
	w, _ := testutil.OrchestrateFixture(t)
	capsule := filepath.Join(w.Root, "repos", "backend", "cache-lookup")
	os.MkdirAll(filepath.Join(capsule, "docs"), 0o755)
	docFile := filepath.Join(capsule, "docs", "proposal.md")
	write := func(text string) { os.WriteFile(docFile, []byte(text), 0o644) }
	read := func() string { b, _ := os.ReadFile(docFile); return string(b) }
	commit := func(msg string) {
		testutil.GitCmd(t, capsule, "add", "-A")
		testutil.GitCmd(t, capsule, "commit", "-q", "-m", msg)
	}
	write("# Cache\n\nKey it per user.\n\n## Rollout\n\nShip on Friday.\n")
	commit("proposal")
	ref := "backend/cache-lookup:docs/proposal.md"
	testutil.Run(t, w, nil, "mission", "artifact", "add", "-m", "cache", docFile, "Cache proposal", "--kind", "proposal")
	threads := conversation.For(w.Root, "cache")
	open := func() *web.ArtifactContent {
		a, err := testutil.WebArtifact(w.Root)("cache", ref)
		if err != nil {
			t.Fatal(err)
		}
		threads.KeepVersion([]byte(a.Text)) // as the page's GET does
		return a
	}
	save := testutil.WebSaveDoc(w.Root)
	head := func() string { return strings.TrimSpace(testutil.GitOut(t, capsule, "rev-parse", "HEAD")) }

	// Saved as it was opened: committed, just that file, as you.
	a := open()
	if !a.Editable || a.Uneditable != "" {
		t.Fatalf("a capsule's document isn't editable: %+v", a)
	}
	(doc.Drafts{Root: w.Root, Scope: doc.OfMission("cache")}).Save(doc.Draft{Ref: ref, Base: doc.Version([]byte(a.Text)), Text: "draft", At: time.Now()})
	s, err := save("cache", web.DocSave{Ref: ref, Base: doc.Version([]byte(a.Text)), Text: strings.Replace(a.Text, "per user", "per tenant", 1)})
	if err != nil || s.Conflict != nil || !strings.Contains(s.Message, "Saved: committed") || s.Commit != head() {
		t.Fatalf("saving: %+v, %v", s, err)
	}
	if got := read(); got != "# Cache\n\nKey it per tenant.\n\n## Rollout\n\nShip on Friday.\n" {
		t.Errorf("document = %q", got)
	}
	if sub := strings.TrimSpace(testutil.GitOut(t, capsule, "log", "-1", "--format=%s")); sub != "Edit docs/proposal.md" {
		t.Errorf("commit subject = %q", sub)
	}
	if files := strings.TrimSpace(testutil.GitOut(t, capsule, "show", "--name-only", "--format=", "HEAD")); files != "docs/proposal.md" {
		t.Errorf("committed %q", files)
	}
	if d, _ := (doc.Drafts{Root: w.Root, Scope: doc.OfMission("cache")}).Get(ref); d != nil {
		t.Errorf("the draft's kept after saving: %+v", d)
	}
	if s.Version != doc.Version([]byte(read())) {
		t.Errorf("version after = %s", s.Version)
	}

	// Crew change another part while you edit: both stand, and the
	// commit's subject is yours.
	a = open()
	base := doc.Version([]byte(a.Text))
	write(strings.Replace(read(), "Ship on Friday.", "Ship on Tuesday.", 1))
	commit("crew: ship on Tuesday")
	mine := strings.Replace(a.Text, "# Cache", "# Token cache", 1)
	s, err = save("cache", web.DocSave{Ref: ref, Base: base, Text: mine, Message: "Name it for what it caches"})
	if err != nil || s.Conflict != nil || !s.Merged || !strings.Contains(s.Message, "merged with the changes made while you edited") {
		t.Fatalf("saving over crew's change: %+v, %v", s, err)
	}
	if got := read(); got != "# Token cache\n\nKey it per tenant.\n\n## Rollout\n\nShip on Tuesday.\n" {
		t.Errorf("merged document = %q", got)
	}
	if sub := strings.TrimSpace(testutil.GitOut(t, capsule, "log", "-1", "--format=%s")); sub != "Name it for what it caches" {
		t.Errorf("commit subject = %q", sub)
	}

	// Crew change the same line you did: nothing's written; the clash
	// comes back marked, to settle from what's there now.
	a = open()
	base = doc.Version([]byte(a.Text))
	write(strings.Replace(read(), "Ship on Tuesday.", "Ship on Wednesday.", 1))
	commit("crew: Wednesday")
	before, was := head(), read()
	s, err = save("cache", web.DocSave{Ref: ref, Base: base, Text: strings.Replace(a.Text, "Ship on Tuesday.", "Ship on Monday.", 1)})
	if err != nil || s.Conflict == nil || s.Conflict.Clashes != 1 {
		t.Fatalf("a clash: %+v, %v", s, err)
	}
	for _, want := range []string{"<<<<<<< your edit\nShip on Monday.\n=======\nShip on Wednesday.\n>>>>>>> the document now"} {
		if !strings.Contains(s.Conflict.Text, want) {
			t.Errorf("the clash lacks %q:\n%s", want, s.Conflict.Text)
		}
	}
	if read() != was || head() != before {
		t.Error("a clash was written")
	}
	if s.Version != doc.Version([]byte(was)) {
		t.Errorf("a clash is settled from the version now: %s", s.Version)
	}
	// Settled, it saves from there.
	settled := strings.Replace(was, "Ship on Wednesday.", "Ship on Monday.", 1)
	if s, err = save("cache", web.DocSave{Ref: ref, Base: s.Version, Text: settled}); err != nil || s.Conflict != nil || read() != settled {
		t.Fatalf("saving it settled: %+v, %v", s, err)
	}

	// Saving what's there already writes nothing.
	before = head()
	if s, err = save("cache", web.DocSave{Ref: ref, Base: doc.Version([]byte(settled)), Text: settled}); err != nil || !strings.Contains(s.Message, "Nothing to save") || head() != before {
		t.Errorf("saving no change: %+v, %v", s, err)
	}

	// Crew's uncommitted work in the file: your edit goes in alongside it,
	// uncommitted.
	write(settled + "\nCrew's draft line.\n")
	a = open()
	s, err = save("cache", web.DocSave{Ref: ref, Base: doc.Version([]byte(a.Text)), Text: strings.Replace(a.Text, "per tenant", "per tenant and region", 1)})
	if err != nil || !strings.Contains(s.Message, "not committed") || head() != before {
		t.Fatalf("saving into crew's work: %+v, %v", s, err)
	}
	if !strings.Contains(read(), "per tenant and region") || !strings.Contains(read(), "Crew's draft line.") {
		t.Errorf("document = %q", read())
	}

	// A version that isn't kept can't be merged with: it says so.
	write(read() + "More.\n")
	if _, err := save("cache", web.DocSave{Ref: ref, Base: "000000000000", Text: "# Mine\n"}); err == nil || !strings.Contains(err.Error(), "isn't kept") {
		t.Errorf("saving from a version that isn't kept: %v", err)
	}
}
