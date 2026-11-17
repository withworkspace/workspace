package ops

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/withworkspace/workspace/internal/artifact"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/workspace"
)

// Artifacts: versioned HTML prototypes (internal/artifact), added by you
// by hand and by crew, kept in .ws/artifacts. Crew's are recorded on their
// mission, as one of its artifacts (artifacts.go: a pointer, a3, among the
// pull requests and documents it made).

// ArtifactKind is the kind a mission's record gives an artifact.
const ArtifactKind = "artifact"

// ArtifactsConfig is [artifacts] in ws.toml (and its path in
// ws.local.toml), read: where artifacts are kept, and the commits that
// keep them in a repo. What can't be used is in its Problems (ws doctor
// says them), and it's left out.
func (w *Workspace) ArtifactsConfig() artifact.Config {
	c := w.Config().Artifacts
	return artifact.ReadConfig(artifact.Said{
		Path: c.Path, Local: c.Local,
		Add: c.Add, Revise: c.Revise, Promote: c.Promote, Discard: c.Discard, Restore: c.Restore, Clear: c.Clear,
		ExpireAfter: c.ExpireAfter, Guide: c.Guide,
	}, w.WS().ResolveAlias)
}

// Artifacts is the workspace's artifacts: those on this machine, and
// those in the repo [artifacts] names, read from its ground and from the
// mission capsules of it, where crew's wait until they land.
func (w *Workspace) Artifacts() artifact.Store {
	st := artifact.Store{Root: w.Root()}
	c := w.ArtifactsConfig()
	if c.Repo == "" {
		return st
	}
	r := &artifact.Repo{Name: c.Repo, Ground: w.WS().MainWorktree(c.Repo), Dir: c.Dir}
	missions, _ := w.Missions.List()
	for _, m := range missions {
		for _, cap := range m.Capsules {
			if cap.Repo != c.Repo {
				continue
			}
			dir := filepath.Join(w.WS().RepoDir(c.Repo), cap.Name)
			if info, err := os.Stat(dir); err != nil || !info.IsDir() {
				continue
			}
			r.Capsules = append(r.Capsules, artifact.Capsule{Ref: c.Repo + "/" + cap.Name, Path: dir, Mission: m.Slug})
		}
	}
	st.Repo = r
	return st
}

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
// said to people: records:artifacts/2026-11-16-horizon-glow/, or
// .ws/artifacts/2026-11-16-horizon-glow/. A number or a -2 is worked out
// when it's added.
func (w *Workspace) ArtifactWhere(title string) string {
	c := w.ArtifactsConfig()
	if !c.InRepo() {
		return ".ws/artifacts/" + artifact.FolderName(w.now(), title) + "/"
	}
	parent := filepath.Join(w.WS().MainWorktree(c.Repo), filepath.FromSlash(c.Dir))
	var names []string
	if entries, err := os.ReadDir(parent); err == nil {
		for _, e := range entries {
			names = append(names, e.Name())
		}
	}
	name, _ := artifact.NameFolder(c.Name, w.now(), title, names, func(n string) bool {
		_, err := os.Lstat(filepath.Join(parent, n))
		return err == nil
	})
	return c.Repo + ":" + path.Join(c.Dir, name) + "/"
}

// ArtifactAdd is an artifact, or a version of one, to add.
type ArtifactAdd struct {
	HTML []byte
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
	// In is where crew ran it from: the capsule it's in is where theirs
	// goes, where artifacts are kept in a repo.
	In string
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
	// Path is the version's file.
	Path string
	// Warnings are what in it won't work in the web UI's frame.
	Warnings []string
	// Recorded is the mission it was recorded on: crew's.
	Recorded string
	// Commit is the commit it landed in, kept in a repo (yours), and
	// Landed what landing it did. Capsule is the capsule crew's waits in,
	// to land with it.
	Commit, Landed, Capsule string
}

// Ref is the version's ref: a3.v2.
func (a ArtifactAdded) Ref() string { return artifact.VersionRef(a.Entry.ID, a.Version) }

func (a ArtifactAdded) String() string {
	where := a.Entry.Where()
	if a.Capsule != "" && a.Entry.InFlight() == "" {
		where = a.Capsule + ":" + a.Entry.Folder + "/"
	}
	s := fmt.Sprintf("Added %s, %q, as %s%s", a.Ref(), a.Entry.Title, where, artifact.VersionFile(cmpInt(a.wasNumber(), a.Version)))
	if v, ok := a.Entry.Version(a.Version); ok && v.BasedOn != 0 && v.BasedOn != a.Version-1 {
		s += fmt.Sprintf(", based on v%d", v.BasedOn)
	}
	switch {
	case a.Landed != "":
		s += ": " + a.Landed
	case a.Capsule != "":
		s += " (in flight: commit it with your work, and it lands with the capsule"
		if was := a.wasNumber(); was != 0 {
			s += fmt.Sprintf(", as v%d, v%d having landed meanwhile", a.Version, was)
		}
		s += ")"
	case !a.Entry.InRepo():
		s += " (on this machine only)"
	}
	if a.Recorded != "" {
		s += ", and recorded it on " + a.Recorded
	}
	return s
}

