package conversation

import (
	"errors"
	"fmt"
	"github.com/withworkspace/workspace/internal/doc"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/events"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/store"
	"github.com/withworkspace/workspace/internal/store/storetest"
)

// owned is a workspace whose records the test owns.
func owned(t *testing.T) string {
	root := t.TempDir()
	storetest.Own(t, root)
	return root
}

var at = time.Date(2026, 11, 9, 10, 0, 0, 0, time.UTC)

func threads(t *testing.T) Store {
	t.Helper()
	root := owned(t)
	if _, err := (mission.Store{Root: root}).Create("Cache"); err != nil {
		t.Fatal(err)
	}
	return For(root, "cache")
}

var anchor = Anchor{Doc: "records/tokens:proposal.md", Path: "proposal.md", Quote: &Quote{Exact: "per user"}, Line: 3, EndLine: 3, Version: "aaaaaaaaaaaa"}

func TestThreads(t *testing.T) {
	s := threads(t)
	for _, bad := range []NewThread{
		{Kind: "blocking", Anchor: anchor, By: You, Body: "x"},
		{Kind: Ask, Anchor: anchor, By: You, Body: "  "},
		{Kind: Ask, Anchor: Anchor{Doc: "x"}, By: You, Body: "x"},
	} {
		if _, err := s.OpenThread(bad, at); err == nil {
			t.Errorf("%+v: no error", bad)
		}
	}
	c, err := s.OpenThread(NewThread{Kind: Ask, Anchor: anchor, By: You, Body: "Why per user?"}, at)
	if err != nil {
		t.Fatal(err)
	}
	if c.ID != "t1" || c.Status != Open || len(c.Turns) != 1 || c.Turns[0].Version != "aaaaaaaaaaaa" || c.WaitingOnYou() {
		t.Fatalf("t1 = %+v", c)
	}
	s.OpenThread(NewThread{Kind: Change, Anchor: Anchor{Doc: "mission:plan.md", Path: "plan.md", Line: 1}, By: You, Body: "Shorter."}, at)

	c, err = s.Say("t1", Turn{By: "c2", At: at.Add(time.Minute), Body: "Users never share tokens.", Commit: "abc1234def"})
	if err != nil || len(c.Turns) != 2 || c.Turns[1].N != 2 || !c.WaitingOnYou() {
		t.Fatalf("crew's answer: %+v, %v", c, err)
	}
	if _, err := s.Say("t1", Turn{By: You, Body: ""}); err == nil {
		t.Error("an empty turn")
	}
	if _, err := s.SetStatus("t1", "c2", Resolved, at); !errors.Is(err, ErrNotYours) {
		t.Errorf("crew resolving: %v", err)
	}
	if c, _ = s.SetStatus("t1", You, Resolved, at.Add(2*time.Minute)); c.Status != Resolved || c.WaitingOnYou() {
		t.Errorf("resolved = %+v", c)
	}
	if _, err := s.Say("t1", Turn{By: "c2", Body: "One more thing."}); err == nil {
		t.Error("crew spoke on a resolved thread")
	}
	if c, _ = s.SetStatus("t1", You, Open, at.Add(3*time.Minute)); c.Status != Open {
		t.Errorf("reopened = %+v", c)
	}

	// It reads back the same, and per document.
	got, err := s.Load("t1")
	if err != nil || len(got.Turns) != 2 || got.Status != Open || got.StatusBy != You || got.Anchor.Quote.Exact != "per user" {
		t.Errorf("loaded = %+v, %v", got, err)
	}
	if all, _ := s.List(); len(all) != 2 || all[0].ID != "t1" || all[1].ID != "t2" {
		t.Errorf("list = %v", all)
	}
	if on, _ := s.OnDoc("records/tokens:proposal.md"); len(on) != 1 || on[0].ID != "t1" {
		t.Errorf("on the proposal = %v", on)
	}
	for _, bad := range []string{"t9", "x", "r1"} {
		if _, err := s.Load(bad); !errors.Is(err, ErrNotFound) {
			t.Errorf("%s: %v", bad, err)
		}
	}

	events, _ := s.Missions.ReadLog("cache", 0)
	var texts []string
	for _, e := range events {
		if e.Kind == mission.EventThread {
			texts = append(texts, e.Crew+"|"+e.Text)
		}
	}
	want := "|t1 asked on proposal.md: Why per user? ; |t2 asked for a change on plan.md: Shorter. ; c2|t1 c2 (abc1234): Users never share tokens. ; |t1 resolved ; |t1 reopened"
	if got := strings.Join(texts, " ; "); got != want {
		t.Errorf("log =\n%s\nwant\n%s", got, want)
	}
}

