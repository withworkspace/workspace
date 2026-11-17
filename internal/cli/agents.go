package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/withworkspace/workspace/internal/workspace"
)

// kindsView is ws show kinds: the kinds of document the workspace has.
func kindsView(ws *workspace.Workspace) *view {
	v := &view{head: fmt.Sprintf("The workspace's kinds of document (%d)", len(ws.Kinds))}
	if len(ws.Kinds) == 0 {
		v.body = append(v.body, "It has none: [documents.<name>] in ws.toml says what one is.")
		return v
	}
	v.body = append(v.body, "A document is of a kind when its path fits the kind's. In a path, {date} is the day one's started (2006-01-02), {slug} its title in lowercase words joined by hyphens, and {number} one above the highest there, to four digits. In a commit's first line, {title} is its first heading.", "")
	for _, k := range ws.Kinds {
		v.body = append(v.body, fmt.Sprintf("- %s (%s): %s", k.Title, k.Name, k.Where()))
		switch {
		case k.Add != "" && k.Amend != "":
			v.body = append(v.body, fmt.Sprintf("  Added as %q, changed as %q", k.Add, k.Amend))
		case k.Add != "":
			v.body = append(v.body, fmt.Sprintf("  Added as %q", k.Add))
		case k.Amend != "":
			v.body = append(v.body, fmt.Sprintf("  Changed as %q", k.Amend))
		}
		if len(k.Passes) > 0 {
			v.body = append(v.body, "  Its passes: "+strings.Join(k.Passes, ", "))
		}
		if guide := ws.KindGuide(k); guide != "" {
			if _, err := os.Stat(guide); err != nil {
				v.body = append(v.body, "  Its guide would be "+guide+", which isn't there")
			} else {
				v.body = append(v.body, "  How one's written: "+guide)
			}
		}
	}
	return v
}

// skillsView is ws show skills: the workspace's guide and skills.
func skillsView(ws *workspace.Workspace) *view {
	skills := ws.Skills()
	v := &view{head: fmt.Sprintf("The workspace's skills (%d): how particular kinds of work are done here", len(skills))}
	if guide := ws.Guide(); guide != "" {
		v.body = append(v.body, "The workspace's guide for agents: "+guide, "")
	}
	if len(skills) == 0 {
		v.body = append(v.body, "It has no skills.")
		return v
	}
	v.body = append(v.body, "To follow one, read its file: it says when it applies and what to do. One that changes things (commits, pushes) is for crew doing the work, or the user: an agent that only reads can still follow one that's about reading and judging.", "")
	for _, s := range skills {
		v.body = append(v.body, "- "+s.Name+": "+firstNonEmpty(clipString(s.About, 400), "(it doesn't say what it's for)"), "  "+s.Path)
	}
	return v
}
