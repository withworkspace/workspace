package ops

import (
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/mission"
)

func item(slug string, h mission.Headline) MissionItem {
	return MissionItem{MissionView: MissionView{M: &mission.Mission{Slug: slug, Title: "The " + slug}}, Headline: h}
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
	got := Needs(missions, talks)
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
	key := func(it MissionItem) string { return Needs([]MissionItem{it}, nil)[0].Key }
	if key(review) != key(again) {
		t.Errorf("the same need, said another way, has another key: %q, %q", key(review), key(again))
	}
	if key(review) == key(asks) {
		t.Errorf("a mission that asks after a review has the review's key: %q", key(asks))
	}
	first := Needs(nil, []*conversation.Conversation{talk("w1", at, conversation.You, conversation.Agent)})[0].Key
	later := Needs(nil, []*conversation.Conversation{talk("w1", at, conversation.You, conversation.Agent, conversation.You, conversation.Agent)})[0].Key
	if first == later {
		t.Errorf("a conversation answered a second time has the first answer's key: %q", first)
	}
}

func TestNothingNeedsYou(t *testing.T) {
	if got := Needs(nil, nil); got == nil || len(got) != 0 {
		t.Errorf("Needs(nil, nil) = %v, want an empty list", got)
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
	needs := Needs([]MissionItem{working}, []*conversation.Conversation{talk("w1", at, conversation.You, conversation.Agent)})
	if len(needs) != 2 || needs[0].Ref != "w1" || needs[1].Ref != "t1" || needs[1].In != "dark" || needs[1].Of != NeedConversation {
		t.Fatalf("needs = %+v, want the workspace's conversation, then the mission's with the mission it's about", needs)
	}
	if needs[0].Key == needs[1].Key {
		t.Errorf("two conversations share a key: %q", needs[0].Key)
	}
}