// wasNumber is the number crew's version has in their capsule, where it's
// to land as another; 0 where it isn't.
func (a ArtifactAdded) wasNumber() int {
	if v, ok := a.Entry.Version(a.Version); ok {
		return v.Was
	}
	return 0
}

func cmpInt(a, b int) int {
	if a != 0 {
		return a
	}
	return b
}

// AddArtifact adds an artifact, or a version of one (To). A version goes
// where its artifact is; a new one where [artifacts] says: on this
// machine, or in a repo, where yours is a commit landed on its default
// branch and crew's is written in their capsule of it, to land with it.
// Crew's says it came from their mission, and is recorded on it; yours,
// added by hand, has you as its only provenance, and one you confirmed in
// a conversation says it came from there (From). The orchestrator makes
// none, and a conversation's agent proposes (cli refuses it).
func (w *Workspace) AddArtifact(a ArtifactAdd, by Actor) (ArtifactAdded, error) {
	switch by.Kind {
	case Orchestrator:
		return ArtifactAdded{}, refused("not-yours", "the orchestrator doesn't make artifacts: dispatch crew to make one")
	case WS:
		return ArtifactAdded{}, refused("not-yours", "ws adds no artifacts of its own")
	}
	checked, err := artifact.Check(a.HTML)
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
	// What it's added to, and where: a version where its artifact is, a
	// new one where [artifacts] says.
	add := artifactAdd{store: store, cfg: w.ArtifactsConfig(), html: a.HTML, title: artifact.CleanTitle(a.Title), v: v}
	if a.To == "" {
		if add.title == "" {
			add.title = checked.Title
		}
		if add.title == "" {
			return ArtifactAdded{}, invalid("not added: give it a title (--title), or give its HTML one (<title>)")
		}
		add.place = add.cfg.Place()
	} else {
		id, on, ok := artifact.ParseRef(a.To)
		if !ok {
			return ArtifactAdded{}, invalid("%q isn't an artifact: they're named a1, a2, … (ws artifact list)", a.To)
		}
		if a.On != 0 {
			on = a.On
		}
		e, err := store.Get(ctx, id)
		if err != nil {
			return ArtifactAdded{}, wrapArtifact(err)
		}
		add.to, add.on, add.place = &e, on, e.Place
	}
	switch {
	case add.place == artifact.Local && add.to == nil:
		out.Entry, err = store.Add(ctx, artifact.New{Title: add.title, HTML: a.HTML, Version: v, At: w.now()})
		out.Version = 1
	case add.place == artifact.Local:
		out.Entry, out.Version, err = store.AddVersion(ctx, add.to.ID, artifact.Revision{HTML: a.HTML, On: add.on, Title: a.Title, Version: v, At: w.now()})
	case store.Repo == nil || store.Repo.Name != add.place:
		return ArtifactAdded{}, invalid("%s is kept in %s, which [artifacts] in ws.toml doesn't name now: ws reads none there (ws doctor)", add.to.ID, add.place)
	case by.Kind == Crew:
		out, err = w.addArtifactInCapsule(add, m, by, a.In, out)
	default:
		out, err = w.addArtifactLanded(add, out)
	}
	if err != nil {
		return ArtifactAdded{}, wrapArtifact(err)
	}
	if out.Path == "" {
		out.Path = store.VersionPath(out.Entry, out.Version)
	}
	if m != nil {
		rec := mission.Artifact{Ref: out.Entry.ID, Kind: ArtifactKind, Label: out.Entry.Title, Crew: by.ID}
		if err := w.Missions.AddArtifact(m.Slug, rec); err != nil {
			return out, fmt.Errorf("added %s, but couldn't record it on %s: %w", out.Ref(), m.Slug, err)
		}
		out.Recorded = m.Slug
	}
	return out, nil
}

// artifactAdd is an artifact, or a version, being added: where (place:
// Local, or the repo's name), to what (nil for a new one), and what.
type artifactAdd struct {
	store artifact.Store
	cfg   artifact.Config
	place string
	to    *artifact.Entry
	on    int
	html  []byte
	title string
	v     artifact.Version
}

