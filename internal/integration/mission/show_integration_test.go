package mission_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/ref"
	"github.com/withworkspace/workspace/internal/review"
	"github.com/withworkspace/workspace/internal/testutil"
)

// ws show finds its way around a mission from any ref: a stage, its crew,
// their capsule, what they recorded and handed off, and back.
func TestShow_FollowsLinks(t *testing.T) {
	w, store := testutil.OrchestrateFixture(t)
	capsule := filepath.Join(w.Root, "repos", "backend", "cache-lookup")
	store.WriteBrief("cache", "## Goal\nCache the lookup.\n\n## Stages\n1. Write a proposal in `backend/cache-lookup`. Check in with me.\n2. Build it.\n")
	if _, err := store.JoinStage("cache", "c1", 1); err != nil {
		t.Fatal(err)
	}
	os.MkdirAll(filepath.Join(capsule, "docs"), 0o755)
	os.WriteFile(filepath.Join(capsule, "docs", "proposal.md"), []byte("# Cache\n\nPer tenant.\n"), 0o644)
	testutil.GitCmd(t, capsule, "add", "-A")
	testutil.GitCmd(t, capsule, "commit", "-q", "-m", "Propose a per-tenant cache")
	doc := "backend/cache-lookup:docs/proposal.md"
	t.Setenv(mission.EnvMission, "cache")
	t.Setenv(mission.EnvCrew, "c1")
	testutil.Run(t, w, nil, "mission", "artifact", "add", filepath.Join(capsule, "docs", "proposal.md"), "Cache proposal", "--kind", "proposal")
	testutil.Run(t, w, nil, "mission", "done", "Wrote the proposal", "--handoff", writeHandoff(t, "- "+doc+" — the proposal"))
	t.Setenv(mission.EnvCrew, "")

	show := func(args ...string) string {
		t.Helper()
		return testutil.Run(t, w, nil, append([]string{"show", "-f", "llm"}, args...)...).Stdout
	}
	has := func(what, out string, wants ...string) {
		t.Helper()
		for _, want := range wants {
			if !strings.Contains(out, want) {
				t.Errorf("%s lacks %q:\n%s", what, want, out)
			}
		}
	}

	has("the mission", show(),
		"Crew (ws show c<n>", "c1 cursor in backend/cache-lookup: running, reported done: Wrote the proposal",
		"ws show s<n> for one", "ws show <ref> shows any of these")
	has("a stage", show("s1"),
		"Stage s1 of 2 (current): Write a proposal", "Check in with the user after it",
		"c1  ", "worked on it", doc, "made for it by c1", "backend/cache-lookup", "the capsule it works in", "s2", "the stage after: Build it")
	has("crew", show("c1"),
		"Crew c1 (cursor; running, reported done: Wrote the proposal)", "Worked in backend/cache-lookup.",
		"Its handoff (mission:handoffs/c1.md):", "Lookups happen twice per request.",
		"the stage it worked on", "it recorded this proposal: Cache proposal")
	has("the document", show(doc),
		`Document backend/cache-lookup:docs/proposal.md — "Cache proposal" (proposal, draft, in capsule backend/cache-lookup, no PR yet)`,
		"Recorded by c1", "(3 lines)", "Its history, newest first:", "Propose a per-tenant cache",
		"recorded it", "the stage it was made for: Write a proposal", "the capsule it's in", "mission:handoffs/c1.md", "c1's handoff mentions it")
	has("the capsule", show("backend/cache-lookup"),
		"Capsule backend/cache-lookup", "commit", "ahead of origin/main", "Propose a per-tenant cache",
		"worked in it", "a stage that works in it", doc)

	// From inside the capsule, a path names the artifact.
	t.Setenv(mission.EnvMission, "")
	t.Chdir(capsule)
	if r := testutil.RunCommand(t, capsule, nil, "show", "-f", "llm", "docs/proposal.md"); r.Err != nil || !strings.Contains(r.Stdout, "Document "+doc) {
		t.Errorf("by path from the capsule: %v\n%s", r.Err, r.Stdout)
	}
	if r := testutil.RunCommand(t, w.Root, nil, "show", "-m", "cache", "s9"); r.Err == nil || !strings.Contains(r.Err.Error(), "s1 to s2, not s9") {
		t.Errorf("a stage that isn't: %v", r.Err)
	}
	if r := testutil.RunCommand(t, w.Root, nil, "show", "-m", "cache", "what"); r.Err == nil || !strings.Contains(r.Err.Error(), "A ref is one of") {
		t.Errorf("nonsense: %v", r.Err)
	}
}

