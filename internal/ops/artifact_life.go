package ops

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/withworkspace/plugin/protocol"
	"github.com/withworkspace/workspace/internal/artifact"
	"github.com/withworkspace/workspace/internal/events"
	"github.com/withworkspace/workspace/internal/git"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/space"
)

// An artifact's lifetime: you promote one worth keeping, choosing a
// version and saying what it went into; discard one, and restore it; and
// the expired go. Each is a change to its folder on this machine.
//
// A promoted one may live in a repo too, for everyone with it: its home,
// a folder there holding the version chosen. Where that is is chosen the
// first time it's promoted (what [artifacts] path says, or a folder you
// name), and kept in its record: promoting it again, four versions on,
// puts the version chosen then in the same folder, in one commit landed
// on the repo's default branch, with nothing asked. Discarding it takes
// its folder out of the repo, and restoring it puts it back.
//
// Only you do any of it. Crew say in their handoff which they'd keep or
// drop, and why; ws itself discards only what's expired (the daemon,
// ExpireArtifacts), which is never in a repo: only a live one expires.

// ArtifactPromote is an artifact to promote: the version chosen (in Ref,
// a3.v2, or Version; its latest for neither), and what it went into: a
// mission (its slug) or a document (its ref) in Into, or a mission started
// from it (Start, what the mission is).
type ArtifactPromote struct {
	Ref     string
	Version int
	Into    string
	Start   string
	// Home is where it's to live in a repo, for one that lives in none
	// yet: a folder, as repo:folder/. "" is where it lives already, or,
	// for one that lives in none, where [artifacts] says promoted ones do
	// (nowhere: it stays on this machine).
	Home string
	// Here keeps one that lives in no repo on this machine only, whatever
	// [artifacts] says.
	Here bool
}

// ArtifactDone is what promoting, discarding or restoring an artifact
// did.
type ArtifactDone struct {
	Entry artifact.Entry
	// Home is the folder in a repo it was put in, taken out of, or put
	// back in (records:artifacts/…/), Commit the commit that did, and
	// Landed what landing it did. First: it's the first time it was put
	// there.
	Home, Commit, Landed string
	First                bool
	// Into is what it was promoted into, as its record says it; Started,
	// a mission started from it, and Recorded the mission it was recorded
	// on (that one, or the one it went into).
	Into, Started, Recorded string
}

// youOnly refuses anyone but you an artifact's lifetime.
func youOnly(by Actor, what string) error {
	if by.Kind == You {
		return nil
	}
	return refused("not-yours", "only the user %s an artifact: say in your handoff which, and why", what)
}

// artifactRef reads an artifact's ref, a3, and the version it names.
func artifactRef(ref string) (string, int, error) {
	id, v, ok := artifact.ParseRef(ref)
	if !ok {
		return "", 0, invalid("%q is not an artifact: artifacts are named a1, a2, … (ws artifact list)", ref)
	}
	return id, v, nil
}

// homeFor is the folder in a repo a promoted artifact is to be put in:
// the repo, the folder ("" for one to be named as [artifacts] names
// them, once the repo's default branch is in hand), and whether it's the
// first time.
type homeFor struct {
	repo, folder string
	first        bool
}

// homeOf works out where a promotion puts an artifact: where it lives;
// or, for one that lives in no repo, the folder asked for, else, unless
// it was chosen to stay here, where [artifacts] says promoted ones live.
// Nil is nowhere: it stays on this machine, and here says that's chosen
// now.
func (w *Workspace) homeOf(e artifact.Entry, cfg artifact.Config, p ArtifactPromote) (h *homeFor, here bool, err error) {
	asked := strings.TrimSpace(p.Home)
	switch {
	case p.Here && asked != "":
		return nil, false, invalid("choose %s or this machine only, not both", asked)
	case e.Home != nil:
		if p.Here {
			return nil, false, invalid("%s already lives in %s: cannot keep on this machine only", e.ID, e.Home.Where())
		}
		if asked != "" {
			repo, folder, err := w.homeNamed(asked)
			if err != nil {
				return nil, false, err
			}
			if repo != e.Home.Repo || folder != e.Home.Folder {
				return nil, false, invalid("%s already lives in %s and cannot be moved", e.ID, e.Home.Where())
			}
		}
		h = &homeFor{repo: e.Home.Repo, folder: e.Home.Folder}
	case p.Here:
		return nil, true, nil
	case asked != "":
		repo, folder, err := w.homeNamed(asked)
		if err != nil {
			return nil, false, err
		}
		h = &homeFor{repo: repo, folder: folder, first: true}
	case cfg.Homes() && !e.Here:
		h = &homeFor{repo: cfg.Repo, first: true}
	default:
		return nil, false, nil
	}
	if !w.WS().Direct(h.repo) {
		return nil, false, invalid("%s takes pull requests, so a promoted artifact would wait in review: use a repo with landing = \"direct\", or keep on this machine (ws artifact promote --here)", h.repo)
	}
	return h, false, nil
}

