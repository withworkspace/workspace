package mission_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/cli"
	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/testutil"
	"github.com/withworkspace/workspace/internal/web"
)

// A thread the user opens on a document (as the web UI does) is crew's to
// read with ws show and answer with ws thread reply, recording the
// document's version then; only the user resolves it. The mission's view
// lists it while it's open, and the log says what happened in words.
func TestThreads_CrewAnswer(t *testing.T) {
	w, _ := testutil.OrchestrateFixture(t)
	capsule := filepath.Join(w.Root, "repos", "backend", "cache-lookup")
	os.MkdirAll(filepath.Join(capsule, "docs"), 0o755)
	doc := filepath.Join(capsule, "docs", "proposal.md")
	os.WriteFile(doc, []byte("# Cache\n\nWe cache lookups per user.\n"), 0o644)
	testutil.GitCmd(t, capsule, "add", "-A")
	testutil.GitCmd(t, capsule, "commit", "-q", "-m", "proposal")
	ref := "backend/cache-lookup:docs/proposal.md"
	testutil.Run(t, w, nil, "mission", "artifact", "add", "-m", "cache", doc, "Cache proposal", "--kind", "proposal")

	threads := conversation.For(w.Root, "cache")
	a, _ := testutil.WebArtifact(w.Root)("cache", ref)
	v, _ := threads.KeepVersion([]byte(a.Text))
	if _, err := threads.OpenThread(conversation.NewThread{Kind: conversation.Change, By: conversation.You, Body: "Per tenant, surely?",
		Anchor: conversation.Anchor{Doc: ref, Path: "docs/proposal.md", Quote: &conversation.Quote{Exact: "per user"}, Line: 3, EndLine: 3, Version: v, Excerpt: []string{"We cache lookups per user."}}}, time.Now()); err != nil {
		t.Fatal(err)
	}

	t.Setenv(mission.EnvMission, "cache")
	show := testutil.Run(t, w, nil, "show", "-f", "llm", "t1").Stdout
	for _, want := range []string{"Thread t1 (change, open, waiting for an answer) on " + ref, "> per user", "3  We cache lookups per user.", "- you, ", "Per tenant, surely?", "ws thread reply t1"} {
		if !strings.Contains(show, want) {
			t.Errorf("ws show t1 lacks %q:\n%s", want, show)
		}
	}
	if out := testutil.Run(t, w, nil, "show", "-f", "llm").Stdout; !strings.Contains(out, "Open threads on documents") || !strings.Contains(out, "t1 on "+ref+": change, open") {
		t.Errorf("the mission's view doesn't list the thread:\n%s", out)
	}
	if out := testutil.Run(t, w, nil, "show", "-f", "llm", ref).Stdout; !strings.Contains(out, "t1  ") {
		t.Errorf("the document's view doesn't link the thread:\n%s", out)
	}

	// Crew change the document, and say so.
	os.WriteFile(doc, []byte("# Cache\n\nWe cache lookups per tenant.\n"), 0o644)
	t.Setenv(mission.EnvCrew, "c1")
	testutil.Run(t, w, nil, "thread", "reply", "t1", "Keyed per tenant now.")
	if r := testutil.RunCommand(t, w.Root, nil, "thread", "resolve", "t1"); r.Err == nil || !strings.Contains(r.Err.Error(), "only the user resolves") {
		t.Errorf("crew resolving: %v", r.Err)
	}
	t.Setenv(mission.EnvCrew, "")
	c, _ := threads.Load("t1")
	if len(c.Turns) != 2 || c.Turns[1].By != "c1" || c.Turns[1].Version == v || !c.WaitingOnYou() {
		t.Errorf("after crew's answer: %+v", c)
	}
	if out := testutil.Run(t, w, nil, "thread", "list", "-m", "cache", "--open").Stdout; !strings.Contains(out, "t1   docs/proposal.md  change, open, waiting on the user") {
		t.Errorf("list:\n%s", out)
	}
	// It's your turn: the mission says so, and the document's line too.
	hasReplies := func() bool {
		md, _ := testutil.WebMission(w.Root)("cache", false)
		for _, h := range md.Headlines {
			if h.Kind == "replies" && h.Action.Verb == "review-doc" && h.Action.Target == ref && strings.Contains(h.Text, "a reply on docs/proposal.md, waiting for you (t1)") {
				return true
			}
		}
		return false
	}
	if !hasReplies() {
		md, _ := testutil.WebMission(w.Root)("cache", false)
		t.Errorf("no replies headline: %+v", md.Headlines)
	}
	md, _ := testutil.WebMission(w.Root)("cache", false)
	for _, art := range md.Artifacts {
		if art.Ref == ref && (art.Threads == nil || art.Threads.Open != 1 || art.Threads.Waiting != 1) {
			t.Errorf("the document's threads = %+v", art.Threads)
		}
	}
	testutil.Run(t, w, nil, "thread", "resolve", "t1")
	if hasReplies() {
		t.Error("still your turn after resolving")
	}
	if c, _ := threads.Load("t1"); c.Status != conversation.Resolved {
		t.Errorf("not resolved: %+v", c)
	}

	md, _ = testutil.WebMission(w.Root)("cache", false)
	log := eventsText(md.Events)
	for _, want := range []string{"you asked for a change on docs/proposal.md (t1): Per tenant, surely?", "c1 on t1: Keyed per tenant now.", "you resolved t1"} {
		if !strings.Contains(log, want) {
			t.Errorf("log lacks %q:\n%s", want, log)
		}
	}
}

func eventsText(events []web.Event) string {
	var b strings.Builder
	for _, e := range events {
		b.WriteString(e.Text + "\n")
	}
	return b.String()
}