// addArtifactLanded adds yours to the repo artifacts are kept in: in a
// capsule lifted from its default branch as it is on the remote, the
// folder, or the version, committed with [artifacts]' message and its
// provenance in the body, then landed (the repo's check runs first).
// Someone else's version landing first, while this was, it's numbered
// again from what's there then, and tried again.
func (w *Workspace) addArtifactLanded(add artifactAdd, out ArtifactAdded) (ArtifactAdded, error) {
	repo, r := add.place, add.store.Repo
	if !w.WS().Direct(repo) {
		return out, invalid("artifacts are kept in %s, which takes pull requests, so each version would wait in review: keep them in a repo with no pull requests (landing = \"direct\"), or on this machine (path = \".ws\" under [artifacts] in ws.local.toml). ws doctor says the same", repo)
	}
	if e := add.to; e != nil {
		switch {
		case e.Status == artifact.Discarded:
			return out, &Refused{Code: "discarded", Reason: fmt.Sprintf("%s was discarded, so it takes no new versions: add a new artifact", e.ID)}
		case e.InFlight() != "":
			return out, invalid("%s hasn't landed: it's in %s, crew's, and lands with it. Add your version once it has", e.ID, e.InFlight())
		case e.Gone:
			return out, invalid("%s's folder has gone from %s: add a new artifact", e.ID, repo)
		}
	}
	slug := artifact.Slug(add.title)
	if add.to != nil {
		slug = artifact.SlugOf(add.cfg.Name, add.to.Folder)
	}
	for try := 1; ; try++ {
		l, err := w.liftForDefaultBranch(repo, w.freeBranch(repo, "artifact-"+slug))
		if err != nil {
			return out, err
		}
		l.land.Settle = settleNothing // a clash is tried again from the start, numbered as it is then
		var folder, subject string
		var n int
		var rec *artifact.Artifact
		at := w.now()
		f := artifact.Fields{Date: at.Local().Format("2006-01-02")}
		if add.to == nil {
			var name string
			v := add.v
			name, rec, err = artifact.MakeIn(filepath.Join(l.dir, filepath.FromSlash(r.Dir)), add.cfg.Name, artifact.New{Title: add.title, HTML: add.html, Version: v, At: at},
				func(name string) bool { return add.store.Named(repo, r.FolderPath(name)) })
			if err == nil {
				folder, n = r.FolderPath(name), 1
				g, _ := artifact.MatchName(add.cfg.Name, name)
				f.Slug, f.Number, f.Title, f.Version = cmpStr(g.Slug, artifact.SlugOf(add.cfg.Name, name)), g.Number, rec.Title, 1
				subject = f.Subject(add.cfg.Messages.Add)
			}
		} else {
			folder = add.to.Folder
			rec, n, err = artifact.ReviseIn(filepath.Join(l.dir, filepath.FromSlash(folder)), add.to.ID, artifact.Revision{HTML: add.html, On: add.on, Title: add.title, Version: add.v, At: at}, 0)
			if err == nil {
				g, _ := artifact.MatchName(add.cfg.Name, path.Base(folder))
				f.Date = cmpStr(g.Date, f.Date)
				f.Slug, f.Number, f.Title, f.Version = artifact.SlugOf(add.cfg.Name, folder), g.Number, rec.Title, n
				subject = f.Subject(add.cfg.Messages.Revise)
			}
		}
		if err != nil {
			l.undo()
			return out, err
		}
		v, _ := rec.Version(n)
		saved, err := l.commitAndLand(folder, subject, artifact.Body(v), "Added", true)
		var stopped *workspace.LandError
		if errors.As(err, &stopped) && stopped.Step == "rebase" && try < workspace.LandTries {
			continue // someone else's landed first: numbered again from theirs
		}
		if err != nil {
			return out, err
		}
		out.Commit, out.Landed, out.Version = saved.Commit, strings.TrimPrefix(saved.Message, "Added: "), n
		e, err := add.store.At(context.Background(), repo, folder)
		if err != nil {
			// Landed, but the ground hasn't it yet: said as it was written.
			e = artifact.Entry{Place: repo, Folder: folder, Artifact: *rec}
		}
		out.Entry = e
		return out, nil
	}
}

// settleNothing settles no clash: the landing stops at it.
func settleNothing(string, []string) ([]string, bool) { return nil, false }

