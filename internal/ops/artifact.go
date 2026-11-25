package ops

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/withworkspace/workspace/internal/artifact"
	"github.com/withworkspace/workspace/internal/events"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/workspace"
)

// Artifacts: versioned HTML prototypes (internal/artifact), each version
// a page and the files beside it, added by you by hand and by crew, kept
// in .ws/artifacts, on this machine. Crew's are
// recorded on their mission, as one of its artifacts (artifacts.go: a
// pointer, a3, among the pull requests and documents it made). One you
// promote may live in a repo too, for everyone with it (artifact_life.go).

// ArtifactKind is the kind a mission's record gives an artifact.
const ArtifactKind = "artifact"

// ArtifactsConfig is [artifacts] in ws.toml (and its path in
// ws.local.toml), read: where a promoted artifact lives, and the commits
// that put it there. What can't be used is in its Problems (ws doctor
// says them), and it's left out.
func (w *Workspace) ArtifactsConfig() artifact.Config {
	c := w.Config().Artifacts
	return artifact.ReadConfig(artifact.Said{
		Path: c.Path, Local: c.Local,
		Promote: c.Promote, Discard: c.Discard, Restore: c.Restore,
		ExpireAfter: c.ExpireAfter, Guide: c.Guide,
	}, w.WS().ResolveAlias)
}

// Artifacts is the workspace's artifacts: on this machine.
func (w *Workspace) Artifacts() artifact.Store { return artifact.Store{Root: w.Root()} }

// ArtifactGuide is the workspace's guide to how an artifact's written
// ([artifacts] guide): its name as configured and its text. False where
// there's none, or its file isn't there (ws doctor says so), and ws's
// own is shown.
func (w *Workspace) ArtifactGuide() (name, text string, ok bool) {
	name = w.ArtifactsConfig().Guide
	if name == "" {
		return "", "", false
	}
	data, err := os.ReadFile(w.ArtifactGuidePath(name))
	if err != nil {
		return name, "", false
	}
	const most = 24 << 10
	if len(data) > most {
		data = append(data[:most], []byte("\n… (cut short: read the rest in "+name+")")...)
	}
	return name, string(data), true
}

// ArtifactGuidePath is the file a guide names: repo:path on a repo's
// default branch (its ground), or relative to the workspace.
func (w *Workspace) ArtifactGuidePath(name string) string {
	return w.WS().KindGuide(&workspace.Kind{Guide: name})
}

// ArtifactWhere is where a new artifact titled title would go, as it's
// said to people: .ws/artifacts/2026-11-16-horizon-glow/. A -2 is worked
// out when it's added.
func (w *Workspace) ArtifactWhere(title string) string {
	return ".ws/artifacts/" + artifact.FolderName(w.now(), title) + "/"
}

// ArtifactAdd is an artifact, or a version of one, to add.
type ArtifactAdd struct {
	// HTML is its page alone; Files, the page (index.html) with the files
	// beside it, in place of HTML.
	HTML  []byte
	Files artifact.Files
	// Title is the artifact's; "" takes the HTML's <title>. With To, it
	// retitles the artifact ("" keeps its title).
	Title string
	// Note is what the version changed and why; the first's, what the
	// artifact tries out. Crew must give one.
	Note string
	// To is the artifact to add a version to (a3, or a3.v1 for one based
	// on v1); "" for a new artifact.
	To string
	// On is the version it's based on, with To; 0 for the latest.
	On int
	// Mission is the mission crew are on: where the version's from, and
	// where it's recorded.
	Mission string
	// From is the conversation a version you add came from, confirming
	// what its agent proposed (w9, or slug/t3 for a mission's), with the
	// proposal's number and the conversation's title as it is.
	From      string
	FromTitle string
	Proposal  int
}

// ArtifactAdded is what adding one did.
type ArtifactAdded struct {
	Entry   artifact.Entry
	Version int
	// Warnings are what in it won't work in the web UI's frame.
	Warnings []string
	// Recorded is the mission it was recorded on: crew's.
	Recorded string
}

// Ref is the version's ref: a3.v2.
func (a ArtifactAdded) Ref() string { return artifact.VersionRef(a.Entry.ID, a.Version) }

func (a ArtifactAdded) String() string {
	v, _ := a.Entry.Version(a.Version)
	s := fmt.Sprintf("Added %s, %q (%s), in %s", a.Ref(), a.Entry.Title, artifact.FilesWords(v.Files, v.Size), a.Entry.Where())
	if v.BasedOn != 0 && v.BasedOn != a.Version-1 {
		s += fmt.Sprintf(", based on v%d", v.BasedOn)
	}
	if h := a.Entry.Home; h != nil && a.Entry.Lives() != "" {
		s += fmt.Sprintf(" (%s has v%d: promote it to put this one there)", h.Where(), h.Version)
	} else {
		s += " (on this machine: promote it to keep it)"
	}
	if a.Recorded != "" {
		s += ", and recorded it on " + a.Recorded
	}
	return s
}

