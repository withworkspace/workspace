package ops

import (
	"cmp"
	"errors"
	"fmt"
	"github.com/withworkspace/workspace/internal/doc"
	"os"
	"time"

	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/page"
	"github.com/withworkspace/workspace/internal/workspace"
)

// A page tied to a file: a markdown file in one of the workspace's repos,
// opened from Files, is edited as a page (cells, questions, passes, the
// map), and saving it writes it back there. The page keeps itself as you
// type, in .ws/pages; Save to repo writes it as it reads (cells as text,
// questions gone) the way a document is saved: committed in its capsule, or
// on a default branch landed or committed in a capsule lifted for it. The
// page keeps the version of the file it last agreed with: what's changed in
// the file since is brought into the page (Bring in) before it's saved,
// merged with its cells.

// errFileMoved is a save refused because the file has changed since the
// page last took it in.
var errFileMoved = errors.New("changed since you opened it")

// openFilePage is the page a file is edited in: the one tied to it
// already, or a new one with its text. A file a mission records is that
// mission's document, edited there: m says which.
func (w *Workspace) openFilePage(ref string, at time.Time) (id string, m *DocMission, err error) {
	d, err := w.FindDoc(ref)
	if err != nil {
		return "", nil, err
	}
	if mm, mref, ok := w.RecordedBy(d); ok {
		return "", &DocMission{Slug: mm.Slug, Title: mm.Title, Ref: mref}, nil
	}
	if !isMarkdown(d.Path) {
		return "", nil, fmt.Errorf("%s cannot be edited: only markdown can", d.Ref)
	}
	ps := page.Store{Root: w.WS().Root}
	if id := ps.TiedTo(d.Ref); id != "" {
		return id, nil, nil
	}
	data, err := os.ReadFile(d.Path)
	if err != nil {
		return "", nil, err
	}
	p, err := ps.New(data)
	if err != nil {
		return "", nil, err
	}
	v, err := conversation.ForWorkspace(w.WS().Root).KeepVersion(data)
	if err != nil {
		return "", nil, err
	}
	meta := page.Meta{Source: &page.Source{Ref: d.Ref, Version: v, At: at}}
	if k, _ := w.WS().KindOf(d.Repo, d.Rel); k != nil {
		meta.Kind = k.Name
	}
	return p.ID, nil, ps.SetMeta(p.ID, meta)
}

// pageTied is where a page tied to a file stands with it, for the page:
// the file, whether it's changed since the page last agreed with it, and
// how saving writes it. nil for a page that isn't tied to one.
func (w *Workspace) pageTied(id string) *PageTied {
	text, p, err := page.Store{Root: w.WS().Root}.Read(id)
	if err != nil || p.Source == nil {
		return nil
	}
	out := &PageTied{Ref: p.Source.Ref}
	d, err := w.FindDoc(p.Source.Ref)
	if err != nil {
		out.Gone = err.Error()
		return out
	}
	out.Ref, out.In = d.Ref, &DocIn{Repo: d.Repo, Worktree: d.Worktree, Path: d.Rel}
	if now, err := os.ReadFile(d.Path); err == nil {
		out.Changed = doc.Version(now) != p.Source.Version
	}
	out.Saves = w.docSaves(d, page.AsText(text))
	return out
}

