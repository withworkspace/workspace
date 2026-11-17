package web

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/doc"
	"github.com/withworkspace/workspace/internal/job"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/ops"
)

func talkWrite(t *testing.T, s *Server, path string, v any) (*http.Response, TalkDetail) {
	t.Helper()
	data, _ := json.Marshal(v)
	res := do(t, s, "POST", path, true, func(r *http.Request) {
		r.Body = io.NopCloser(bytes.NewReader(data))
		r.ContentLength = int64(len(data))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Origin", s.Info().Origin())
	})
	var d TalkDetail
	if res.StatusCode == 200 {
		json.NewDecoder(res.Body).Decode(&d)
	}
	return res, d
}

// You start a conversation and reply in it, and the agent's asked to
// answer each time; while it's answering, the page sees its calls so far.
// A resolved one is taken up again by replying.
func TestTalkAPI(t *testing.T) {
	s, _ := docServer(t)
	var asked []string
	answersInto(s, &asked)

	if res, _ := talkWrite(t, s, "/api/conversations", map[string]any{"body": " "}); res.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("nothing said: %d", res.StatusCode)
	}
	res, d := talkWrite(t, s, "/api/conversations", map[string]any{"body": "What would a token cache take?"})
	if res.StatusCode != 200 || d.Conversation == nil || d.ID != "w1" || len(d.Turns) != 1 {
		t.Fatalf("start: %d %+v", res.StatusCode, d)
	}
	store := conversation.ForWorkspace(s.Root)
	store.StartAnswering("w1", os.Getpid())
	os.WriteFile(store.RunPath("w1"), []byte(`{"type":"assistant","message":{"content":[{"type":"text","text":"Looking at the repos."},{"type":"tool_use","id":"u1","name":"Bash","input":{"command":"ws status -f llm"}}]}}`+"\n"), 0o644)
	var got TalkDetail
	json.NewDecoder(do(t, s, "GET", "/api/conversations/w1", true, nil).Body).Decode(&got)
	if got.Run == nil || len(got.Run.Calls) != 1 || got.Run.Calls[0].Input != "ws status -f llm" || got.Run.Said != "Looking at the repos." {
		t.Errorf("the run so far: %+v", got.Run)
	}
	store.EndAnswering("w1")
	store.Say("w1", conversation.Turn{By: conversation.Agent, At: time.Now(), Body: "About a day."})

	talks := Talks(s.Root)
	if len(talks) != 1 || !talks[0].Waiting || talks[0].Opening != "What would a token cache take?" {
		t.Errorf("listed: %+v", talks)
	}
	talkWrite(t, s, "/api/conversations/w1/status", map[string]any{"status": "resolved"})
	res, d = talkWrite(t, s, "/api/conversations/w1/say", map[string]any{"body": "And per tenant?"})
	if res.StatusCode != 200 || d.Status != conversation.Open || len(d.Turns) != 3 {
		t.Errorf("reply to a resolved one: %d %+v", res.StatusCode, d.Conversation)
	}
	if strings.Join(asked, ",") != "/w1,/w1" {
		t.Errorf("the agent was asked %v", asked)
	}
	if res := do(t, s, "GET", "/api/conversations/w9", true, nil); res.StatusCode != http.StatusNotFound {
		t.Errorf("no such conversation: %d", res.StatusCode)
	}
}

// Accepting or declining a proposal goes to the hook, with your edits,
// and the page gets the conversation back with what it did.
func TestTalkProposalAPI(t *testing.T) {
	s, _ := docServer(t)
	store := conversation.ForWorkspace(s.Root)
	store.OpenTalk(conversation.You, "A token cache?", time.Now())
	store.Propose("w1", conversation.Proposal{Kind: conversation.ProposeMission, Title: "Token cache", By: conversation.Agent, At: time.Now()})
	var got []string
	fake(s).settle = func(slug, id string, n int, accept bool, e ops.ProposalEdits) (string, string, error) {
		got = append(got, fmt.Sprintf("%s/%s/%d/%v/%s", slug, id, n, accept, e.Title))
		return "Created mission token-cache.", slug, nil
	}
	res, d := talkWrite(t, s, "/api/conversations/w1/proposals/1", map[string]any{"action": "accept", "title": "Token cache, per tenant"})
	if res.StatusCode != 200 || d.Message != "Created mission token-cache." || len(d.Proposals) != 1 {
		t.Errorf("accept: %d %+v", res.StatusCode, d)
	}
	talkWrite(t, s, "/api/conversations/w1/proposals/1", map[string]any{"action": "decline"})
	if strings.Join(got, " ") != "/w1/1/true/Token cache, per tenant /w1/1/false/" {
		t.Errorf("hook got %v", got)
	}
	for path, want := range map[string]int{"/api/conversations/w1/proposals/x": 404, "/api/conversations/w1/proposals/1?": 400} {
		action := "accept"
		if want == 400 {
			action = "maybe"
		}
		if res, _ := talkWrite(t, s, path, map[string]any{"action": action}); res.StatusCode != want {
			t.Errorf("%s %s: %d", path, action, res.StatusCode)
		}
	}
}