// AddArtifact adds an artifact, or a version of one (To), on this
// machine. Crew's says it came from their mission, and is recorded on it;
// yours, added by hand, has you as its only provenance, and one you
// confirmed in a conversation says it came from there (From). The
// orchestrator makes none, and a conversation's agent proposes (cli
// refuses it).
func (w *Workspace) AddArtifact(a ArtifactAdd, by Actor) (ArtifactAdded, error) {
	switch by.Kind {
	case Orchestrator:
		return ArtifactAdded{}, refused("not-yours", "the orchestrator doesn't make artifacts: dispatch crew to make one")
	case WS:
		return ArtifactAdded{}, refused("not-yours", "ws adds no artifacts of its own")
	}
	files := a.Files
	if len(files) == 0 {
		files = artifact.Page(a.HTML)
	}
	checked, err := artifact.Check(files)
	if err != nil {
		return ArtifactAdded{}, invalid("not added: %v", err)
	}
	note := artifact.CleanNote(a.Note)
	v := artifact.Version{By: by.By(), Note: note}
	if by.Kind == You && a.From != "" {
		v.From, v.FromTitle, v.Proposal = a.From, a.FromTitle, a.Proposal
	}
	var m *mission.Mission
	if by.Kind == Crew {
		if note == "" {
			what := "what changed in it and why"
			if a.To == "" {
				what = "what it tries out"
			}
			return ArtifactAdded{}, invalid("not added: give it a note (--note), saying %s in a line or two: whoever looks at it next reads that first", what)
		}
		if a.Mission == "" {
			return ArtifactAdded{}, invalid("not added: crew add artifacts for their mission, and there's none ($%s)", mission.EnvMission)
		}
		if m, err = w.Missions.Load(a.Mission); err != nil {
			return ArtifactAdded{}, wrap(err)
		}
		v.From, v.FromTitle = m.Slug, m.Title
	}
	store := w.Artifacts()
	ctx := context.Background()
	out := ArtifactAdded{Warnings: checked.Warnings}
	if a.To == "" {
		title := artifact.CleanTitle(a.Title)
		if title == "" {
			title = checked.Title
		}
		if title == "" {
			return ArtifactAdded{}, invalid("not added: give it a title (--title), or give its page one (<title>)")
		}
		out.Entry, err = store.Add(ctx, artifact.New{Title: title, Files: checked.Files, Version: v, At: w.now()})
		out.Version = 1
	} else {
		id, on, ok := artifact.ParseRef(a.To)
		if !ok {
			return ArtifactAdded{}, invalid("%q isn't an artifact: they're named a1, a2, … (ws artifact list)", a.To)
		}
		if a.On != 0 {
			on = a.On
		}
		out.Entry, out.Version, err = store.AddVersion(ctx, id, artifact.Revision{Files: checked.Files, On: on, Title: a.Title, Version: v, At: w.now()})
	}
	if err != nil {
		return ArtifactAdded{}, wrapArtifact(err)
	}
	w.artifactAdded(out, m, by)
	if m != nil {
		rec := mission.Artifact{Ref: out.Entry.ID, Kind: ArtifactKind, Label: out.Entry.Title, Crew: by.ID}
		if err := w.Missions.AddArtifact(m.Slug, rec); err != nil {
			return out, fmt.Errorf("added %s, but couldn't record it on %s: %w", out.Ref(), m.Slug, err)
		}
		out.Recorded = m.Slug
	}
	return out, nil
}

// artifactAdded keeps that an artifact, or a version of one, was added.
func (w *Workspace) artifactAdded(out ArtifactAdded, m *mission.Mission, by Actor) {
	e := events.Event{Kind: events.ArtifactAdded, Artifact: out.Entry.ID, Text: out.Entry.Title}
	if out.Version > 1 {
		e.Text = fmt.Sprintf("v%d of %s", out.Version, out.Entry.Title)
	}
	if m != nil {
		e.Mission = m.Slug
	}
	w.emit(e, by)
}

func cmpStr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// wrapArtifact is the artifact store's errors as ours.
func wrapArtifact(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, artifact.ErrNotFound):
		return fmt.Errorf("%w: %v", ErrNotFound, strings.TrimPrefix(err.Error(), artifact.ErrNotFound.Error()+": "))
	case errors.Is(err, artifact.ErrDiscarded):
		return &Refused{Code: "discarded", Reason: strings.TrimPrefix(err.Error(), artifact.ErrDiscarded.Error()+": "), Err: err}
	}
	return err
}
