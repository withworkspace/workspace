package ops

import (
	"fmt"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/mission"
)

// A need is something that can't move without you, and only you: crew
// asked you something, a step is yours to review, a conversation's agent
// has replied. It's worked out from where each thing stands now and never
// kept, so it can't go stale: when the thing moves on, the need is gone.
// Something you'd want to know and needn't act on isn't one.
type Need struct {
	// Key is the same for as long as it's the same need, and changes when
	// the thing needs you for something else: what's announced once goes
	// by it.
	Key string
	// Of is the sort of thing it's on (NeedMission, NeedConversation),
	// and Ref that thing as a ref names it (a mission's slug, w4).
	Of  string
	Ref string
	// In is the mission a conversation is about, where it's about one.
	In string
	// Kind is what it needs of you: a mission's headline's kind
	// (crew-asks, step-review, merge…), or NeedYourTurn. Label is the
	// headline's own word for it ("✓ review"), where it has one.
	Kind  string
	Label string
	// Title is the thing's, and Text the sentence saying what it needs.
	Title string
	Text  string
	// Since is when it began, where the thing knows.
	Since time.Time
}

// What a need is on.
const (
	NeedMission      = "mission"
	NeedConversation = "conversation"
)

// NeedYourTurn is a conversation whose agent has replied: it's your turn.
const NeedYourTurn = "your-turn"

// Needs is what needs you, from the workspace's missions and its own
// conversations: the missions in your court now, as the court orders
// them, then the open conversations whose latest word isn't yours, the
// newest first, the workspace's own and then each mission's. Its sources
// are these, in this order; another is added here.
func Needs(missions []MissionItem, talks []*conversation.Conversation) []Need {
	out := []Need{}
	for _, it := range missions {
		if n, ok := missionNeed(it); ok {
			out = append(out, n)
		}
	}
	for i := len(talks) - 1; i >= 0; i-- {
		if n, ok := talkNeed(talks[i], ""); ok {
			out = append(out, n)
		}
	}
	for _, it := range missions {
		for i := len(it.YourTurn) - 1; i >= 0 && it.M != nil; i-- {
			if n, ok := talkNeed(it.YourTurn[i], it.M.Slug); ok {
				out = append(out, n)
			}
		}
	}
	return out
}

// yourTurns is which of a mission's conversations are your turn and
// nobody's to say but their own: a plain conversation about it whose
// agent has replied. Not one that's for something (its briefing, its
// planner's), nor one that proposed something for you to confirm, nor a
// thread on one of its documents: the mission's own headlines say those.
// Nor an ask that isn't kept.
func yourTurns(threads []*conversation.Conversation) []*conversation.Conversation {
	var out []*conversation.Conversation
	for _, c := range threads {
		if c.Kind != conversation.Talk || c.Anchor.Doc != "" || c.Goal != "" || c.Ephemeral || !c.WaitingOnYou() {
			continue
		}
		proposed := false
		for _, p := range c.Proposals {
			proposed = proposed || p.State == conversation.ProposalOpen
		}
		if !proposed {
			out = append(out, c)
		}
	}
	return out
}

// missionNeed is a mission's need: its headline, where that's in your
// court now. One resting there (to brief, ready, parked, landed) isn't
// waiting on you to move: it's for when you choose.
func missionNeed(it MissionItem) (Need, bool) {
	h := it.Headline
	if it.M == nil || h.Group() != mission.GroupNeedsYou {
		return Need{}, false
	}
	return Need{
		Key: "mission|" + it.M.Slug + "|" + h.Key(),
		Of:  NeedMission, Ref: it.M.Slug,
		Kind: string(h.Kind), Label: h.Label,
		Title: it.M.Title, Text: firstNonEmpty(h.Text, h.Label),
		Since: h.Since,
	}, true
}

// talkNeed is a conversation's need: it's open, it isn't on a document,
// and its agent spoke last. in is the mission it's about, if any.
func talkNeed(c *conversation.Conversation, in string) (Need, bool) {
	if c == nil || c.Kind != conversation.Talk || c.Status != conversation.Open || !c.WaitingOnYou() {
		return Need{}, false
	}
	n := Need{
		Of: NeedConversation, Ref: c.ID, In: in,
		Kind:  NeedYourTurn,
		Title: firstNonEmpty(c.Title, firstNonEmpty(talkOpening(c), "Conversation")),
		Text:  "The agent replied: your turn.",
	}
	if len(c.Turns) > 0 {
		n.Since = c.Turns[len(c.Turns)-1].At
	}
	n.Key = fmt.Sprintf("conversation|%s|%s|%d", in, c.ID, n.Since.Unix())
	return n, true
}

// talkOpening is how a conversation with no title began, in a line.
func talkOpening(c *conversation.Conversation) string {
	if len(c.Turns) == 0 {
		return ""
	}
	words := strings.Join(strings.Fields(c.Turns[0].Body), " ")
	if r := []rune(words); len(r) > 140 {
		return string(r[:139]) + "…"
	}
	return words
}