// homeNamed reads a home as it's named: repo:folder/, a folder of one of
// the workspace's repos.
func (w *Workspace) homeNamed(s string) (repo, folder string, err error) {
	name, rest, ok := strings.Cut(s, ":")
	rest = strings.Trim(rest, "/")
	if !ok || name == "" || rest == "" {
		return "", "", invalid("%q is not a folder in a repo: use repo:folder/, as records:artifacts/horizon-glow/", s)
	}
	parts := strings.Split(rest, "/")
	if slices.Contains(parts, "..") || slices.Contains(parts, ".") || slices.Contains(parts, "") || parts[0] == ".git" {
		return "", "", invalid("%q is not a folder inside %s", s, name)
	}
	repo, known := w.WS().ResolveAlias(name)
	if !known {
		return "", "", invalid("%s is not a repo in this workspace", name)
	}
	return repo, path.Clean(rest), nil
}

// ArtifactHomeOffered is the folder in a repo an artifact has, or is
// offered, to live in, as it's said to people
// (records:artifacts/2026-11-12-horizon-glow/): where it lives, or, for
// one that lives in none, the folder [artifacts] names for it, as the
// repo's ground stands. "" where there's none to offer.
func (w *Workspace) ArtifactHomeOffered(e artifact.Entry) string {
	if e.Home != nil {
		return e.Home.Where()
	}
	cfg := w.ArtifactsConfig()
	if !cfg.Homes() {
		return ""
	}
	name, err := homeName(cfg, e, filepath.Join(w.WS().MainWorktree(cfg.Repo), filepath.FromSlash(cfg.Dir)))
	if err != nil {
		return ""
	}
	return cfg.Repo + ":" + path.Join(cfg.Dir, name) + "/"
}

// homeName names an artifact's folder among those in parent (the folder
// of a worktree [artifacts] keeps them in): its own name, where the repo
// names them the same way and it's free, else named as [artifacts] says,
// from the day it was made.
func homeName(cfg artifact.Config, e artifact.Entry, parent string) (string, error) {
	var names []string
	if entries, err := os.ReadDir(parent); err == nil {
		for _, d := range entries {
			names = append(names, d.Name())
		}
	}
	taken := func(name string) bool { return slices.Contains(names, name) }
	if _, ok := artifact.MatchName(cfg.Name, e.Folder); ok && !taken(e.Folder) {
		return e.Folder, nil
	}
	return artifact.NameFolder(cfg.Name, e.Made, e.Title, names, taken)
}

