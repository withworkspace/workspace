package workspace_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/cli"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/testutil"
	"github.com/withworkspace/workspace/internal/web"
)

// Review is only of a mission's own capsules, compared with origin's
// default branch; the workspace the web UI serves says each capsule's
// commit, so an open review notices new ones.
func TestWebCapsuleAndWorkspace(t *testing.T) {
	t.Setenv(mission.EnvMission, "")
	t.Setenv(cli.ShellPWD, "")
	w := testutil.SetupWorkspace(t, testutil.WorkspaceOpts{Org: "test-org", DefaultBranch: "main", Repos: []testutil.RepoOpts{{Name: "frontend"}, {Name: "backend"}}})
	runOK := func(args ...string) {
		if r := testutil.RunCommand(t, w.Root, nil, args...); r.Err != nil {
			t.Fatalf("ws %v: %v\n%s", args, r.Err, r.Stderr)
		}
	}
	runOK("mission", "new", "Cache", "--brief", "## Goal\nCache it.")
	t.Setenv(mission.EnvMission, "cache")
	runOK("lift", "backend", "cache-lookup")
	t.Setenv(mission.EnvMission, "")
	find := testutil.WebCapsule(w.Root)
	dir, base, err := find("cache", "backend", "cache-lookup")
	if err != nil || dir != filepath.Join(w.Root, "repos", "backend", "cache-lookup") || base != "origin/main" {
		t.Fatalf("capsule = %q %q %v", dir, base, err)
	}
	for _, c := range [][3]string{{"cache", "backend", "other"}, {"nope", "backend", "cache-lookup"}, {"cache", "frontend", "cache-lookup"}} {
		if _, _, err := find(c[0], c[1], c[2]); !errors.Is(err, web.ErrNotFound) {
			t.Errorf("%v: %v", c, err)
		}
	}

	ws, err := testutil.WebWorkspace(w.Root)()
	if err != nil || len(ws.Missions) != 1 || len(ws.Missions[0].Capsules) != 1 {
		t.Fatalf("workspace = %+v, %v", ws, err)
	}
	if c := ws.Missions[0].Capsules[0]; !c.Exists || len(c.Head) != 40 || c.Branch != "cache-lookup" {
		t.Errorf("capsule = %+v", c)
	}
	// Where it stands, as the deck says: briefed, nobody has started it.
	if h := ws.Missions[0].Headline; h.Court != "you" || h.Kind != "ready" || h.Group != "up-next" || h.Action.Verb != "launch" {
		t.Errorf("headline = %+v", h)
	}

	os.RemoveAll(dir)
	if _, _, err := find("cache", "backend", "cache-lookup"); err == nil || errors.Is(err, web.ErrNotFound) {
		t.Errorf("a missing capsule folder: %v", err)
	}
}

