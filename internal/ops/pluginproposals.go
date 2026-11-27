package ops

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/withworkspace/plugin/protocol"
	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/plugin"
)

// A plugin's proposal accepted: its items, all those still open or the
// ones you picked, done by the plugin in the daemon, in order, each told
// what earlier ones made. What's done is kept as it's done; one the
// plugin couldn't do stays open, with why, to accept again, and the
// proposal's settled once none is.

// Proposer is what drafts and does a plugin's proposals: the workspace's
// daemon (plugin.Client), or a test's.
type Proposer interface {
	Draft(ctx context.Context, name string, in protocol.ProposalDraftParams) (plugin.Drafted, error)
	Accept(ctx context.Context, name string, in protocol.ProposalAcceptParams) (protocol.ProposalAcceptResult, error)
	Close()
}

// Proposers is how ops reaches the plugins: through the daemon's socket.
// A test replaces it.
var Proposers = func(root string) (Proposer, error) {
	c, err := plugin.Dial(root)
	if err != nil {
		return nil, fmt.Errorf("the plugins can't be reached (is the workspace's daemon running?): %w", err)
	}
	return c, nil
}

// ProposalKey is ws's key for a plugin's proposal, which the plugin does
// nothing twice by: the same however often it's accepted.
func ProposalKey(slug, id string, n int) string {
	return fmt.Sprintf("%s/%s/%d", firstNonEmpty(slug, "workspace"), id, n)
}

func (w *Workspace) acceptPlugin(store conversation.Store, c *conversation.Conversation, p conversation.Proposal, picked []int) (string, error) {
	return w.acceptPluginBy(store, c, p, picked, conversation.You)
}

// acceptPluginBy is acceptPlugin, as by: you, or an automation you let do
// it without asking (conversation.ByAutomation).
func (w *Workspace) acceptPluginBy(store conversation.Store, c *conversation.Conversation, p conversation.Proposal, picked []int, by string) (string, error) {
	open := p.OpenItems()
	if len(picked) == 0 {
		picked = open
	}
	slices.Sort(picked)
	picked = slices.Compact(picked)
	for _, n := range picked {
		if !slices.Contains(open, n) {
			return "", &Refused{Code: "item", Reason: fmt.Sprintf("item %d of proposal %d isn't open to accept", n, p.N)}
		}
	}
	if len(picked) == 0 {
		return "", &Refused{Code: "item", Reason: fmt.Sprintf("proposal %d has nothing left to accept", p.N)}
	}
	in := protocol.ProposalAcceptParams{ID: p.Proposes, Key: ProposalKey(store.Slug, c.ID, p.N), Made: p.MadeByItem(), Conversation: c.ID, Mission: store.Slug}
	for _, n := range picked {
		in.Items = append(in.Items, protocol.AcceptItem{N: n, Payload: p.Items[n-1].Payload})
	}
	pr, err := Proposers(w.Root())
	if err != nil {
		return "", err
	}
	defer pr.Close()
	got, err := pr.Accept(context.Background(), p.Plugin, in)
	if err != nil {
		return "", err
	}
	answered := map[int]protocol.AcceptedItem{}
	for _, it := range got.Items {
		answered[it.N] = it
	}
	var out []conversation.ItemOutcome
	var made, problems []string
	for _, n := range picked {
		it, ok := answered[n]
		switch {
		case !ok:
			out = append(out, conversation.ItemOutcome{N: n, Problem: p.Plugin + " said nothing of it"})
			problems = append(problems, fmt.Sprintf("item %d: %s said nothing of it", n, p.Plugin))
		case it.Problem != "":
			out = append(out, conversation.ItemOutcome{N: n, Problem: it.Problem})
			problems = append(problems, fmt.Sprintf("item %d: %s", n, it.Problem))
		default:
			out = append(out, conversation.ItemOutcome{N: n, State: conversation.ItemMade, Made: it.Made, Say: it.Say, Open: it.Open})
			made = append(made, firstNonEmpty(it.Made, p.Items[n-1].Title))
		}
	}
	// What you didn't pick is left, once what you did is done.
	if len(problems) == 0 {
		for _, n := range open {
			if !slices.Contains(picked, n) {
				out = append(out, conversation.ItemOutcome{N: n, State: conversation.ItemLeft})
			}
		}
	}
	if _, err := store.ItemsDone(c.ID, p.N, out, by, w.now()); err != nil {
		return "", err
	}
	if len(problems) > 0 {
		said := fmt.Sprintf("%d of %d not done: %s", len(problems), len(picked), strings.Join(problems, "; "))
		if len(made) > 0 {
			said = "Done: " + strings.Join(made, ", ") + ". " + said
		}
		return "", &Refused{Code: "items", Reason: said}
	}
	return "Done: " + strings.Join(made, ", "), nil
}

// DraftPlugin has a plugin draft what a conversation's agent proposed of
// it (name, plugin:id), written as input: with none, how one's written
// (guide). What it drafted is the proposal to record.
func DraftPlugin(root, slug, id, name string, input []byte, now time.Time) (guide, refused string, p conversation.Proposal, err error) {
	pluginName, what, ok := plugin.SplitProposable(name)
	if !ok {
		return "", "", p, fmt.Errorf("%q isn't one of what a plugin proposes: it's the plugin's name, a colon, and what (jira:issues)", name)
	}
	pr, err := Proposers(root)
	if err != nil {
		return "", "", p, err
	}
	defer pr.Close()
	got, err := pr.Draft(context.Background(), pluginName, protocol.ProposalDraftParams{ID: what, Input: input, Conversation: id, Mission: slug})
	if err != nil {
		return "", "", p, err
	}
	if got.Guide != "" || got.Refused != "" {
		return got.Guide, got.Refused, p, nil
	}
	p = conversation.Proposal{Kind: conversation.ProposePlugin, Plugin: pluginName, Proposes: what, Label: got.Proposes.Title, Icon: got.Proposes.Icon,
		Input: input, Title: got.Proposal.Title, At: now}
	for _, it := range got.Proposal.Items {
		p.Items = append(p.Items, conversation.PluginItem{ProposalItem: it})
	}
	return "", "", p, nil
}