func TestVersions(t *testing.T) {
	s := threads(t)
	v, err := s.KeepVersion([]byte("# Plan\n"))
	if err != nil || len(v) != 12 || v != doc.Version([]byte("# Plan\n")) {
		t.Fatalf("version %q, %v", v, err)
	}
	if text, err := s.VersionText(v); err != nil || string(text) != "# Plan\n" {
		t.Errorf("text = %q, %v", text, err)
	}
	for _, bad := range []string{"../../etc/x", "bbbbbbbbbbbb"} {
		if _, err := s.VersionText(bad); !errors.Is(err, ErrNotFound) {
			t.Errorf("%q: %v", bad, err)
		}
	}
}

// A daemon restarted in place has its old pid: the marks it held as that
// pid end, in every scope, and another process's stay.
func TestEndAnsweringBy(t *testing.T) {
	s := threads(t)
	mine, _ := s.OpenThread(NewThread{Kind: Ask, Anchor: anchor, By: You, Body: "Why?"}, at)
	theirs, _ := s.OpenThread(NewThread{Kind: Ask, Anchor: anchor, By: You, Body: "How?"}, at)
	if ok, _ := s.StartAnswering(mine.ID, os.Getpid()); !ok {
		t.Fatal("couldn't mark")
	}
	if ok, _ := s.StartAnswering(theirs.ID, os.Getppid()); !ok {
		t.Fatal("couldn't mark")
	}
	if n, err := EndAnsweringBy(s.Missions.Root, os.Getpid()); err != nil || n != 1 {
		t.Fatalf("ended %d: %v", n, err)
	}
	if c, _ := s.Load(mine.ID); c.Answering {
		t.Error("this process's mark outlived its start")
	}
	if c, _ := s.Load(theirs.ID); !c.Answering {
		t.Error("another process's mark was ended")
	}
}

