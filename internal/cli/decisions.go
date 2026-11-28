package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/ui"
)

// ws mission decide: recording what was decided on a mission and why, so
// whoever comes next (crew, the orchestrator, a thread's agent) knows.

func newMissionDecideCmd() *cobra.Command {
	var slug, why, from, supersedes, withdraw string
	var about []string
	var proposeIt bool
	cmd := &cobra.Command{
		Use:   "decide <what was decided>",
		Short: "Record what was decided on a mission, and why",
		Long: `Record a decision: what was decided, why, and what it's about, so that
whoever works on the mission next knows it and doesn't undo it. ws show
lists the decisions about whatever it shows; the orchestrator and crew
see them.

Decide within your part: an approach taken, an option ruled out. Scope
and direction are the user's: ask them (ws mission ask) rather than
deciding for them.

A decision is never edited. A new one can replace it (--supersedes d1),
or it can be withdrawn (--withdraw d1 "why").

  ws mission decide "Key the cache per tenant." --why "Tokens are issued per tenant." --about records/token-cache:proposals/cache.md`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			text := strings.TrimSpace(strings.Join(args, " "))
			if proposeIt {
				return propose(ctx, conversation.Proposal{Kind: conversation.ProposeDecision, What: text, Why: why})
			}
			if err := refuseInConversation("a decision"); err != nil {
				return err
			}
			m, err := resolveMission(ctx, slug)
			if err != nil {
				return err
			}
			if withdraw != "" {
				if _, err := ctx.WithdrawDecision(m.Slug, withdraw, text, ops.ActorFromEnv()); err != nil {
					return err
				}
				fmt.Fprintf(ui.Stderr, "  %s %s withdrawn\n", ui.Green.Render("✓"), withdraw)
				return nil
			}
			if text == "" {
				return errors.New("say what was decided")
			}
			// What it's about, and where it came from, must be things ws
			// can show.
			g := loadGraph(ctx, m, false)
			for _, ref := range append(append([]string{}, about...), from) {
				if ref == "" {
					continue
				}
				if _, err := g.view(ref); err != nil {
					return fmt.Errorf("--about and --from name things ws show can show: %w", err)
				}
			}
			d, err := ctx.RecordDecision(m.Slug, ops.Deciding{What: text, Why: why, From: from, About: about, Supersedes: supersedes}, ops.ActorFromEnv())
			if err != nil {
				return err
			}
			fmt.Fprintf(ui.Stderr, "  %s %s recorded on %s\n", ui.Green.Render("✓"), d.Decision.ID, m.Slug)
			if h := d.HeldTo; h.Any() {
				printHeldTo(d.Decision.ID, h)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&proposeIt, "propose", false, "in a conversation: propose it, for the user to confirm, instead of recording it")
	revisesFlag(cmd)
	addMissionFlag(cmd, &slug)
	cmd.Flags().StringVar(&why, "why", "", "why it was decided")
	cmd.Flags().StringArrayVar(&about, "about", nil, "a ref it bears on: a stage (s2), a document, a capsule, crew… (repeatable)")
	cmd.Flags().StringVar(&from, "from", "", "where it was decided, as a ref (a review comment r1.c2, crew c1…)")
	cmd.Flags().StringVar(&supersedes, "supersedes", "", "the decision this one replaces (d1)")
	cmd.Flags().StringVar(&withdraw, "withdraw", "", "withdraw this decision instead; the argument says why")
	return cmd
}

// decisionView is ws show d1.
func (g *graph) decisionView(id string) (*view, error) {
	all, err := g.store.Decisions(g.m.Slug)
	if err != nil {
		return nil, err
	}
	d, ok := mission.FindDecision(all, id)
	if !ok {
		return nil, fmt.Errorf("mission %s has no decision %s", g.m.Slug, id)
	}
	v := &view{head: fmt.Sprintf("Decision %s (%s): %s", d.ID, d.State, d.What)}
	if d.Asked != nil {
		v.body = append(v.body, fmt.Sprintf("Answers %s's question: %s", d.Asked.By, d.Asked.Text))
	}
	if d.Why != "" {
		v.body = append(v.body, "Why: "+d.Why)
	}
	v.body = append(v.body, fmt.Sprintf("Decided by %s, %s.", d.By, when(d.At)))
	if d.Withdrawn != nil {
		v.body = append(v.body, fmt.Sprintf("Withdrawn %s: %s", when(*d.Withdrawn), d.WithdrawnWhy))
	}
	if d.From != "" {
		v.add(d.From, "where it was decided")
	}
	for _, a := range d.About {
		v.add(a, "what it's about")
	}
	if d.Supersedes != "" {
		if old, ok := mission.FindDecision(all, d.Supersedes); ok {
			v.add(old.ID, "what it replaced: "+old.What)
		}
	}
	if d.SupersededBy != "" {
		if nd, ok := mission.FindDecision(all, d.SupersededBy); ok {
			v.add(nd.ID, "what replaced it: "+nd.What)
		}
	}
	if c, ok := g.crew(d.By); ok {
		v.add(c.ID, "who decided ("+crewStanding(c)+")")
	}
	return v, nil
}

// decisionLinks adds the decisions that stand about ref to a view.
func (g *graph) decisionLinks(v *view, ref string) {
	all, _ := g.store.Decisions(g.m.Slug)
	for _, d := range mission.CurrentDecisions(all) {
		if d.IsAbout(ref) {
			v.add(d.ID, "decided: "+clipLine(d.What, 100))
		}
	}
}

// printHeldTo is what crew's decision id is held against, and what to do
// if it goes against any of it: said as it's recorded, while it's theirs
// to put right, not found when the check is judged a round of work later.
func printHeldTo(id string, h ops.HeldTo) {
	w := ui.Stderr
	fmt.Fprintf(w, "\n  What your work is held to:\n")
	if len(h.Checks) > 0 {
		fmt.Fprintf(w, "\n  Step %d's checks (yours):\n", h.Step)
		for i, c := range h.Checks {
			fmt.Fprintf(w, "    %d. %s\n", i+1, c)
		}
	}
	for _, l := range h.Later {
		fmt.Fprintf(w, "\n  Step %d's checks (later, which this decision mustn't rule out):\n", l.Step)
		for i, c := range l.Checks {
			fmt.Fprintf(w, "    %d. %s\n", i+1, c)
		}
	}
	if len(h.Yours) > 0 {
		fmt.Fprintf(w, "\n  The user's decisions (they stand over yours):\n")
		for _, d := range h.Yours {
			fmt.Fprintf(w, "    %s\n", d)
		}
	}
	fmt.Fprintf(w, "\n  Please check that %s doesn't conflict with any of the above, thinking of where the code it leads to runs. If it does, withdraw it (ws mission decide --withdraw %s \"why\"), replace it (ws mission decide \"<what instead>\" --supersedes %s), or ask the user (ws mission ask) if it's what it's held to that's wrong.\n", id, id, id)
}