// PromoteArtifact promotes an artifact: it's kept for good, with the
// version chosen and what it went into, if anything, and the version
// chosen is put where it lives in a repo, if it lives in one (homeOf).
// Into a mission, it's recorded on it as one of its artifacts, so its
// crew read it; into a document, the document cites it by its folder in
// the repo, so it must live in one. Promoted again, it says what it says
// now.
func (w *Workspace) PromoteArtifact(p ArtifactPromote, by Actor) (ArtifactDone, error) {
	if err := youOnly(by, "promotes"); err != nil {
		return ArtifactDone{}, err
	}
	id, v, err := artifactRef(p.Ref)
	if err != nil {
		return ArtifactDone{}, err
	}
	switch {
	case p.Version != 0 && v != 0 && p.Version != v:
		return ArtifactDone{}, invalid("%s names v%d, and --version v%d: choose one", p.Ref, v, p.Version)
	case p.Version != 0:
		v = p.Version
	}
	into, start := strings.TrimSpace(p.Into), strings.TrimSpace(p.Start)
	if into != "" && start != "" {
		return ArtifactDone{}, invalid("promote into a mission or document, or start a new mission: not both")
	}
	store, cfg := w.Artifacts(), w.ArtifactsConfig()
	ctx := context.Background()
	e, err := store.Get(ctx, id)
	if err != nil {
		return ArtifactDone{}, wrapArtifact(err)
	}
	switch {
	case e.Status == artifact.Discarded:
		return ArtifactDone{}, &Refused{Code: "discarded", Reason: fmt.Sprintf("%s was discarded: restore first (ws artifact restore %s)", id, id)}
	case e.Gone || e.Away():
		return ArtifactDone{}, invalid("%s's folder is not on this machine: nothing to keep", id)
	}
	if v == 0 {
		latest, _ := e.Latest()
		v = latest.N
	}
	if _, ok := e.Version(v); !ok {
		return ArtifactDone{}, invalid("%s has no v%d: ws show %s lists its versions", id, v, id)
	}
	home, here, err := w.homeOf(e, cfg, p)
	if err != nil {
		return ArtifactDone{}, err
	}
	// What it went into: a mission that's there, or a document, which
	// cites it by its folder in the repo.
	var went string
	var m *mission.Mission
	switch {
	case into != "":
		if got, err := w.Missions.Load(into); err == nil {
			m, went = got, got.Slug
			break
		}
		d, err := w.FindDoc(into)
		if err != nil {
			return ArtifactDone{}, invalid("%q is not a mission or a document: --into takes a mission slug or a document ref (repo:path)", into)
		}
		if home == nil {
			return ArtifactDone{}, invalid("documents cite artifacts by their folder in a repo, and %s would stay on this machine: choose a folder (--home repo:folder/), or set [artifacts] path in ws.toml", id)
		}
		went = d.Ref
	case start != "":
		// Started once everything that can be checked first has been: a
		// promotion its landing stops leaves the mission, and says so.
		if m, err = w.Missions.CreateBriefing(start); err != nil {
			return ArtifactDone{}, wrap(err)
		}
		went = m.Slug
	}
	promo := artifact.Promotion{At: w.now(), By: by.By(), Version: v, Into: went}
	var out ArtifactDone
	var lives *artifact.Home
	if home != nil {
		out, lives, err = w.putHome(store, cfg, e, promo, home, cfg.Messages.Promote, "Promoted")
	}
	if err == nil {
		out.Entry, err = store.Promote(ctx, id, promo, lives, here)
		if err != nil && lives != nil {
			err = fmt.Errorf("v%d is in %s (%s), but could not update %s's record on this machine: %w", v, lives.Where(), out.Landed, id, err)
		}
	}
	if err != nil {
		if start != "" {
			return ArtifactDone{}, fmt.Errorf("%w (mission %s was started, and waits to be briefed)", wrapArtifact(err), went)
		}
		return ArtifactDone{}, wrapArtifact(err)
	}
	out.Into = went
	promoted := events.Event{Kind: events.ArtifactPromoted, Artifact: id, Text: fmt.Sprintf("v%d of %s", v, out.Entry.Title)}
	switch {
	case m != nil:
		promoted.Mission = m.Slug
	case went != "":
		promoted.Doc = went
	}
	w.emit(promoted, by)
	if m != nil {
		if start != "" {
			out.Started = m.Slug
		}
		rec := mission.Artifact{Ref: id, Kind: ArtifactKind, Label: out.Entry.Title}
		if err := w.Missions.AddArtifact(m.Slug, rec); err != nil {
			return out, fmt.Errorf("promoted %s, but could not record on %s: %w", id, m.Slug, err)
		}
		out.Recorded = m.Slug
	}
	return out, nil
}

// homeFields is what the messages of an artifact's commits in the repo it
// lives in are filled in from: its folder's slug, date and number there,
// as its name says them, and its title.
func homeFields(cfg artifact.Config, e artifact.Entry, folder string, version int) artifact.Fields {
	g, _ := artifact.MatchName(cfg.Name, path.Base(folder))
	return artifact.Fields{
		Date:    cmpStr(g.Date, e.Made.Local().Format("2006-01-02")),
		Slug:    cmpStr(g.Slug, artifact.SlugOf(cfg.Name, folder)),
		Number:  g.Number,
		Title:   e.Title,
		Version: version,
	}
}