// The thread agent answers a thread you've asked something in: a session
// started from ws show, resumed when you reply; a note on its own isn't
// answered, and an agent that fails says so in the thread.
func TestThreads_AgentAnswers(t *testing.T) {
	w, _ := testutil.OrchestrateFixture(t)
	harness.Register(testutil.FakeConverser{})
	harness.Register(testutil.FakeConverser{Fail: true})
	local := filepath.Join(w.Root, "ws.local.toml")
	conf, _ := os.ReadFile(local)
	os.WriteFile(local, append(conf, []byte("\n[conversation]\nharness = \"fake-converse\"\n")...), 0o644)

	threads := conversation.For(w.Root, "cache")
	anchor := conversation.Anchor{Doc: "mission:notes.md", Path: "notes.md", Quote: &conversation.Quote{Exact: "per user"}, Line: 1, EndLine: 1}
	threads.OpenThread(conversation.NewThread{Kind: conversation.Ask, By: conversation.You, Body: "Why per user?", Anchor: anchor}, time.Now())
	threads.OpenThread(conversation.NewThread{Kind: conversation.Note, By: conversation.You, Body: "Nice.", Anchor: anchor}, time.Now())

	testutil.Run(t, w, nil, "thread", "answer", "-m", "cache", "t1")
	c, _ := threads.Load("t1")
	if len(c.Turns) != 2 || c.Turns[1].By != conversation.Agent || c.Turns[1].Body != "first: read the thread on fake-small" || c.Session == nil || c.Session.ID != "sess-1" || c.Answering || !c.WaitingOnYou() {
		t.Fatalf("after the first answer: %+v", c)
	}
	testutil.Run(t, w, nil, "thread", "answer", "-m", "cache", "t1") // nothing new to answer
	if c, _ = threads.Load("t1"); len(c.Turns) != 2 {
		t.Errorf("answered twice: %+v", c.Turns)
	}

	threads.Say("t1", conversation.Turn{By: conversation.You, At: time.Now(), Body: "And per tenant?"})
	testutil.Run(t, w, nil, "thread", "answer", "-m", "cache", "t1")
	if c, _ = threads.Load("t1"); len(c.Turns) != 4 || c.Turns[3].Body != "next: read the reply on fake-small" {
		t.Errorf("after your reply: %+v", c.Turns)
	}

	testutil.Run(t, w, nil, "thread", "answer", "-m", "cache", "t2")
	if c, _ = threads.Load("t2"); len(c.Turns) != 1 {
		t.Errorf("a note was answered: %+v", c.Turns)
	}

	os.WriteFile(local, append(conf, []byte("\n[conversation]\nharness = \"fail-converse\"\n")...), 0o644)
	threads.Say("t1", conversation.Turn{By: conversation.You, At: time.Now(), Body: "Still there?"})
	testutil.RunCommand(t, w.Root, nil, "thread", "answer", "-m", "cache", "t1")
	c, _ = threads.Load("t1")
	if last := c.Last(); last.By != conversation.WS || !strings.Contains(last.Body, "The agent couldn't answer") || !strings.Contains(last.Body, "not signed in") {
		t.Errorf("a failed answer: %+v", last)
	}
	if !c.WaitingOnYou() {
		t.Error("a failed answer should be back with you")
	}
}

// A suggested edit is written into the document when you accept it:
// committed (just that file, as you) in a capsule, and the thread
// resolved. One whose text has changed, or a file with uncommitted
// changes, isn't; declining keeps the thread open.
func TestThreads_SuggestAndAccept(t *testing.T) {
	w, _ := testutil.OrchestrateFixture(t)
	capsule := filepath.Join(w.Root, "repos", "backend", "cache-lookup")
	os.MkdirAll(filepath.Join(capsule, "docs"), 0o755)
	doc := filepath.Join(capsule, "docs", "proposal.md")
	os.WriteFile(doc, []byte("# Cache\n\nShip on Friday.\nKeep it small.\n"), 0o644)
	testutil.GitCmd(t, capsule, "add", "-A")
	testutil.GitCmd(t, capsule, "commit", "-q", "-m", "proposal")
	ref := "backend/cache-lookup:docs/proposal.md"
	testutil.Run(t, w, nil, "mission", "artifact", "add", "-m", "cache", doc, "Cache proposal", "--kind", "proposal")
	threads := conversation.For(w.Root, "cache")
	anchor := conversation.Anchor{Doc: ref, Path: "docs/proposal.md", Quote: &conversation.Quote{Exact: "on Friday"}, Line: 3, EndLine: 3}
	threads.OpenThread(conversation.NewThread{Kind: conversation.Change, By: conversation.You, Body: "Not on a Friday.", Anchor: anchor}, time.Now())
	t.Setenv(mission.EnvMission, "cache")

	// The agent suggests, as the agent, on stdin; text that isn't there once is refused.
	t.Setenv(mission.EnvCrew, "agent")
	testutil.WithStdin(t, "<<<<<<<\nShip on Monday.\n=======\nx\n>>>>>>>\n")
	if r := testutil.RunCommand(t, w.Root, nil, "thread", "suggest", "t1"); r.Err == nil || !strings.Contains(r.Err.Error(), "isn't there") {
		t.Errorf("text that isn't there: %v", r.Err)
	}
	testutil.WithStdin(t, "<<<<<<<\nShip on Friday.\n=======\nShip on Tuesday.\n>>>>>>>\n")
	testutil.Run(t, w, nil, "thread", "suggest", "t1")
	if r := testutil.RunCommand(t, w.Root, nil, "thread", "accept", "t1"); r.Err == nil {
		t.Error("the agent accepted its own suggestion")
	}
	t.Setenv(mission.EnvCrew, "")
	c, _ := threads.Load("t1")
	if p := c.Pending(); p == nil || p.By != conversation.Agent || p.From != "Ship on Friday." || p.To != "Ship on Tuesday." {
		t.Fatalf("suggestion = %+v", c.Suggestions)
	}
	if out := testutil.Run(t, w, nil, "show", "-f", "llm", "t1").Stdout; !strings.Contains(out, "Suggested edit by agent") || !strings.Contains(out, "waiting for the user to accept") {
		t.Errorf("ws show t1:\n%s", out)
	}

	before := strings.TrimSpace(testutil.GitOut(t, capsule, "rev-parse", "HEAD"))
	testutil.Run(t, w, nil, "thread", "accept", "t1")
	if data, _ := os.ReadFile(doc); string(data) != "# Cache\n\nShip on Tuesday.\nKeep it small.\n" {
		t.Errorf("document = %q", data)
	}
	head := strings.TrimSpace(testutil.GitOut(t, capsule, "rev-parse", "HEAD"))
	if head == before || !strings.Contains(testutil.GitOut(t, capsule, "log", "-1", "--format=%s"), "Accept suggestion on docs/proposal.md (t1)") {
		t.Errorf("not committed: %s", testutil.GitOut(t, capsule, "log", "-1", "--format=%s"))
	}
	if files := strings.TrimSpace(testutil.GitOut(t, capsule, "show", "--name-only", "--format=", "HEAD")); files != "docs/proposal.md" {
		t.Errorf("committed %q", files)
	}
	c, _ = threads.Load("t1")
	if c.Status != conversation.Resolved || c.Suggestions[0].State != conversation.SuggestAccepted || c.Suggestions[0].Commit != head {
		t.Errorf("after accepting: %+v", c)
	}

	// A file with uncommitted changes (crew's draft) gets the edit written
	// in alongside them, and nothing's committed.
	threads.OpenThread(conversation.NewThread{Kind: conversation.Change, By: conversation.You, Body: "Say small.", Anchor: anchor}, time.Now())
	testutil.Run(t, w, nil, "thread", "suggest", "t2", "--from", "Keep it small.", "--to", "Keep it very small.")
	os.WriteFile(doc, []byte("# Cache\n\nShip on Tuesday.\nKeep it small.\nCrew's draft line.\n"), 0o644)
	r := testutil.Run(t, w, nil, "thread", "accept", "t2")
	if !strings.Contains(r.Stderr, "not committed: docs/proposal.md already had uncommitted changes in backend/cache-lookup") {
		t.Errorf("accepting into a dirty file said: %s", r.Stderr)
	}
	if data, _ := os.ReadFile(doc); string(data) != "# Cache\n\nShip on Tuesday.\nKeep it very small.\nCrew's draft line.\n" {
		t.Errorf("document = %q", data)
	}
	if now := strings.TrimSpace(testutil.GitOut(t, capsule, "rev-parse", "HEAD")); now != head {
		t.Error("committed crew's uncommitted changes")
	}
	if c, _ = threads.Load("t2"); c.Status != conversation.Resolved || c.Suggestions[0].State != conversation.SuggestAccepted || c.Suggestions[0].Commit != "" {
		t.Errorf("after accepting into a dirty file: %+v", c)
	}
	testutil.GitCmd(t, capsule, "commit", "-q", "-am", "crew's draft")

	// Declining keeps the thread open; a suggestion whose text has since
	// gone can't be accepted.
	threads.OpenThread(conversation.NewThread{Kind: conversation.Change, By: conversation.You, Body: "Smaller.", Anchor: anchor}, time.Now())
	testutil.Run(t, w, nil, "thread", "suggest", "t3", "--from", "Keep it very small.", "--to", "Keep it tiny.")
	testutil.Run(t, w, nil, "thread", "decline", "t3")
	if c, _ = threads.Load("t3"); c.Status != conversation.Open || c.Suggestions[0].State != conversation.SuggestDeclined {
		t.Errorf("declined: %+v", c)
	}
	testutil.Run(t, w, nil, "thread", "suggest", "t3", "--from", "Keep it very small.", "--to", "Keep it lean.")
	os.WriteFile(doc, []byte("# Cache\n\nShip on Tuesday.\nKeep it smaller.\n"), 0o644)
	testutil.GitCmd(t, capsule, "commit", "-q", "-am", "crew edited it")
	if r := testutil.RunCommand(t, w.Root, nil, "thread", "accept", "t3"); r.Err == nil || !strings.Contains(r.Err.Error(), "changed since") {
		t.Errorf("text that's gone: %v", r.Err)
	}

	md, _ := testutil.WebMission(w.Root)("cache", false)
	log := eventsText(md.Events)
	for _, want := range []string{"agent suggested an edit on t1: Ship on Tuesday.", "you accepted the edit on t1 (" + head[:7] + ")", "you declined the edit on t3"} {
		if !strings.Contains(log, want) {
			t.Errorf("log lacks %q:\n%s", want, log)
		}
	}
}

