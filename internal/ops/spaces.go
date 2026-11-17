package ops

import (
	"cmp"
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/withworkspace/workspace/internal/ref"
	"github.com/withworkspace/workspace/internal/space"
	"github.com/withworkspace/workspace/internal/store"
)

// Records is the workspace's records: the daemon's file, borrowed over
// its socket in any other process, the daemon started if it's down.
func (w *Workspace) Records() (*store.DB, error) { return store.Open(w.Root()) }

// Spaces is the workspace's records with what's new on each space's
// ground read in: a space whose ground's head has moved is read again,
// only its documents a commit changed.
func (w *Workspace) Spaces(ctx context.Context) (*store.DB, space.Stats, error) {
	db, err := w.Records()
	if err != nil {
		return nil, space.Stats{}, err
	}
	var once sync.Once
	var known *ref.Known
	st, err := space.Refresh(ctx, db, w.WS(), func() *ref.Known {
		once.Do(func() { known = w.RefsKnown(ctx, true) })
		return known
	})
	return db, st, err
}

// SpaceView is a space of documents as a list of them says it: its repo
// as ws labels it, its folder, and how many documents it has.
type SpaceView struct {
	Repo, Label, Color, Folder string
	Count                      int
	Cloned                     bool
	// Folders are the folders at its top (under its own folder), each
	// with how many documents are in it, by name; documents at the top
	// itself aren't in one.
	Folders []SpaceFolder
	// Kinds are the workspace's kinds of document whose path is in it, by
	// name: its library's lists.
	Kinds []SpaceKind
}

// SpaceKind is a kind of document a space has: its name and title, and
// where one can stand (none, for a kind with no moves).
type SpaceKind struct {
	Name, Title string
	States      []string
}

// SpaceDoc is a document in a space, as its library lists it: of one of
// the workspace's kinds, where it stands, and what its path says of it.
type SpaceDoc struct {
	Path, Title        string
	Kind, State        string
	Date, Number       string
	Changed, ChangedBy string
}

// SpaceFolder is a folder at a space's top, and how many documents it has.
type SpaceFolder struct {
	Name  string
	Count int
}

// SpaceList is the workspace's spaces, read fresh. A space that couldn't
// be read is listed as it was last read; why is the error.
func (w *Workspace) SpaceList(ctx context.Context) ([]SpaceView, error) {
	db, _, err := w.Spaces(ctx)
	if db == nil {
		return nil, err
	}
	ws := w.WS()
	colors := w.repoColors()
	out := []SpaceView{}
	for _, s := range ws.Spaces() {
		docs, derr := space.Docs(ctx, db, s.Repo)
		if derr != nil {
			return nil, derr
		}
		v := SpaceView{Repo: s.Repo, Label: ws.DisplayNameFor(s.Repo), Color: colors[s.Repo], Folder: s.Folder, Count: len(docs), Folders: []SpaceFolder{}, Kinds: []SpaceKind{}}
		for _, k := range ws.Kinds {
			if k.Repo == s.Repo && strings.HasPrefix(k.Pattern, s.Folder) {
				v.Kinds = append(v.Kinds, SpaceKind{Name: k.Name, Title: k.Title, States: k.States()})
			}
		}
		at := map[string]int{}
		for _, d := range docs {
			if top, _, ok := strings.Cut(strings.TrimPrefix(d.Path, s.Folder), "/"); ok {
				if _, seen := at[top]; !seen {
					at[top] = len(v.Folders)
					v.Folders = append(v.Folders, SpaceFolder{Name: top})
				}
				v.Folders[at[top]].Count++
			}
		}
		if info, err := os.Stat(ws.MainWorktree(s.Repo)); err == nil && info.IsDir() {
			v.Cloned = true
		}
		out = append(out, v)
	}
	return out, err
}

