package web

import (
	"context"

	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/space"
)

// Documents is reading, saving and filing documents and pages: a mission's
// artifacts, files outside a mission, and pages tied to files.
type Documents interface {
	Artifact(slug, ref string) (*ops.DocText, error)
	Doc(ref string) (*ops.DocText, error)
	SaveDocument(slug, ref, base, text, message string) (*ops.DocSaved, error)
	AcceptSuggestion(slug, id string) (string, error)
	Kinds() []ops.Kind
	FilePage(id string, r ops.PageFile) (*ops.PageFiling, error)
	OpenFile(ref string) (string, *ops.DocMission, error)
	PageTied(id string) *ops.PageTied
	SavePageToFile(id, message string) (*ops.DocSaved, error)
	BringIn(id string) (*ops.PageBroughtIn, error)
	// Spaces of documents, read fresh from their grounds.
	SpaceList(ctx context.Context) ([]ops.SpaceView, error)
	SpaceDocs(ctx context.Context, repo string) (*ops.SpaceView, []ops.SpaceDoc, error)
	SpaceActivity(ctx context.Context, repo string) (*ops.SpaceActivity, error)
	FetchSpace(ctx context.Context, repo string) (ops.SpaceFresh, error)
	PullSpace(ctx context.Context, repo string) (ops.SpaceFresh, error)
	SearchDocuments(ctx context.Context, q ops.DocQuery) ([]ops.DocHit, int, error)
	// What ws records, by what it says (Find's said group).
	Find(ctx context.Context, q ops.FindQuery) ([]ops.Found, int, error)
	DocCitations(ctx context.Context, repo, path string) (citedBy, cites []space.Citation, err error)
}

// ArtifactContentOf is a document as the API serves it.
func ArtifactContentOf(t *ops.DocText) *ArtifactContent {
	a := &ArtifactContent{
		Artifact: Artifact{Ref: t.Ref, Kind: t.Kind, Label: t.Label, State: t.State, URL: t.URL, Path: t.Path, By: t.By, Added: t.Added, Readable: true},
		Text:     t.Text, Markdown: t.Markdown, Editable: t.Editable, Uneditable: t.Uneditable, Page: t.Page, Media: t.Media,
	}
	if t.Stage != nil {
		st := DocStage(*t.Stage)
		a.Stage = &st
	}
	if t.In != nil {
		in := DocIn(*t.In)
		a.In = &in
	}
	if t.Mission != nil {
		m := DocMission(*t.Mission)
		a.Mission = &m
	}
	a.Saves = docSaves(t.Saves)
	if t.DocKind != nil {
		k := DocKind(*t.DocKind)
		a.DocKind = &k
	}
	return a
}

func docSaves(s *ops.DocSaves) *DocSaves {
	if s == nil {
		return nil
	}
	out := DocSaves(*s)
	return &out
}

// DocSavedOf is what a save did, as the API serves it.
func DocSavedOf(d *ops.DocSaved) *DocSaved {
	if d == nil {
		return nil
	}
	out := &DocSaved{Message: d.Message, Version: d.Version, Commit: d.Commit, Merged: d.Merged, Ref: d.Ref}
	if d.Conflict != nil {
		out.Conflict = &DocConflict{Text: d.Conflict.Text, Clashes: d.Conflict.Clashes}
	}
	return out
}

// PageTiedOf is where a page tied to a file stands, as the API serves it.
func PageTiedOf(p *ops.PageTied) *PageTied {
	if p == nil {
		return nil
	}
	out := &PageTied{Ref: p.Ref, Changed: p.Changed, Gone: p.Gone, Saves: docSaves(p.Saves)}
	if p.In != nil {
		in := DocIn(*p.In)
		out.In = &in
	}
	return out
}

// artifactText is a mission's artifact, as the API serves it.
func (s *Server) artifactText(slug, ref string) (*ArtifactContent, error) {
	t, err := s.App.Artifact(slug, ref)
	if err != nil {
		return nil, err
	}
	return ArtifactContentOf(t), nil
}

// docText is a document outside a mission, as the API serves it.
func (s *Server) docText(ref string) (*ArtifactContent, error) {
	t, err := s.App.Doc(ref)
	if err != nil {
		return nil, err
	}
	return ArtifactContentOf(t), nil
}