// Who answers next, a session to resume, and one answering run at a time.
func TestAnswering(t *testing.T) {
	s := threads(t)
	c, _ := s.OpenThread(NewThread{Kind: Ask, Anchor: anchor, By: You, Body: "Why?"}, at)
	if !c.NeedsAnswer() || c.WaitingOnYou() {
		t.Errorf("a new question: needs %v, waiting on you %v", c.NeedsAnswer(), c.WaitingOnYou())
	}
	n, _ := s.OpenThread(NewThread{Kind: Note, Anchor: anchor, By: You, Body: "Nice."}, at)
	if n.NeedsAnswer() {
		t.Error("a note on its own needs no answer")
	}
	if n, _ = s.Say(n.ID, Turn{By: You, At: at, Body: "Actually, why?"}); !n.NeedsAnswer() {
		t.Error("a reply in a note does")
	}

	// What the last run printed isn't this answer's: marked afresh, it goes.
	os.WriteFile(s.RunPath(c.ID), []byte("the last answer's steps\n"), 0o644)
	ok, err := s.StartAnswering(c.ID, os.Getpid())
	if err != nil || !ok {
		t.Fatalf("start: %v, %v", ok, err)
	}
	if _, err := os.Stat(s.RunPath(c.ID)); !os.IsNotExist(err) {
		t.Errorf("the last run's output outlived a new answer: %v", err)
	}
	os.WriteFile(s.RunPath(c.ID), []byte("this answer's steps\n"), 0o644)
	if ok, _ := s.StartAnswering(c.ID, os.Getppid()); ok {
		t.Error("two runs at once")
	}
	if ok, _ := s.StartAnswering(c.ID, os.Getpid()); !ok {
		t.Error("the run it was marked for couldn't start")
	}
	if _, err := os.Stat(s.RunPath(c.ID)); err != nil {
		t.Errorf("the run it was marked for lost what it's printed: %v", err)
	}
	c, _ = s.Say(c.ID, Turn{By: Agent, At: at, Body: "Because."})
	if c, _ = s.Load(c.ID); !c.Answering || c.WaitingOnYou() {
		t.Errorf("while answering: %+v", c)
	}
	s.EndAnswering(c.ID)
	if c, _ = s.Load(c.ID); c.Answering || !c.WaitingOnYou() || c.NeedsAnswer() {
		t.Errorf("answered: %+v", c)
	}
	// A run that died leaves its mark, which doesn't count.
	s.write(func(tx *store.Tx) error { return s.setAnswering(tx, c.ID, 999999) })
	if ok, _ := s.StartAnswering(c.ID, os.Getpid()); !ok {
		t.Error("a dead run's mark stopped a new one")
	}
	s.EndAnswering(c.ID)

	if err := s.SetSession(c.ID, Session{Harness: "claude", ID: "abc"}); err != nil {
		t.Fatal(err)
	}
	if c, _ = s.Load(c.ID); c.Session == nil || c.Session.ID != "abc" || len(c.Turns) != 2 {
		t.Errorf("session = %+v", c)
	}

	// A model chosen for its turns, then the role's again.
	if err := s.SetModel(c.ID, Model{Harness: "claude", Model: "opus", By: You, At: at}); err != nil {
		t.Fatal(err)
	}
	if c, _ = s.Load(c.ID); c.Model == nil || c.Model.Model != "opus" || c.Model.Harness != "claude" || c.Session == nil {
		t.Errorf("model = %+v", c.Model)
	}
	s.SetModel(c.ID, Model{Harness: "claude", By: You, At: at})
	if c, _ = s.Load(c.ID); c.Model != nil {
		t.Errorf("model after going back = %+v", c.Model)
	}
}

// A suggestion replaces text that's in the document once; a newer one
// supersedes it; only you accept or decline; your own is yours to accept,
// not the agent's to answer.
func TestSuggestions(t *testing.T) {
	s := threads(t)
	docText := []byte("# Plan\n\nShip on Friday.\nShip it well.\n")
	c, _ := s.OpenThread(NewThread{Kind: Change, Anchor: anchor, By: You, Body: "Not Friday."}, at)
	for _, bad := range []Suggestion{{From: "", To: "x"}, {From: "Ship", To: "Send"}, {From: "Monday", To: "Tuesday"}, {From: "Friday", To: "Friday"}} {
		bad.By = Agent
		if _, err := s.Suggest(c.ID, bad, docText); err == nil {
			t.Errorf("%+v: no error", bad)
		}
	}
	c, err := s.Suggest(c.ID, Suggestion{From: "on Friday", To: "on Monday", By: Agent, At: at}, docText)
	if err != nil || c.Pending() == nil || c.Pending().N != 1 || c.Pending().Version != doc.Version(docText) {
		t.Fatalf("suggest: %+v, %v", c, err)
	}
	c, _ = s.Suggest(c.ID, Suggestion{From: "on Friday", To: "on Tuesday", By: Agent, At: at}, docText)
	if c, _ = s.Load(c.ID); len(c.Suggestions) != 2 || c.Suggestions[0].State != SuggestSuperseded || c.Pending().To != "on Tuesday" {
		t.Errorf("superseded: %+v", c.Suggestions)
	}
	if _, err := s.SettleSuggestion(c.ID, Agent, SuggestAccepted, "", at); err == nil {
		t.Error("the agent accepted its own suggestion")
	}
	c, err = s.SettleSuggestion(c.ID, You, SuggestAccepted, "abc1234def", at)
	if err != nil || c.Pending() != nil || c.Suggestions[1].State != SuggestAccepted || c.Suggestions[1].Commit != "abc1234def" {
		t.Errorf("accepted: %+v, %v", c.Suggestions, err)
	}
	if _, err := s.SettleSuggestion(c.ID, You, SuggestDeclined, "", at); !errors.Is(err, ErrNotFound) {
		t.Errorf("nothing waiting: %v", err)
	}

	// Your own suggestion, as you open a thread: yours to accept.
	y, _ := s.OpenThread(NewThread{Kind: Change, Anchor: anchor, By: You, Body: "Clearer."}, at)
	y, _ = s.Suggest(y.ID, Suggestion{From: "Ship it well.", To: "Ship it carefully.", By: You, At: at}, docText)
	if y.NeedsAnswer() {
		t.Error("the agent would answer your own suggestion")
	}
	y, _ = s.Say(y.ID, Turn{By: You, At: at.Add(time.Minute), Body: "Or is there better?"})
	if !y.NeedsAnswer() {
		t.Error("asking about it after does need an answer")
	}
}

