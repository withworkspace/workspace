package cli_test

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

// A conversation's agent asked for a document proposes one, of one of the
// workspace's kinds (ws doc add --propose), and changes nothing: it can't
// add one itself. Accepting it adds it where its kind says, named from its
// first heading, in a commit with its kind's message, landed in a repo
// with no pull requests; the conversation says what it made. Declining
// another changes nothing.
func TestDocAdd_ProposedInATalkAndAccepted(t *testing.T) {
	w := kindsWorkspace(t)
	src := w.Sources["records"]
	before := subjects(t, src, "main")
	talks := conversation.ForWorkspace(w.Root)
	talks.OpenTalk(conversation.You, "Turn this into an ADR", time.Now())

	withStdin(t, "# How loud\n")
	if r := testutil.RunCommand(t, w.Root, nil, "doc", "add", "--propose", "adr", "-"); r.Err == nil || !strings.Contains(r.Err.Error(), "for a conversation's agent") {
		t.Errorf("proposing outside a conversation: %v", r.Err)
	}
	t.Setenv(conversation.EnvConversation, "w1")
	t.Setenv(mission.EnvCrew, conversation.Agent)
	withStdin(t, "# How loud\n\nWhy it matters.\n")
	if r := testutil.RunCommand(t, w.Root, nil, "doc", "add", "adr", "-"); r.Err == nil || !strings.Contains(r.Err.Error(), "propose") {
		t.Errorf("adding one in a conversation: %v", r.Err)
	}
	withStdin(t, "Why it matters.\n")
	if r := testutil.RunCommand(t, w.Root, nil, "doc", "add", "--propose", "adr", "-"); r.Err == nil || !strings.Contains(r.Err.Error(), "start it with a heading") {
		t.Errorf("one with no title: %v", r.Err)
	}
	withStdin(t, "# How loud\n")
	if r := testutil.RunCommand(t, w.Root, nil, "doc", "add", "--propose", "bp", "-"); r.Err == nil || !strings.Contains(r.Err.Error(), "it has adr, note") {
		t.Errorf("a kind the workspace hasn't: %v", r.Err)
	}
	withStdin(t, "# How loud\n\nWhy it matters.\n")
	r := run(t, w, nil, "doc", "add", "--propose", "adr", "-")
	if !strings.Contains(r.Stderr, "proposed in w1 (proposal 1)") || !strings.Contains(r.Stderr, `it's added as records:adr/0008-how-loud.md, committed as "docs: add adr 0008 How loud"`) {
		t.Errorf("proposing: %s", r.Stderr)
	}
	withStdin(t, "# Too quiet\n\nWhy not.\n")
	run(t, w, nil, "doc", "add", "--propose", "adr", "-")
	t.Setenv(conversation.EnvConversation, "")
	t.Setenv(mission.EnvCrew, "")

	c, _ := talks.Load("w1")
	if len(c.Proposals) != 2 {
		t.Fatalf("proposals = %+v", c.Proposals)
	}
	if p := c.Proposals[0]; p.Kind != conversation.ProposeDocument || p.DocKind != "adr" || p.Title != "How loud" || p.Text != "# How loud\n\nWhy it matters." || p.Path != "records:adr/0008-how-loud.md" || p.Subject != "docs: add adr 0008 How loud" {
		t.Errorf("the proposal: %+v", p)
	}
	// Proposed, nothing's changed.
	if got := subjects(t, src, "main"); got != before {
		t.Errorf("proposing pushed: %s", got)
	}
	if entries, _ := os.ReadDir(filepath.Join(w.Root, "repos", "records")); len(entries) != 2 {
		t.Errorf("proposing lifted a capsule: %v", entries)
	}
	if show := run(t, w, nil, "show", "-f", "llm", "w1").Stdout; !strings.Contains(show, "1. document (adr): How loud, to add as records:adr/0008-how-loud.md (waiting for the user)") {
		t.Errorf("ws show w1:\n%s", show)
	}

	// Declined: nothing's added.
	if r := run(t, w, nil, "conversation", "decline", "w1", "2"); !strings.Contains(r.Stderr, "Declined.") {
		t.Errorf("declining: %s", r.Stderr)
	}
	if got := subjects(t, src, "main"); got != before {
		t.Errorf("declining pushed: %s", got)
	}

	// Accepted: added where its kind says, as its kind says it's committed.
	r = run(t, w, nil, "conversation", "accept", "w1", "1")
	if !strings.Contains(r.Stderr, "Added records:adr/0008-how-loud.md: landed on main") {
		t.Errorf("accepting: %s", r.Stderr)
	}
	if got := subjects(t, src, "main"); got != "docs: add adr 0008 How loud | "+before {
		t.Errorf("the remote's main has: %s", got)
	}
	if got := gitOut(t, src, "show", "main:adr/0008-how-loud.md"); got != "# How loud\n\nWhy it matters.\n" {
		t.Errorf("what was added: %q", got)
	}
	if body := gitOut(t, src, "log", "-1", "--format=%b", "main"); !strings.Contains(body, "Proposed in conversation w1") {
		t.Errorf("the commit's body: %q", body)
	}
	if got := gitOut(t, src, "log", "-1", "--format=%s", "main", "--", "adr/0009-too-quiet.md"); got != "" {
		t.Errorf("the declined one was added: %s", got)
	}
	if entries, _ := os.ReadDir(filepath.Join(w.Root, "repos", "records")); len(entries) != 2 {
		t.Errorf("a capsule was left: %v", entries)
	}
	c, _ = talks.Load("w1")
	if p := c.Proposals[0]; p.State != conversation.ProposalAccepted || p.Result != "records:adr/0008-how-loud.md" {
		t.Errorf("accepted: %+v", p)
	}
	if p := c.Proposals[1]; p.State != conversation.ProposalDeclined || p.Result != "" {
		t.Errorf("declined: %+v", p)
	}
	// The agent's told of each in a turn of yours that ws wrote: the
	// decline wakes it, and the document added waits for its next turn.
	if len(c.Turns) != 3 {
		t.Fatalf("turns: %+v", c.Turns)
	}
	if d := c.Turns[1]; d.By != conversation.You || d.Origin != "proposal:2" || d.Quiet || d.Body != "Declined proposal 2, the document “Too quiet”." {
		t.Errorf("the decline's turn: %+v", d)
	}
	if a := c.Turns[2]; a.By != conversation.You || a.Origin != "proposal:1" || !a.Quiet || a.Body != "Accepted proposal 1, the document “How loud”: added as records:adr/0008-how-loud.md." {
		t.Errorf("the accept's turn: %+v", a)
	}
	show := run(t, w, nil, "show", "-f", "llm", "w1").Stdout
	for _, want := range []string{"1. document (adr): How loud, to add as records:adr/0008-how-loud.md (accepted: records:adr/0008-how-loud.md)", "2. document (adr): Too quiet", "(declined)", "Came out of it", "records/adr/0008-how-loud.md  a document, the workspace's",
		"- you (settled proposal 1), "} {
		if !strings.Contains(show, want) {
			t.Errorf("ws show w1 lacks %q:\n%s", want, show)
		}
	}
	if r := testutil.RunCommand(t, w.Root, nil, "conversation", "accept", "w1", "1"); r.Err == nil {
		t.Error("accepted it twice")
	}
}