func cmpStr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// addArtifactInCapsule adds crew's to the repo artifacts are kept in: in
// their mission's capsule of it (the one they're in, else the one their
// crew works in, else its only one), written there, for them to commit
// with the rest of their work. It lands when the capsule does, and until
// then it's in flight. A version of one that landed after their capsule
// was lifted brings the artifact into it as the ground has it.
func (w *Workspace) addArtifactInCapsule(add artifactAdd, m *mission.Mission, by Actor, in string, out ArtifactAdded) (ArtifactAdded, error) {
	repo, r := add.place, add.store.Repo
	c, err := w.crewCapsuleOf(m, by, repo, in)
	if err != nil {
		return out, err
	}
	var folder string
	var n int
	ctx := context.Background()
	if add.to == nil {
		var name string
		name, _, err = artifact.MakeIn(filepath.Join(c.Path, filepath.FromSlash(r.Dir)), add.cfg.Name, artifact.New{Title: add.title, HTML: add.html, Version: add.v, At: w.now()},
			func(name string) bool {
				_, err := os.Lstat(filepath.Join(r.Ground, filepath.FromSlash(r.FolderPath(name))))
				return err == nil || add.store.Named(repo, r.FolderPath(name))
			})
		folder, n = r.FolderPath(name), 1
	} else {
		e := add.to
		switch {
		case e.Status == artifact.Discarded:
			return out, &Refused{Code: "discarded", Reason: fmt.Sprintf("%s was discarded, so it takes no new versions: add a new artifact", e.ID)}
		case e.InFlight() != "" && e.InFlight() != c.Ref:
			return out, invalid("%s hasn't landed: it's in %s, another capsule, and lands with it. Add yours once it has", e.ID, e.InFlight())
		}
		folder = e.Folder
		dir := filepath.Join(c.Path, filepath.FromSlash(folder))
		if _, err := os.Stat(filepath.Join(dir, artifact.RecordFile)); err != nil {
			// It landed after the capsule was lifted: it's brought in as
			// the ground has it.
			if err := copyArtifact(filepath.Join(r.Ground, filepath.FromSlash(folder)), dir); err != nil {
				return out, fmt.Errorf("%s isn't in %s, and couldn't be brought in from %s's ground: %w", e.ID, c.Ref, repo, err)
			}
		}
		past := 0
		for _, v := range e.Versions {
			if v.Capsule == "" && v.N > past {
				past = v.N
			}
		}
		_, n, err = artifact.ReviseIn(dir, e.ID, artifact.Revision{HTML: add.html, On: add.on, Title: add.title, Version: add.v, At: w.now()}, past)
	}
	if err != nil {
		return out, err
	}
	out.Capsule = c.Ref
	e, err := add.store.At(ctx, repo, folder)
	if err != nil {
		return out, err
	}
	out.Entry, out.Version = e, n
	// Listed under the number it'll land as, where another's taken it.
	for _, v := range e.Versions {
		if v.Capsule == c.Ref && cmpInt(v.Was, v.N) == n {
			out.Version = v.N
		}
	}
	out.Path = filepath.Join(c.Path, filepath.FromSlash(folder), artifact.VersionFile(n))
	return out, nil
}

// crewCapsuleOf is the capsule of repo crew's artifacts go in: of their
// mission, the one they're in (in), else the one their crew works in,
// else its only one. None: they're to lift one.
func (w *Workspace) crewCapsuleOf(m *mission.Mission, by Actor, repo, in string) (artifact.Capsule, error) {
	var mine []artifact.Capsule
	for _, c := range m.Capsules {
		if c.Repo != repo {
			continue
		}
		dir := filepath.Join(w.WS().RepoDir(repo), c.Name)
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			mine = append(mine, artifact.Capsule{Ref: repo + "/" + c.Name, Path: dir, Mission: m.Slug})
		}
	}
	if len(mine) == 0 {
		return artifact.Capsule{}, invalid("artifacts are kept in %s, and crew's go in their mission's capsule of it, to land with the rest of their work: %s has none. Lift one (ws lift %s <name>), and add it again", repo, m.Slug, repo)
	}
	if in != "" {
		for _, c := range mine {
			if in == c.Path || strings.HasPrefix(in, c.Path+string(filepath.Separator)) {
				return c, nil
			}
		}
	}
	for _, crew := range m.Crew {
		if crew.ID == by.ID {
			for _, c := range mine {
				if c.Ref == crew.Capsule {
					return c, nil
				}
			}
		}
	}
	if len(mine) == 1 {
		return mine[0], nil
	}
	var refs []string
	for _, c := range mine {
		refs = append(refs, c.Ref)
	}
	return artifact.Capsule{}, invalid("artifacts are kept in %s, and %s has %s: run it from the one it's to go in", repo, m.Slug, strings.Join(refs, " and "))
}

// copyArtifact copies an artifact's folder (its record and its files) to
// to, which mustn't be there.
func copyArtifact(from, to string) error {
	entries, err := os.ReadDir(from)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(to, 0o755); err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(from, e.Name()))
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(to, e.Name()), data, 0o644); err != nil {
			return err
		}
	}
	return nil
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
