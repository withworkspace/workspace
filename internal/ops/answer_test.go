package ops

import (
	"errors"
	"github.com/withworkspace/workspace/internal/store/storetest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/config"
	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/doc"
	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/page"
	"github.com/withworkspace/workspace/internal/prompt"
	"github.com/withworkspace/workspace/internal/workspace"
)

// writePage writes a page's text as the browser does: page id, started
// with it if it isn't there yet (as the next page).
func writePage(t *testing.T, w *Workspace, id string, text []byte) {
	t.Helper()
	was, _, err := w.pages().Read(id)
	if errors.Is(err, page.ErrNotFound) {
		if p, err := w.pages().New(text); err != nil || p.ID != id {
			t.Fatalf("started %s, not %s: %v", p.ID, id, err)
		}
		return
	}
	if _, err := w.pages().Write(id, doc.Version(was), text); err != nil {
		t.Fatal(err)
	}
}

// passFirst is what the agent of a pass (named) over c's page is told
// first, as a turn readies it.
func passFirst(t *testing.T, w *Workspace, c *conversation.Conversation, name string) string {
	t.Helper()
	c.Goal = name
	ps, ok := w.passOf(c)
	if !ok {
		t.Fatalf("there's no pass %s", name)
	}
	return prompt.PassFirst(w.here(doc.Scope{}, c), c, ps)
}