// You add one yourself with ws doc add: in a repo that takes pull
// requests, it's left in a capsule lifted for it. --dry-run says where it
// would go and adds nothing; crew are refused, and told where theirs goes.
func TestDocAdd_YoursAndNotCrews(t *testing.T) {
	w := kindsWorkspace(t)
	file := filepath.Join(t.TempDir(), "caching.md")
	os.WriteFile(file, []byte("# Caching requirements\n\nCaching, in short.\n"), 0o644)
	day := time.Now().Format("2006-01-02")

	t.Setenv(mission.EnvCrew, "c1")
	if r := testutil.RunCommand(t, w.Root, nil, "doc", "add", "note", file); r.Err == nil || !strings.Contains(r.Err.Error(), "only the user adds a document") || !strings.Contains(r.Err.Error(), "notes/"+day+"-caching-requirements.md") {
		t.Errorf("crew adding one: %v", r.Err)
	}
	t.Setenv(mission.EnvCrew, "")

	if r := run(t, w, nil, "doc", "add", "--dry-run", "note", file); !strings.Contains(r.Stderr, "would add api:notes/"+day+"-caching-requirements.md") || !strings.Contains(r.Stderr, "left for you to open a pull request from") {
		t.Errorf("--dry-run: %s", r.Stderr)
	}
	if _, err := os.Stat(capsuleDir(w, "api", "add-caching-requirements")); err == nil {
		t.Error("--dry-run lifted a capsule")
	}

	r := run(t, w, nil, "doc", "add", "note", file)
	ref := "api/add-caching-requirements:notes/" + day + "-caching-requirements.md"
	if strings.TrimSpace(r.Stdout) != ref || !strings.Contains(r.Stderr, "in a new capsule, api/add-caching-requirements") {
		t.Errorf("adding: %q, %s", r.Stdout, r.Stderr)
	}
	capsule := capsuleDir(w, "api", "add-caching-requirements")
	if got := subjects(t, capsule, "HEAD"); !strings.HasPrefix(got, "Add notes/"+day+"-caching-requirements.md | docs: add the plan") {
		t.Errorf("the capsule has: %s", got)
	}
	if got := subjects(t, w.Sources["api"], "main"); got != "docs: add the plan | initial" {
		t.Errorf("something was pushed: %s", got)
	}
	// One whose path is on the default branch already is refused, and
	// nothing's left of it.
	commitIn(t, w.Sources["api"], "notes/"+day+"-taken.md", "# Taken\n", "a note")
	os.WriteFile(file, []byte("# Taken\n\nAgain.\n"), 0o644)
	if r := testutil.RunCommand(t, w.Root, nil, "doc", "add", "note", file); r.Err == nil || !strings.Contains(r.Err.Error(), "already") {
		t.Errorf("one there already: %v", r.Err)
	}
	if _, err := os.Stat(capsuleDir(w, "api", "add-taken")); err == nil {
		t.Error("a capsule was left of one refused")
	}
}