// A change you asked for on the current stage's document holds up
// accepting the stage (a question doesn't); once every thread on it is
// resolved, it's yours to approve, and approving it as it is clears that.
func TestThreads_StageWaitsOnChanges(t *testing.T) {
	w, store := testutil.OrchestrateFixture(t)
	store.EndCrew("cache", "c1", "exited")
	briefFile := filepath.Join(t.TempDir(), "brief.md")
	os.WriteFile(briefFile, []byte(twoStages), 0o644)
	t.Setenv(mission.EnvMission, "cache")
	testutil.Run(t, w, nil, "mission", "brief", "--set", briefFile)
	capsule := filepath.Join(w.Root, "repos", "backend", "cache-lookup")
	os.MkdirAll(filepath.Join(capsule, "docs"), 0o755)
	doc := filepath.Join(capsule, "docs", "proposal.md")
	os.WriteFile(doc, []byte("# Cache\n\nWe cache lookups per user.\n"), 0o644)
	testutil.GitCmd(t, capsule, "add", "-A")
	testutil.GitCmd(t, capsule, "commit", "-q", "-m", "proposal")
	ref := "backend/cache-lookup:docs/proposal.md"
	testutil.Run(t, w, nil, "mission", "artifact", "add", doc, "Cache proposal", "--kind", "proposal")

	threads := conversation.For(w.Root, "cache")
	anchor := conversation.Anchor{Doc: ref, Path: "docs/proposal.md", Quote: &conversation.Quote{Exact: "per user"}, Line: 3, EndLine: 3}
	threads.OpenThread(conversation.NewThread{Kind: conversation.Change, By: conversation.You, Body: "Per tenant.", Anchor: anchor}, time.Now())
	threads.OpenThread(conversation.NewThread{Kind: conversation.Ask, By: conversation.You, Body: "Why?", Anchor: anchor}, time.Now())

	if r := testutil.RunCommand(t, w.Root, nil, "mission", "accept", "looks done"); r.Err == nil || !strings.Contains(r.Err.Error(), "asked for changes to "+ref+" that are still open (t1)") {
		t.Errorf("accepting with a change open: %v", r.Err)
	}
	headlines := func() string {
		md, _ := testutil.WebMission(w.Root)("cache", false)
		var b strings.Builder
		for _, h := range md.Headlines {
			fmt.Fprintf(&b, "%s|%s|%s|%s\n", h.Kind, h.Text, h.Action.Verb, h.Action.Target)
		}
		return b.String()
	}
	if h := headlines(); strings.Contains(h, "approve") {
		t.Errorf("approve with threads open:\n%s", h)
	}
	testutil.Run(t, w, nil, "thread", "resolve", "t1")
	if h := headlines(); strings.Contains(h, "approve|") {
		t.Errorf("approve with a question open:\n%s", h)
	}
	testutil.Run(t, w, nil, "thread", "resolve", "t2")
	if h := headlines(); !strings.Contains(h, "approve|docs/proposal.md has no open threads: approve it?|review-doc|"+ref) {
		t.Errorf("no approve headline:\n%s", h)
	}

	text, _ := os.ReadFile(doc)
	if _, err := threads.Approve(ref, "docs/proposal.md", text, conversation.You, time.Now()); err != nil {
		t.Fatal(err)
	}
	if h := headlines(); strings.Contains(h, "approve|") {
		t.Errorf("approve after approving:\n%s", h)
	}
	if out := testutil.Run(t, w, nil, "show", "-f", "llm", ref).Stdout; !strings.Contains(out, "Approved by the user") || !strings.Contains(out, "as it is now") {
		t.Errorf("ws show of the document:\n%s", out)
	}
	// Changed since: it's yours to approve again.
	os.WriteFile(doc, []byte("# Cache\n\nWe cache lookups per tenant.\n"), 0o644)
	if h := headlines(); !strings.Contains(h, "approve|") {
		t.Errorf("no approve headline after a change:\n%s", h)
	}
	if out := testutil.Run(t, w, nil, "show", "-f", "llm", ref).Stdout; !strings.Contains(out, "an earlier version") {
		t.Errorf("ws show of a changed document:\n%s", out)
	}
	testutil.Run(t, w, nil, "mission", "accept", "approved docs/proposal.md")
	md, _ := testutil.WebMission(w.Root)("cache", false)
	if log := eventsText(md.Events); !strings.Contains(log, "you approved docs/proposal.md") {
		t.Errorf("log:\n%s", log)
	}
}