// A cell is written from the files its agent read: those of this run as
// they now are, with those of the version before that are still there. The
// page itself, what wasn't read, and what failed aren't among them.
func TestCellSources(t *testing.T) {
	root := t.TempDir()
	storetest.Own(t, root)
	write := func(rel, text string) string {
		path := filepath.Join(root, rel)
		os.MkdirAll(filepath.Dir(path), 0o755)
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	routes := write("repos/api/.ground/routes.go", "package api\n")
	auth := write("repos/api/.ground/auth.go", "package api\n")
	gone := write("repos/api/.ground/old.go", "package api\n")
	page := write(".ws/pages/p1.md", "# Page\n")

	first := cellSources(root, &conversation.Conversation{Kind: conversation.Cell}, []harness.ToolCall{
		{Tool: "Read", Input: page},
		{Tool: "Read", Input: routes},
		{Tool: "Read", Input: gone},
		{Tool: "Bash", Input: "git -C " + filepath.Dir(routes) + " ls-files"},
		{Tool: "Read", Input: filepath.Join(root, "repos/api/.ground/missing.go"), Error: true},
		{Tool: "Read", Input: "/etc/hosts"},
		{Tool: "Read", Input: routes},
	})
	if len(first) != 2 || first[0].Path != "repos/api/.ground/routes.go" || first[1].Path != "repos/api/.ground/old.go" {
		t.Fatalf("the first version's: %+v", first)
	}

	// Written again: routes.go has changed and is read again, auth.go is
	// read for the first time, old.go has gone.
	os.WriteFile(routes, []byte("package api\n\nfunc Routes() {}\n"), 0o644)
	os.Remove(gone)
	c := &conversation.Conversation{Kind: conversation.Cell, Turns: []conversation.Turn{
		{N: 1, By: conversation.You, Body: "routes"},
		{N: 2, By: conversation.Agent, At: time.Now(), Body: "…", Sources: first},
	}}
	if first[0].State(root) != conversation.SourceChanged || first[1].State(root) != conversation.SourceGone {
		t.Errorf("since: %q %q", first[0].State(root), first[1].State(root))
	}
	next := cellSources(root, c, []harness.ToolCall{{Tool: "read", Input: routes}, {Tool: "Read", Input: "repos/api/.ground/auth.go"}})
	if len(next) != 2 || next[0].Path != "repos/api/.ground/routes.go" || next[0].Hash == first[0].Hash || next[1].Path != "repos/api/.ground/auth.go" {
		t.Fatalf("the next version's: %+v", next)
	}
	if next[0].State(root) != conversation.SourceSame || next[1].State(root) != conversation.SourceSame {
		t.Errorf("read just now, they're as they were")
	}
	_ = auth
}

// A pass's agent is told what the pass looks for and how to reply; a cold
// one is given the page and no way to read anything else; run again, it's
// told what the user settled.
func TestPassPrompts(t *testing.T) {
	root := t.TempDir()
	storetest.Own(t, root)
	w := New(&config.Config{Pages: config.PagesConfig{Passes: map[string]config.PassConfig{
		"security": {Looks: "Find what could leak a secret.", Kinds: []string{"risk"}},
		"NOT OK":   {Looks: "x"},
		"empty":    {},
	}}}, &workspace.Workspace{Root: root}, nil, Options{})
	defs := w.Passes()
	if len(defs) != 4 || defs[3].Name != "security" || defs[3].Title != "Security" {
		t.Fatalf("the workspace's passes: %+v", defs)
	}
	text := []byte("# Greet by name\n\nIt should be fast.\n")
	writePage(t, w, "p1", text)
	c := &conversation.Conversation{ID: "w5", Kind: conversation.Pass, Goal: "check", Anchor: conversation.Anchor{Page: "p1"}, Turns: []conversation.Turn{{N: 1, By: conversation.You, Body: "Run it."}}}

	p := passFirst(t, w, c, "check")
	for _, want := range []string{"never written into it", ".ws/pages/p1.md", "## <its kind: question, ambiguity, missing>", "```suggest", "You can read anything here"} {
		if !strings.Contains(p, want) {
			t.Errorf("check's prompt lacks %q:\n%s", want, p)
		}
	}
	p = passFirst(t, w, c, "cold")
	if !strings.Contains(p, "It should be fast.") || strings.Contains(p, "You can read anything here") || strings.Contains(p, ".ws/pages/p1.md") || !strings.Contains(p, "assumption") {
		t.Errorf("cold's prompt:\n%s", p)
	}
	if p = passFirst(t, w, c, "security"); !strings.Contains(p, "Find what could leak a secret.") || !strings.Contains(p, "## <its kind: risk>") {
		t.Errorf("a workspace's own pass:\n%s", p)
	}

	// A pass whose instructions are a file: its agent is pointed at it, or,
	// cold, given it. A cell's agent is pointed at the workspace's style.
	guide := filepath.Join(root, "guides", "plain.md")
	os.MkdirAll(filepath.Dir(guide), 0o755)
	os.WriteFile(guide, []byte("# Plain words\n\nNo triads.\n"), 0o644)
	cfg := &config.Config{Pages: config.PagesConfig{Style: "guides/plain.md", Passes: map[string]config.PassConfig{
		"wording": {File: "guides/plain.md", Kinds: []string{"tell"}},
		"coldly":  {File: "guides/plain.md", Cold: true},
	}}}
	w = New(cfg, &workspace.Workspace{Root: root}, nil, Options{})
	if p = passFirst(t, w, c, "wording"); !strings.Contains(p, "in the file "+guide) || !strings.Contains(p, "Don't rewrite the page") || strings.Contains(p, "No triads.") {
		t.Errorf("a pass from a file:\n%s", p)
	}
	if p = passFirst(t, w, c, "coldly"); !strings.Contains(p, "No triads.") || strings.Contains(p, "in the file "+guide) {
		t.Errorf("a cold pass from a file:\n%s", p)
	}
	cell := &conversation.Conversation{ID: "w6", Kind: conversation.Cell, Anchor: conversation.Anchor{Page: "p1"}, Turns: []conversation.Turn{{N: 1, By: conversation.You, Body: "the routes"}}}
	if p = prompt.CellFirst(w.here(doc.Scope{}, cell), cell); !strings.Contains(p, "should read, in the file "+guide) {
		t.Errorf("a cell, with a style:\n%s", p)
	}
	cfg.Pages.Style = "guides/gone.md"
	if p = prompt.CellFirst(w.here(doc.Scope{}, cell), cell); strings.Contains(p, "This workspace says more about how its pages should read") {
		t.Errorf("a style that isn't there:\n%s", p)
	}
	if GuidePath("/ws", "/abs/x.md") != "/abs/x.md" || GuidePath("/ws", "") != "" || !strings.HasSuffix(GuidePath("/ws", "~/x.md"), "/x.md") || GuidePath("/ws", "~/x.md") == "/ws/~/x.md" {
		t.Errorf("where a guide is")
	}

	_, fs := conversation.ParseFindings("## question\n> It should be fast.\nHow fast?\n\n## missing\nWho reviews it.\n\n## ambiguity\n> by name\nWhose?\n")
	c.Turns = append(c.Turns, conversation.Turn{N: 2, By: conversation.Agent, Body: "Not ready.", Findings: fs})
	c.Settled = map[string]string{fs[0].Key: conversation.FindingDone, fs[1].Key: conversation.FindingDismissed}
	c.Goal = "check"
	ps, _ := w.passOf(c)
	p = prompt.PassNext(w.here(doc.Scope{}, c), c, ps)
	if !strings.Contains(p, "dealt with") || !strings.Contains(p, "- question: It should be fast.") || !strings.Contains(p, "dismissed these") || !strings.Contains(p, "- missing: Who reviews it.") || strings.Contains(p, "- ambiguity") {
		t.Errorf("run again:\n%s", p)
	}
}

// A pass that's one of the workspace's skills is found by the skill's
// name, and its agent is told to follow it for what's asked, reporting in
// ws's form; one whose skill isn't there says so.
func TestSkillPasses(t *testing.T) {
	root := t.TempDir()
	storetest.Own(t, root)
	skill := filepath.Join(root, ".agents", "skills", "build-proposals", "SKILL.md")
	os.MkdirAll(filepath.Dir(skill), 0o755)
	os.WriteFile(skill, []byte("---\nname: build-proposals\ndescription: BPs.\n---\n"), 0o644)
	w := New(&config.Config{Pages: config.PagesConfig{Passes: map[string]config.PassConfig{
		"share": {Title: "Ready to share?", Skill: "build-proposals", Args: "ready to share?"},
		"lint":  {Skill: "vale"},
	}}}, &workspace.Workspace{Root: root}, nil, Options{})

	defs := w.Passes()

	share, ok := page.PassNamed(defs, "share")
	if !ok || share.File != skill || share.Unavailable != "" || len(share.Kinds) != 0 {
		t.Fatalf("share: %+v", share)
	}
	if lint, _ := page.PassNamed(defs, "lint"); !strings.Contains(lint.Unavailable, "no skill named vale") {
		t.Errorf("a skill that isn't there: %+v", lint)
	}
	writePage(t, w, "p1", []byte("# A page\n"))
	c, _ := conversation.ForWorkspace(root).OpenPass("p1", "share", time.Now())
	p := passFirst(t, w, c, "share")
	for _, want := range []string{`uses the workspace's skill "build-proposals", asked: "ready to share?"`, "Its instructions are in " + skill, "replaced by the form below", "## <its kind: what it is as the skill would file it"} {
		if !strings.Contains(p, want) {
			t.Errorf("the prompt says nothing of %q:\n%s", want, p)
		}
	}
}

// fakeConverser answers every turn with the reply it's set to, printed by
// a process as a harness's would be, and keeps what it was told. during,
// if set, is done as it's told: what you do while the agent's answering.
type fakeConverser struct {
	reply   string
	prompts []string
	during  func()
}

func (f *fakeConverser) Name() string     { return "fake-answer" }
func (f *fakeConverser) Available() error { return nil }
func (f *fakeConverser) Start(harness.Brief) (harness.Launch, error) {
	return harness.Launch{}, harness.ErrNoResume
}
func (f *fakeConverser) Resume(string, string) (harness.Launch, error) {
	return harness.Launch{}, harness.ErrNoResume
}
func (f *fakeConverser) ConverseModel() string { return "" }
func (f *fakeConverser) Converse(r harness.ConverseRun) (harness.Launch, error) {
	f.prompts = append(f.prompts, r.Prompt)
	if f.during != nil {
		f.during()
	}
	return harness.Launch{Args: []string{"printf", "%s", f.reply}}, nil
}

var fake = &fakeConverser{}

func init() { harness.Register(fake) }

// One loop answers each kind of conversation: talk's reply has its Title:
// and Summary: taken off, which name it; a thread's is as it is; a cell's
// is what goes in the page, with its notes apart; a pass that's gone says
// so; an agent that fails, or none, is said in the conversation; and a
// resolved thread's agent drafts what was decided.
func TestAnswerConversation(t *testing.T) {
	w, _ := fixture(t, "")
	w.cur.cfg.Conversation.Harness = fake.Name()
	ws := conversation.ForWorkspace(w.Root())
	last := func(store conversation.Store, id string) (*conversation.Conversation, conversation.Turn) {
		t.Helper()
		c, err := store.Load(id)
		if err != nil {
			t.Fatal(err)
		}
		return c, c.Turns[len(c.Turns)-1]
	}

	talk, _ := ws.OpenTalk(conversation.You, "how do we cache?", time.Now())
	fake.reply, fake.prompts = "In memory.\n\nTitle: Caching\nSummary: Where to cache.", nil
	if err := w.AnswerConversation(doc.Scope{}, talk.ID); err != nil {
		t.Fatal(err)
	}
	c, turn := last(ws, talk.ID)
	if turn.By != conversation.Agent || turn.Body != "In memory." || c.Title != "Caching" || c.Summary != "Where to cache." || c.What != conversation.WhatTalk {
		t.Errorf("talk: %+v, titled %q, summed up %q", turn, c.Title, c.Summary)
	}
	if len(fake.prompts) != 1 || !strings.Contains(fake.prompts[0], "how do we cache?") || !strings.Contains(fake.prompts[0], "Title: <") {
		t.Errorf("talk's agent was told: %q", fake.prompts)
	}

	ms := conversation.For(w.Root(), "cache")
	thread, err := ms.OpenThread(conversation.NewThread{Kind: conversation.Ask, Anchor: conversation.Anchor{Doc: "s1/notes", Line: 1}, By: conversation.You, Body: "why?"}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	fake.reply = "Because.\n\nTitle: Not this"
	if err := w.AnswerConversation(doc.OfMission("cache"), thread.ID); err != nil {
		t.Fatal(err)
	}
	if c, turn = last(ms, thread.ID); turn.By != conversation.Agent || turn.Body != fake.reply || c.Title != "" {
		t.Errorf("a thread: %+v, titled %q", turn, c.Title)
	}
	ms.SetStatus(thread.ID, conversation.You, conversation.Resolved, time.Now())
	fake.reply = "What: Cache in memory.\nWhy: One process."
	if err := w.DraftDecision("cache", thread.ID); err != nil {
		t.Fatal(err)
	}
	if c, _ = last(ms, thread.ID); c.Draft == nil || c.Draft.What != "Cache in memory." || c.Draft.Why != "One process." {
		t.Errorf("drafted: %+v", c.Draft)
	}

	cell, _ := ws.OpenCell("p1", "a table", "", "", "", time.Now())
	fake.reply = "| a |\n\n<!-- notes -->\nUnchecked."
	if err := w.AnswerConversation(doc.Scope{}, cell.ID); err != nil {
		t.Fatal(err)
	}
	if _, turn = last(ws, cell.ID); turn.By != conversation.Agent || turn.Body != "| a |" || turn.Notes != "Unchecked." {
		t.Errorf("a cell: %+v", turn)
	}

	gone, _ := ws.OpenPass("p1", "nosuch", time.Now())
	if err := w.AnswerConversation(doc.Scope{}, gone.ID); err != nil {
		t.Fatal(err)
	}
	if _, turn = last(ws, gone.ID); turn.By != conversation.WS || !strings.Contains(turn.Body, "Pass nosuch no longer exists") {
		t.Errorf("a pass that's gone: %+v", turn)
	}

	failed, _ := ws.OpenTalk(conversation.You, "and?", time.Now())
	fake.reply = ""
	if err := w.AnswerConversation(doc.Scope{}, failed.ID); err == nil {
		t.Error("an agent that said nothing answered")
	}
	if _, turn = last(ws, failed.ID); turn.By != conversation.WS || turn.Body != "Agent could not answer: no reply" {
		t.Errorf("an agent that failed: %+v", turn)
	}

	w.cur.cfg.Conversation.Harness = "none-such"
	none, _ := ws.OpenTalk(conversation.You, "anyone?", time.Now())
	if err := w.AnswerConversation(doc.Scope{}, none.ID); err == nil {
		t.Error("answered with no agent")
	}
	if _, turn = last(ws, none.ID); turn.By != conversation.WS || !strings.HasPrefix(turn.Body, "No agent available: ") {
		t.Errorf("no agent: %+v", turn)
	}
}

// A run marked as answering is taken for stuck only after the longest a
// turn can be given, so a long turn isn't answered over while it runs.
func TestAnswerForOutlastsTurns(t *testing.T) {
	longest := config.ConversationConfig{TurnMinutes: config.MaxConversationTurnMinutes}.TurnFor()
	if conversation.AnswerFor <= longest || conversation.AnswerFor <= TurnFor {
		t.Errorf("answering counts as underway for %v, but a turn may take %v", conversation.AnswerFor, longest)
	}
}