// promoteBody is the body of the commit that puts a promoted artifact's
// version where it lives: who, the version chosen and what it went into,
// what the version is, and that the rest of its versions aren't here.
func promoteBody(p artifact.Promotion, a *artifact.Artifact, first bool) string {
	latest, _ := a.Latest()
	s := fmt.Sprintf("Promoted by %s: v%d of %d chosen", p.By, p.Version, latest.N)
	if p.Into != "" {
		s += ", into " + p.Into
	}
	s += "."
	if v, ok := a.Version(p.Version); ok {
		s += "\n\n" + artifact.Body(v)
	}
	if first {
		s += fmt.Sprintf("\n\nThis folder holds the version chosen (%s, and the files beside it) and a record of it (%s). Its other versions are on the machine it was promoted on.", artifact.HomeFile, artifact.RecordFile)
	}
	return s
}

// inLift is a change to a repo's default branch, made in a capsule lifted
// from it as it is on the remote, committed and landed: change makes it,
// in the capsule, and says the subject and body of its commit, and the
// path in the repo to commit (rel). Nothing's left of one that stops.
// Someone else's change to the same lines landing between the lift and
// the push stops it, unsettled.
func (w *Workspace) inLift(repo, branch, what string, change func(capsule string) (rel, subject, body string, add bool, err error)) (*DocSaved, error) {
	l, err := w.liftForDefaultBranch(repo, w.freeBranch(repo, branch))
	if err != nil {
		return nil, err
	}
	l.land.Settle = settleNothing
	rel, subject, body, add, err := change(l.dir)
	if err != nil {
		l.undo()
		return nil, err
	}
	return l.commitAndLand(rel, subject, body, what, add)
}

// settleNothing settles no clash: the landing stops at it.
func settleNothing(string, []string) ([]string, bool) { return nil, false }

// putHome puts a version of an artifact where it lives in a repo: its
// files and a record of it, in the folder (named now, for one [artifacts]
// names), in place of whatever was there, in one commit landed on the
// repo's default branch. msg is the commit's first line, as [artifacts]
// has it, and what how what it did is said ("Promoted", "Restored"). It's
// where it lives now, as its record is to say.
func (w *Workspace) putHome(store artifact.Store, cfg artifact.Config, e artifact.Entry, p artifact.Promotion, h *homeFor, msg, what string) (ArtifactDone, *artifact.Home, error) {
	files, _, _, err := store.AllFiles(context.Background(), e.ID, p.Version)
	if err != nil {
		return ArtifactDone{}, nil, err
	}
	folder := h.folder
	saved, err := w.inLift(h.repo, "artifact-"+artifact.SlugOf("{date}-{slug}", e.Folder), what, func(capsule string) (string, string, string, bool, error) {
		switch {
		case folder == "":
			name, err := homeName(cfg, e, filepath.Join(capsule, filepath.FromSlash(cfg.Dir)))
			if err != nil {
				return "", "", "", false, err
			}
			folder = path.Join(cfg.Dir, name)
		case h.first:
			if _, err := os.Lstat(filepath.Join(capsule, filepath.FromSlash(folder))); err == nil {
				return "", "", "", false, invalid("%s:%s/ already exists: choose another folder for %s", h.repo, folder, e.ID)
			}
		}
		dir := filepath.Join(capsule, filepath.FromSlash(folder))
		if err := artifact.WriteHome(dir, &e.Artifact, p, files); err != nil {
			return "", "", "", false, fmt.Errorf("could not write %s into %s: %w", artifact.VersionRef(e.ID, p.Version), h.repo, err)
		}
		// What says the folder is an artifact, to every ws that has the
		// repo: it's found there, as a plugin's thing in a repo is.
		manifest, err := space.Manifest(artifact.ThingKind, e.Title, artifact.MainFile, "")
		if err != nil {
			return "", "", "", false, err
		}
		if err := os.WriteFile(filepath.Join(dir, protocol.ManifestName), manifest, 0o644); err != nil {
			return "", "", "", false, err
		}
		body := promoteBody(p, &e.Artifact, h.first)
		if what == "Restored" {
			body = fmt.Sprintf("Restored by %s: put back as it was, at v%d.", p.By, p.Version)
		}
		return folder, homeFields(cfg, e, folder, p.Version).Subject(msg), body, true, nil
	})
	if err != nil {
		return ArtifactDone{}, nil, err
	}
	lives := &artifact.Home{Repo: h.repo, Folder: folder, Version: p.Version, Commit: saved.Commit, At: w.now()}
	return ArtifactDone{Home: lives.Where(), Commit: saved.Commit, Landed: strings.TrimPrefix(saved.Message, what+": "), First: h.first}, lives, nil
}