// A mission's conversations have the same routes under the mission; its
// threads aren't conversations to them, and a mission that isn't there
// has none.
func TestMissionTalkAPI(t *testing.T) {
	s, text := docServer(t)
	var asked []string
	answersInto(s, &asked, job.Answer)
	// A thread on its document takes t1.
	v := doc.Version([]byte(*text))
	threadWrite(t, s, "POST", "/api/missions/m/threads?ref=records%2Ftokens%3Aproposal.md", map[string]any{"version": v, "line": 3, "quote": map[string]string{"exact": "per user"}, "kind": "note", "body": "Hm."})

	res, d := talkWrite(t, s, "/api/missions/m/conversations", map[string]any{"body": "Is stage 2 right?"})
	if res.StatusCode != 200 || d.Conversation == nil || d.ID != "t2" || d.Kind != conversation.Talk {
		t.Fatalf("start: %d %+v", res.StatusCode, d)
	}
	talkWrite(t, s, "/api/missions/m/conversations/t2/say", map[string]any{"body": "And stage 3?"})
	// Each asks for an answer; one run at a time answers what's been said.
	if strings.Join(asked, ",") != "m/t2,m/t2" {
		t.Errorf("the agent was asked %v", asked)
	}
	if res := do(t, s, "GET", "/api/missions/m/conversations/t1", true, nil); res.StatusCode != http.StatusNotFound {
		t.Errorf("a thread as a conversation: %d", res.StatusCode)
	}
	if res, _ := talkWrite(t, s, "/api/missions/m/conversations/t1/say", map[string]any{"body": "x"}); res.StatusCode != http.StatusNotFound {
		t.Errorf("saying something in a thread as a conversation: %d", res.StatusCode)
	}
	if res, _ := talkWrite(t, s, "/api/missions/nope/conversations", map[string]any{"body": "x"}); res.StatusCode != http.StatusNotFound {
		t.Errorf("a mission that isn't there: %d", res.StatusCode)
	}
	if ts := TalksIn(conversation.For(s.Root, "m")); len(ts) != 1 || ts[0].ID != "t2" {
		t.Errorf("the mission's talks: %+v", ts)
	}
}

// Briefing may rename the mission: the conversation's read from where it
// went, and the page told where that is.
func TestTalkProposalAPI_Renamed(t *testing.T) {
	s, _ := docServer(t)
	store := conversation.For(s.Root, "m")
	store.OpenBriefing("fe issue 45", time.Now())
	store.Propose("t1", conversation.Proposal{Kind: conversation.ProposeBrief, Brief: "## Goal\nx", Slug: "fe-45", By: conversation.Agent, At: time.Now()})
	fake(s).settle = func(slug, id string, n int, accept bool, e ops.ProposalEdits) (string, string, error) {
		store.SettleProposal(id, n, conversation.ProposalAccepted, "brief", conversation.You, time.Now())
		return "Its slug is fe-45 now.", "fe-45", fake(s).Workspace.RenameMission(slug, "fe-45")
	}
	res, d := talkWrite(t, s, "/api/missions/m/conversations/t1/proposals/1", map[string]any{"action": "accept"})
	if res.StatusCode != 200 || d.Path != "/missions/fe-45/conversations/t1" || d.Proposals[0].State != conversation.ProposalAccepted {
		t.Errorf("accept: %d %+v", res.StatusCode, d)
	}
}

// A replan's button, through the API: accepting (as you edited it) asks
// the mission's planner in its own conversation, which the agent's
// conversation's proposal then names; declining asks the planner nothing,
// and wakes only the conversation's own agent, to take the decline in.
func TestTalkProposalAPI_Replan(t *testing.T) {
	s, _ := docServer(t)
	var asked []string
	answersInto(s, &asked)
	if err := (mission.Store{Root: s.Root}).WritePlan("m", "## Z\n- Done.\n\n## Stops early\n- None.\n\n## Steps\n1. Build it.\n   - who: crew, in `api/build`\n"); err != nil {
		t.Fatal(err)
	}
	store := conversation.For(s.Root, "m")
	c, _ := store.OpenTalk(conversation.You, "Can you replan?", time.Now())
	for _, text := range []string{"Add a step for the dropdown.", "Drop step 1."} {
		if _, err := store.Propose(c.ID, conversation.Proposal{Kind: conversation.ProposeReplan, Text: text, By: conversation.Agent, At: time.Now()}); err != nil {
			t.Fatal(err)
		}
	}
	path := "/api/missions/m/conversations/" + c.ID + "/proposals/"
	if res, d := talkWrite(t, s, path+"2", map[string]any{"action": "decline"}); res.StatusCode != 200 || d.Proposals[1].State != conversation.ProposalDeclined {
		t.Fatalf("decline: %d %+v", res.StatusCode, d.Proposals)
	}
	if store.Planner() != nil || strings.Join(asked, ",") != "m/"+c.ID {
		t.Fatalf("declining asked the planner, or woke another agent than %s's: %v", c.ID, asked)
	}
	res, d := talkWrite(t, s, path+"1", map[string]any{"action": "accept", "text": "Add a step for the AppBar's dropdown."})
	p := store.Planner()
	if res.StatusCode != 200 || p == nil || d.Proposals[0].State != conversation.ProposalAccepted || d.Proposals[0].Result != p.ID {
		t.Fatalf("accept: %d %+v, the planner %+v", res.StatusCode, d.Proposals, p)
	}
	// The planner's asked once; the conversation's own agent, still to
	// answer the decline here, is all else that's asked to answer.
	planner, others := 0, 0
	for _, a := range asked {
		switch a {
		case "m/" + p.ID:
			planner++
		case "m/" + c.ID:
		default:
			others++
		}
	}
	if p.Last().By != conversation.You || p.Last().Body != "Add a step for the AppBar's dropdown." || planner != 1 || others != 0 {
		t.Errorf("the planner was asked %+v; answered %v", p.Last(), asked)
	}
}