// ws show of a file says what's been said of it in any mission: a comment
// still open on its lines, a decision about it, a talk that named it, each
// with the mission it's read in. Asked of some lines, it leads with what's
// on them and counts the rest; a talk says what was decided in it.
func TestShow_WhatsBeenSaidOfAFile(t *testing.T) {
	w, _ := testutil.OrchestrateFixture(t)
	capsule := filepath.Join(w.Root, "repos", "backend", "cache-lookup")
	os.WriteFile(filepath.Join(capsule, "README.md"), []byte("hello\nIt caches lookups.\n"), 0o644)
	testutil.GitCmd(t, capsule, "commit", "-q", "-am", "Say what it caches")
	d, err := review.Compare(capsule, "origin/main")
	if err != nil {
		t.Fatal(err)
	}
	at := time.Now()
	reviews := review.For(w.Root, "cache")
	diffed := review.Diffed{Dir: capsule, BaseRef: "origin/main", Base: d.Base, Head: d.Head}
	if _, err := reviews.AddNote("backend/cache-lookup", diffed, review.NewNote{Path: "README.md", Side: "new", Line: 2, Kind: "blocking", Body: "Say what it caches, and for how long."}, at); err != nil {
		t.Fatal(err)
	}
	if _, err := reviews.Submit("backend/cache-lookup", diffed, review.Changes, "", at); err != nil {
		t.Fatal(err)
	}
	talk, err := conversation.ForWorkspace(w.Root).OpenTalk(conversation.You, "What does the backend's README promise?", at)
	if err != nil {
		t.Fatal(err)
	}
	conversation.ForWorkspace(w.Root).Say(talk.ID, conversation.Turn{By: conversation.Agent, At: at, Body: "backend/README.md:1 says hello.",
		Refs: []ref.Pinned{{Said: "backend/README.md:1", Ref: ref.Ref{Kind: ref.File, Repo: "backend", Worktree: ref.Ground, Path: "README.md", Line: 1}}}})
	testutil.Run(t, w, nil, "mission", "new", "Notes", "--brief", "## Goal\nNotes.")
	if _, err := (mission.Store{Root: w.Root}).Decide("notes", mission.Decision{What: "The README stays one line", By: "you", From: talk.ID, About: []string{"backend:README.md"}}); err != nil {
		t.Fatal(err)
	}
	t.Setenv(mission.EnvMission, "")

	show := func(args ...string) string {
		t.Helper()
		return testutil.Run(t, w, nil, append([]string{"show", "-f", "llm"}, args...)...).Stdout
	}
	file := show("backend/README.md")
	for _, want := range []string{
		"Said of it, in any mission",
		"Comments on it (1, 1 still standing):", "r1.c1  a comment, mission cache, open, at 2: Say what it caches, and for how long.",
		"About it (1, 1 still standing):", "d1  a decision, mission notes, current: The README stays one line",
		"Named in (1, 1 still standing):", "w1  a talk, the workspace's, open, at 1: What does the backend's README promise?",
	} {
		if !strings.Contains(file, want) {
			t.Errorf("the file lacks %q:\n%s", want, file)
		}
	}
	lines := show("backend/README.md:2")
	if !strings.Contains(lines, "On lines 2, or the whole of it:") || !strings.Contains(lines, "r1.c1") || !strings.Contains(lines, "d1") ||
		strings.Contains(lines, "w1  ") || !strings.Contains(lines, "… 1 more elsewhere in it: ws show backend:README.md") {
		t.Errorf("lines 2:\n%s", lines)
	}
	if said := show("w1"); !strings.Contains(said, "In it (1, 1 still standing):") || !strings.Contains(said, "d1  a decision, mission notes, current") {
		t.Errorf("the talk:\n%s", said)
	}
	// In a mission, what's said of a thing elsewhere: the capsule's file,
	// from the mission that decided about it.
	if said := show("-m", "cache", "r1.c1"); strings.Contains(said, "Said of it elsewhere") {
		t.Errorf("a comment nothing elsewhere points at:\n%s", said)
	}
}