// savePageToFile writes a page tied to a file back to it, as the page
// reads (its cells' words kept, its questions gone), the way a document is
// saved. message is the commit's first line (none: the kind's, or "Edit
// <path>"). One whose file has changed since the page last took it in is
// refused: what changed is brought in first.
func (w *Workspace) savePageToFile(id, message string, at time.Time) (*DocSaved, error) {
	ps := page.Store{Root: w.WS().Root}
	text, p, err := ps.Read(id)
	if err != nil {
		return nil, err
	}
	if p.Source == nil {
		return nil, fmt.Errorf("%s is not linked to a file: use File as…", id)
	}
	if busy := w.cellBeingWritten(id); busy != "" {
		return nil, fmt.Errorf("a cell of %s is still being written (%s): save once done", id, busy)
	}
	d, err := w.FindDoc(p.Source.Ref)
	if err != nil {
		return nil, err
	}
	now, err := os.ReadFile(d.Path)
	if err != nil {
		return nil, err
	}
	if doc.Version(now) != p.Source.Version {
		return nil, fmt.Errorf("%s has %w: bring in the changes, then save", d.Rel, errFileMoved)
	}
	body := page.AsText(text)
	saved, err := w.SaveWorkspaceDoc(d.Ref, p.Source.Version, string(body), message, fmt.Sprintf("Saved from page %s in ws's web UI.", id))
	if err != nil {
		return nil, err
	}
	if saved.Conflict != nil {
		return nil, fmt.Errorf("%s has %w, and clashes with the page: bring in the changes, resolve them on the page, and save again", d.Rel, errFileMoved)
	}
	// What was written is what the page agrees with now (a save merged with
	// what landed meanwhile leaves the file ahead of it: brought in next).
	v, err := conversation.ForWorkspace(w.WS().Root).KeepVersion(body)
	if err != nil {
		return saved, err
	}
	meta := ps.Meta(id)
	meta.Source = &page.Source{Ref: cmp.Or(saved.Ref, d.Ref), Version: v, At: at}
	if err := ps.SetMeta(id, meta); err != nil {
		saved.Message += fmt.Sprintf("; could not save the file location on the page (%v)", err)
	}
	return saved, nil
}

// bringIn brings what's changed in a page's file, since the page last
// agreed with it, into the page: merged with the page's own edits and
// cells, three ways. Where the two change the same lines, both are kept,
// marked, to settle on the page. It says how many clash.
func (w *Workspace) bringIn(id string, at time.Time) (*PageBroughtIn, error) {
	ps := page.Store{Root: w.WS().Root}
	text, p, err := ps.Read(id)
	if err != nil {
		return nil, err
	}
	if p.Source == nil {
		return nil, fmt.Errorf("%s is not linked to a file", id)
	}
	d, err := w.FindDoc(p.Source.Ref)
	if err != nil {
		return nil, err
	}
	now, err := os.ReadFile(d.Path)
	if err != nil {
		return nil, err
	}
	store := conversation.ForWorkspace(w.WS().Root)
	nowV := doc.Version(now)
	out := &PageBroughtIn{}
	if nowV != p.Source.Version {
		was, err := store.VersionText(p.Source.Version)
		if err != nil {
			return nil, fmt.Errorf("could not compare changes to %s: the earlier version is gone. Copy what you need and open the file again", d.Rel)
		}
		merged, clashes, err := mergeText(was, text, now)
		if err != nil {
			return nil, err
		}
		if _, err := ps.Write(id, doc.Version(text), merged); err != nil {
			return nil, err
		}
		logFailed(store.KeepVersion(now))
		out.Clashes, out.Changed = clashes, true
	}
	meta := ps.Meta(id)
	meta.Source = &page.Source{Ref: d.Ref, Version: nowV, At: at}
	return out, ps.SetMeta(id, meta)
}

// cellBeingWritten is a cell of a page an agent's writing now, if one is.
func (w *Workspace) cellBeingWritten(id string) string {
	for _, c := range conversation.ForWorkspace(w.WS().Root).OnPage(id) {
		if c.Kind == conversation.Cell && c.Answering {
			return c.ID
		}
	}
	return ""
}

// tiePage ties a page to the file it was just filed as: saving it from
// now on writes there. body is what was filed.
func tiePage(ws *workspace.Workspace, ps page.Store, id, ref string, body []byte, at time.Time) error {
	v, err := conversation.ForWorkspace(ws.Root).KeepVersion(body)
	if err != nil {
		return err
	}
	meta := ps.Meta(id)
	meta.Source = &page.Source{Ref: ref, Version: v, At: at}
	return ps.SetMeta(id, meta)
}

// isFileMoved reports whether err is a save refused because the file has
// changed since the page took it in.
func isFileMoved(err error) bool { return errors.Is(err, errFileMoved) }

// fileMoved is that refusal as the web UI knows it (ErrChangedSince),
// saying what it says.
type fileMoved struct{ err error }

func (e fileMoved) Error() string        { return e.err.Error() }
func (e fileMoved) Is(target error) bool { return target == ErrChangedSince }
