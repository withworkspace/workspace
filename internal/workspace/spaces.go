package workspace

import "strings"

// Spaces: a repo, or a folder of one, that's read as documents rather than
// as code, in the web UI's Documents.

// Space is a repo of documents, or a folder of one.
type Space struct {
	Repo   string // its canonical name
	Folder string // "" for the whole repo; else the folder whose markdown it is, ending in /
}

// Has reports whether rel, a slash-separated path in the space's repo, is
// one of its documents: markdown, under its folder.
func (s Space) Has(rel string) bool {
	return isMarkdownPath(rel) && strings.HasPrefix(rel, s.Folder)
}

// SpaceOf is the space repo is, if it's one: as the workspace says
// (ws.toml, then ws.local.toml), else as the repo says of itself (its
// ws.repo.toml), else the records repo is one without saying.
func (w *Workspace) SpaceOf(repo string) (Space, bool) {
	d := w.DocumentsSaid[repo]
	if d == nil {
		d = w.repoFile(repo).Documents
	}
	switch {
	case d == nil:
		return Space{Repo: repo}, repo != "" && repo == w.Records
	case !d.On:
		return Space{}, false
	}
	return Space{Repo: repo, Folder: d.Folder}, true
}

// Spaces is every space the workspace has, by its repo's name.
func (w *Workspace) Spaces() []Space {
	var out []Space
	for _, repo := range w.RepoNames {
		if s, ok := w.SpaceOf(repo); ok {
			out = append(out, s)
		}
	}
	return out
}
