package ops

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/artifact"
	"github.com/withworkspace/workspace/internal/ref"
	"github.com/withworkspace/workspace/internal/space"
	"github.com/withworkspace/workspace/internal/store"
)

// An artifact that's promoted lives in a repo: a folder there, with a
// thing.toml that says it's an artifact (putHome writes it). So the
// spaces are where ws finds them, as it finds a plugin's things
// (remotes.go), and artifact is a kind ws has itself: nothing's asked of a
// plugin. One found there that no artifact on this machine lives at is
// someone else's, or another machine's: it's kept (artifact.Found) so it's
// found, listed and opened here, from the repo's files.

// folderName is a folder in a repo as ws names one: what an artifact found
// there is called.
func folderName(repo, folder string) string {
	return ref.Ref{Kind: ref.Dir, Repo: repo, Worktree: ref.Ground, Path: strings.TrimSuffix(folder, "/")}.Name()
}

// syncFoundArtifacts brings the artifacts ws keeps as found in step with
// the spaces: one that's new there, or changed, is read and kept; one
// that's gone, or that an artifact on this machine lives at, isn't kept.
// Nothing's read when nothing's changed.
func (w *Workspace) syncFoundArtifacts(ctx context.Context, db *store.DB) error {
	things, err := space.Things(ctx, db, artifact.ThingKind)
	if err != nil {
		return err
	}
	st := w.Artifacts()
	had, err := st.Found(ctx)
	if err != nil {
		return err
	}
	if len(things) == 0 && len(had) == 0 {
		return nil
	}
	here := w.artifactsLivingAt(ctx)
	kept := map[string]string{}
	for _, f := range had {
		kept[f.Name] = f.Version
	}
	want := map[string]bool{}
	var keep []artifact.Found
	for _, t := range things {
		name := t.Name()
		if here[name] != "" {
			continue // this machine's: it's listed, and found, as a3
		}
		want[name] = true
		if kept[name] == t.Version() {
			continue
		}
		keep = append(keep, w.foundArtifact(ctx, t))
	}
	var gone []string
	for _, f := range had {
		if !want[f.Name] {
			gone = append(gone, f.Name)
		}
	}
	return st.KeepFound(ctx, keep, gone)
}

// artifactsLivingAt is each artifact on this machine that lives in a
// repo, by its folder's name there.
func (w *Workspace) artifactsLivingAt(ctx context.Context) map[string]string {
	out := map[string]string{}
	list, err := w.Artifacts().List(ctx)
	if err != nil {
		return out
	}
	for _, e := range list {
		if e.Home != nil && e.Lives() != "" {
			repo, _ := w.WS().ResolveAlias(e.Home.Repo)
			out[folderName(repo, e.Home.Folder)] = e.ID
		}
	}
	return out
}

// foundArtifact is an artifact as a space's thing says it, with what its
// record there says of it, where it has one ws can read.
func (w *Workspace) foundArtifact(ctx context.Context, t space.Thing) artifact.Found {
	f := artifact.Found{Name: t.Name(), Repo: t.Repo, Folder: strings.TrimSuffix(t.Path, "/"), Title: t.Title, Version: t.Version(), ChangedBy: t.ChangedBy}
	f.Changed, _ = time.Parse(time.RFC3339, t.Changed)
	if text, _, err := space.FileOf(ctx, w.WS(), t, artifact.RecordFile); err == nil {
		if a, err := artifact.Parse(text); err == nil {
			f.Record = a
			if f.Title == "" {
				f.Title = a.Title
			}
		}
	}
	return f
}

// FoundArtifact is an artifact ws found in a repo, with where it is among
// the artifacts there, to list them as they're organised: what ws calls
// the repo, and the folders it's grouped in under where [artifacts] says
// promoted ones live (kept, 2026/q4: "" for one straight in it). One
// that's nowhere under that is grouped by the folders it's in from the
// repo's top.
type FoundArtifact struct {
	artifact.Found
	Label, Group string
}