// resumableHarness resumes crew: the line typed in is written to $OUT.
type resumableHarness struct{}

func (resumableHarness) Name() string     { return "resumable" }
func (resumableHarness) Available() error { return nil }
func (resumableHarness) Start(b harness.Brief) (harness.Launch, error) {
	return harness.Launch{}, harness.ErrNoResume
}
func (resumableHarness) Resume(dir, conv string) (harness.Launch, error) {
	return harness.Launch{Args: []string{"/bin/sh", "-c", `printf 'ready> '; read line; printf '%s' "$line" > "$OUT"; sleep 5`}, Dir: dir}, nil
}

// Threads sent to crew: fresh crew dispatched into the document's capsule
// for a document you recorded, the crew that wrote one resumed for its.
// They're crew's (the thread agent stays out) until crew say they've
// changed the document, or finish without saying so.
func TestThreads_SendToCrew(t *testing.T) {
	w, store, out := dispatchFixture(t)
	t.Setenv("WS_ROLE", "")
	harness.Register(resumableHarness{})
	store.StartCrew("cache", "resumable", "conv-2")
	store.SetCrewProcess("cache", "c2", 0, "backend/cache-lookup")
	store.EndCrew("cache", "c2", "exit 0")

	capsule := filepath.Join(w.Root, "repos", "backend", "cache-lookup")
	os.MkdirAll(filepath.Join(capsule, "docs"), 0o755)
	for _, name := range []string{"a.md", "b.md"} {
		os.WriteFile(filepath.Join(capsule, "docs", name), []byte("# Cache\n\nWe cache lookups per user.\n"), 0o644)
	}
	testutil.GitCmd(t, capsule, "add", "-A")
	testutil.GitCmd(t, capsule, "commit", "-q", "-m", "proposals")
	testutil.Run(t, w, nil, "mission", "artifact", "add", filepath.Join(capsule, "docs", "a.md"), "Yours", "--kind", "proposal")
	t.Setenv(mission.EnvCrew, "c2")
	testutil.Run(t, w, nil, "mission", "artifact", "add", filepath.Join(capsule, "docs", "b.md"), "c2's", "--kind", "proposal")
	t.Setenv(mission.EnvCrew, "")

	threads := conversation.For(w.Root, "cache")
	open := func(name, kind string) {
		threads.OpenThread(conversation.NewThread{Kind: kind, By: conversation.You, Body: "Rewrite this per tenant.",
			Anchor: conversation.Anchor{Doc: "backend/cache-lookup:docs/" + name, Path: "docs/" + name, Quote: &conversation.Quote{Exact: "per user"}, Line: 3, EndLine: 3}}, time.Now())
	}
	open("a.md", conversation.Change) // t1
	open("b.md", conversation.Change) // t2
	open("b.md", conversation.Ask)    // t3

	if r := testutil.RunCommand(t, w.Root, nil, "thread", "send", "t1", "t2"); r.Err == nil || !strings.Contains(r.Err.Error(), "one document at a time") {
		t.Errorf("sending threads on two documents: %v", r.Err)
	}

	// Yours: fresh crew, told which threads and how to hand them back.
	r := testutil.Run(t, w, nil, "thread", "send", "t1")
	if !strings.Contains(r.Stderr, "Started c3 in backend/cache-lookup on thread t1.") {
		t.Errorf("send said: %s", r.Stderr)
	}
	got := waitFile(t, out)
	for _, want := range []string{"dispatch: The user wants docs/a.md (backend/cache-lookup:docs/a.md) changed, in thread t1", "ws show t1", "(and commit it)", "ws thread changed <thread>"} { // the first line: the fake crew reads one
		if !strings.Contains(got, want) {
			t.Errorf("crew got %q, lacking %q", got, want)
		}
	}
	c, _ := threads.Load("t1")
	if !c.WithCrew() || c.Sent.Crew != "c3" || c.WaitingOnYou() {
		t.Errorf("after sending: %+v", c.Sent)
	}
	// You say more: the thread agent stays out of it while crew have it.
	if c, _ = threads.Say("t1", conversation.Turn{By: conversation.You, At: time.Now(), Body: "And keep it short."}); c.NeedsAnswer() {
		t.Error("the agent would answer a thread crew have")
	}
	if out := testutil.Run(t, w, nil, "show", "-f", "llm", "t1").Stdout; !strings.Contains(out, "open, sent to c3 to change the document for") || !strings.Contains(out, "ws thread changed t1") {
		t.Errorf("ws show t1:\n%s", out)
	}
	// Crew that finish without saying they've changed it hand it back.
	store.EndCrew("cache", "c3", "exit 0")
	if c, _ = threads.Load("t1"); c.WithCrew() || !c.WaitingOnYou() {
		t.Errorf("after c3 ended: %+v", c.Sent)
	}

	// c2's: c2 is resumed, and told.
	os.Remove(out)
	r = testutil.Run(t, w, nil, "thread", "send", "t2")
	if !strings.Contains(r.Stderr, "Resumed c2, which wrote docs/b.md.") {
		t.Errorf("send said: %s", r.Stderr)
	}
	if got := waitFile(t, out); !strings.HasPrefix(got, "The user wants docs/b.md (backend/cache-lookup:docs/b.md) changed, in thread t2.") {
		t.Errorf("c2 got %q", got)
	}
	if c, _ := threads.Load("t2"); !c.WithCrew() || c.Sent.Crew != "c2" {
		t.Errorf("t2 sent: %+v", c.Sent)
	}
	if r := testutil.RunCommand(t, w.Root, nil, "thread", "send", "t2"); r.Err == nil || !strings.Contains(r.Err.Error(), "with c2 already") {
		t.Errorf("sending it again: %v", r.Err)
	}
	t.Setenv(mission.EnvCrew, "c2")
	testutil.Run(t, w, nil, "thread", "changed", "t2", "Keyed per tenant.")
	t.Setenv(mission.EnvCrew, "")
	if c, _ := threads.Load("t2"); c.Sent != nil || !c.WaitingOnYou() || !c.Last().Changed {
		t.Errorf("after c2 changed it: %+v", c)
	}
	md, _ := testutil.WebMission(w.Root)("cache", false)
	log := eventsText(md.Events)
	for _, want := range []string{"you sent t1 to c3", "you sent t2 to c2", "c2 on t2: changed the document: Keyed per tenant."} {
		if !strings.Contains(log, want) {
			t.Errorf("log lacks %q:\n%s", want, log)
		}
	}
}