// DiscardArtifact discards an artifact: you do, saying why if you like.
// Its folder goes to .ws/artifacts/.discarded, deleted for good 30 days
// on, and where it lives in a repo, its folder there is taken away first,
// in a commit landed on the repo's default branch. Its ref still says
// what it was, and restoring it puts it back, there too.
func (w *Workspace) DiscardArtifact(ref, reason string, by Actor) (ArtifactDone, error) {
	if err := youOnly(by, "discards"); err != nil {
		return ArtifactDone{}, err
	}
	id, v, err := artifactRef(ref)
	if err != nil {
		return ArtifactDone{}, err
	}
	if v != 0 {
		return ArtifactDone{}, invalid("artifacts are discarded with all versions: discard %s", id)
	}
	store, cfg := w.Artifacts(), w.ArtifactsConfig()
	ctx := context.Background()
	e, err := store.Get(ctx, id)
	if err != nil {
		return ArtifactDone{}, wrapArtifact(err)
	}
	d := artifact.Discard{At: w.now(), By: by.By(), Reason: strings.TrimSpace(reason)}
	var out ArtifactDone
	if h := e.Home; h != nil && e.Status != artifact.Discarded && !e.Gone {
		saved, err := w.inLift(h.Repo, "artifact-"+artifact.SlugOf("{date}-{slug}", e.Folder), "Discarded", func(capsule string) (string, string, string, bool, error) {
			if _, err := os.Stat(filepath.Join(capsule, filepath.FromSlash(h.Folder))); err != nil {
				return "", "", "", false, errNotThere // taken away already
			}
			if out, err := git.Combined(capsule, "rm", "-r", "-q", "--", h.Folder); err != nil {
				return "", "", "", false, fmt.Errorf("could not remove %s: %v: %s", h.Folder, err, clipString(out, 300))
			}
			return h.Folder, homeFields(cfg, e, h.Folder, h.Version).Subject(cfg.Messages.Discard), artifact.DiscardBody(d.By, d.Reason, &e.Artifact), false, nil
		})
		switch {
		case errors.Is(err, errNotThere):
		case err != nil:
			return ArtifactDone{}, err
		default:
			out = ArtifactDone{Home: h.Where(), Commit: saved.Commit, Landed: strings.TrimPrefix(saved.Message, "Discarded: ")}
		}
	}
	out.Entry, err = store.Discard(ctx, id, d)
	if err != nil && out.Landed != "" {
		err = fmt.Errorf("folder removed from %s (%s), but could not discard on this machine: %w", e.Home.Repo, out.Landed, err)
	}
	return out, wrapArtifact(err)
}

var errNotThere = errors.New("not there")