func (w *Workspace) labelled(f artifact.Found) FoundArtifact {
	var places []thingPlace
	if cfg := w.ArtifactsConfig(); cfg.Repo != "" {
		places = []thingPlace{{repo: cfg.Repo, folder: path.Join(cfg.Dir, cfg.Name)}}
	}
	return FoundArtifact{Found: f, Label: w.WS().DisplayNameFor(f.Repo), Group: thingGroupIn(places, f.Repo, f.Folder+"/")}
}

// FoundArtifacts are the artifacts in the workspace's repos that no
// artifact on this machine lives at: someone else's, or another
// machine's. They're as the repos' grounds had them when the spaces were
// last read: a page's read of them starts no git, and where a ground has
// moved since, the spaces are read again behind it (spacesKept), which
// moves spacesSeen for the page to read again.
func (w *Workspace) FoundArtifacts(ctx context.Context) ([]FoundArtifact, error) {
	_, _ = w.spacesKept()
	found, err := w.Artifacts().Found(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]FoundArtifact, 0, len(found))
	for _, f := range found {
		out = append(out, w.labelled(f))
	}
	return out, nil
}

// FoundArtifactFile is one of an artifact's files in a repo.
type FoundArtifactFile struct {
	Path string
	Size int
}

// FoundArtifactDetail is an artifact found in a repo, with its files.
type FoundArtifactDetail struct {
	FoundArtifact
	Files []FoundArtifactFile
	Size  int
}

// artifactThing is the artifact ws found in the folder named (as ws names
// one, or as it's said to people: records:artifacts/x/), as its space has
// it now.
func (w *Workspace) artifactThing(ctx context.Context, name string) (space.Thing, error) {
	none := noThingAt("ws has found no artifact at " + name)
	r, ok := ref.Parse(strings.TrimPrefix(strings.TrimSpace(name), "ws:"))
	if !ok || r.Kind != ref.Dir || r.InCapsule() {
		return space.Thing{}, none
	}
	repo, known := w.WS().ResolveAlias(r.Repo)
	if !known {
		return space.Thing{}, none
	}
	db, _, err := w.Spaces(ctx)
	if db == nil {
		return space.Thing{}, err
	}
	t, ok, err := space.ThingAt(ctx, db, repo, r.Path)
	if err != nil {
		return space.Thing{}, err
	}
	if !ok || t.Kind != artifact.ThingKind {
		return space.Thing{}, none
	}
	return t, nil
}

// FoundArtifact is the artifact ws found in the folder named, with its
// files as its repo has them.
func (w *Workspace) FoundArtifact(ctx context.Context, name string) (FoundArtifactDetail, error) {
	t, err := w.artifactThing(ctx, name)
	if err != nil {
		return FoundArtifactDetail{}, err
	}
	out := FoundArtifactDetail{FoundArtifact: w.labelled(w.foundArtifact(ctx, t))}
	files, err := space.Files(ctx, w.WS(), t)
	if err != nil {
		return out, err
	}
	for _, f := range files {
		out.Files = append(out.Files, FoundArtifactFile{Path: f.Path, Size: f.Size})
		out.Size += f.Size
	}
	return out, nil
}

// FoundArtifactAt reads one of the files of an artifact ws found in a
// repo, by the version it was found at (its folder, as git names it) and
// the file's path in it: its bytes, and git's name for them. The version
// has to be one an artifact in that repo is at now: what ws serves is
// what it found, not whatever a repo has ever held.
func (w *Workspace) FoundArtifactAt(ctx context.Context, repo, version, file string) ([]byte, string, error) {
	none := noThingAt(fmt.Sprintf("ws has found no artifact in %s at that version", repo))
	repo, known := w.WS().ResolveAlias(repo)
	if !known {
		return nil, "", none
	}
	db, err := w.Records()
	if err != nil {
		return nil, "", err
	}
	things, err := space.ThingsAt(ctx, db, repo, version)
	if err != nil {
		return nil, "", err
	}
	for _, t := range things {
		if t.Kind != artifact.ThingKind {
			continue
		}
		data, blob, err := space.FileOf(ctx, w.WS(), t, file)
		if errors.Is(err, space.ErrNoFile) {
			return nil, "", artifact.ErrNoFile
		}
		return data, blob, err
	}
	return nil, "", none
}
