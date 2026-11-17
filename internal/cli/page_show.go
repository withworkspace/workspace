package cli

import (
	"bytes"
	"fmt"
	"slices"
	"strings"

	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/page"
	"github.com/withworkspace/workspace/internal/store"
)

// pageView is ws show p3: one of the workspace's pages, what it is (its
// file, its kind, where it was filed, what it's tied to), its cells and
// passes, what it says as it reads, and what's said of it elsewhere.
func pageView(ctx *ops.Workspace, id string, full bool) (*view, error) {
	root := ctx.WS().Root
	ps := page.Store{Root: root}
	text, p, err := ps.Read(id)
	if err != nil {
		return nil, err
	}
	v := &view{head: fmt.Sprintf("Page %s — %q (the workspace's, changed %s)", p.ID, p.Title, when(p.Updated))}
	v.body = append(v.body, fmt.Sprintf("The file: %s (%d lines). It's the page as it's written: a cell is between its `<!-- ws:cell <id> … -->` and `<!-- /ws:cell -->` comments, and holds that conversation's latest reply.", ps.Path(id), bytes.Count(text, []byte("\n"))))
	if p.Kind != "" {
		said := fmt.Sprintf("It's written as one of the workspace's kinds of document, %s (ws show kinds).", p.Kind)
		if k, ok := ctx.WS().KindNamed(p.Kind); ok {
			said = fmt.Sprintf("It's written as one of the workspace's kinds of document, %s (%s): ws show kinds.", k.Title, k.Name)
		}
		v.body = append(v.body, said)
	}
	switch {
	case p.Filed != nil:
		how := "landed on the default branch"
		if !p.Filed.Landed {
			how = "left in a capsule for its pull request"
		}
		v.body = append(v.body, fmt.Sprintf("Filed as %s on %s, %s.", p.Filed.Ref, p.Filed.At.Local().Format("2006-01-02"), how))
		v.add(p.Filed.Ref, "the document it was filed as")
	case p.To != "":
		v.body = append(v.body, fmt.Sprintf("To be filed in %s.", p.To))
	}
	if s := p.Source; s != nil && (p.Filed == nil || s.Ref != p.Filed.Ref) {
		v.body = append(v.body, fmt.Sprintf("Tied to %s: saving it to the repo writes there.", s.Ref))
		v.add(s.Ref, "the file it's tied to")
	}

	talk := conversation.ForWorkspace(root)
	mine := map[string]bool{}
	for _, c := range talk.OnPage(id) {
		mine[c.ID] = true
		what := "a cell"
		if c.Goal == conversation.GoalAsk {
			what = "a question beside the page"
		}
		standing := "written"
		switch {
		case c.Answering:
			standing = "its agent writing it now"
		case c.Goal != conversation.GoalAsk && !page.HasCell(text, c.ID):
			standing = "taken out of the page"
		}
		said := ""
		if len(c.Turns) > 0 {
			said = ": " + clipString(strings.Join(strings.Fields(c.Turns[0].Body), " "), 100)
		}
		v.add(c.ID, fmt.Sprintf("%s (%s)%s", what, standing, said))
	}
	for _, c := range talk.PassesOn(id) {
		mine[c.ID] = true
		open := 0
		if w := c.Written(); w != nil {
			for _, f := range w.Findings {
				if c.FindingState(f.Key) == conversation.FindingOpen {
					open++
				}
			}
		}
		standing := fmt.Sprintf("%d open %s", open, plural(open, "finding"))
		if c.Answering {
			standing = "running now"
		}
		v.add(c.ID, fmt.Sprintf("the pass %s over it (%s)", c.Goal, standing))
	}

	reads := strings.TrimSpace(string(page.AsText(text)))
	if reads == "" {
		v.body = append(v.body, "", "Nothing's written in it yet.")
	} else {
		v.body = append(v.body, "", "As it reads (its cells' words, without their comments; its questions left out):", "", clipText(reads, full))
	}

	// What's said of it elsewhere: its own cells and passes are above.
	if said, err := ctx.SaidOf(store.Key{Name: id}); err == nil {
		said = slices.DeleteFunc(said, func(x ops.Said) bool { return x.Scope == "" && mine[x.Ref] })
		v.addSaid(saidSection{title: "Said of it", said: said})
	}
	return v, nil
}
