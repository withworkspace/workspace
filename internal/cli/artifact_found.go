package cli

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/withworkspace/workspace/internal/artifact"
	"github.com/withworkspace/workspace/internal/ops"
)

// An artifact ws found in a repo (ops/artifact_found.go) isn't on this
// machine: it has no a3, and it's named by its folder there. ws artifact
// list says which there are, and ws show <folder> shows one, with where
// its files can be read.

// foundArtifactLine is one of them as ws artifact list has it: where it
// lives, the version there, and who promoted it.
func foundArtifactLine(f ops.FoundArtifact) string {
	version, by := "", ""
	if r := f.Record; r != nil && r.Promoted != nil {
		version, by = fmt.Sprintf("v%d", r.Promoted.Version), fmt.Sprintf(" · promoted by %s, %s", r.Promoted.By, when(r.Promoted.At))
	} else if f.ChangedBy != "" {
		by = fmt.Sprintf(" · changed by %s, %s", f.ChangedBy, when(f.Changed))
	}
	return fmt.Sprintf("%s %-4s %-9s %s%s", f.Where(), version, artifact.Promoted, f.Title, by)
}

// foundArtifactView is the view of an artifact ws found in the folder ref
// names (records:artifacts/2026-11-20-horizon-glow/), and whether there's
// one: anything else a folder is, is someone else's to show.
func foundArtifactView(ctx *ops.Workspace, ref string) (*view, bool) {
	if !strings.HasSuffix(strings.TrimSpace(ref), "/") {
		return nil, false
	}
	d, err := ctx.FoundArtifact(context.Background(), ref)
	if err != nil {
		return nil, false
	}
	there := "as it is there now"
	if r := d.Record; r != nil && r.Promoted != nil {
		there = fmt.Sprintf("v%d there", r.Promoted.Version)
	}
	v := &view{head: fmt.Sprintf("Artifact at %s — %q (promoted, in %s; %s)", d.Where(), d.Title, d.Repo, there)}
	v.body = append(v.body,
		"An artifact is a page that shows an idea, to look at and use: index.html, alone or with the files it loads beside it.",
		fmt.Sprintf("This one lives in %s, for everyone with the repo, and isn't on this machine: someone else promoted it, or you did on another machine. It has no ref here (a3): it's named by its folder.", d.Repo))
	if r := d.Record; r != nil {
		if p := r.Promoted; p != nil {
			line := fmt.Sprintf("Promoted %s by %s: v%d was chosen", when(p.At), p.By, p.Version)
			if p.Into != "" {
				line += ", into " + p.Into
			}
			v.body = append(v.body, line+".")
		}
		for _, ver := range r.Versions {
			if ver.Note != "" {
				v.body = append(v.body, fmt.Sprintf("v%d (%s, %s): %s", ver.N, provenance(ver), when(ver.At), ver.Note))
			}
		}
	}
	names := make([]string, 0, len(d.Files))
	for _, f := range d.Files {
		names = append(names, f.Path)
	}
	dir := filepath.Join(ctx.WS().MainWorktree(d.Repo), filepath.FromSlash(d.Folder))
	v.body = append(v.body, fmt.Sprintf("Its files (%s), as the default branch of %s has them, are in %s: %s.", artifact.Size(d.Size), d.Repo, dir, strings.Join(names, ", ")))
	return v, true
}
