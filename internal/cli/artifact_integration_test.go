package cli_test

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

const glowV1 = `<!doctype html>
<title>Horizon glow</title>
<style>body { background: radial-gradient(ellipse at top, #6cf3 0, transparent 55%) }</style>
`

const glowV2 = `<!doctype html>
<title>Horizon glow</title>
<style>@media (prefers-color-scheme: dark) { body { background: radial-gradient(ellipse at top, #6cf3 0, transparent 55%) } }</style>
`

func writeHTML(t *testing.T, dir, name, text string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// Crew on a mission add an artifact and a second version of it: each
// version says what changed, who made it and where, the artifact is
// recorded on the mission, and ws show a1 says all of it to whoever reads
// it next, with nothing else to go on.
func TestArtifact_CrewAddTwoVersionsAndShowSaysWhatEachIs(t *testing.T) {
	w := missionWorkspace(t)
	run(t, w, nil, "mission", "new", "Glow", "--brief", "## Goal\nTry a glow.\n\n## Stages\n1. Sketch it.\n2. Build it.")
	t.Setenv(mission.EnvMission, "glow")
	t.Setenv(mission.EnvCrew, "c1")
	dir := t.TempDir()

	r := run(t, w, nil, "artifact", "add", writeHTML(t, dir, "glow.html", glowV1), "--note", "Accent at 20% in dark and light, fading out by 55% of the height.")
	if strings.TrimSpace(r.Stdout) != "a1.v1" || !strings.Contains(r.Stderr, "Added a1.v1, \"Horizon glow\"") || !strings.Contains(r.Stderr, "recorded it on glow") {
		t.Errorf("add said:\n%s%s", r.Stdout, r.Stderr)
	}
	r = run(t, w, nil, "artifact", "add", writeHTML(t, dir, "glow2.html", glowV2), "--to", "a1", "--note", "No glow in light: it looked grey there.")
	if strings.TrimSpace(r.Stdout) != "a1.v2" {
		t.Errorf("the second version: %s%s", r.Stdout, r.Stderr)
	}

	// On disk: the folder, its record, and each version whole.
	folder := filepath.Join(w.Root, ".ws", "artifacts")
	entries, _ := os.ReadDir(folder)
	if len(entries) != 1 || !strings.HasSuffix(entries[0].Name(), "-horizon-glow") {
		t.Fatalf(".ws/artifacts holds %v", entries)
	}
	folder = filepath.Join(folder, entries[0].Name())
	for name, want := range map[string]string{"v1.html": glowV1, "v2.html": glowV2} {
		if got, _ := os.ReadFile(filepath.Join(folder, name)); string(got) != want {
			t.Errorf("%s is %q", name, got)
		}
	}
	record, _ := os.ReadFile(filepath.Join(folder, "artifact.toml"))
	for _, want := range []string{`title = "Horizon glow"`, `status = "live"`, "based_on = 1", `by = "c1"`, `from = "glow"`, `from_title = "Glow"`, `note = "No glow in light: it looked grey there."`} {
		if !strings.Contains(string(record), want) {
			t.Errorf("artifact.toml doesn't say %s:\n%s", want, record)
		}
	}

	// Recorded on the mission, once, by the crew, for its stage.
	m := loadMission(t, w, "glow")
	if len(m.Artifacts) != 1 || m.Artifacts[0].Ref != "a1" || m.Artifacts[0].Kind != "artifact" || m.Artifacts[0].Crew != "c1" || m.Artifacts[0].Label != "Horizon glow" {
		t.Errorf("the mission's artifacts: %+v", m.Artifacts)
	}
	if show := run(t, w, nil, "show", "-f", "llm").Stdout; !strings.Contains(show, "[artifact, live] a1 — Horizon glow — ws show a1") {
		t.Errorf("the mission's view doesn't list it:\n%s", show)
	}

	// ws show a1, from anywhere: what each version changed, and where it
	// came from.
	t.Setenv(mission.EnvMission, "")
	t.Setenv(mission.EnvCrew, "")
	show := run(t, w, nil, "show", "a1", "-f", "llm").Stdout
	for _, want := range []string{
		`Artifact a1 — "Horizon glow" (live, on this machine; 2 versions, the latest v2)`,
		filepath.Join(".ws", "artifacts", entries[0].Name()),
		"by c1, crew on mission glow (\"Glow\")",
		"Accent at 20% in dark and light, fading out by 55% of the height.",
		"v2 · based on v1 ·",
		"No glow in light: it looked grey there.",
		"ws:glow",
		"ws artifact show a1.v2   prints v2's HTML",
	} {
		if !strings.Contains(show, want) {
			t.Errorf("ws show a1 doesn't say %q:\n%s", want, show)
		}
	}
	if v1 := run(t, w, nil, "show", "a1.v1").Stdout; !strings.Contains(v1, "v1 of 2") || !strings.Contains(v1, "← asked for") {
		t.Errorf("ws show a1.v1:\n%s", v1)
	}
	if html := run(t, w, nil, "artifact", "show", "a1.v1").Stdout; html != glowV1 {
		t.Errorf("ws artifact show a1.v1 printed %q", html)
	}
	if html := run(t, w, nil, "artifact", "show", "a1").Stdout; html != glowV2 {
		t.Errorf("ws artifact show a1 printed %q, not the latest", html)
	}
	if list := run(t, w, nil, "artifact", "list").Stdout; !strings.Contains(list, "a1   v2   live      Horizon glow · by c1, crew on mission glow") {
		t.Errorf("ws artifact list:\n%s", list)
	}
	if found := run(t, w, nil, "find", "grey", "-f", "llm").Stdout; !strings.Contains(found, "ws show a1") {
		t.Errorf("ws find doesn't find its note:\n%s", found)
	}
}

// What each may do: crew give a note; the orchestrator and a
// conversation's agent add nothing; a version isn't based on one there
// isn't; only you discard, and a discarded artifact is still shown, and
// takes no more versions.
func TestArtifact_WhoMayDoWhat(t *testing.T) {
	w := missionWorkspace(t)
	run(t, w, nil, "mission", "new", "Glow", "--brief", "## Goal\nTry a glow.")
	dir := t.TempDir()
	file := writeHTML(t, dir, "glow.html", glowV1)

	refused := func(want string, args ...string) {
		t.Helper()
		r := testutil.RunCommand(t, w.Root, nil, args...)
		if r.Err == nil || !strings.Contains(r.Err.Error(), want) {
			t.Errorf("ws %s: %v, want %q", strings.Join(args, " "), r.Err, want)
		}
	}
	t.Setenv(mission.EnvMission, "glow")
	t.Setenv(mission.EnvCrew, "c1")
	refused("give it a note", "artifact", "add", file)
	refused("isn't HTML", "artifact", "add", writeHTML(t, dir, "notes.md", "# Notes\n"), "--note", "x")
	t.Setenv(mission.EnvCrew, "")
	t.Setenv(mission.EnvRole, mission.RoleOrchestrator)
	refused("the orchestrator doesn't make artifacts", "artifact", "add", file, "--note", "x")
	t.Setenv(mission.EnvRole, "")
	t.Setenv(mission.EnvMission, "")
	t.Setenv(conversation.EnvConversation, "w4")
	refused("you're in a conversation", "artifact", "add", file)
	t.Setenv(conversation.EnvConversation, "")

	// By hand: yours, with no note needed, its title from its <title>.
	r := run(t, w, nil, "artifact", "add", writeHTML(t, dir, "remote.html", "<!doctype html><title>Remote</title><script src=\"https://cdn.example.com/x.js\"></script>"))
	if !strings.Contains(r.Stderr, "Added a1.v1, \"Remote\"") || !strings.Contains(r.Stderr, "loads https://cdn.example.com/x.js") {
		t.Errorf("by hand:\n%s", r.Stderr)
	}
	if show := run(t, w, nil, "show", "a1").Stdout; !strings.Contains(show, "by you, by hand") {
		t.Errorf("by hand:\n%s", show)
	}
	refused("has no v5", "artifact", "add", file, "--to", "a1", "--on", "v5")
	refused("no artifact a9", "artifact", "add", file, "--to", "a9")

	t.Setenv(mission.EnvMission, "glow")
	t.Setenv(mission.EnvCrew, "c1")
	refused("only the user discards", "artifact", "discard", "a1")
	t.Setenv(mission.EnvMission, "")
	t.Setenv(mission.EnvCrew, "")
	run(t, w, nil, "artifact", "discard", "a1", "it", "loads", "from", "a", "CDN")
	if list := run(t, w, nil, "artifact", "list"); strings.Contains(list.Stdout, "a1") || !strings.Contains(list.Stderr, "1 discarded") {
		t.Errorf("listed after it was discarded: %s%s", list.Stdout, list.Stderr)
	}
	if show := run(t, w, nil, "show", "a1").Stdout; !strings.Contains(show, "(discarded, on this machine") || !strings.Contains(show, "by you: it loads from a CDN") {
		t.Errorf("a discarded artifact's view:\n%s", show)
	}
	refused("was discarded", "artifact", "add", file, "--to", "a1")
}

// ws doctor finds what's in .ws/artifacts that belongs to no artifact.
func TestArtifact_DoctorFindsLitter(t *testing.T) {
	w := missionWorkspace(t)
	dir := t.TempDir()
	run(t, w, nil, "artifact", "add", writeHTML(t, dir, "glow.html", glowV1))
	stray := filepath.Join(w.Root, ".ws", "artifacts", "2026-01-01-half-written")
	os.MkdirAll(stray, 0o755)
	os.Chtimes(stray, mustOld(), mustOld())
	r := testutil.RunCommand(t, w.Root, nil, "doctor")
	if !strings.Contains(r.Stdout+r.Stderr, "1 artifacts'") {
		t.Errorf("doctor:\n%s%s", r.Stdout, r.Stderr)
	}
}

func mustOld() time.Time { return time.Now().Add(-2 * time.Minute) }