// The workspace's conversations are w1, w2, … in .ws/conversations, apart
// from any mission's; a talk needs no document, and keeps the agent's
// latest title and summary.
func TestWorkspaceTalks(t *testing.T) {
	root := owned(t)
	ws := ForWorkspace(root)
	if _, err := ws.OpenTalk(You, " ", at); err == nil {
		t.Error("opened a talk with nothing said")
	}
	c, err := ws.OpenTalk(You, "What would a token cache take?", at)
	if err != nil || c.ID != "w1" || c.Kind != Talk || !c.NeedsAnswer() {
		t.Fatalf("w1 = %+v, %v", c, err)
	}
	c2, _ := ws.OpenTalk(You, "Why is billing slow?", at)
	if c2.ID != "w2" {
		t.Errorf("second is %s", c2.ID)
	}
	if ids := ws.IDs(); len(ids) != 2 || ids[0] != "w1" {
		t.Errorf("ids = %v", ids)
	}
	if _, err := ws.Load("t1"); !errors.Is(err, ErrNotFound) {
		t.Errorf("a mission's ref in the workspace: %v", err)
	}
	ws.Say("w1", Turn{By: Agent, At: at, Body: "About a day.", Calls: []Call{{Tool: "Bash", Input: "ws status -f llm"}}})
	ws.SetSummary("w1", "Token cache", "What a cache would take.", at)
	ws.SetSummary("w1", "", "A day's work, per tenant.", at)
	c, _ = ws.Load("w1")
	if c.Title != "Token cache" || c.Summary != "A day's work, per tenant." || !c.WaitingOnYou() || c.Turns[1].Calls[0].Input != "ws status -f llm" {
		t.Errorf("after the agent's turn: %+v", c)
	}
	if list, _ := ws.List(); len(list) != 2 {
		t.Errorf("list = %d", len(list))
	}
	if ws.Updated("w1").IsZero() {
		t.Error("no updated time")
	}
}

// An agent proposes in an open conversation; you accept or decline each
// once. A decision kept with a workspace conversation moves to the mission
// it becomes, settled again with where.
func TestProposals(t *testing.T) {
	ws := ForWorkspace(owned(t))
	ws.OpenTalk(You, "A token cache?", at)
	if _, err := ws.Propose("w1", Proposal{Kind: ProposeDecision, What: " ", By: Agent, At: at}); err == nil {
		t.Error("an empty decision")
	}
	if _, err := ws.Propose("w1", Proposal{Kind: "dispatch", Title: "x", By: Agent, At: at}); err == nil {
		t.Error("a kind there isn't")
	}
	ws.Propose("w1", Proposal{Kind: ProposeDecision, What: "Key per tenant.", Why: "Tokens are.", By: Agent, At: at})
	c, _ := ws.Propose("w1", Proposal{Kind: ProposeMission, Title: "Token cache", Brief: "## Goal\nCache.", By: Agent, At: at})
	if len(c.Proposals) != 2 || c.Proposals[1].N != 2 || c.Proposals[1].State != ProposalOpen {
		t.Fatalf("proposals = %+v", c.Proposals)
	}
	if _, err := ws.SettleProposal("w1", 1, ProposalAccepted, "", Agent, at); err == nil {
		t.Error("the agent accepted its own proposal")
	}
	ws.SettleProposal("w1", 1, ProposalAccepted, "", You, at)
	c, _ = ws.Load("w1")
	if kept := c.KeptDecisions(); len(kept) != 1 || kept[0].What != "Key per tenant." || c.Became() != "" {
		t.Errorf("kept = %+v, became %q", kept, c.Became())
	}
	ws.SettleProposal("w1", 2, ProposalAccepted, "token-cache", You, at)
	ws.SettleProposal("w1", 1, ProposalAccepted, "token-cache:d1", You, at) // moved
	if _, err := ws.SettleProposal("w1", 2, ProposalDeclined, "", You, at); err == nil {
		t.Error("settled twice")
	}
	c, _ = ws.Load("w1")
	if c.Became() != "token-cache" || len(c.KeptDecisions()) != 0 || c.Proposal(1).Result != "token-cache:d1" {
		t.Errorf("after: %+v", c.Proposals)
	}
}

