package artifact_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/artifact"
	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/testutil"
)

// w9, replayed: a workspace conversation proposes the glow as an artifact
// (ws artifact add --propose), then a revision of it (--to, --on), and
// changes nothing itself: told to add one, it's refused. Each proposal is
// added with one accept, a1.v1 then a1.v2, each recorded as from that
// conversation and its proposal. A revision proposed on v1 before v2 was
// added is still added, as v3 based on v1; one declined leaves nothing.
func TestArtifact_ProposedInATalkAndAccepted(t *testing.T) {
	w := testutil.MissionWorkspace(t)
	talks := conversation.ForWorkspace(w.Root)
	talks.OpenTalk(conversation.You, "Radial horizon gradient on web UI background", time.Now())
	stored := filepath.Join(w.Root, ".ws", "artifacts")
	refused := func(want string, args ...string) {
		t.Helper()
		r := testutil.RunCommand(t, w.Root, nil, args...)
		if r.Err == nil || !strings.Contains(r.Err.Error(), want) {
			t.Errorf("ws %s: %v, want %q", strings.Join(args, " "), r.Err, want)
		}
	}

	testutil.WithStdin(t, glowV1)
	refused("for a conversation's agent", "artifact", "add", "--propose", "--note", "x", "-")
	t.Setenv(conversation.EnvConversation, "w1")
	t.Setenv(mission.EnvCrew, conversation.Agent)

	// How one's written, with nothing proposed.
	guide := testutil.Run(t, w, nil, "artifact", "add", "--propose")
	for _, want := range []string{"One HTML file, starting <!doctype html>, with its CSS and JS inline", "No network", "At most 1 MB", "prefers-color-scheme", "Don't depend on localStorage", "ws.tweaks({", "The first preset is what the page opens as", `data-density="compact"`, "ws artifact add --propose --title", "ws artifact add --propose --to a3 --on v2"} {
		if !strings.Contains(guide.Stdout, want) {
			t.Errorf("the guide lacks %q:\n%s", want, guide.Stdout)
		}
	}
	// Told to add one itself, it's refused: it proposes.
	dir := t.TempDir()
	refused("propose it instead", "artifact", "add", writeHTML(t, dir, "glow.html", glowV1), "--note", "x")
	testutil.WithStdin(t, glowV1)
	refused("give it a note", "artifact", "add", "--propose", "-")
	testutil.WithStdin(t, "# The glow\n")
	refused("isn't HTML", "artifact", "add", "--propose", "--note", "x", "-")
	testutil.WithStdin(t, "<!doctype html><p>no title</p>")
	refused("give it a title", "artifact", "add", "--propose", "--note", "x", "-")
	testutil.WithStdin(t, glowV2)
	refused("no artifact a1", "artifact", "add", "--propose", "--to", "a1", "--note", "x", "-")

	testutil.WithStdin(t, glowV1+`<link rel="stylesheet" href="https://fonts.example.com/inter.css">`)
	r := testutil.Run(t, w, nil, "artifact", "add", "--propose", "--title", "Horizon glow on the web UI background", "--note", "Accent at 8% in dark and 3% in light, fading out by 55% of the height.", "-")
	if !strings.Contains(r.Stderr, "proposed in w1 (proposal 1)") || !strings.Contains(r.Stderr, "loads https://fonts.example.com/inter.css") || !strings.Contains(r.Stderr, "-horizon-glow-on-the-web-ui-background/v1.html (on this machine)") {
		t.Errorf("proposing:\n%s", r.Stderr)
	}
	if entries, _ := os.ReadDir(stored); len(entries) != 0 {
		t.Errorf("proposing wrote %v", entries)
	}

	// The user adds it: a1.v1, from w1.
	t.Setenv(conversation.EnvConversation, "")
	t.Setenv(mission.EnvCrew, "")
	if r := testutil.Run(t, w, nil, "conversation", "accept", "w1", "1"); !strings.Contains(r.Stderr, `Added a1.v1, "Horizon glow on the web UI background"`) {
		t.Errorf("accepting:\n%s", r.Stderr)
	}

	// A revision, read first: ws show a1 tells the agent how to propose one.
	t.Setenv(conversation.EnvConversation, "w1")
	t.Setenv(mission.EnvCrew, conversation.Agent)
	if show := testutil.Run(t, w, nil, "show", "a1").Stdout; !strings.Contains(show, "ws artifact add --propose --to a1 --on v1") {
		t.Errorf("ws show a1, in a conversation:\n%s", show)
	}
	testutil.WithStdin(t, glowV2)
	refused("has no v4", "artifact", "add", "--propose", "--to", "a1", "--on", "v4", "--note", "x", "-")
	testutil.WithStdin(t, glowV2)
	r = testutil.Run(t, w, nil, "artifact", "add", "--propose", "--to", "a1", "--on", "v1", "--note", "No glow in light: at 3% the page looked grey.", "-")
	if !strings.Contains(r.Stderr, "proposed in w1 (proposal 2)") || !strings.Contains(r.Stderr, "added to a1 as v2") {
		t.Errorf("proposing a revision:\n%s", r.Stderr)
	}
	testutil.WithStdin(t, strings.Replace(glowV1, "#6cf3", "#6cf2", 1))
	testutil.Run(t, w, nil, "artifact", "add", "--propose", "--to", "a1", "--note", "Softer: 6% in dark.", "-")
	testutil.WithStdin(t, glowV1)
	testutil.Run(t, w, nil, "artifact", "add", "--propose", "--to", "a1.v1", "--note", "The same again.", "-")
	t.Setenv(conversation.EnvConversation, "")
	t.Setenv(mission.EnvCrew, "")

	c, _ := talks.Load("w1")
	if len(c.Proposals) != 4 {
		t.Fatalf("proposals: %+v", c.Proposals)
	}
	if p := c.Proposals[1]; p.Kind != conversation.ProposeArtifact || p.To != "a1" || p.On != 1 || p.Latest != 1 || p.HTML != glowV2 || p.Size != len(glowV2) || !strings.HasSuffix(p.Path, "-horizon-glow-on-the-web-ui-background/v2.html") {
		t.Errorf("the revision proposed: %+v", p)
	}
	if p := c.Proposals[3]; p.To != "a1" || p.On != 1 {
		t.Errorf("--to a1.v1: %+v", p)
	}
	if show := testutil.Run(t, w, nil, "show", "-f", "llm", "w1").Stdout; !strings.Contains(show, "2. a version of a1, on v1 — No glow in light: at 3% the page looked grey. (waiting for the user)") {
		t.Errorf("ws show w1:\n%s", show)
	}

	// Two clicks: a1.v2 is the revision; the one proposed on v1 before it
	// was added is v3, based on v1; the one declined leaves nothing.
	if r := testutil.Run(t, w, nil, "conversation", "accept", "w1", "2"); !strings.Contains(r.Stderr, "Added a1.v2") {
		t.Errorf("accepting the revision:\n%s", r.Stderr)
	}
	if r := testutil.Run(t, w, nil, "conversation", "accept", "w1", "3"); !strings.Contains(r.Stderr, "Added a1.v3") || !strings.Contains(r.Stderr, "based on v1") {
		t.Errorf("accepting one out of date:\n%s", r.Stderr)
	}
	testutil.Run(t, w, nil, "conversation", "decline", "w1", "4")

	entries, _ := os.ReadDir(stored)
	if len(entries) != 1 {
		t.Fatalf(".ws/artifacts holds %v", entries)
	}
	data, _ := os.ReadFile(filepath.Join(stored, entries[0].Name(), artifact.RecordFile))
	a, err := artifact.Parse(data)
	if err != nil || len(a.Versions) != 3 {
		t.Fatalf("the record: %v\n%s", err, data)
	}
	for i, want := range []artifact.Version{{N: 1, Proposal: 1}, {N: 2, BasedOn: 1, Proposal: 2}, {N: 3, BasedOn: 1, Proposal: 3}} {
		v := a.Versions[i]
		if v.N != want.N || v.BasedOn != want.BasedOn || v.Proposal != want.Proposal || v.By != "you" || v.From != "w1" || v.FromTitle != "Radial horizon gradient on web UI background" {
			t.Errorf("v%d: %+v", i+1, v)
		}
	}
	if got, _ := os.ReadFile(filepath.Join(stored, entries[0].Name(), "v2.html")); string(got) != glowV2 {
		t.Errorf("v2.html: %q", got)
	}
	if _, err := os.Stat(filepath.Join(stored, entries[0].Name(), "v4.html")); err == nil {
		t.Error("the declined proposal was added")
	}

	c, _ = talks.Load("w1")
	for i, want := range []string{"a1.v1", "a1.v2", "a1.v3", ""} {
		if c.Proposals[i].Result != want {
			t.Errorf("proposal %d's result: %q, want %q", i+1, c.Proposals[i].Result, want)
		}
	}
	show := testutil.Run(t, w, nil, "show", "a1").Stdout
	for _, want := range []string{"by you, accepting proposal 1 in conversation w1 (\"Radial horizon gradient on web UI background\")", "v2 · based on v1", "accepting proposal 2 in conversation w1", "v3 · based on v1", "No glow in light: at 3% the page looked grey.", "ws artifact add <file> --to a1"} {
		if !strings.Contains(show, want) {
			t.Errorf("ws show a1 lacks %q:\n%s", want, show)
		}
	}
	show = testutil.Run(t, w, nil, "show", "-f", "llm", "w1").Stdout
	for _, want := range []string{"1. artifact: Horizon glow on the web UI background — Accent at 8%", "(accepted: a1.v1)", "(accepted: a1.v2)", "(declined)", "Came out of it", "a1"} {
		if !strings.Contains(show, want) {
			t.Errorf("ws show w1 lacks %q:\n%s", want, show)
		}
	}
	if r := testutil.RunCommand(t, w.Root, nil, "conversation", "accept", "w1", "1"); r.Err == nil {
		t.Error("accepted it twice")
	}
}

