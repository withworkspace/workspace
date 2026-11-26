package ops

import (
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/pulls"
)

func item(slug string, h mission.Headline) MissionItem {
	return MissionItem{M: &mission.Mission{Slug: slug, Title: "The " + slug}, Headline: h}
}

func talk(id string, at time.Time, by ...string) *conversation.Conversation {
	c := &conversation.Conversation{ID: id, Kind: conversation.Talk, Status: conversation.Open}
	for i, b := range by {
		c.Turns = append(c.Turns, conversation.Turn{N: i + 1, By: b, At: at.Add(time.Duration(i) * time.Minute), Body: "said " + b})
	}
	return c
}

func TestNeedsAreTheMissionsInYourCourtThenYourTurns(t *testing.T) {
	at := time.Date(2026, 11, 21, 9, 0, 0, 0, time.UTC)
	missions := []MissionItem{
		item("review", mission.Headline{Court: mission.CourtYou, Kind: mission.KindStepReview, Label: "✓ review", Text: "Step 3 is done", Since: at}),
		item("working", mission.Headline{Court: mission.CourtAgent, Kind: mission.KindWorking, Label: "● working"}),
		item("asks", mission.Headline{Court: mission.CourtYou, Kind: mission.KindCrewAsks, Label: "? asks", By: "c1", Text: "c1 asks: which?", Since: at}),
		// Yours, but resting: for when you choose, not waiting on you.
		item("ready", mission.Headline{Court: mission.CourtYou, Kind: mission.KindReady, Label: "ready"}),
		item("checks", mission.Headline{Court: mission.CourtOutside, Kind: mission.KindChecksRunning}),
	}
	talks := []*conversation.Conversation{
		talk("w1", at, conversation.You, conversation.Agent), // your turn
		talk("w2", at, conversation.You),                     // you spoke last
		talk("w3", at.Add(time.Hour), conversation.You, conversation.Agent),
	}
	got := Needs(missions, talks, nil)
	var said []string
	for _, n := range got {
		said = append(said, n.Of+" "+n.Ref+" "+n.Kind)
	}
	want := []string{"mission review step-review", "mission asks crew-asks", "conversation w3 your-turn", "conversation w1 your-turn"}
	if len(said) != len(want) {
		t.Fatalf("needs = %v, want %v", said, want)
	}
	for i := range want {
		if said[i] != want[i] {
			t.Errorf("need %d = %q, want %q", i, said[i], want[i])
		}
	}
	if got[0].Title != "The review" || got[0].Text != "Step 3 is done" || got[0].Label != "✓ review" {
		t.Errorf("a mission's need = %+v: want its title, its headline's sentence and its label", got[0])
	}
	if got[2].Title != "said you" {
		t.Errorf("a conversation with no title is called %q, want how it began", got[2].Title)
	}
}

func TestANeedsKeyChangesWhenItNeedsYouForSomethingElse(t *testing.T) {
	at := time.Date(2026, 11, 21, 9, 0, 0, 0, time.UTC)
	review := item("m", mission.Headline{Court: mission.CourtYou, Kind: mission.KindStepReview, Since: at})
	again := item("m", mission.Headline{Court: mission.CourtYou, Kind: mission.KindStepReview, Since: at, Text: "said another way"})
	asks := item("m", mission.Headline{Court: mission.CourtYou, Kind: mission.KindCrewAsks, By: "c1", Since: at.Add(time.Minute)})
	key := func(it MissionItem) string { return Needs([]MissionItem{it}, nil, nil)[0].Key }
	if key(review) != key(again) {
		t.Errorf("the same need, said another way, has another key: %q, %q", key(review), key(again))
	}
	if key(review) == key(asks) {
		t.Errorf("a mission that asks after a review has the review's key: %q", key(asks))
	}
	first := Needs(nil, []*conversation.Conversation{talk("w1", at, conversation.You, conversation.Agent)}, nil)[0].Key
	later := Needs(nil, []*conversation.Conversation{talk("w1", at, conversation.You, conversation.Agent, conversation.You, conversation.Agent)}, nil)[0].Key
	if first == later {
		t.Errorf("a conversation answered a second time has the first answer's key: %q", first)
	}
}

func TestNothingNeedsYou(t *testing.T) {
	if got := Needs(nil, nil, nil); got == nil || len(got) != 0 {
		t.Errorf("Needs(nil, nil, nil) = %v, want an empty list", got)
	}
}

func TestAMissionsOwnConversationIsYourTurnToo(t *testing.T) {
	at := time.Date(2026, 11, 21, 9, 0, 0, 0, time.UTC)
	plain := talk("t1", at, conversation.You, conversation.Agent)
	briefing := talk("t2", at, conversation.You, conversation.Agent)
	briefing.Goal = conversation.GoalBrief
	proposed := talk("t3", at, conversation.You, conversation.Agent)
	proposed.Proposals = []conversation.Proposal{{Kind: "decision", State: conversation.ProposalOpen}}
	onDoc := talk("t4", at, conversation.You, conversation.Agent)
	onDoc.Anchor.Doc = "records:adr/0003.md"
	yours := talk("t5", at, conversation.You)
	got := yourTurns([]*conversation.Conversation{plain, briefing, proposed, onDoc, yours})
	if len(got) != 1 || got[0].ID != "t1" {
		t.Fatalf("a mission's conversations that are your turn = %v, want only the plain one its agent answered", got)
	}

	// The mission's at work, and no headline of its says the conversation:
	// the need is the conversation's, and says which mission it's about.
	working := item("dark", mission.Headline{Court: mission.CourtAgent, Kind: mission.KindWorking})
	working.YourTurn = got
	needs := Needs([]MissionItem{working}, []*conversation.Conversation{talk("w1", at, conversation.You, conversation.Agent)}, nil)
	if len(needs) != 2 || needs[0].Ref != "w1" || needs[1].Ref != "t1" || needs[1].In != "dark" || needs[1].Of != NeedConversation {
		t.Fatalf("needs = %+v, want the workspace's conversation, then the mission's with the mission it's about", needs)
	}
	if needs[0].Key == needs[1].Key {
		t.Errorf("two conversations share a key: %q", needs[0].Key)
	}
}