// Resolving a thread something was said in has its agent draft what was
// decided, in its session, for you to record or skip; a thread with only
// your words isn't drafted from.
func TestThreads_DraftDecisionOnResolve(t *testing.T) {
	w, _ := testutil.OrchestrateFixture(t)
	harness.Register(testutil.FakeConverser{})
	local := filepath.Join(w.Root, "ws.local.toml")
	conf, _ := os.ReadFile(local)
	os.WriteFile(local, append(conf, []byte("\n[conversation]\nharness = \"fake-converse\"\n")...), 0o644)

	threads := conversation.For(w.Root, "cache")
	anchor := conversation.Anchor{Doc: "mission:notes.md", Path: "notes.md", Quote: &conversation.Quote{Exact: "per user"}, Line: 1, EndLine: 1}
	threads.OpenThread(conversation.NewThread{Kind: conversation.Ask, By: conversation.You, Body: "Why per user?", Anchor: anchor}, time.Now())
	threads.OpenThread(conversation.NewThread{Kind: conversation.Note, By: conversation.You, Body: "Nice.", Anchor: anchor}, time.Now())
	testutil.Run(t, w, nil, "thread", "answer", "-m", "cache", "t1")

	testutil.Run(t, w, nil, "thread", "answer", "-m", "cache", "--draft", "t1") // open: nothing to draft
	if c, _ := threads.Load("t1"); c.Draft != nil {
		t.Errorf("drafted from an open thread: %+v", c.Draft)
	}
	testutil.Run(t, w, nil, "thread", "resolve", "-m", "cache", "t1")
	testutil.Run(t, w, nil, "thread", "answer", "-m", "cache", "--draft", "t1")
	c, _ := threads.Load("t1")
	if d := c.Draft; d == nil || d.State != conversation.DraftOpen || d.What != "Key the cache per tenant." || d.Why != "Tokens are issued per tenant, not per user." || d.By != conversation.Agent {
		t.Fatalf("draft = %+v", c.Draft)
	}
	if c.Answering || len(c.Turns) != 2 {
		t.Errorf("drafting said something in the thread: %+v", c)
	}
	t.Setenv(mission.EnvMission, "cache")
	if out := testutil.Run(t, w, nil, "show", "-f", "llm", "t1").Stdout; !strings.Contains(out, "What was decided, as agent drafted it for the user to record: Key the cache per tenant. Why: Tokens") {
		t.Errorf("ws show t1:\n%s", out)
	}
	testutil.Run(t, w, nil, "thread", "answer", "-m", "cache", "--draft", "t1") // drafted already
	if c, _ := threads.Load("t1"); c.Draft.What != "Key the cache per tenant." {
		t.Errorf("drafted again: %+v", c.Draft)
	}

	testutil.Run(t, w, nil, "thread", "resolve", "-m", "cache", "t2")
	testutil.Run(t, w, nil, "thread", "answer", "-m", "cache", "--draft", "t2")
	if c, _ := threads.Load("t2"); c.Draft != nil {
		t.Errorf("drafted from your own note: %+v", c.Draft)
	}
}

// A conversation about the workspace is answered by its agent in a
// session, resumed for what you say next; the reply loses its Title: and
// Summary: lines to the conversation's own; ws show w1 has it.
func TestTalk_AgentAnswers(t *testing.T) {
	w, _ := testutil.OrchestrateFixture(t)
	harness.Register(testutil.FakeConverser{})
	local := filepath.Join(w.Root, "ws.local.toml")
	conf, _ := os.ReadFile(local)
	os.WriteFile(local, append(conf, []byte("\n[conversation]\nharness = \"fake-converse\"\n")...), 0o644)

	talks := conversation.ForWorkspace(w.Root)
	talks.OpenTalk(conversation.You, "What would a token cache take?", time.Now())
	testutil.Run(t, w, nil, "conversation", "answer", "w1")
	c, _ := talks.Load("w1")
	if len(c.Turns) != 2 || !strings.HasPrefix(c.Turns[1].Body, "first: talked (") || strings.Contains(c.Turns[1].Body, "Title:") || c.Title != "Token cache" || c.Summary != "What a cache would take." || !c.WaitingOnYou() {
		t.Fatalf("after the first answer: %+v", c)
	}
	if strings.HasPrefix(c.Turns[1].Body, "first: talked (0 paths)") {
		t.Error("the agent may read no capsule or .ground")
	}
	talks.Say("w1", conversation.Turn{By: conversation.You, At: time.Now(), Body: "And per tenant?"})
	testutil.Run(t, w, nil, "conversation", "answer", "w1")
	if c, _ = talks.Load("w1"); len(c.Turns) != 4 || !strings.HasPrefix(c.Turns[3].Body, "talked (") {
		t.Errorf("after your reply: %+v", c.Turns)
	}
	out := testutil.Run(t, w, nil, "show", "-f", "llm", "w1").Stdout
	for _, want := range []string{"Conversation w1 — \"Token cache\" (the workspace's, open, the agent replied)", "Summary: What a cache would take.", "- you, ", "And per tenant?"} {
		if !strings.Contains(out, want) {
			t.Errorf("ws show w1 lacks %q:\n%s", want, out)
		}
	}
	if out := testutil.Run(t, w, nil, "conversation", "list").Stdout; !strings.Contains(out, "w1   Token cache (the agent replied): What a cache would take.") {
		t.Errorf("list:\n%s", out)
	}
	testutil.Run(t, w, nil, "conversation", "resolve", "w1")
	if out := testutil.Run(t, w, nil, "conversation", "list").Stdout; strings.Contains(out, "w1") {
		t.Errorf("a resolved one listed:\n%s", out)
	}
}