// A quiet turn of yours (a card settled that needs nothing of the agent)
// doesn't change whose turn it is: what the agent said is still waiting
// on you, and it isn't asked to answer; what you'd said before it is
// still to answer; and something you say after it is answered, as is a
// turn of yours that isn't quiet.
func TestQuietTurns(t *testing.T) {
	s := threads(t)
	c, _ := s.OpenTalk(You, "how do we cache tokens?", at)
	c, _ = s.Say(c.ID, Turn{By: Agent, At: at, Body: "In memory. I've proposed a decision."})
	quiet := Turn{By: You, At: at, Body: "Accepted proposal 1, the decision “In memory”: recorded as d1 on mission cache.", Origin: ProposalOrigin(1), Quiet: true}
	if c, _ = s.Say(c.ID, quiet); c.NeedsAnswer() || !c.WaitingOnYou() {
		t.Errorf("after a quiet turn: needs %v, waiting on you %v", c.NeedsAnswer(), c.WaitingOnYou())
	}
	if c, _ = s.Say(c.ID, Turn{By: You, At: at, Body: "And Redis?"}); !c.NeedsAnswer() || c.WaitingOnYou() {
		t.Errorf("said after it: needs %v, waiting on you %v", c.NeedsAnswer(), c.WaitingOnYou())
	}
	quiet.Origin = ProposalOrigin(2)
	if c, _ = s.Say(c.ID, quiet); !c.NeedsAnswer() {
		t.Error("a quiet turn after something unanswered: it's not to answer any more")
	}
	c, _ = s.Say(c.ID, Turn{By: Agent, At: at, Body: "Redis too, later."})
	if c, _ = s.Say(c.ID, Turn{By: You, At: at, Body: "Declined proposal 3, the mission “Redis”.", Origin: ProposalOrigin(3)}); !c.NeedsAnswer() {
		t.Error("a turn of ws's that isn't quiet isn't answered")
	}
	if n := c.Last().OriginProposal(); n != 3 {
		t.Errorf("its proposal: %d", n)
	}
	if n := c.Turns[0].OriginProposal(); n != 0 {
		t.Errorf("one you typed is from proposal %d", n)
	}
	// One that failed waits for you to say more: a quiet turn isn't more.
	c, _ = s.Say(c.ID, Turn{By: WS, At: at, Body: "The agent couldn't answer: no agent."})
	if c, _ = s.Say(c.ID, quiet); c.NeedsAnswer() {
		t.Error("a quiet turn after a failure is answered")
	}
}

// Settling a card wakes the agent on every decline, and on accepting a
// brief or a plan; accepting anything else is quiet.
func TestWakes(t *testing.T) {
	for _, kind := range []string{ProposeMission, ProposeDecision, ProposeDocument, ProposeBrief, ProposePlan, ProposeDispatch, ProposeComment, ProposeArtifact} {
		if !Wakes(kind, ProposalDeclined) {
			t.Errorf("declining a %s doesn't wake it", kind)
		}
		want := kind == ProposeBrief || kind == ProposePlan
		if got := Wakes(kind, ProposalAccepted); got != want {
			t.Errorf("accepting a %s wakes it: %v, want %v", kind, got, want)
		}
	}
}