// A mission's conversation proposes one too: added, it says it came from
// that conversation (glow/t1), and it's recorded on the mission, as
// crew's are.
func TestArtifact_ProposedInAMissionsTalk(t *testing.T) {
	w := testutil.MissionWorkspace(t)
	testutil.Run(t, w, nil, "mission", "new", "Glow", "--brief", "## Goal\nTry a glow.")
	talks := conversation.For(w.Root, "glow")
	c, err := talks.OpenTalk(conversation.You, "Show me the glow", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(mission.EnvMission, "glow")
	t.Setenv(conversation.EnvConversation, c.ID)
	t.Setenv(mission.EnvCrew, conversation.Agent)
	testutil.WithStdin(t, glowV1)
	if r := testutil.Run(t, w, nil, "artifact", "add", "--propose", "--note", "A first glow.", "-"); !strings.Contains(r.Stderr, "proposed in "+c.ID+" (proposal 1)") {
		t.Errorf("proposing:\n%s", r.Stderr)
	}
	t.Setenv(mission.EnvMission, "")
	t.Setenv(conversation.EnvConversation, "")
	t.Setenv(mission.EnvCrew, "")
	if r := testutil.Run(t, w, nil, "conversation", "accept", "-m", "glow", c.ID, "1"); !strings.Contains(r.Stderr, `Added a1.v1, "Horizon glow"`) || !strings.Contains(r.Stderr, "recorded it on glow") {
		t.Errorf("accepting:\n%s", r.Stderr)
	}
	show := testutil.Run(t, w, nil, "show", "a1").Stdout
	if !strings.Contains(show, "by you, accepting proposal 1 in conversation "+c.ID+" on mission glow") {
		t.Errorf("ws show a1:\n%s", show)
	}
	if show := testutil.Run(t, w, nil, "show", "-m", "glow", "-f", "llm").Stdout; !strings.Contains(show, "a1") {
		t.Errorf("the mission doesn't list a1:\n%s", show)
	}
}