// A conversation's agent proposes with --propose on the verbs it can't
// use itself; you confirm. A decision kept with a workspace conversation
// moves to the mission it becomes; a briefed mission is created ready and
// its capsules lifted, an idea waits to be briefed; each records where it
// came from.
func TestTalk_Proposals(t *testing.T) {
	w, store := testutil.OrchestrateFixture(t)
	var lifted []string
	t.Cleanup(testutil.UseLifter(func(root, slug, repo, branch string) error {
		lifted = append(lifted, slug+":"+repo+"/"+branch)
		return nil
	}))
	talks := conversation.ForWorkspace(w.Root)
	talks.OpenTalk(conversation.You, "What would a token cache take?", time.Now())

	if r := testutil.RunCommand(t, w.Root, nil, "mission", "decide", "--propose", "x"); r.Err == nil || !strings.Contains(r.Err.Error(), "for a conversation's agent") {
		t.Errorf("proposing outside a conversation: %v", r.Err)
	}
	t.Setenv(conversation.EnvConversation, "w1")
	t.Setenv(mission.EnvCrew, conversation.Agent)
	for _, args := range [][]string{{"mission", "decide", "-m", "cache", "Key per tenant."}, {"mission", "new", "Token cache"}} {
		if r := testutil.RunCommand(t, w.Root, nil, args...); r.Err == nil || !strings.Contains(r.Err.Error(), "propose") {
			t.Errorf("%v in a conversation: %v", args, r.Err)
		}
	}
	testutil.Run(t, w, nil, "mission", "decide", "--propose", "Key the cache per tenant.", "--why", "Tokens are issued per tenant.")
	testutil.WithStdin(t, "## Goal\nCache tokens.\n\n## Stages\n1. Build it in `backend/token-cache`.\n")
	testutil.Run(t, w, nil, "mission", "new", "--propose", "Token cache", "--brief-file", "-", "--capsule", "backend/token-bench", "--about", "backend#7")
	testutil.Run(t, w, nil, "mission", "new", "--propose", "Rate limits, later")
	// What it's about is a name ws reads: one it doesn't is said to the
	// agent, and nothing's proposed.
	if r := testutil.RunCommand(t, w.Root, nil, "mission", "new", "--propose", "Encode it", "--about", "UTF-8"); r.Err == nil || !strings.Contains(r.Err.Error(), "ws names nothing UTF-8") {
		t.Errorf("proposing a mission about a word: %v", r.Err)
	}
	t.Setenv(conversation.EnvConversation, "")
	t.Setenv(mission.EnvCrew, "")
	c, _ := talks.Load("w1")
	if len(c.Proposals) != 3 || c.Proposals[1].Brief != "## Goal\nCache tokens.\n\n## Stages\n1. Build it in `backend/token-cache`." || c.Proposals[1].Capsules[0] != "backend/token-bench" || c.Proposals[2].Brief != "" {
		t.Fatalf("proposals = %+v", c.Proposals)
	}
	if got := c.Proposals[1].About; len(got) != 1 || got[0] != "backend#7" || len(c.Proposals[2].About) != 0 {
		t.Errorf("what the proposals are about: %q, %q", got, c.Proposals[2].About)
	}

	// A decision before there's a mission is kept with the conversation.
	if r := testutil.Run(t, w, nil, "conversation", "accept", "w1", "1"); !strings.Contains(r.Stderr, "kept with w1") {
		t.Errorf("accepting a decision: %s", r.Stderr)
	}
	r := testutil.Run(t, w, nil, "conversation", "accept", "w1", "2")
	if !strings.Contains(r.Stderr, "Created mission token-cache with its brief: its planner plans it (Briefing in t1), and confirming the plan starts it.") || !strings.Contains(r.Stderr, "Lifted backend/token-bench, backend/token-cache.") || !strings.Contains(r.Stderr, "1 decision from w1 moved to it.") {
		t.Errorf("accepting a mission: %s", r.Stderr)
	}
	if strings.Join(lifted, " ") != "token-cache:backend/token-bench token-cache:backend/token-cache" {
		t.Errorf("lifted %v", lifted)
	}
	// It's about what it was proposed as about.
	if r := testutil.RunCommand(t, w.Root, nil, "mission", "about", "-m", "token-cache"); r.Stdout != "backend#7\n" {
		t.Errorf("the mission accepted is about %q", r.Stdout)
	}
	m, err := store.Load("token-cache")
	// It's to brief: its planner plans it from its brief, in its own
	// conversation, and confirming the plan starts it.
	if err != nil || m.Conversation != "w1" || !m.ToBrief() {
		t.Fatalf("the mission: %+v, %v", m, err)
	}
	if b := conversation.For(w.Root, "token-cache").Briefing(); b == nil || b.ID != "t1" {
		t.Errorf("its planner's conversation: %+v", b)
	}
	if brief, _ := store.ReadBrief("token-cache"); !strings.Contains(brief, "Cache tokens.") {
		t.Errorf("brief = %q", brief)
	}
	all, _ := store.Decisions("token-cache")
	if len(all) != 1 || all[0].What != "Key the cache per tenant." || all[0].From != "w1" || all[0].By != "you" {
		t.Errorf("its decisions: %+v", all)
	}
	// An idea waits to be briefed, from the conversation.
	if r := testutil.Run(t, w, nil, "conversation", "accept", "w1", "3"); !strings.Contains(r.Stderr, "Saved as an idea: mission rate-limits-later, waiting to be briefed") {
		t.Errorf("accepting an idea: %s", r.Stderr)
	}
	if m, _ := store.Load("rate-limits-later"); m == nil || !m.ToBrief() || m.Conversation != "w1" {
		t.Errorf("the idea: %+v", m)
	}
	if r := testutil.RunCommand(t, w.Root, nil, "conversation", "decline", "w1", "3"); r.Err == nil {
		t.Error("declined an accepted proposal")
	}

	show := testutil.Run(t, w, nil, "show", "-f", "llm", "w1").Stdout
	for _, want := range []string{"It became mission rate-limits-later", "1. decision: Key the cache per tenant. (accepted: token-cache:d1)", "2. mission: Token cache, about backend#7 (accepted: token-cache)", "3. mission: Rate limits, later (an idea, to brief later) (accepted: rate-limits-later)"} {
		if !strings.Contains(show, want) {
			t.Errorf("ws show w1 lacks %q:\n%s", want, show)
		}
	}
	if out := testutil.Run(t, w, nil, "mission", "show", "token-cache", "-f", "llm").Stdout; !strings.Contains(out, "Came out of conversation w1") {
		t.Errorf("the mission doesn't say where it came from:\n%s", out)
	}
}

