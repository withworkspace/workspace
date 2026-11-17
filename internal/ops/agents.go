package ops

import (
	"fmt"
	"os"
	"strings"

	"github.com/withworkspace/workspace/internal/workspace"
)

// AgentsNote is what every agent ws starts is told of the workspace's own
// way of working: where its guide is, and how to list its skills. Said in
// the prompt, it reaches an agent whichever harness it runs in, and one
// that can only read can still follow a skill: by reading its file. ""
// for a workspace with neither.
func AgentsNote(ws *workspace.Workspace) string {
	guide, skills := ws.Guide(), ws.Skills()
	if _, err := os.Stat(guide); guide != "" && err != nil {
		guide = ""
	}
	var b strings.Builder
	if guide != "" {
		fmt.Fprintf(&b, "This workspace has a guide for agents: read %s before you start, and follow it.", guide)
	}
	if len(skills) > 0 {
		if b.Len() > 0 {
			b.WriteString(" ")
		}
		names := make([]string, 0, 4)
		for _, s := range skills {
			if len(names) == 4 {
				break
			}
			names = append(names, s.Name)
		}
		such := strings.Join(names, ", ")
		if len(skills) > len(names) {
			such += ", …"
		}
		fmt.Fprintf(&b, "It has skills, each saying how one kind of work is done here (%s): `ws show skills` lists them with what each is for and the file that says how. When your work is of a kind one covers, read that file first and do it that way.", such)
	}
	if len(ws.Kinds) > 0 {
		if b.Len() > 0 {
			b.WriteString(" ")
		}
		titles := make([]string, len(ws.Kinds))
		for i, k := range ws.Kinds {
			titles[i] = k.Title
		}
		fmt.Fprintf(&b, "Its documents are of kinds (%s): `ws show kinds` says where each goes and what it's named, the first line of the commit that adds or changes one, and the file that says how one's written. A document of one of these kinds is written, and judged, as its guide says; a new one goes where its kind says.", strings.Join(titles, ", "))
	}
	return b.String()
}