// A pull request that asks for your review by name needs you, after the
// missions and the conversations: it's other people's work, and never
// puts crew's ask further down. A stack is one need, and says so.
func TestPullRequestsAskedOfYouByNameComeLast(t *testing.T) {
	at := time.Date(2026, 11, 23, 9, 0, 0, 0, time.UTC)
	missions := []MissionItem{item("asks", mission.Headline{Court: mission.CourtYou, Kind: mission.KindCrewAsks, Label: "? asks", Since: at})}
	talks := []*conversation.Conversation{talk("w1", at, conversation.You, conversation.Agent)}
	asked := []pulls.Asked{
		{Key: "api#95", PR: pulls.PR{Repo: "api", Number: 95, Title: "Rate limits", Author: "carol", Words: "needs your review"}},
		{Key: "api/stack-3", PR: pulls.PR{Repo: "api", Number: 89, Title: "Key by tenant", Author: "alice", Words: "needs your review"},
			Stack: &pulls.Stack{Title: "Storage layer"}, More: 1},
	}
	got := Needs(missions, talks, asked)
	var said []string
	for _, n := range got {
		said = append(said, n.Of+" "+n.Ref+" "+n.Kind)
	}
	want := []string{"mission asks crew-asks", "conversation w1 your-turn", "pull api#95 your-review", "pull api#89 your-review"}
	if len(said) != len(want) {
		t.Fatalf("needs = %v, want %v", said, want)
	}
	for i := range want {
		if said[i] != want[i] {
			t.Errorf("need %d = %q, want %q", i, said[i], want[i])
		}
	}
	if got[2].Title != "Rate limits" || got[2].Text != "api#95, from carol, needs your review." {
		t.Errorf("a pull request's need = %+v", got[2])
	}
	if got[3].Title != "Storage layer" || got[3].Text != "api#89, from alice, needs your review, and 1 more of its stack do." {
		t.Errorf("a stack's need = %+v", got[3])
	}
}

// A stack whose next layer asks once you've reviewed one is the need it
// was; a pull request that asks again after your review is a new one.
func TestAPullRequestsNeedIsNewWhenItAsksAgain(t *testing.T) {
	key := func(a pulls.Asked) string { return Needs(nil, nil, []pulls.Asked{a})[0].Key }
	layer := func(n int) pulls.Asked {
		return pulls.Asked{Key: "api/stack-3", PR: pulls.PR{Repo: "api", Number: n}, Stack: &pulls.Stack{}}
	}
	if key(layer(89)) != key(layer(90)) {
		t.Error("a stack's next layer arrived as a new need")
	}
	first := pulls.Asked{Key: "api#95", PR: pulls.PR{Repo: "api", Number: 95}}
	again := first
	again.PR.Reviewed = &pulls.MyReview{State: "changes_requested", Commit: "abc123"}
	if key(first) == key(again) {
		t.Error("asked again after your review, and it's the same need")
	}
}

// A mission in your court says what moves it on, from its lead card:
// crew asking, their conversation; a step's checks yours to say, the
// bundle that shows them. One resting in your court doesn't.
func TestAMissionsNeedSaysWhatMovesItOn(t *testing.T) {
	asks := item("asks", mission.Headline{Court: mission.CourtYou, Kind: mission.KindCrewAsks, By: "c1", Text: "c1 asks: which?"})
	asks.Act = &mission.Act{Do: mission.DoReply, Target: "c1"}
	got := Needs([]MissionItem{asks}, nil, nil)
	if len(got) != 1 || got[0].Act == nil || got[0].Act.Do != mission.DoReply || got[0].Act.Target != "c1" {
		t.Fatalf("needs = %+v, want the mission's with its act", got)
	}
}

func TestLeadActIsTheLeadCardsFirst(t *testing.T) {
	snap := mission.Snapshot{M: &mission.Mission{Slug: "dark"}}
	reply := []mission.Headline{{Court: mission.CourtYou, Kind: mission.KindCrewAsks, By: "c1",
		Action: mission.Action{Verb: mission.VerbReply, Target: "c1"}}}
	if a := leadAct(snap, reply, conversation.For(t.TempDir(), "dark")); a == nil || a.Do != mission.DoReply || a.Target != "c1" {
		t.Errorf("crew asking: act = %+v, want reply to c1", a)
	}
	verify := []mission.Headline{{Court: mission.CourtYou, Kind: mission.KindStepReview,
		Action: mission.Action{Verb: mission.VerbReviewStep, Target: "1", Look: []mission.Look{{Name: "the checks", Bundle: "b1"}}}}}
	if a := leadAct(snap, verify, conversation.For(t.TempDir(), "dark")); a == nil || a.Do != mission.DoVerify || a.Target != "b1" {
		t.Errorf("a step to review: act = %+v, want verify b1", a)
	}
	resting := []mission.Headline{{Court: mission.CourtYou, Kind: mission.KindReady,
		Action: mission.Action{Verb: mission.VerbLaunch}}}
	if a := leadAct(snap, resting, conversation.For(t.TempDir(), "dark")); a != nil {
		t.Errorf("a mission resting in your court: act = %+v, want none", a)
	}
}