// What's said while the agent answers is written before its reply, which
// says how many turns it was written from (Seen): those after are Missed,
// told it next. One that isn't quiet has it answer again, and isn't
// waiting on you; a quiet one waits for what you say next. A reply that
// doesn't say what it saw missed nothing.
func TestMissed(t *testing.T) {
	s := threads(t)
	c, _ := s.OpenTalk(You, "how?", at)
	s.Say(c.ID, Turn{By: You, At: at, Body: "Accepted proposal 1, …", Origin: ProposalOrigin(1), Quiet: true})
	c, _ = s.Say(c.ID, Turn{By: Agent, At: at, Body: "Like this.", Seen: 1})
	if m := c.Missed(); len(m) != 1 || m[0].Origin != "proposal:1" || c.NeedsAnswer() || !c.WaitingOnYou() {
		t.Errorf("a quiet one missed: %+v, needs %v, waiting on you %v", m, c.NeedsAnswer(), c.WaitingOnYou())
	}
	s.Say(c.ID, Turn{By: You, At: at, Body: "and?"})
	s.Say(c.ID, Turn{By: You, At: at, Body: "Declined proposal 2, …", Origin: ProposalOrigin(2)})
	c, _ = s.Say(c.ID, Turn{By: Agent, At: at, Body: "So.", Seen: 4})
	if m := c.Missed(); len(m) != 1 || m[0].Origin != "proposal:2" || !c.NeedsAnswer() || c.WaitingOnYou() {
		t.Errorf("a waking one missed: %+v, needs %v, waiting on you %v", m, c.NeedsAnswer(), c.WaitingOnYou())
	}
	if c, _ = s.Say(c.ID, Turn{By: Agent, At: at, Body: "Noted.", Seen: 6}); len(c.Missed()) != 0 || c.NeedsAnswer() {
		t.Errorf("answered: missed %+v", c.Missed())
	}
	if c, _ = s.Say(c.ID, Turn{By: Agent, At: at, Body: "Also."}); len(c.Missed()) != 0 {
		t.Errorf("a reply that doesn't say what it saw: missed %+v", c.Missed())
	}
}

// An artifact is proposed with its HTML and a note, and a title unless
// it's a version of one; it's kept whole in the conversation, and
// accepting it records the version it made.
func TestProposals_Artifact(t *testing.T) {
	ws := ForWorkspace(owned(t))
	ws.OpenTalk(You, "Show me the glow", at)
	html := "<!doctype html><title>Glow</title>"
	for want, p := range map[string]Proposal{
		"nothing in it":  {Title: "Glow", Note: "x"},
		"has a title":    {HTML: html, Note: "x"},
		"has a note":     {HTML: html, Title: "Glow", Note: " "},
		"has a note too": {HTML: html, To: "a1", On: 1},
	} {
		p.Kind, p.By, p.At = ProposeArtifact, Agent, at
		if _, err := ws.Propose("w1", p); err == nil {
			t.Errorf("proposed one that %s", want)
		}
	}
	ws.Propose("w1", Proposal{Kind: ProposeArtifact, Title: "Glow", Note: " A first. ", HTML: html, By: Agent, At: at})
	c, err := ws.Propose("w1", Proposal{Kind: ProposeArtifact, To: " a1 ", On: 1, Latest: 1, Note: "Softer.", HTML: html, By: Agent, At: at})
	if err != nil {
		t.Fatal(err)
	}
	if p := c.Proposals[1]; p.N != 2 || p.To != "a1" || p.Note != "Softer." || p.HTML != html || p.State != ProposalOpen {
		t.Errorf("a version proposed: %+v", p)
	}
	ws.SettleProposal("w1", 1, ProposalAccepted, "a1.v1", You, at)
	c, _ = ws.Load("w1")
	if p := c.Proposal(1); p.Result != "a1.v1" || p.Note != "A first." || p.HTML != html {
		t.Errorf("accepted: %+v", p)
	}
}