// A mission's conversations (its Talk tab) start from the mission and
// propose for it: an edit to its brief (refused once the brief has moved
// on), crew to start (queued where they can't start here), a decision
// (the mission's at once) and a follow-up mission, raised from it. They're
// listed with the mission, not as threads.
func TestTalk_MissionProposals(t *testing.T) {
	w, store := testutil.OrchestrateFixture(t)
	harness.Register(testutil.FakeConverser{})
	local := filepath.Join(w.Root, "ws.local.toml")
	conf, _ := os.ReadFile(local)
	os.WriteFile(local, append(conf, []byte("\n[conversation]\nharness = \"fake-converse\"\n")...), 0o644)
	store.EndCrew("cache", "c1", "exit 0")
	talks := conversation.For(w.Root, "cache")
	talks.OpenTalk(conversation.You, "Is stage 2 still right?", time.Now())
	testutil.Run(t, w, nil, "conversation", "answer", "-m", "cache", "t1")
	c, _ := talks.Load("t1")
	if len(c.Turns) != 2 || c.Turns[1].Body != "about the mission, knowing what it may propose" || c.Title != "Stage 2" {
		t.Fatalf("after its answer: %+v", c)
	}

	t.Setenv(conversation.EnvConversation, "t1")
	t.Setenv(mission.EnvCrew, conversation.Agent)
	t.Setenv(mission.EnvMission, "cache")
	for _, args := range [][]string{{"mission", "brief", "--set", "-"}, {"mission", "dispatch", "Do it"}} {
		testutil.WithStdin(t, "## Goal\nx\n")
		if r := testutil.RunCommand(t, w.Root, nil, args...); r.Err == nil || !strings.Contains(r.Err.Error(), "propose") {
			t.Errorf("%v in a conversation: %v", args, r.Err)
		}
	}
	testutil.WithStdin(t, "## Goal\nCache the lookup, per tenant.\n## Acceptance\n- tests pass\n")
	testutil.Run(t, w, nil, "mission", "brief", "--propose", "--set", "-")
	testutil.Run(t, w, nil, "mission", "dispatch", "--propose", "Benchmark the cache", "--capsule", "backend/cache-lookup")
	testutil.Run(t, w, nil, "mission", "decide", "--propose", "Key per tenant.", "--why", "Tokens are.")
	testutil.Run(t, w, nil, "mission", "new", "--propose", "Evict on logout")
	testutil.WithStdin(t, "## Goal\nCache the lookup.\n## Stages\n1. Build it.\n")
	testutil.Run(t, w, nil, "mission", "brief", "--propose", "--set", "-")
	t.Setenv(conversation.EnvConversation, "")
	t.Setenv(mission.EnvCrew, "")
	if c, _ = talks.Load("t1"); len(c.Proposals) != 5 || c.Proposals[0].Was == "" || c.Proposals[1].Capsule != "backend/cache-lookup" {
		t.Fatalf("proposals = %+v", c.Proposals)
	}

	// The brief edited again (5) takes the place of the edit before it (1):
	// that one's nothing to confirm now.
	if r := testutil.RunCommand(t, w.Root, nil, "conversation", "accept", "-m", "cache", "t1", "1"); r.Err == nil || !strings.Contains(r.Err.Error(), "proposal 1 in t1 is superseded already") {
		t.Errorf("an edit to the brief superseded by a later one: %v", r.Err)
	}
	if r := testutil.Run(t, w, nil, "conversation", "accept", "-m", "cache", "t1", "5"); !strings.Contains(r.Stderr, "Wrote the new brief.") {
		t.Errorf("accepting the brief: %s", r.Stderr)
	}
	if brief, _ := store.ReadBrief("cache"); brief != "## Goal\nCache the lookup.\n## Stages\n1. Build it.\n" {
		t.Errorf("brief = %q", brief)
	}
	// Accepting a dispatch starts the crew, as you.
	t.Cleanup(cli.UseLocalDispatchHost(w.Root))
	t.Setenv(harness.EnvDefault, "shell")
	if r := testutil.Run(t, w, nil, "conversation", "accept", "-m", "cache", "t1", "2"); !strings.Contains(r.Stderr, "Started c2 in backend/cache-lookup.") {
		t.Errorf("accepting a dispatch: %s", r.Stderr)
	}
	if m, _ := store.Load("cache"); len(m.Crew) != 2 || m.Crew[1].Dispatch != "Benchmark the cache" {
		t.Errorf("crew: %+v", m.Crew)
	}
	if r := testutil.Run(t, w, nil, "conversation", "accept", "-m", "cache", "t1", "3"); !strings.Contains(r.Stderr, "Recorded d1 on cache") {
		t.Errorf("accepting a decision: %s", r.Stderr)
	}
	if r := testutil.Run(t, w, nil, "conversation", "accept", "-m", "cache", "t1", "4"); !strings.Contains(r.Stderr, "Saved as an idea: mission evict-on-logout") {
		t.Errorf("accepting a follow-up: %s", r.Stderr)
	}
	if m, _ := store.Load("evict-on-logout"); m == nil || m.From != "cache" || m.Conversation != "cache/t1" {
		t.Errorf("the follow-up: %+v", m)
	}

	t.Setenv(mission.EnvMission, "cache")
	if out := testutil.Run(t, w, nil, "show", "-f", "llm", "t1").Stdout; !strings.Contains(out, "Conversation t1 — \"Stage 2\" (about mission cache") || !strings.Contains(out, "d1") {
		t.Errorf("ws show t1:\n%s", out)
	}
	if out := testutil.Run(t, w, nil, "show", "-f", "llm").Stdout; !strings.Contains(out, "Conversations about it") || !strings.Contains(out, "t1 Stage 2") {
		t.Errorf("the mission's view:\n%s", out)
	}
	if out := testutil.Run(t, w, nil, "conversation", "list", "-m", "cache").Stdout; !strings.Contains(out, "t1   Stage 2") {
		t.Errorf("conversation list:\n%s", out)
	}
	if out := testutil.Run(t, w, nil, "thread", "list", "-m", "cache").Stdout; strings.Contains(out, "t1") {
		t.Errorf("a conversation listed as a thread:\n%s", out)
	}
	if out := testutil.Run(t, w, nil, "mission", "show", "evict-on-logout", "-f", "llm").Stdout; !strings.Contains(out, "Came out of conversation cache/t1 (ws show -m cache t1") {
		t.Errorf("the follow-up doesn't say where it came from:\n%s", out)
	}
}

