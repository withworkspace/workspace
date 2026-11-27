package doc_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/doc"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/testutil"
)

// amendsWorkspace is kindsWorkspace with an amend message for the ADR
// kind, and an ADR on records' main (and its ground) to change.
func amendsWorkspace(t *testing.T, adr string) *testutil.Workspace {
	t.Helper()
	w := kindsWorkspace(t)
	toml := filepath.Join(w.Root, "ws.toml")
	data, _ := os.ReadFile(toml)
	os.WriteFile(toml, []byte(strings.Replace(string(data), "add = \"docs: add adr {number} {title}\"\n", "add = \"docs: add adr {number} {title}\"\namend = \"docs: amend adr {number}\"\n", 1)), 0o644)
	testutil.CommitIn(t, w.Sources["records"], "adr/0007-one-binary.md", adr, "docs: say why adr 0007")
	bringGroundUp(t, w, "records")
	return w
}

func bringGroundUp(t *testing.T, w *testutil.Workspace, repo string) {
	t.Helper()
	ground := filepath.Join(w.Root, "repos", repo, ".ground")
	testutil.GitCmd(t, ground, "fetch", "origin")
	testutil.GitCmd(t, ground, "merge", "--ff-only", "origin/main")
}

// A conversation's agent asked to change a document it proposed, once it's
// added, proposes the edit (ws doc amend --propose) and changes nothing
// itself. It's shown the document as it is first, and proposes with the
// version it was shown. Accepting saves it where it is, in a commit with
// its kind's amend message, landed in a repo with no pull requests; the
// agent's told where.
func TestDocAmend_ProposedInATalkAndAccepted(t *testing.T) {
	const ref = "records:adr/0007-one-binary.md"
	was := "# One binary\n\nIt ships as one file.\n"
	w := amendsWorkspace(t, was)
	src := w.Sources["records"]
	before := testutil.Subjects(t, src, "main")
	talks := conversation.ForWorkspace(w.Root)
	talks.OpenTalk(conversation.You, "It's missing why: amend it", time.Now())
	version := doc.Version([]byte(was))

	testutil.WithStdin(t, "# One binary\n\nIt ships as one file, so installing it is copying it.\n")
	if r := testutil.RunCommand(t, w.Root, nil, "doc", "amend", "--propose", ref, "--on", version, "-"); r.Err == nil || !strings.Contains(r.Err.Error(), "for a conversation's agent") {
		t.Errorf("proposing outside a conversation: %v", r.Err)
	}
	t.Setenv(conversation.EnvConversation, "w1")
	t.Setenv(mission.EnvCrew, conversation.Agent)
	testutil.WithStdin(t, "# One binary\n\nChanged.\n")
	if r := testutil.RunCommand(t, w.Root, nil, "doc", "amend", ref, "-"); r.Err == nil || !strings.Contains(r.Err.Error(), "propose") {
		t.Errorf("amending it in a conversation: %v", r.Err)
	}
	if r := testutil.RunCommand(t, w.Root, nil, "doc", "amend", "--propose", "records:docs/plan.md"); r.Err == nil || !strings.Contains(r.Err.Error(), "is not one of the workspace's document kinds") {
		t.Errorf("a file of no kind: %v", r.Err)
	}

	// Shown as it is, with the version to propose with.
	r := testutil.Run(t, w, nil, "doc", "amend", "--propose", ref)
	for _, want := range []string{"of the kind ADR (adr), as it is now (version " + version + ")", "It ships as one file.", "ws doc amend --propose " + ref + " --on " + version + " - <<'EOF'", `committed as "docs: amend adr 0007"`} {
		if !strings.Contains(r.Stdout, want) {
			t.Errorf("shown lacks %q:\n%s", want, r.Stdout)
		}
	}
	testutil.WithStdin(t, "# One binary\n\nChanged.\n")
	if r := testutil.RunCommand(t, w.Root, nil, "doc", "amend", "--propose", ref, "-"); r.Err == nil || !strings.Contains(r.Err.Error(), "--on "+version) {
		t.Errorf("proposing without --on: %v", r.Err)
	}
	testutil.WithStdin(t, "# One binary\n\nChanged.\n")
	if r := testutil.RunCommand(t, w.Root, nil, "doc", "amend", "--propose", ref, "--on", "000000000000", "-"); r.Err == nil || !strings.Contains(r.Err.Error(), "has changed since the version you give") {
		t.Errorf("proposing on an old version: %v", r.Err)
	}
	testutil.WithStdin(t, was)
	if r := testutil.RunCommand(t, w.Root, nil, "doc", "amend", "--propose", ref, "--on", version, "-"); r.Err == nil || !strings.Contains(r.Err.Error(), "the document as it is") {
		t.Errorf("proposing it unchanged: %v", r.Err)
	}
	testutil.WithStdin(t, "# One binary\n\nIt ships as one file, so installing it is copying it.\n")
	r = testutil.Run(t, w, nil, "doc", "amend", "--propose", ref, "--on", version, "-")
	if !strings.Contains(r.Stderr, "proposed in w1 (proposal 1)") || !strings.Contains(r.Stderr, `it's landed on main, committed as "docs: amend adr 0007"`) {
		t.Errorf("proposing: %s", r.Stderr)
	}
	t.Setenv(conversation.EnvConversation, "")
	t.Setenv(mission.EnvCrew, "")

	c, _ := talks.Load("w1")
	if len(c.Proposals) != 1 {
		t.Fatalf("proposals = %+v", c.Proposals)
	}
	if p := c.Proposals[0]; p.Kind != conversation.ProposeDocument || p.To != ref || p.Path != ref || p.DocKind != "adr" || p.Title != "One binary" || p.Was != was ||
		p.Text != "# One binary\n\nIt ships as one file, so installing it is copying it." || p.Subject != "docs: amend adr 0007" {
		t.Errorf("the proposal: %+v", p)
	}
	if got := testutil.Subjects(t, src, "main"); got != before {
		t.Errorf("proposing pushed: %s", got)
	}
	if show := testutil.Run(t, w, nil, "show", "-f", "llm", "w1").Stdout; !strings.Contains(show, "1. an edit to the document "+ref+" (adr): One binary (waiting for the user)") {
		t.Errorf("ws show w1:\n%s", show)
	}

	r = testutil.Run(t, w, nil, "conversation", "accept", "w1", "1")
	if !strings.Contains(r.Stderr, "Amended "+ref+": landed on main") {
		t.Errorf("accepting: %s", r.Stderr)
	}
	if got := testutil.Subjects(t, src, "main"); got != "docs: amend adr 0007 | "+before {
		t.Errorf("the remote's main has: %s", got)
	}
	if got := testutil.GitOut(t, src, "show", "main:adr/0007-one-binary.md"); got != "# One binary\n\nIt ships as one file, so installing it is copying it.\n" {
		t.Errorf("what was saved: %q", got)
	}
	if body := testutil.GitOut(t, src, "log", "-1", "--format=%b", "main"); !strings.Contains(body, "Proposed in conversation w1") {
		t.Errorf("the commit's body: %q", body)
	}
	if entries, _ := os.ReadDir(filepath.Join(w.Root, "repos", "records")); len(entries) != 2 {
		t.Errorf("a capsule was left: %v", entries)
	}
	c, _ = talks.Load("w1")
	if p := c.Proposals[0]; p.State != conversation.ProposalAccepted || p.Result != ref {
		t.Errorf("accepted: %+v", p)
	}
	if a := c.Turns[len(c.Turns)-1]; a.Origin != "proposal:1" || !a.Quiet || a.Body != "Accepted proposal 1, the edit to the document “One binary” ("+ref+"): saved to "+ref+"." {
		t.Errorf("the accept's turn: %+v", a)
	}
}

