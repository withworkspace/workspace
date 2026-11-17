package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/page"
)

// Passes over a page: a pass's agent reads the whole page and says things
// about it, which ws reads into findings and keeps beside the page, never
// in it. Each pass is a conversation (conversation.Pass) whose runs are
// turns; run again, it's told what you've settled since.

// pagePasses are the workspace's passes: see ops.PassDefs.
func pagePasses(ctx *ops.Workspace) []page.PassDef {
	if ctx == nil {
		return ops.PassDefs(nil, nil)
	}
	return ops.PassDefs(ctx.Config(), ctx.WS())
}

// passesView is ws show passes: each pass, over every page (or, a
// review's pass, every pull request and capsule) it's been run on, with what came of
// what it last found there: taken, dismissed (as wrong, as what was
// already said, or as so but not worth it), or still open. Of the findings
// you've judged, how many were right and how many you acted on is how well
// it does.
func passesView(ctx *ops.Workspace) *view {
	type tally struct {
		name, title                         string
		review                              bool // a review's pass, over pull requests
		pages                               map[string]bool
		runs, found                         int
		done, wrong, already, minor, unsaid int
	}
	defs := pagePasses(ctx)
	reviewDefs := ops.ReviewPassDefs(ctx.Config(), ctx.WS())
	by := map[string]*tally{}
	var order []string
	all, _ := conversation.ForWorkspace(ctx.WS().Root).Passes()
	for _, c := range all {
		if c.Kind != conversation.Pass {
			continue
		}
		// A page's pass and a review's may share a name: they're two passes.
		key, over, from := c.Goal, c.Anchor.Page, defs
		if of := c.Anchor.Reviewed(); of != "" {
			key, over, from = "review "+c.Goal, of, reviewDefs
		}
		t := by[key]
		if t == nil {
			t = &tally{name: c.Goal, title: c.Goal, review: c.Anchor.Reviewed() != "", pages: map[string]bool{}}
			if def, ok := page.PassNamed(from, c.Goal); ok {
				t.title = def.Title
			}
			by[key] = t
			order = append(order, key)
		}
		t.pages[over] = true
		for _, turn := range c.Turns {
			if turn.By == conversation.Agent {
				t.runs++
			}
		}
		w := c.Written()
		if w == nil {
			continue
		}
		for _, f := range w.Findings {
			t.found++
			switch c.FindingState(f.Key) {
			case conversation.FindingDone:
				t.done++
			case conversation.FindingDismissed:
				switch c.DismissedAs(f.Key) {
				case conversation.DismissWrong:
					t.wrong++
				case conversation.DismissAlready:
					t.already++
				case conversation.DismissMinor:
					t.minor++
				default:
					t.unsaid++
				}
			}
		}
	}
	sort.Strings(order)
	v := &view{head: fmt.Sprintf("Passes over pages and reviews (%d run): what they found, and what came of it", len(order))}
	if len(order) == 0 {
		v.body = append(v.body, "No pass has been run over a page or a review yet.")
		return v
	}
	v.body = append(v.body, "Each pass's findings are those of its latest run over each page or review (a pull request's, or a mission's capsule's). Dismissing one says why (wrong, already said, not worth it): that's what's counted here. Taken, a review's finding went into the user's review.", "")
	for _, key := range order {
		t := by[key]
		name, over := t.name, "page"
		if t.review {
			name, over = t.name+", a review's", "review"
		}
		open := t.found - t.done - t.wrong - t.already - t.minor - t.unsaid
		line := fmt.Sprintf("- %s (%s): %d %s over %d %s; %d %s", t.title, name, t.runs, plural(t.runs, "run"), len(t.pages), plural(len(t.pages), over), t.found, plural(t.found, "finding"))
		var parts []string
		for _, p := range []struct {
			n    int
			what string
		}{{t.done, "taken"}, {t.minor, "not worth it"}, {t.wrong, "wrong"}, {t.already, "already said"}, {t.unsaid, "dismissed, no reason given"}, {open, "open"}} {
			if p.n > 0 {
				parts = append(parts, fmt.Sprintf("%d %s", p.n, p.what))
			}
		}
		if len(parts) > 0 {
			line += ": " + strings.Join(parts, ", ")
		}
		v.body = append(v.body, line)
		if judged := t.done + t.minor + t.wrong + t.already; judged > 0 {
			right := t.done + t.minor
			v.body = append(v.body, fmt.Sprintf("  Of %d judged: %d right (%d%%), %d acted on (%d%%)", judged, right, right*100/judged, t.done, t.done*100/judged))
		}
	}
	return v
}
