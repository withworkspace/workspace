package doc_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/testutil"
)

// A space's top: what changed lately on its default branch, the capsules
// in flight with changes to its documents, and its ground against its
// remote, fetched and then moved forward.
func TestDocuments_ChangedLatelyInFlightAndFresh(t *testing.T) {
	w := docsWorkspace(t, "")
	f, _ := os.OpenFile(filepath.Join(w.Root, "ws.toml"), os.O_APPEND|os.O_WRONLY, 0)
	f.WriteString("\n[missions]\nrecords = \"records\"\n")
	f.Close()
	ctx := context.Background()
	o := testutil.OpsAt(w.Root, time.Now())
	// What's kept is said at once, and read again behind the read where
	// it's moved: the read after that look has what it found.
	activity := func() (*ops.SpaceActivity, error) {
		t.Helper()
		if _, err := o.SpaceActivity(ctx, "records"); err != nil {
			return nil, err
		}
		select {
		case <-o.Seen():
		case <-time.After(10 * time.Second):
			t.Fatal("the look behind the read found nothing")
		}
		return o.SpaceActivity(ctx, "records")
	}

	a, err := activity()
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Lately) != 2 || a.Lately[0].Subject != "docs: add the plan" || len(a.Lately[0].Docs) != 1 || a.Lately[0].Docs[0].Path != "docs/plan.md" || a.Lately[0].Docs[0].How != "added" || a.Lately[0].Docs[0].Title != "Plan" {
		t.Fatalf("lately: %+v", a.Lately)
	}
	if len(a.InFlight) != 0 || a.Fresh.Behind != 0 || a.Fresh.Branch != "main" {
		t.Fatalf("in flight %+v, fresh %+v", a.InFlight, a.Fresh)
	}

	// A capsule that changes the plan (uncommitted) and adds a note.
	testutil.Run(t, w, nil, "lift", "records", "edit-plan")
	capsule := filepath.Join(w.Root, "repos", "records", "edit-plan")
	os.WriteFile(filepath.Join(capsule, "docs", "plan.md"), []byte("# Plan\n\nKey it per tenant.\n"), 0o644)
	os.WriteFile(filepath.Join(capsule, "docs", "note.md"), []byte("# A note\n"), 0o644)
	if a, _ = activity(); len(a.InFlight) != 1 || a.InFlight[0].Capsule != "edit-plan" || len(a.InFlight[0].Docs) != 2 ||
		a.InFlight[0].Docs[0].Path != "docs/note.md" || a.InFlight[0].Docs[0].How != "added" || a.InFlight[0].Docs[0].Title != "A note" || a.InFlight[0].Docs[1].How != "changed" {
		t.Fatalf("in flight: %+v", a.InFlight)
	}

	// Someone lands a document: fetched, the ground's behind; pulled, it's there.
	testutil.CommitIn(t, w.Sources["records"], "docs/later.md", "# Later\n", "docs: add later")
	fresh, err := o.FetchSpace(ctx, "records")
	if err != nil || fresh.Behind != 1 || fresh.Fetched.IsZero() {
		t.Fatalf("fetched: %+v %v", fresh, err)
	}
	if fresh, err = o.PullSpace(ctx, "records"); err != nil || fresh.Behind != 0 {
		t.Fatalf("pulled: %+v %v", fresh, err)
	}
	if _, err := os.Stat(filepath.Join(w.Root, "repos", "records", ".ground", "docs", "later.md")); err != nil {
		t.Errorf("the ground after: %v", err)
	}
	// The pull read its documents in: the page that pulled reads them at once.
	_, docs, _ := o.SpaceDocs(ctx, "records")
	pulled := false
	for _, d := range docs {
		pulled = pulled || d.Path == "docs/later.md"
	}
	if !pulled {
		t.Errorf("after the pull, the space's documents are %+v", docs)
	}
	if _, err := o.SpaceActivity(ctx, "api"); err == nil {
		t.Error("api isn't a space")
	}
}
