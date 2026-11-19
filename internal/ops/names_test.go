package ops

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/config"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/store"
)

// A test's machine may be busy: what's written waits for the repos' files
// as long as it takes, so that what's linked doesn't depend on it.
func init() { namesWait = 30 * time.Second }

// saidBy is what's said of a thing, each as how it points and what it is:
// mentions brief cache/brief.
func saidBy(t *testing.T, w *Workspace, k store.Key) []string {
	t.Helper()
	said, err := w.SaidOf(k)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, s := range said {
		line := fmt.Sprintf("%s %s %s/%s", s.How, s.Kind, s.Scope, s.Ref)
		if s.Lines != "" {
			line += ":" + s.Lines
		}
		out = append(out, line)
	}
	slices.Sort(out)
	return out
}

// What a mission's brief, a handoff of its and a decision name is said of
// it, wherever it was written: a plugin's thing and a file alike. The
// brief and the handoff are things in their mission, and each lets go of
// what it named when it's edited, or goes.
func TestWhatABriefAHandoffAndADecisionNameIsSaidOfIt(t *testing.T) {
	w, _ := fixture(t, "")
	groundRepo(t, w)
	installed(t, w.WS().Root, "jira", jiraIssue)
	ctx := context.Background()
	slug := "cache"
	dir := filepath.Join(w.Root(), ".ws", "missions", slug)
	reread := func() {
		t.Helper()
		if _, err := w.Missions.Reread(ctx); err != nil {
			t.Fatal(err)
		}
	}

	if err := w.Missions.WriteBrief(slug, "# Tokens\n\n## Goal\n\nWhat RAM-12 asks for: see frontend/plan.md:3, in the frontend repo.\n"); err != nil {
		t.Fatal(err)
	}
	os.MkdirAll(filepath.Join(dir, "files", "handoffs"), 0o755)
	handoff := filepath.Join(dir, "files", "handoffs", "c1.md")
	os.WriteFile(handoff, []byte("# Half way\n\nRAM-12 is half done; RAP-3 is next.\n"), 0o644)
	reread()
	d, err := w.Missions.Decide(slug, mission.Decision{What: "Do RAM-12 first", Why: "RAP-3 waits on it", By: "you", At: time.Now()})
	if err != nil {
		t.Fatal(err)
	}

	issue := store.Key{Name: "RAM-12"}
	want := []string{"mentions brief cache/brief", "mentions decision cache/" + d.ID, "mentions handoff cache/mission:handoffs/c1.md"}
	if got := saidBy(t, w, issue); !slices.Equal(got, want) {
		t.Errorf("said of RAM-12: %q, want %q", got, want)
	}
	if got, want := saidBy(t, w, store.Key{Name: "RAP-3"}), []string{"mentions decision cache/" + d.ID, "mentions handoff cache/mission:handoffs/c1.md"}; !slices.Equal(got, want) {
		t.Errorf("said of RAP-3: %q, want %q", got, want)
	}
	if got, want := saidBy(t, w, store.Key{Name: "frontend/plan.md"}), []string{"mentions brief cache/brief:3"}; !slices.Equal(got, want) {
		t.Errorf("said of the file: %q, want %q", got, want)
	}
	if got := saidBy(t, w, store.Key{Name: "frontend"}); len(got) != 0 {
		t.Errorf("the repo, named by its name alone, is said of: %q", got)
	}
	// The brief and the handoff are in their mission, called what they are.
	if got, want := saidBy(t, w, store.Key{Name: slug}), []string{"in brief cache/brief", "in handoff cache/mission:handoffs/c1.md"}; !slices.Equal(got, want) {
		t.Errorf("in the mission: %q, want %q", got, want)
	}
	said, _ := w.SaidOf(issue)
	for _, s := range said {
		if s.Kind == "brief" && s.Title != "Cache" || s.Kind == "handoff" && s.Title != "Half way" {
			t.Errorf("%s is called %q", s.Ref, s.Title)
		}
	}

	// Retitled and parked, the brief is called what its mission is, and it
	// and the handoff stand as the mission does.
	for _, s := range said {
		if s.Kind != "decision" && !s.Standing() {
			t.Errorf("%s of a mission in play doesn't stand: %q", s.Ref, s.State)
		}
	}
	if _, err := w.Missions.Update(slug, func(m *mission.Mission) error { m.Title, m.State = "Token cache", mission.Parked; return nil }); err != nil {
		t.Fatal(err)
	}
	said, _ = w.SaidOf(issue)
	if i := slices.IndexFunc(said, func(s Said) bool { return s.Kind == "brief" }); i < 0 || said[i].Title != "Token cache" || said[i].State != "parked" {
		t.Errorf("after the mission's retitled and parked, its brief: %+v", said)
	}
	if i := slices.IndexFunc(said, func(s Said) bool { return s.Kind == "handoff" }); i < 0 || said[i].State != "parked" {
		t.Errorf("after the mission's parked, its handoff: %+v", said)
	}

	// The brief's edited, and the handoff's taken away: neither names it now.
	if err := w.Missions.WriteBrief(slug, "# Tokens\n\n## Goal\n\nWhat was asked for.\n"); err != nil {
		t.Fatal(err)
	}
	os.Remove(handoff)
	reread()
	if got, want := saidBy(t, w, issue), []string{"mentions decision cache/" + d.ID}; !slices.Equal(got, want) {
		t.Errorf("said of RAM-12 afterwards: %q, want %q", got, want)
	}
	if got, want := saidBy(t, w, store.Key{Name: slug}), []string{"in brief cache/brief"}; !slices.Equal(got, want) {
		t.Errorf("in the mission afterwards: %q, want %q", got, want)
	}
}