// guidedWorkspace is docsWorkspace with an ADR kind whose guide is a file
// on records' default branch, and a note kind with none.
func guidedWorkspace(t *testing.T) (*testutil.Workspace, string) {
	t.Helper()
	w := docsWorkspace(t, "")
	f, err := os.OpenFile(filepath.Join(w.Root, "ws.toml"), os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("\n[documents.adr]\ntitle = \"ADR\"\npath = \"records:adr/{number}-{slug}.md\"\nadd = \"docs: add adr {number} {title}\"\nguide = \"records:GUIDE.md\"\n\n[documents.note]\ntitle = \"Note\"\npath = \"records:notes/{slug}.md\"\n")
	f.Close()
	guide := "# How an ADR is written\n\nSay what was decided, then why, in that order. One decision to an ADR.\n"
	commitIn(t, w.Sources["records"], "GUIDE.md", guide, "docs: how an ADR is written")
	ground := filepath.Join(w.Root, "repos", "records", ".ground")
	testutil.GitCmd(t, ground, "fetch", "origin")
	testutil.GitCmd(t, ground, "merge", "--ff-only", "origin/main")
	return w, guide
}

// A conversation's agent is shown a kind's guide before it can propose
// one of that kind: ws doc add --propose <kind>, with no file, prints the
// guide and its version, and proposes nothing; a proposal without that
// version, or with one the guide has moved on from, is refused, with the
// guide printed. The proposal records the guide it was written to. A kind
// with no guide needs none.
func TestDocAdd_TheGuideIsReadBeforeProposing(t *testing.T) {
	w, guide := guidedWorkspace(t)
	version := doc.Version([]byte(guide))
	talks := conversation.ForWorkspace(w.Root)
	talks.OpenTalk(conversation.You, "Make it an ADR", time.Now())
	t.Setenv(conversation.EnvConversation, "w1")
	t.Setenv(mission.EnvCrew, conversation.Agent)
	proposals := func() []conversation.Proposal {
		c, _ := talks.Load("w1")
		return c.Proposals
	}

	// First, the guide.
	r := run(t, w, nil, "doc", "add", "--propose", "adr")
	for _, want := range []string{"The ADR kind (adr): how one's written, from its guide records:GUIDE.md (version " + version + ")", "Say what was decided, then why, in that order.", "It's added at records:adr/{number}-{slug}.md", "{number} is filled in when it's added: leave any number out of its title.", `committed as "docs: add adr {number} {title}"`, "ws doc add --propose adr --guide " + version + " - <<'EOF'"} {
		if !strings.Contains(r.Stdout, want) {
			t.Errorf("showing the guide lacks %q:\n%s\nstderr: %s", want, r.Stdout, r.Stderr)
		}
	}
	if n := len(proposals()); n != 0 {
		t.Errorf("showing the guide proposed %d", n)
	}

	// Proposed without having read it: refused, and shown it.
	adr := "# Key the cache per tenant\n\nWe key it per tenant, because tokens are issued per tenant.\n"
	withStdin(t, adr)
	r = testutil.RunCommand(t, w.Root, nil, "doc", "add", "--propose", "adr", "-")
	if r.Err == nil || !strings.Contains(r.Err.Error(), "not proposed") || !strings.Contains(r.Err.Error(), "--guide "+version) || !strings.Contains(r.Stdout, "Say what was decided") {
		t.Errorf("proposing without the guide: %v\n%s", r.Err, r.Stdout)
	}
	withStdin(t, adr)
	if r := testutil.RunCommand(t, w.Root, nil, "doc", "add", "--propose", "adr", "--guide", "000000000000", "-"); r.Err == nil || !strings.Contains(r.Err.Error(), "has changed since") {
		t.Errorf("proposing with another version: %v", r.Err)
	}
	if n := len(proposals()); n != 0 {
		t.Fatalf("refused, and yet %d proposed", n)
	}

	// Having read it: proposed, written to it.
	withStdin(t, adr)
	run(t, w, nil, "doc", "add", "--propose", "adr", "--guide", version, "-")
	if ps := proposals(); len(ps) != 1 || ps[0].Guide != "records:GUIDE.md" || ps[0].DocKind != "adr" {
		t.Fatalf("proposed: %+v", ps)
	}

	// The guide changes: what was read of it before isn't enough.
	commitIn(t, w.Sources["records"], "GUIDE.md", guide+"\nName the options ruled out.\n", "docs: ADRs name what's ruled out")
	ground := filepath.Join(w.Root, "repos", "records", ".ground")
	testutil.GitCmd(t, ground, "fetch", "origin")
	testutil.GitCmd(t, ground, "merge", "--ff-only", "origin/main")
	withStdin(t, adr)
	r = testutil.RunCommand(t, w.Root, nil, "doc", "add", "--propose", "adr", "--guide", version, "-")
	if r.Err == nil || !strings.Contains(r.Err.Error(), "has changed since") || !strings.Contains(r.Stdout, "Name the options ruled out.") {
		t.Errorf("proposing to an old guide: %v\n%s", r.Err, r.Stdout)
	}

	// A kind with no guide needs none.
	if r := run(t, w, nil, "doc", "add", "--propose", "note"); !strings.Contains(r.Stdout, "The Note kind (note) has no guide") || !strings.Contains(r.Stdout, "ws doc add --propose note - <<'EOF'") || strings.Contains(r.Stdout, "{number}") {
		t.Errorf("a kind with no guide: %s", r.Stdout)
	}
	withStdin(t, "# Tenants\n\nWhat a tenant is.\n")
	run(t, w, nil, "doc", "add", "--propose", "note", "-")
	if ps := proposals(); len(ps) != 2 || ps[1].Guide != "" || ps[1].Path != "records:notes/tenants.md" {
		t.Errorf("a note proposed: %+v", ps)
	}

	// The guide is for proposing: the user adds one without it.
	t.Setenv(conversation.EnvConversation, "")
	t.Setenv(mission.EnvCrew, "")
	file := filepath.Join(t.TempDir(), "adr.md")
	os.WriteFile(file, []byte(adr), 0o644)
	if r := run(t, w, nil, "doc", "add", "--dry-run", "adr", file); !strings.Contains(r.Stderr, "would add records:adr/0001-key-the-cache-per-tenant.md") {
		t.Errorf("yours: %s", r.Stderr)
	}
}
