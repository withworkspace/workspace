package cli

import (
	"fmt"
	"os"

	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/ui"
)

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

// artifactStateStyle colours a state: landed and merged are done, drafts
// and reviews are in progress, missing is a problem.
func artifactStateStyle(state string) string {
	switch state {
	case "landed", "merged":
		return ui.Green.Render(state)
	case "in review", "open", "draft":
		return ui.Blue.Render(state)
	case "missing":
		return ui.Orange.Render(state)
	}
	return ui.Dim.Render(state)
}

// artifactText is one artifact for people: kind, state, label (or the
// reference), linked when there's a link, with the reference after.
func artifactText(av ops.ArtifactView) string {
	name := av.Label
	if name == "" {
		name = av.Ref
	}
	if av.URL != "" {
		name = ui.Hyperlink(av.URL, name)
	}
	line := fmt.Sprintf("%s  %s  %s", ui.Dim.Render(av.Kind), artifactStateStyle(av.State), name)
	if av.Label != "" {
		line += ui.Dim.Render("  " + av.Ref)
	}
	return line
}
