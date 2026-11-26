package ops

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/withworkspace/plugin/protocol"
	"github.com/withworkspace/workspace/internal/acp"
	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/job"
	"github.com/withworkspace/workspace/internal/store"
)

// ask is AnswerPlugin, its answer as a plugin reads it: as JSON, into the
// protocol's types.
func ask[T any](t *testing.T, w *Workspace, method string, params any) (T, error) {
	t.Helper()
	var out T
	data, err := json.Marshal(params)
	if err != nil {
		t.Fatal(err)
	}
	res, err := w.AnswerPlugin(context.Background(), "jira", method, data)
	if err != nil {
		return out, err
	}
	data, err = json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	return out, nil
}

func rpcCode(err error) int {
	if e, ok := errors.AsType[*acp.RPCError](err); ok {
		return e.Code
	}
	return 0
}

func TestPluginAsksForMissions(t *testing.T) {
	w, _ := fixture(t, "## Goal\nCache it.\n", "cache-a")
	m, err := w.Missions.Create("Old")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Archive(m.Slug, false, AsYou); err != nil {
		t.Fatal(err)
	}

	got, err := ask[protocol.MissionListResult](t, w, protocol.MethodMissionList, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Missions) != 1 {
		t.Fatalf("in play: %+v", got.Missions)
	}
	c := got.Missions[0]
	if c.Slug != "cache" || c.Title != "Cache" || c.Path != "/missions/cache" || c.Created.IsZero() || c.Updated.IsZero() {
		t.Errorf("mission = %+v", c)
	}
	if c.State == "" || c.Headline == nil || c.Headline.Court == "" || c.Headline.Text == "" {
		t.Errorf("where it stands: state %q, headline %+v", c.State, c.Headline)
	}
	if len(c.Capsules) != 1 || c.Capsules[0].Repo != "frontend" || c.Capsules[0].Name != "cache-a" || c.Capsules[0].Path == "" || c.Capsules[0].PullRequest != nil {
		t.Errorf("capsules = %+v", c.Capsules)
	}

	all, err := ask[protocol.MissionListResult](t, w, protocol.MethodMissionList, protocol.MissionListParams{Archived: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(all.Missions) != 2 || all.Missions[1].Slug != m.Slug || all.Missions[1].State != protocol.MissionArchived || all.Missions[1].Headline != nil {
		t.Errorf("archived too: %+v", all.Missions)
	}
}

func TestPluginAsksForAMission(t *testing.T) {
	w, _ := fixture(t, "## Goal\nCache it.\n", "cache-a")
	d, err := ask[protocol.MissionDetail](t, w, protocol.MethodMissionShow, protocol.MissionShowParams{Slug: "cache"})
	if err != nil {
		t.Fatal(err)
	}
	if d.Slug != "cache" || d.Brief != "## Goal\nCache it.\n" || len(d.Capsules) != 1 || d.Headline == nil {
		t.Errorf("mission = %+v", d)
	}

	_, err = ask[protocol.MissionDetail](t, w, protocol.MethodMissionShow, protocol.MissionShowParams{Slug: "nope"})
	if rpcCode(err) != protocol.CodeNotFound {
		t.Errorf("no such mission: %v", err)
	}
	_, err = ask[protocol.MissionDetail](t, w, protocol.MethodMissionShow, nil)
	if rpcCode(err) != protocol.CodeInvalidParams {
		t.Errorf("no slug: %v", err)
	}
}

func TestPluginAsksForRepos(t *testing.T) {
	w, _ := fixture(t, "", "cache-a")
	ground := filepath.Join(w.WS().RepoDir("frontend"), ".ground")
	for _, dir := range []string{ground, filepath.Join(w.WS().RepoDir("frontend"), "loose")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	got, err := ask[protocol.RepoListResult](t, w, protocol.MethodRepoList, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Repos) != 1 {
		t.Fatalf("repos = %+v", got.Repos)
	}
	r := got.Repos[0]
	if r.Name != "frontend" || r.Title != "frontend" || !r.Cloned || r.Path != w.WS().RepoDir("frontend") {
		t.Errorf("repo = %+v", r)
	}
	byName := map[string]protocol.Capsule{}
	for _, c := range r.Capsules {
		byName[c.Name] = c
	}
	if len(r.Capsules) != 3 || r.Capsules[0].Name != ".ground" || !r.Capsules[0].Ground || r.Capsules[0].Path != ground {
		t.Errorf("capsules = %+v", r.Capsules)
	}
	if byName["cache-a"].Mission != "cache" || byName["loose"].Mission != "" {
		t.Errorf("missions of capsules = %+v", r.Capsules)
	}
}

func TestPluginAsksWhatWSHasNot(t *testing.T) {
	w, _ := fixture(t, "")
	if _, err := w.AnswerPlugin(context.Background(), "jira", "link/put", nil); rpcCode(err) != acp.CodeMethodNotFound {
		t.Errorf("a method ws hasn't: %v", err)
	}
}

// A plugin has ws start a conversation for the person: it's theirs, it
// says which plugin it came from, and its agent answers. A mission its
// agent then proposes as about something is about it once accepted.
func TestPluginStartsAConversation(t *testing.T) {
	w, _ := fixture(t, "")
	var answers []string
	w.opts.Jobs = JobsOf(func(j job.Job) { answers = append(answers, j.Kind+" "+j.Mission+"/"+j.ID) }, job.Answer, job.Thread, job.Draft)

	got, err := ask[protocol.ConversationStarted](t, w, protocol.MethodConversationStart, protocol.ConversationStartParams{Message: " Let's work out a mission for frontend#12. "})
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "w1" || got.Path != "/conversations/w1" {
		t.Errorf("started: %+v", got)
	}
	talks := conversation.ForWorkspace(w.Root())
	c, err := talks.Load("w1")
	if err != nil {
		t.Fatal(err)
	}
	if c.Via != "jira" || c.By != conversation.You || c.Kind != conversation.Talk || len(c.Turns) != 1 || c.Turns[0].By != conversation.You || c.Turns[0].Body != "Let's work out a mission for frontend#12." {
		t.Errorf("the conversation: via %q, by %q, turns %+v", c.Via, c.By, c.Turns)
	}
	if want := []string{job.Answer + " /w1"}; !slices.Equal(answers, want) {
		t.Errorf("answered %q, want %q", answers, want)
	}
	// What its first message names says the conversation mentions it,
	// from the start.
	named, err := w.SaidOf(store.Key{Name: "frontend#12"})
	if err != nil || len(named) != 1 || named[0].Ref != "w1" || named[0].Scope != "" || named[0].How != store.Mentions || named[0].Kind != conversation.WhatTalk {
		t.Errorf("said of what it names: %v %+v", err, named)
	}

	// Nothing said starts nothing.
	if _, err := ask[protocol.ConversationStarted](t, w, protocol.MethodConversationStart, protocol.ConversationStartParams{Message: " \n"}); rpcCode(err) != acp.CodeInvalidParams {
		t.Errorf("nothing said: %v", err)
	}
	if all, _ := talks.List(); len(all) != 1 || len(answers) != 1 {
		t.Errorf("after nothing said: %d conversations, answered %q", len(all), answers)
	}

	// The mission its agent proposes, about what was named: accepted,
	// its record says so.
	if _, err := talks.Propose("w1", conversation.Proposal{Kind: conversation.ProposeMission, By: conversation.Agent, Title: "Fix the refresh", Brief: "## Goal\nFix it.\n", About: []string{"frontend#12"}}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := w.SettleProposal("", "w1", 1, true, ProposalEdits{}); err != nil {
		t.Fatal(err)
	}
	m, err := w.Missions.Load("fix-the-refresh")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(m.About, []string{"frontend#12"}) || m.Conversation != "w1" {
		t.Errorf("the mission: about %q, from %q", m.About, m.Conversation)
	}

	// One proposed as about a name ws doesn't read (its plugin's gone
	// since) is made all the same, and says what it couldn't record.
	if _, err := talks.Propose("w1", conversation.Proposal{Kind: conversation.ProposeMission, By: conversation.Agent, Title: "Cache the sessions", About: []string{"RAM-12"}}); err != nil {
		t.Fatal(err)
	}
	said, _, err := w.SettleProposal("", "w1", 2, true, ProposalEdits{})
	if err != nil {
		t.Fatal(err)
	}
	if m, err := w.Missions.Load("cache-the-sessions"); err != nil || len(m.About) != 0 || !strings.Contains(said, "It isn't recorded as about RAM-12") {
		t.Errorf("about a name ws doesn't read: %v, about %v, said %q", err, m, said)
	}
}