// What changed in a document between an edit's being proposed and its
// being accepted stands: the edit's merged into it, from the text it was
// proposed on. Where the two change the same lines, nothing's saved, and
// the proposal waits, to be declined and asked for again.
func TestDocAmend_MergedWithWhatChangedSince(t *testing.T) {
	const ref = "records:adr/0007-one-binary.md"
	was := "# One binary\n\nIt ships as one file.\n\n## Why\n\nFewer parts.\n"
	w := amendsWorkspace(t, was)
	src := w.Sources["records"]
	talks := conversation.ForWorkspace(w.Root)
	talks.OpenTalk(conversation.You, "Add what's happened since", time.Now())
	proposeAmend := func(on, text string) {
		t.Helper()
		t.Setenv(conversation.EnvConversation, "w1")
		t.Setenv(mission.EnvCrew, conversation.Agent)
		testutil.WithStdin(t, text)
		testutil.Run(t, w, nil, "doc", "amend", "--propose", ref, "--on", doc.Version([]byte(on)), "-")
		t.Setenv(conversation.EnvConversation, "")
		t.Setenv(mission.EnvCrew, "")
	}

	proposeAmend(was, was+"\n## Since\n\nNo regrets.\n")
	testutil.CommitIn(t, src, "adr/0007-one-binary.md", strings.Replace(was, "one file", "one static file", 1), "docs: be exact")
	r := testutil.Run(t, w, nil, "conversation", "accept", "w1", "1")
	if !strings.Contains(r.Stderr, "Amended "+ref+": landed on main") || !strings.Contains(r.Stderr, "merged with changes on main meanwhile") {
		t.Errorf("accepting: %s", r.Stderr)
	}
	merged := "# One binary\n\nIt ships as one static file.\n\n## Why\n\nFewer parts.\n\n## Since\n\nNo regrets.\n"
	if got := testutil.GitOut(t, src, "show", "main:adr/0007-one-binary.md"); got != merged {
		t.Errorf("what was saved: %q", got)
	}

	bringGroundUp(t, w, "records")
	proposeAmend(merged, strings.Replace(merged, "Fewer parts.", "Fewer moving parts.", 1))
	testutil.CommitIn(t, src, "adr/0007-one-binary.md", strings.Replace(merged, "Fewer parts.", "Less to go wrong.", 1), "docs: say it better")
	before := testutil.Subjects(t, src, "main")
	r = testutil.RunCommand(t, w.Root, nil, "conversation", "accept", "w1", "2")
	if r.Err == nil || !strings.Contains(r.Err.Error(), "changed since this was proposed, in the same lines (1 place)") || !strings.Contains(r.Err.Error(), "Ask in w1 again") {
		t.Errorf("accepting a clash: %v", r.Err)
	}
	if got := testutil.Subjects(t, src, "main"); got != before {
		t.Errorf("a clash pushed: %s", got)
	}
	if entries, _ := os.ReadDir(filepath.Join(w.Root, "repos", "records")); len(entries) != 2 {
		t.Errorf("a capsule was left: %v", entries)
	}
	if c, _ := talks.Load("w1"); c.Proposals[1].State != conversation.ProposalOpen {
		t.Errorf("the clash: %+v", c.Proposals[1])
	}
}