// The mission page has what the deck's mission view has; artifacts are
// readable by their reference, and only the mission's own.
func TestWebMissionAndArtifact(t *testing.T) {
	t.Setenv(mission.EnvMission, "")
	t.Setenv(cli.ShellPWD, "")
	w := testutil.SetupWorkspace(t, testutil.WorkspaceOpts{Org: "test-org", DefaultBranch: "main", Repos: []testutil.RepoOpts{{Name: "backend"}}})
	runOK := func(args ...string) {
		if r := testutil.RunCommand(t, w.Root, nil, args...); r.Err != nil {
			t.Fatalf("ws %v: %v\n%s", args, r.Err, r.Stderr)
		}
	}
	runOK("mission", "new", "Cache", "--brief", "## Goal\nCache it.\n\n## Stages\n1. Propose it in `backend/cache-lookup`. Check in with me.\n2. Build it.\n")
	t.Setenv(mission.EnvMission, "cache")
	runOK("lift", "backend", "cache-lookup")
	t.Setenv(mission.EnvMission, "")
	store := mission.Store{Root: w.Root}
	os.MkdirAll(store.FilesDir("cache"), 0o755)
	os.WriteFile(filepath.Join(store.FilesDir("cache"), "notes.md"), []byte("# Notes\n\n```mermaid\ngraph TD; A-->B\n```\n"), 0o644)
	store.AddArtifact("cache", mission.Artifact{Ref: "mission:notes.md", Kind: "note", Label: "Spike notes"})
	store.AddArtifact("cache", mission.Artifact{Ref: "https://example.com/design", Kind: "link"})
	store.Ask("cache", "Ship it?", "orchestrator", "Yes", "No")

	d, err := testutil.WebMission(w.Root)("cache")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(d.Brief, "Cache it.") || d.Flag == nil || d.Flag.Options[1] != "No" || len(d.Tasks) != 1 || d.Tasks[0].Verb != "answer" || d.GitHub {
		t.Errorf("detail = %+v", d)
	}
	// The flag is the mission's headline, with its answers; the list says
	// the same.
	if len(d.Headlines) == 0 || d.Headlines[0].Kind != "flag" || d.Headlines[0].Group != "needs-you" ||
		d.Headlines[0].Text != "The orchestrator asks: Ship it?" || len(d.Headlines[0].Action.Options) != 2 {
		t.Errorf("headlines = %+v", d.Headlines)
	}
	if ws, _ := testutil.WebWorkspace(w.Root)(); ws.Missions[0].Headline.Text != d.Headlines[0].Text {
		t.Errorf("the list's headline %+v isn't the page's", ws.Missions[0].Headline)
	}
	if len(d.Stages) != 2 || d.Stages[0].State != "current" || !d.Stages[0].CheckIn || d.Stages[1].State != "todo" {
		t.Errorf("stages = %+v", d.Stages)
	}
	if len(d.Capsules) != 1 || !d.Capsules[0].Exists || len(d.Capsules[0].Head) != 40 {
		t.Errorf("capsules = %+v", d.Capsules)
	}
	// A capsule never pushed has no upstream: it's ahead by what it has that
	// main hasn't (what a pull request would open with).
	capsule := filepath.Join(w.Root, "repos", "backend", "cache-lookup")
	os.WriteFile(filepath.Join(capsule, "cache.go"), []byte("package cache\n"), 0o644)
	for _, args := range [][]string{{"add", "cache.go"}, {"-c", "commit.gpgsign=false", "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-qm", "cache"}} {
		if out, err := exec.Command("git", append([]string{"-C", capsule}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	if d, _ := testutil.WebMissionRead(w.Root)("cache"); len(d.Capsules) != 1 || d.Capsules[0].Ahead != 1 {
		t.Errorf("a capsule never pushed, one commit on: %+v", d.Capsules)
	}
	arts := map[string]web.Artifact{}
	for _, a := range d.Artifacts {
		arts[a.Ref] = a
	}
	if a := arts["mission:notes.md"]; !a.Readable || a.State != "private" || a.Label != "Spike notes" {
		t.Errorf("notes = %+v", a)
	}
	if a := arts["https://example.com/design"]; a.Readable || a.State != "link" || a.URL != "https://example.com/design" {
		t.Errorf("link = %+v", a)
	}
	words := []string{}
	for _, e := range d.Events {
		words = append(words, e.Text)
	}
	if all := strings.Join(words, " | "); !strings.Contains(all, "capsule backend/cache-lookup added") {
		t.Errorf("events = %s", all)
	}
	if _, err := testutil.WebMission(w.Root)("nope"); !errors.Is(err, web.ErrNotFound) {
		t.Errorf("no mission: %v", err)
	}

	read := testutil.WebArtifact(w.Root)
	a, err := read("cache", "mission:notes.md")
	if err != nil || !a.Markdown || !strings.Contains(a.Text, "```mermaid") {
		t.Fatalf("read = %+v, %v", a, err)
	}
	if _, err := read("cache", "https://example.com/design"); err == nil || errors.Is(err, web.ErrNotFound) {
		t.Errorf("a link isn't a file: %v", err)
	}
	// An image or a video crew recorded as proof is shown as it is, not
	// read; anything else that isn't text is refused.
	os.WriteFile(filepath.Join(store.FilesDir("cache"), "proof.png"), []byte("\x89PNG\r\n\x1a\n\x00\x00"), 0o644)
	os.WriteFile(filepath.Join(store.FilesDir("cache"), "run.webm"), []byte("\x1aE\xdf\xa3\x00"), 0o644)
	os.WriteFile(filepath.Join(store.FilesDir("cache"), "blob.bin"), []byte("\x00\x01"), 0o644)
	store.AddArtifact("cache", mission.Artifact{Ref: "mission:proof.png", Kind: "proof", Label: "The page, dark"})
	store.AddArtifact("cache", mission.Artifact{Ref: "mission:run.webm", Kind: "proof", Label: "A run"})
	store.AddArtifact("cache", mission.Artifact{Ref: "mission:blob.bin", Kind: "note"})
	if a, err := read("cache", "mission:proof.png"); err != nil || a.Media != "image/png" || a.Text != "" || a.Markdown || a.Editable {
		t.Errorf("an image: %+v, %v", a, err)
	}
	if a, err := read("cache", "mission:run.webm"); err != nil || a.Media != "video/webm" {
		t.Errorf("a video: %+v, %v", a, err)
	}
	if _, err := read("cache", "mission:blob.bin"); err == nil || !strings.Contains(err.Error(), "isn't text, or an image or a video") {
		t.Errorf("a binary that isn't an image or a video: %v", err)
	}
	// Only references the mission recorded: not any path.
	for _, ref := range []string{"mission:../../../ws.toml", "mission:other.md", "backend:README.md"} {
		if _, err := read("cache", ref); !errors.Is(err, web.ErrNotFound) {
			t.Errorf("%s: %v", ref, err)
		}
	}

	// A change to the mission's log shows in the workspace, so open pages
	// hear of it.
	before, _ := testutil.WebWorkspace(w.Root)()
	time.Sleep(20 * time.Millisecond)
	store.AppendLog("cache", mission.Event{Kind: mission.EventNote, Text: "hi"})
	after, _ := testutil.WebWorkspace(w.Root)()
	if !after.Missions[0].Updated.After(before.Missions[0].Updated) {
		t.Errorf("updated %v → %v", before.Missions[0].Updated, after.Missions[0].Updated)
	}
}

// From the web UI you answer or dismiss the mission's question, accept a
// stage, archive and unpark; talking to crew is their crew page's.
func TestWebAct(t *testing.T) {
	t.Setenv(mission.EnvMission, "")
	t.Setenv(cli.ShellPWD, "")
	runs, restore := cli.CaptureOrchestratorRuns()
	defer restore()
	w := testutil.SetupWorkspace(t, testutil.WorkspaceOpts{Org: "test-org", DefaultBranch: "main", Repos: []testutil.RepoOpts{{Name: "backend"}}})
	if r := testutil.RunCommand(t, w.Root, nil, "mission", "new", "Cache", "--brief", "## Goal\nCache it.\n\n## Stages\n1. Propose it.\n2. Build it.\n"); r.Err != nil {
		t.Fatal(r.Err)
	}
	store := mission.Store{Root: w.Root}
	act := testutil.WebAct(w.Root)
	say := func(verb, text string) (string, error) { return act("cache", web.ActRequest{Verb: verb, Text: text}) }

	store.Ask("cache", "Ship it?", "orchestrator", "Yes", "No")
	if said, err := say("answer", "Yes"); err != nil || !strings.Contains(said, "Answered") {
		t.Errorf("answer: %q, %v", said, err)
	}
	if m, _ := store.Load("cache"); m.Asking() != nil || m.Answer == nil || m.Answer.Text != "Yes" || len(*runs) != 1 {
		t.Errorf("after answering: flag %+v, answer %+v, runs %v", m.Asking(), m.Answer, *runs)
	}
	if _, err := say("answer", "again"); err == nil {
		t.Error("answered a question nobody asked")
	}
	if _, err := say("dismiss", ""); err == nil {
		t.Error("dismissed nothing")
	}
	store.QueueDispatch("cache", &mission.Dispatch{Text: "build it"})
	if _, err := say("dismiss", ""); err != nil {
		t.Error(err)
	}
	if m, _ := store.Load("cache"); m.Dispatch != nil {
		t.Error("queued crew still queued")
	}

	if said, err := say("accept", ""); err != nil || !strings.HasPrefix(said, "Accepted stage 1: stage 2 names no capsule") {
		t.Errorf("accept: %q, %v", said, err)
	}
	if _, err := say("reply", "hello"); err == nil || !strings.Contains(err.Error(), "isn't something a mission's page does") {
		t.Errorf("talking to crew through act: %v", err)
	}
	if _, err := say("unpark", ""); err == nil {
		t.Error("unparked a mission that isn't parked")
	}
	store.Update("cache", func(m *mission.Mission) error { m.State = mission.Parked; return nil })
	if _, err := say("unpark", ""); err != nil {
		t.Error(err)
	}
	if _, err := say("archive", ""); err != nil {
		t.Error(err)
	}
	if m, _ := store.Load("cache"); m.State != mission.Archived {
		t.Errorf("state = %q", m.State)
	}
	if _, err := act("nope", web.ActRequest{Verb: "archive"}); !errors.Is(err, web.ErrNotFound) {
		t.Errorf("no mission: %v", err)
	}
}