// What the agent's latest reply proposed, all settled (a mission made, a
// decision kept), leaves nothing waiting on you: what it asked of you is
// done. One still open, or a reply that proposed nothing, is your turn.
func TestWaitingOnYou_AfterItsProposalsAreSettled(t *testing.T) {
	ws := ForWorkspace(owned(t))
	ws.OpenTalk(You, "A token cache?", at)
	ws.Propose("w1", Proposal{Kind: ProposeDecision, What: "Key per tenant.", By: Agent, At: at.Add(time.Minute)})
	ws.Propose("w1", Proposal{Kind: ProposeMission, Title: "Token cache", Brief: "## Goal\nCache.", By: Agent, At: at.Add(time.Minute)})
	c, _ := ws.Say("w1", Turn{By: Agent, Body: "I've proposed both.", At: at.Add(2 * time.Minute)})
	if !c.WaitingOnYou() {
		t.Error("its proposals, open: not your turn")
	}
	ws.SettleProposal("w1", 1, ProposalAccepted, "", You, at.Add(3*time.Minute))
	if c, _ = ws.Load("w1"); !c.WaitingOnYou() {
		t.Error("one still open: not your turn")
	}
	ws.SettleProposal("w1", 2, ProposalAccepted, "token-cache", You, at.Add(4*time.Minute))
	if c, _ = ws.Load("w1"); c.WaitingOnYou() {
		t.Error("all settled: still your turn")
	}
	ws.Say("w1", Turn{By: You, Body: "And how long do they live?", At: at.Add(5 * time.Minute)})
	if c, _ = ws.Say("w1", Turn{By: Agent, Body: "An hour.", At: at.Add(6 * time.Minute)}); !c.WaitingOnYou() {
		t.Error("a reply that proposed nothing: not your turn")
	}
}

// A proposal drafted again takes the place of the one before, which is
// nothing to confirm now: the brief again (with its plan, the plan too),
// the plan again, a mission or a document of the same title. Others stay.
func TestProposals_Superseded(t *testing.T) {
	s := threads(t)
	c, _ := s.OpenTalk(You, "brief it", at)
	s.Propose(c.ID, Proposal{Kind: ProposeBrief, Brief: "## Goal\nA.\n", By: Agent, At: at})
	s.Propose(c.ID, Proposal{Kind: ProposePlan, Plan: "## Z\n- A.\n", By: Agent, At: at})
	s.Propose(c.ID, Proposal{Kind: ProposeDecision, What: "Keep it small.", By: Agent, At: at})
	c, _ = s.Propose(c.ID, Proposal{Kind: ProposeBrief, Brief: "## Goal\nB.\n", Plan: "## Z\n- B.\n", By: Agent, At: at})
	states := func(c *Conversation) string {
		var out []string
		for _, p := range c.Proposals {
			out = append(out, fmt.Sprintf("%d %s %s", p.N, p.State, p.Result))
		}
		return strings.Join(out, ", ")
	}
	if got := states(c); got != "1 superseded 4, 2 superseded 4, 3 open , 4 open " {
		t.Errorf("after the brief again, with its plan: %s", got)
	}

	ws := ForWorkspace(owned(t))
	ws.OpenTalk(You, "A token cache?", at)
	ws.Propose("w1", Proposal{Kind: ProposeMission, Title: "Token cache", Brief: "## Goal\nCache.", By: Agent, At: at})
	ws.Propose("w1", Proposal{Kind: ProposeMission, Title: "Session cache", By: Agent, At: at})
	c, _ = ws.Propose("w1", Proposal{Kind: ProposeMission, Title: " token cache ", Brief: "## Goal\nCache, per tenant.", By: Agent, At: at})
	if got := states(c); got != "1 superseded 3, 2 open , 3 open " {
		t.Errorf("a mission of the same title again: %s", got)
	}
	if _, err := ws.SettleProposal("w1", 1, ProposalAccepted, "token-cache", You, at); err == nil {
		t.Error("accepted one that was superseded")
	}

	// An edit to a document drafted again takes the place of the one
	// before to that document; a new document of its title doesn't, nor
	// does it take a new one's place.
	ws.OpenTalk(You, "It's missing why", at)
	doc := func(to, title string) Proposal {
		return Proposal{Kind: ProposeDocument, DocKind: "adr", To: to, Title: title, Was: "# One binary\n", Text: "# One binary\n\nWhy: " + title, By: Agent, At: at}
	}
	ws.Propose("w2", doc("", "One binary"))
	ws.Propose("w2", doc("records:adr/0007-one-binary.md", "One binary"))
	ws.Propose("w2", doc("records:adr/0008-two.md", "Two"))
	c, _ = ws.Propose("w2", doc("records:adr/0007-one-binary.md", "One binary"))
	if got := states(c); got != "1 open , 2 superseded 4, 3 open , 4 open " {
		t.Errorf("an edit to the same document again: %s", got)
	}
	if _, err := ws.Propose("w2", Proposal{Kind: ProposeDocument, DocKind: "adr", To: "records:adr/0007-one-binary.md", Was: "# One binary\n", Text: "# One binary\n", By: Agent, At: at}); err == nil || !strings.Contains(err.Error(), "the document as it is") {
		t.Errorf("an edit that changes nothing: %v", err)
	}
}