// You change one yourself with ws doc amend: in a repo that takes pull
// requests, it's left in a capsule lifted for it. Crew are refused, and
// told to change it in a capsule.
func TestDocAmend_YoursAndNotCrews(t *testing.T) {
	w := kindsWorkspace(t)
	const ref = "api:notes/2026-11-10-caching.md"
	testutil.CommitIn(t, w.Sources["api"], "notes/2026-11-10-caching.md", "# Caching\n\nPer user.\n", "add a note")
	bringGroundUp(t, w, "api")
	file := filepath.Join(t.TempDir(), "caching.md")
	os.WriteFile(file, []byte("# Caching\n\nPer user, for an hour.\n"), 0o644)

	t.Setenv(mission.EnvCrew, "c1")
	if r := testutil.RunCommand(t, w.Root, nil, "doc", "amend", ref, file); r.Err == nil || !strings.Contains(r.Err.Error(), "only the user changes a document this way, not c1") {
		t.Errorf("crew amending: %v", r.Err)
	}
	t.Setenv(mission.EnvCrew, "")

	r := testutil.Run(t, w, nil, "doc", "amend", ref, file)
	if !strings.Contains(r.Stderr, "Saved in a new capsule, api/edit-2026-11-10-caching") {
		t.Errorf("amending: %s", r.Stderr)
	}
	dir := filepath.Join(w.Root, "repos", "api", "edit-2026-11-10-caching")
	if got, _ := os.ReadFile(filepath.Join(dir, "notes", "2026-11-10-caching.md")); string(got) != "# Caching\n\nPer user, for an hour.\n" {
		t.Errorf("the capsule has: %q", got)
	}
	if got := testutil.Subjects(t, dir, "-1"); got != "Edit notes/2026-11-10-caching.md" {
		t.Errorf("committed as: %s", got)
	}
}