// RestoreArtifact puts a discarded artifact back, as it was before it was
// discarded (live, or promoted if it had been): its folder out of
// .discarded, and, where it lived in a repo, the version that was there
// put back, in a commit landed on its default branch. Its expiry counts
// from now.
func (w *Workspace) RestoreArtifact(ref string, by Actor) (ArtifactDone, error) {
	if err := youOnly(by, "restores"); err != nil {
		return ArtifactDone{}, err
	}
	id, v, err := artifactRef(ref)
	if err != nil {
		return ArtifactDone{}, err
	}
	if v != 0 {
		return ArtifactDone{}, invalid("artifacts are restored with all versions: restore %s", id)
	}
	store, cfg := w.Artifacts(), w.ArtifactsConfig()
	ctx := context.Background()
	e, err := store.Restore(ctx, id, artifact.Restoration{At: w.now(), By: by.By()})
	if err != nil {
		return ArtifactDone{}, wrapArtifact(err)
	}
	out := ArtifactDone{Entry: e}
	h := e.Home
	if h == nil || e.Promoted == nil {
		return out, nil
	}
	// It lived in a repo: the version that was there goes back.
	back := func() error {
		if !w.WS().Direct(h.Repo) {
			return fmt.Errorf("%s takes pull requests now", h.Repo)
		}
		p := *e.Promoted
		p.Version, p.By = h.Version, by.By()
		done, lives, err := w.putHome(store, cfg, e, p, &homeFor{repo: h.Repo, folder: h.Folder}, cfg.Messages.Restore, "Restored")
		if err != nil {
			return err
		}
		out.Home, out.Commit, out.Landed = done.Home, done.Commit, done.Landed
		out.Entry, err = store.PutHome(ctx, id, *lives)
		return err
	}
	if err := back(); err != nil {
		return out, fmt.Errorf("restored %s on this machine, but could not put back in %s: %w. Promote again to put it there", id, h.Where(), err)
	}
	return out, nil
}

// ArtifactsCleared is what clearing the expired did: those it discarded.
type ArtifactsCleared struct {
	Local []artifact.Entry
}

// ClearExpiredArtifacts clears the expired artifacts now, as you: each is
// discarded, "expired" its reason, as the daemon would within the day.
func (w *Workspace) ClearExpiredArtifacts(by Actor) (ArtifactsCleared, error) {
	if err := youOnly(by, "clears expired"); err != nil {
		return ArtifactsCleared{}, err
	}
	var out ArtifactsCleared
	var err error
	out.Local, err = w.discardExpired(by)
	return out, err
}

// discardExpired discards every expired artifact, by by, and says which.
func (w *Workspace) discardExpired(by Actor) ([]artifact.Entry, error) {
	store, cfg := w.Artifacts(), w.ArtifactsConfig()
	ctx := context.Background()
	list, err := store.List(ctx)
	if err != nil {
		return nil, err
	}
	now := w.now()
	var out []artifact.Entry
	for _, e := range list {
		if !cfg.Expired(e, now) {
			continue
		}
		d, err := store.Discard(ctx, e.ID, artifact.Discard{At: now, By: by.By(), Reason: "expired"})
		if err != nil {
			return out, fmt.Errorf("could not discard %s: %w", e.ID, wrapArtifact(err))
		}
		out = append(out, d)
	}
	return out, nil
}

// BringArtifactsHome brings onto this machine the artifacts that were
// kept in a repo whole, from before they all lived here (ws doctor, and
// the daemon as it starts): those it brought.
func (w *Workspace) BringArtifactsHome() ([]artifact.Entry, error) {
	return w.Artifacts().BringHome(context.Background(), func(repo string) string {
		if name, known := w.WS().ResolveAlias(repo); known {
			if dir := w.WS().MainWorktree(name); dir != "" {
				if info, err := os.Stat(dir); err == nil && info.IsDir() {
					return dir
				}
			}
		}
		return ""
	})
}

// ExpiredArtifacts is what the daemon's sweep did: the artifacts it
// discarded, expired, those it deleted for good, discarded 30 days
// before, those it brought onto this machine from a repo they were kept
// in whole, and those whose versions it put in a repo of their own, from
// when each was a file.
type ExpiredArtifacts struct {
	Discarded, Erased, Brought, Converted []artifact.Entry
}

// ExpireArtifacts is the daemon's sweep of the artifacts: each expired
// one is discarded, by ws, "expired" its reason, and each folder that's
// been in .discarded for 30 days is deleted for good.
func (w *Workspace) ExpireArtifacts() (ExpiredArtifacts, error) {
	var out ExpiredArtifacts
	var err error
	if out.Brought, err = w.BringArtifactsHome(); err != nil {
		return out, err
	}
	if out.Converted, err = w.Artifacts().Convert(context.Background()); err != nil {
		return out, err
	}
	if out.Discarded, err = w.discardExpired(AsWS); err != nil {
		return out, err
	}
	out.Erased, err = w.Artifacts().Erase(context.Background(), w.now().Add(-artifact.KeepDiscarded))
	return out, err
}