// What's said in a conversation by its agent, and what's proposed and
// settled in it, are the workspace's events, once each.
func TestConversationEvents(t *testing.T) {
	root := owned(t)
	ws := ForWorkspace(root)
	ws.OpenTalk(You, "A token cache?", at)
	ws.Say("w1", Turn{By: Agent, Body: "Yes: per tenant.\nFor an hour.", At: at})
	ws.Say("w1", Turn{By: You, Body: "Agreed.", At: at})
	ws.Propose("w1", Proposal{Kind: ProposeDecision, What: "Key per tenant.", By: Agent, At: at})
	ws.SettleProposal("w1", 1, ProposalAccepted, "", You, at)
	ws.SettleProposal("w1", 1, ProposalAccepted, "token-cache:d1", You, at) // moved to its mission
	got, err := events.List(root, events.Filter{Kinds: []string{events.ConversationAnswered, events.ProposalMade, events.ProposalAccepted, events.ProposalDeclined}})
	if err != nil {
		t.Fatal(err)
	}
	var said []string
	for _, e := range got {
		said = append(said, fmt.Sprintf("%s %s %s %d %s", e.Kind, e.By, e.Conversation, e.Proposal, e.ProposalKind))
	}
	want := []string{
		"conversation.answered ws w1 0 ",
		"proposal.made ws w1 1 decision",
		"proposal.accepted you w1 1 decision",
	}
	if strings.Join(said, "\n") != strings.Join(want, "\n") {
		t.Errorf("events:\n%s\nwant:\n%s", strings.Join(said, "\n"), strings.Join(want, "\n"))
	}
	if got[0].Text != "Yes: per tenant. For an hour." {
		t.Errorf("answered: %q", got[0].Text)
	}
}

// A conversation an automation started says so in its events: the
// automations that led to it, so what they set off can't loop.
func TestConversationEvents_Chained(t *testing.T) {
	root := owned(t)
	ws := ForWorkspace(root)
	c, err := ws.OpenTalkChained("automation:triage", []string{"ci-red", "triage"}, Turn{By: You, Body: "Why did CI fail?", At: at})
	if err != nil {
		t.Fatal(err)
	}
	if c, _ := ws.Load(c.ID); strings.Join(c.Chain, ",") != "ci-red,triage" {
		t.Errorf("kept chain %q", c.Chain)
	}
	ws.Say(c.ID, Turn{By: Agent, Body: "A flaky test.", At: at})
	got, _ := events.List(root, events.Filter{Kinds: []string{events.ConversationAnswered}})
	if len(got) != 1 || strings.Join(got[0].Chain, ",") != "ci-red,triage" {
		t.Errorf("events %+v", got)
	}
}