// A document on a space's default branch is a thing, and what it names is
// said of it. One written before a plugin named its things names them once
// the records are said again (ws doctor --reindex): its text hasn't
// changed, so nothing else would read it again.
func TestWhatADocumentNamesIsSaidOfIt(t *testing.T) {
	w, _ := fixture(t, "")
	ground := w.WS().MainWorktree("frontend")
	os.MkdirAll(ground, 0o755)
	os.WriteFile(filepath.Join(ground, "notes.md"), []byte("# Notes\n\nRAM-12 wants this, as frontend/main.go would do it.\n"), 0o644)
	os.WriteFile(filepath.Join(ground, "main.go"), []byte("package main\n"), 0o644)
	groundRepo(t, w)
	w.WS().DocumentsSaid = map[string]*config.Documents{"frontend": {On: true}}
	ctx := context.Background()
	issue := store.Key{Name: "RAM-12"}

	if _, st, err := w.Spaces(ctx); err != nil || st.Read != 2 {
		t.Fatalf("reading the space: %+v, %v", st, err)
	}
	if got, want := saidBy(t, w, store.Key{Name: "frontend/main.go"}), []string{"mentions document /frontend/notes.md"}; !slices.Equal(got, want) {
		t.Errorf("said of the file it names: %q, want %q", got, want)
	}
	if got := saidBy(t, w, issue); len(got) != 0 {
		t.Fatalf("said of RAM-12 before anything names issues: %q", got)
	}

	installed(t, w.WS().Root, "jira", jiraIssue)
	if _, st, err := w.Spaces(ctx); err != nil || st.Read != 0 {
		t.Fatalf("nothing's moved, and it was read again: %+v, %v", st, err)
	}
	n, err := w.SaidWords(ctx)
	if err != nil || n.Documents != 2 {
		t.Fatalf("saying the records again: %+v, %v", n, err)
	}
	if got, want := saidBy(t, w, issue), []string{"mentions document /frontend/notes.md"}; !slices.Equal(got, want) {
		t.Errorf("said of RAM-12 once the records are said again: %q, want %q", got, want)
	}
	said, _ := w.SaidOf(issue)
	if len(said) != 1 || said[0].Title != "Notes" {
		t.Errorf("the document that names it: %+v", said)
	}
}