// A mission waiting to be briefed is briefed in a conversation: its agent
// starts from the request (and the conversation it came out of), asks, and
// proposes the brief with its title and capsules. While that waits, the
// mission's in your court; confirming it names the mission, lifts its
// capsules and makes it ready.
func TestTalk_Briefing(t *testing.T) {
	w, store := testutil.OrchestrateFixture(t)
	harness.Register(testutil.FakeConverser{})
	local := filepath.Join(w.Root, "ws.local.toml")
	conf, _ := os.ReadFile(local)
	os.WriteFile(local, append(conf, []byte("\n[conversation]\nharness = \"fake-converse\"\n")...), 0o644)
	var lifted []string
	t.Cleanup(testutil.UseLifter(func(root, slug, repo, branch string) error {
		lifted = append(lifted, slug+":"+repo+"/"+branch)
		return nil
	}))
	m, _ := store.CreateBriefing("fe issue 45")
	store.Update(m.Slug, func(mm *mission.Mission) error { mm.Conversation = "w1"; return nil })
	talks := conversation.For(w.Root, m.Slug)
	talks.OpenBriefing("fe issue 45", time.Now())
	testutil.Run(t, w, nil, "conversation", "answer", "-m", m.Slug, "t1")
	c, _ := talks.Load("t1")
	if c.Goal != conversation.GoalBrief || c.Last().Body != "Which repo? (from w1)" {
		t.Fatalf("the briefing agent: %+v", c.Turns)
	}
	headline := func() string {
		md, _ := testutil.WebMission(w.Root)(m.Slug, false)
		h := md.Headlines[0]
		return string(h.Court) + "|" + h.Kind + "|" + h.Text + "|" + h.Action.Verb + "|" + h.Action.Target
	}
	if h := headline(); h != "you|brief-talk|The briefing agent is waiting for you, in t1|talk|t1" {
		t.Errorf("waiting on you: %s", h)
	}

	t.Setenv(conversation.EnvConversation, "t1")
	t.Setenv(mission.EnvCrew, conversation.Agent)
	t.Setenv(mission.EnvMission, m.Slug)
	testutil.WithStdin(t, "## Goal\nFix the flaky test.\n\n## Stages\n1. Fix it in `backend/flaky-test`.\n")
	testutil.Run(t, w, nil, "mission", "brief", "--propose", "--set", "-", "--title", "Fix the flaky test", "--capsule", "backend/flaky-test", "--dock", "backend#12", "--slug", "flaky-test")
	t.Setenv(conversation.EnvConversation, "")
	t.Setenv(mission.EnvCrew, "")
	t.Setenv(mission.EnvMission, "")
	if h := headline(); h != "you|brief-talk|The brief's proposed, in t1: confirm it, and the mission starts|talk|t1" {
		t.Errorf("a brief to confirm: %s", h)
	}

	var docked []string
	t.Cleanup(testutil.UseDocker(func(root, slug, repo, ref string) error {
		docked = append(docked, slug+":"+repo+" "+ref)
		return nil
	}))
	r := testutil.Run(t, w, nil, "conversation", "accept", "-m", m.Slug, "t1", "1")
	for _, want := range []string{"Wrote the new brief.", "Named it “Fix the flaky test”.", "Its slug is flaky-test now.", "Lifted backend/flaky-test, docked backend#12.", "It's ready for crew."} {
		if !strings.Contains(r.Stderr, want) {
			t.Errorf("accepting it said %q, lacking %q", r.Stderr, want)
		}
	}
	got, err := store.Load("flaky-test")
	if err != nil || got.ToBrief() || got.Title != "Fix the flaky test" || strings.Join(lifted, " ") != "flaky-test:backend/flaky-test" || strings.Join(docked, " ") != "flaky-test:backend 12" {
		t.Errorf("after: %+v %v, lifted %v, docked %v", got, err, lifted, docked)
	}
	// Its conversations moved with it.
	if c, err := conversation.For(w.Root, "flaky-test").Load("t1"); err != nil || c.Proposals[0].State != conversation.ProposalAccepted {
		t.Errorf("its briefing conversation: %v", err)
	}
	m.Slug = "flaky-test"
	if h := headline(); strings.Contains(h, "brief") {
		t.Errorf("still briefing: %s", h)
	}
}

// A crew's transcript is read from where its harness keeps the session,
// found by the conversation the crew recorded; a Cursor crew's can't be,
// and says so.
func TestCrewTranscript(t *testing.T) {
	w, store := testutil.OrchestrateFixture(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	data, _ := os.ReadFile(filepath.Join("..", "..", "harness", "testdata", "claude-session.jsonl"))
	os.MkdirAll(filepath.Join(home, ".claude", "projects", "-w-repos-backend-cache-lookup"), 0o755)
	os.WriteFile(filepath.Join(home, ".claude", "projects", "-w-repos-backend-cache-lookup", "sess-claude.jsonl"), data, 0o644)
	store.StartCrew("cache", "claude", "sess-claude")

	tr, err := testutil.WebCrewTranscript(w.Root)("cache", "c2")
	if err != nil || !tr.Available || tr.Harness != "claude" || len(tr.Entries) < 3 || tr.Entries[0].By != "user" {
		t.Fatalf("claude crew: %+v, %v", tr, err)
	}
	// c1 is Cursor's (orchestrateFixture).
	if tr, _ := testutil.WebCrewTranscript(w.Root)("cache", "c1"); tr.Available || !strings.Contains(tr.Why, "Cursor keeps its chats in a database") {
		t.Errorf("cursor crew: %+v", tr)
	}
	if _, err := testutil.WebCrewTranscript(w.Root)("cache", "c9"); err == nil {
		t.Error("a crew that isn't there")
	}
}