// SpaceDocs is a space and its documents, read fresh. ErrNotFound for a
// repo that isn't one.
func (w *Workspace) SpaceDocs(ctx context.Context, repo string) (*SpaceView, []SpaceDoc, error) {
	name, ok := w.WS().ResolveAlias(repo)
	if !ok {
		return nil, nil, fmt.Errorf("%w: the workspace has no repo %s", ErrNotFound, repo)
	}
	list, err := w.SpaceList(ctx)
	if list == nil {
		return nil, nil, err
	}
	for i := range list {
		if list[i].Repo != name {
			continue
		}
		db, _ := w.Records()
		docs, derr := space.Docs(ctx, db, name)
		if derr != nil {
			return nil, nil, derr
		}
		out := make([]SpaceDoc, 0, len(docs))
		for _, d := range docs {
			sd := SpaceDoc{Path: d.Path, Title: d.Title, Changed: d.Changed, ChangedBy: d.ChangedBy}
			for _, k := range w.WS().Kinds {
				if k.Repo != name {
					continue
				}
				if f, state, ok := k.MatchState(d.Path); ok {
					sd.Kind, sd.State, sd.Date, sd.Number = k.Name, state, f.Date, f.Number
					break
				}
			}
			out = append(out, sd)
		}
		return &list[i], out, err
	}
	return nil, nil, fmt.Errorf("%w: %s isn't a space of documents", ErrNotFound, name)
}

// DocHit is a document a search found: where it is, the section it's led
// to, and of which kind and where it stands, when it's one of the
// workspace's kinds.
type DocHit struct {
	space.Hit
	Label       string // its space's
	Kind, State string
}

// DocQuery is a search of the spaces' documents: the words, the spaces
// (repos; none for all), and a kind and where it stands to keep to.
type DocQuery struct {
	Words       string
	Repos       []string
	Kind, State string
	Limit       int
}

// SearchDocuments is the documents of the workspace's spaces that best
// match, as the records have them (not read again: a page that lists the
// spaces does that), and how many more matched.
func (w *Workspace) SearchDocuments(ctx context.Context, q DocQuery) ([]DocHit, int, error) {
	if len(w.WS().Spaces()) == 0 {
		return nil, 0, nil // nothing to look through: no need for the records
	}
	db, err := w.Records()
	if err != nil {
		return nil, 0, err
	}
	limit := cmp.Or(q.Limit, 20)
	ask := space.Query{Words: q.Words, Repos: q.Repos, Limit: limit}
	if q.Kind != "" || q.State != "" {
		ask.Limit = 500 // kept to after: look further
	}
	hits, more, err := space.Search(ctx, db, ask)
	if err != nil {
		return nil, 0, err
	}
	ws := w.WS()
	out := []DocHit{}
	for _, h := range hits {
		d := DocHit{Hit: h, Label: ws.DisplayNameFor(h.Repo)}
		for _, k := range ws.Kinds {
			if k.Repo != h.Repo {
				continue
			}
			if _, state, ok := k.MatchState(h.Path); ok {
				d.Kind, d.State = k.Name, state
				break
			}
		}
		if (q.Kind != "" && d.Kind != q.Kind) || (q.State != "" && d.State != q.State) {
			continue
		}
		if len(out) == limit {
			more++
			continue
		}
		out = append(out, d)
	}
	return out, more, nil
}

// DocCitations is what cites a space's document, and what it cites, as
// the records have them.
func (w *Workspace) DocCitations(ctx context.Context, repo, path string) (citedBy, cites []space.Citation, err error) {
	s, err := w.spaceNamed(repo)
	if err != nil {
		return nil, nil, err
	}
	db, _, err := w.Spaces(ctx)
	if db == nil {
		return nil, nil, err
	}
	if citedBy, err = space.CitedBy(ctx, db, s.Repo, path); err != nil {
		return nil, nil, err
	}
	cites, err = space.Cites(ctx, db, s.Repo, path)
	return citedBy, cites, err
}
