package ops

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/artifact"
	"github.com/withworkspace/workspace/internal/git"
	"github.com/withworkspace/workspace/internal/mission"
)

// An artifact's lifetime: you promote one worth keeping, choosing a
// version and saying what it went into; discard one, and restore it; and
// the expired go. On this machine each is a change to its folder; in the
// repo artifacts are kept in, a commit landed on its default branch, as
// adding one is.
//
// Only you do any of it. Crew say in their handoff which they'd keep or
// drop, and why; ws itself discards only what's expired on this machine
// (the daemon, ExpireArtifacts), and commits nothing to a shared branch
// on its own: in a repo, expired ones wait for Clear expired.

// ArtifactPromote is an artifact to promote: the version chosen (in Ref,
// a3.v2, or Version; its latest for neither), and what it went into: a
// mission (its slug) or a document (its ref) in Into, or a mission started
// from it (Start, what the mission is). Move moves one on this machine
// into the repo artifacts are kept in, in the same commit.
type ArtifactPromote struct {
	Ref     string
	Version int
	Into    string
	Start   string
	Move    bool
}

// ArtifactDone is what promoting, discarding or restoring an artifact
// did.
type ArtifactDone struct {
	Entry artifact.Entry
	// Commit is the commit it landed in, kept in a repo, and Landed what
	// landing did.
	Commit, Landed string
	// Moved: promoted, it moved from this machine into the repo.
	Moved bool
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
		return "", 0, invalid("%q isn't an artifact: they're named a1, a2, … (ws artifact list)", ref)
	}
	return id, v, nil
}

// PromoteArtifact promotes an artifact: it's kept for good, with the
// version chosen and what it went into, if anything. Into a mission, it's
// recorded on it as one of its artifacts, so its crew read it; into a
// document, the document cites it by its folder in the repo, so it must be
// there (Move takes one on this machine there). Promoted again, it says
// what it says now.
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
		return ArtifactDone{}, invalid("it goes into a mission or a document, or a mission's started from it: not both")
	}
	store, cfg := w.Artifacts(), w.ArtifactsConfig()
	ctx := context.Background()
	e, err := store.Get(ctx, id)
	if err != nil {
		return ArtifactDone{}, wrapArtifact(err)
	}
	switch {
	case e.Status == artifact.Discarded:
		return ArtifactDone{}, &Refused{Code: "discarded", Reason: fmt.Sprintf("%s was discarded: restore it first (ws artifact restore %s)", id, id)}
	case e.Gone:
		return ArtifactDone{}, invalid("%s's folder has gone from %s: there's nothing to keep", id, e.Kept())
	case e.InFlight() != "":
		return ArtifactDone{}, invalid("%s hasn't landed: it's in %s, crew's, and lands with it. Promote it once it has", id, e.InFlight())
	}
	if v == 0 {
		for _, ver := range e.Versions {
			if ver.Capsule == "" && ver.N > v {
				v = ver.N // the latest that's landed
			}
		}
	}
	chosen, ok := e.Version(v)
	switch {
	case !ok:
		return ArtifactDone{}, invalid("%s has no v%d: ws show %s lists its versions", id, v, id)
	case chosen.Capsule != "":
		return ArtifactDone{}, invalid("%s is in flight, crew's, in %s: choose a version that's landed, or promote it once it has", artifact.VersionRef(id, v), chosen.Capsule)
	}
	if e.InRepo() && (store.Repo == nil || store.Repo.Name != e.Place) {
		return ArtifactDone{}, invalid("%s is kept in %s, which [artifacts] in ws.toml doesn't name now: ws reads none there (ws doctor)", id, e.Place)
	}
	move := p.Move && !e.InRepo()
	if move {
		switch {
		case cfg.Repo == "":
			return ArtifactDone{}, invalid("there's no repo for artifacts to move %s into: [artifacts] path in ws.toml names one", id)
		case !w.WS().Direct(cfg.Repo):
			return ArtifactDone{}, invalid("artifacts are kept in %s, which takes pull requests: ws doesn't move one there (ws doctor)", cfg.Repo)
		}
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
			return ArtifactDone{}, invalid("%q is neither a mission nor a document: --into takes a mission's slug, or a document's ref (repo:path)", into)
		}
		if !e.InRepo() && !move {
			if cfg.Repo == "" {
				return ArtifactDone{}, invalid("a document cites an artifact by its folder in a repo, so the link works for everyone, and %s is on this machine, with no repo for artifacts ([artifacts] path in ws.toml): promote it into nothing, or a mission", id)
			}
			return ArtifactDone{}, invalid("a document cites an artifact by its folder in a repo, so the link works for everyone, and %s is on this machine: move it to %s as you promote it (--move)", id, cfg.Repo)
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
	switch {
	case move:
		out, err = w.promoteMoving(store, cfg, e, promo)
	case e.InRepo():
		out, err = w.promoteLanded(store, cfg, e, promo)
	default:
		out.Entry, err = store.Promote(ctx, id, promo)
	}
	if err != nil {
		if start != "" {
			return ArtifactDone{}, fmt.Errorf("%w (mission %s was started from it, and waits to be briefed)", wrapArtifact(err), went)
		}
		return ArtifactDone{}, wrapArtifact(err)
	}
	out.Into = went
	if m != nil {
		if start != "" {
			out.Started = m.Slug
		}
		rec := mission.Artifact{Ref: id, Kind: ArtifactKind, Label: out.Entry.Title}
		if err := w.Missions.AddArtifact(m.Slug, rec); err != nil {
			return out, fmt.Errorf("promoted %s, but couldn't record it on %s: %w", id, m.Slug, err)
		}
		out.Recorded = m.Slug
	}
	return out, nil
}

// fieldsOf is what an artifact's commits' messages are filled in from: its
// folder's slug, date and number, as its name says them, and its title.
func fieldsOf(cfg artifact.Config, e artifact.Entry) artifact.Fields {
	g, _ := artifact.MatchName(cfg.Name, path.Base(e.Folder))
	return artifact.Fields{Date: cmpStr(g.Date, e.Made.Local().Format("2006-01-02")), Slug: artifact.SlugOf(cfg.Name, e.Folder), Number: g.Number, Title: e.Title}
}

// promoteBody is the body of the commit that promotes an artifact: who,
// the version chosen and what it went into, and, moved from this machine,
// that.
func promoteBody(p artifact.Promotion, a *artifact.Artifact, moved bool) string {
	latest, _ := a.Latest()
	s := fmt.Sprintf("Promoted by %s: v%d of %d chosen", p.By, p.Version, latest.N)
	if p.Into != "" {
		s += ", into " + p.Into
	}
	s += "."
	if moved {
		s += fmt.Sprintf("\n\nIt was on one machine only (.ws/artifacts), and moved here with all its versions and where each came from (%s).", artifact.RecordFile)
	}
	return s
}

// promoted is a record as promoting it says: promoted, as p says, and
// written in its folder dir.
func promoted(dir string, p artifact.Promotion) (*artifact.Artifact, error) {
	file := filepath.Join(dir, artifact.RecordFile)
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	a, err := artifact.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("its record can't be read: %w", err)
	}
	p.At = p.At.UTC().Truncate(time.Second)
	a.Status, a.Promoted = artifact.Promoted, &p
	return a, writeArtifactRecord(dir, a)
}

func writeArtifactRecord(dir string, a *artifact.Artifact) error {
	data, err := artifact.Encode(a)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, artifact.RecordFile), data, 0o644)
}

// inLift is a change to an artifact kept in a repo, made in a capsule
// lifted from its default branch, committed and landed: change makes it,
// in the capsule's folder of the artifacts (dir), and says the subject and
// body of its commit, and the path in the repo to commit (rel). Nothing's
// left of one that stops. Someone else's change landing between the lift
// and the push stops it: it's done again from the start, as it is then,
// by the caller, not settled.
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

// before is the commit before c, in repo: where a folder c took away is
// kept.
func (w *Workspace) before(repo, c string) string {
	out, _ := git.Line(w.WS().BareDir(repo), "rev-parse", c+"^")
	return out
}

// promoteLanded promotes yours kept in a repo: its record's [promoted], in
// a commit landed on its default branch.
func (w *Workspace) promoteLanded(store artifact.Store, cfg artifact.Config, e artifact.Entry, p artifact.Promotion) (ArtifactDone, error) {
	f := fieldsOf(cfg, e)
	f.Version = p.Version
	saved, err := w.inLift(e.Place, "artifact-"+f.Slug, "Promoted", func(capsule string) (string, string, string, bool, error) {
		dir := filepath.Join(capsule, filepath.FromSlash(e.Folder))
		a, err := promoted(dir, p)
		if errors.Is(err, os.ErrNotExist) {
			return "", "", "", false, fmt.Errorf("%s isn't on %s's %s any more: fetch, and ws show %s says where it is", e.Where(), e.Place, w.WS().DefaultBranch, e.ID)
		}
		if err != nil {
			return "", "", "", false, err
		}
		return e.Folder, f.Subject(cfg.Messages.Promote), promoteBody(p, a, false), true, nil
	})
	if err != nil {
		return ArtifactDone{}, err
	}
	out := ArtifactDone{Commit: saved.Commit, Landed: strings.TrimPrefix(saved.Message, "Promoted: ")}
	out.Entry, err = store.At(context.Background(), e.Place, e.Folder)
	if err != nil {
		// Landed, but the ground hasn't it yet: said as it was written.
		out.Entry, err = e, nil
		out.Entry.Status, out.Entry.Promoted = artifact.Promoted, &p
	}
	return out, err
}

// promoteMoving promotes yours on this machine and moves it into the repo
// artifacts are kept in: its folder, all its versions and its record,
// promoted, in one commit landed on its default branch, then taken from
// .ws/artifacts. Its ref goes with it. Its folder keeps its name where the
// repo names them the same way and it's free, else it's named as a new
// one there would be, from the day it was made.
func (w *Workspace) promoteMoving(store artifact.Store, cfg artifact.Config, e artifact.Entry, p artifact.Promotion) (ArtifactDone, error) {
	repo, r := cfg.Repo, store.Repo
	from := store.Path(e)
	var folder string
	var moving *artifact.Moving
	saved, err := w.inLift(repo, "artifact-"+artifact.SlugOf("{date}-{slug}", e.Folder), "Promoted", func(capsule string) (string, string, string, bool, error) {
		parent := filepath.Join(capsule, filepath.FromSlash(r.Dir))
		var names []string
		if entries, err := os.ReadDir(parent); err == nil {
			for _, d := range entries {
				names = append(names, d.Name())
			}
		}
		taken := func(name string) bool {
			if store.Named(repo, r.FolderPath(name)) {
				return true
			}
			_, err := os.Lstat(filepath.Join(parent, name))
			return err == nil
		}
		name := e.Folder
		if _, ok := artifact.MatchName(cfg.Name, name); !ok || taken(name) {
			var err error
			if name, err = artifact.NameFolder(cfg.Name, e.Made, e.Title, names, taken); err != nil {
				return "", "", "", false, err
			}
		}
		folder = r.FolderPath(name)
		dir := filepath.Join(parent, name)
		if err := copyArtifact(from, dir); err != nil {
			return "", "", "", false, fmt.Errorf("copying %s into %s: %w", e.Where(), repo, err)
		}
		a, err := promoted(dir, p)
		if err != nil {
			return "", "", "", false, err
		}
		moved := e
		moved.Folder = folder
		f := fieldsOf(cfg, moved)
		f.Version = p.Version
		// Its row goes to the repo's folder, and its folder here aside, as
		// it lands: listed once, whenever it's listed.
		if moving, err = store.StartMove(context.Background(), e.ID, repo, folder); err != nil {
			return "", "", "", false, err
		}
		return folder, f.Subject(cfg.Messages.Promote), promoteBody(p, a, true), true, nil
	})
	if err != nil {
		if moving != nil {
			if uerr := moving.Undo(context.Background()); uerr != nil {
				return ArtifactDone{}, fmt.Errorf("%w; and %s couldn't be put back on this machine: %v", err, e.ID, uerr)
			}
		}
		return ArtifactDone{}, err
	}
	out := ArtifactDone{Commit: saved.Commit, Landed: strings.TrimPrefix(saved.Message, "Promoted: "), Moved: true}
	if err := moving.Done(); err != nil {
		out.Landed += fmt.Sprintf("; its old folder on this machine couldn't be removed (%v): it's %s", err, filepath.Join(store.Dir(), ".moving-"+e.Folder))
	}
	out.Entry, err = store.At(context.Background(), repo, folder)
	if err != nil {
		out.Entry, err = e, nil
		out.Entry.Place, out.Entry.Folder, out.Entry.Status, out.Entry.Promoted = repo, folder, artifact.Promoted, &p
	}
	return out, err
}

// DiscardArtifact discards an artifact: you do, saying why if you like.
// On this machine its folder goes to .ws/artifacts/.discarded, deleted for
// good 30 days on; in a repo, a commit takes its folder away, and history
// keeps it. Either way its ref still says what it was, and restoring it
// puts it back.
func (w *Workspace) DiscardArtifact(ref, reason string, by Actor) (ArtifactDone, error) {
	if err := youOnly(by, "discards"); err != nil {
		return ArtifactDone{}, err
	}
	id, v, err := artifactRef(ref)
	if err != nil {
		return ArtifactDone{}, err
	}
	if v != 0 {
		return ArtifactDone{}, invalid("an artifact is discarded whole, with its versions: discard %s", id)
	}
	store := w.Artifacts()
	ctx := context.Background()
	e, err := store.Get(ctx, id)
	if err != nil {
		return ArtifactDone{}, wrapArtifact(err)
	}
	if !e.InRepo() {
		e, err := store.Discard(ctx, id, artifact.Discard{At: w.now(), By: by.By(), Reason: reason})
		return ArtifactDone{Entry: e}, wrapArtifact(err)
	}
	switch {
	case e.Status == artifact.Discarded:
		return ArtifactDone{}, &Refused{Code: "discarded", Reason: fmt.Sprintf("%s was discarded already, on %s", id, e.Discarded.At.Local().Format("2006-01-02"))}
	case e.InFlight() != "":
		return ArtifactDone{}, invalid("%s hasn't landed: it's in %s, crew's, and lands with it. Say so in a review of it, or discard it once it's landed", id, e.InFlight())
	case e.Gone:
		return ArtifactDone{}, invalid("%s's folder has gone from %s already", id, e.Place)
	case store.Repo == nil || store.Repo.Name != e.Place:
		return ArtifactDone{}, invalid("%s is kept in %s, which [artifacts] in ws.toml doesn't name now: ws reads none there (ws doctor)", id, e.Place)
	}
	cfg := w.ArtifactsConfig()
	d := artifact.Discard{At: w.now(), By: by.By(), Reason: strings.TrimSpace(reason)}
	saved, err := w.inLift(e.Place, "artifact-"+fieldsOf(cfg, e).Slug, "Discarded", func(capsule string) (string, string, string, bool, error) {
		if _, err := os.Stat(filepath.Join(capsule, filepath.FromSlash(e.Folder))); err != nil {
			return "", "", "", false, fmt.Errorf("%s isn't on %s's %s any more: fetch, and ws show %s says where it is", e.Where(), e.Place, w.WS().DefaultBranch, id)
		}
		if out, err := git.Combined(capsule, "rm", "-r", "-q", "--", e.Folder); err != nil {
			return "", "", "", false, fmt.Errorf("taking %s away: %v: %s", e.Folder, err, clipString(out, 300))
		}
		return e.Folder, fieldsOf(cfg, e).Subject(cfg.Messages.Discard), artifact.DiscardBody(d.By, d.Reason, &e.Artifact), false, nil
	})
	if err != nil {
		return ArtifactDone{}, err
	}
	d.Commit = w.before(e.Place, saved.Commit)
	out := ArtifactDone{Commit: saved.Commit, Landed: strings.TrimPrefix(saved.Message, "Discarded: ")}
	out.Entry, err = store.MarkDiscarded(ctx, id, d)
	return out, wrapArtifact(err)
}

// RestoreArtifact puts a discarded artifact back, as it was before it was
// discarded (live, or promoted if it had been): on this machine, its folder
// out of .discarded; in a repo, its folder from history, in a new commit.
// Its expiry counts from now.
func (w *Workspace) RestoreArtifact(ref string, by Actor) (ArtifactDone, error) {
	if err := youOnly(by, "restores"); err != nil {
		return ArtifactDone{}, err
	}
	id, v, err := artifactRef(ref)
	if err != nil {
		return ArtifactDone{}, err
	}
	if v != 0 {
		return ArtifactDone{}, invalid("an artifact is restored whole, with its versions: restore %s", id)
	}
	store := w.Artifacts()
	ctx := context.Background()
	e, err := store.Get(ctx, id)
	if err != nil {
		return ArtifactDone{}, wrapArtifact(err)
	}
	r := artifact.Restoration{At: w.now(), By: by.By()}
	if !e.InRepo() {
		e, err := store.Restore(ctx, id, r)
		return ArtifactDone{Entry: e}, wrapArtifact(err)
	}
	switch {
	case e.Status != artifact.Discarded:
		return ArtifactDone{}, invalid("%s isn't discarded: it's %s", id, e.Status)
	case e.Discarded == nil || e.Discarded.Commit == "":
		return ArtifactDone{}, invalid("ws doesn't know which commit of %s keeps %s: put its folder (%s) back by hand", e.Place, id, e.Folder)
	case store.Repo == nil || store.Repo.Name != e.Place:
		return ArtifactDone{}, invalid("%s is kept in %s, which [artifacts] in ws.toml doesn't name now: ws reads none there (ws doctor)", id, e.Place)
	}
	cfg := w.ArtifactsConfig()
	f := fieldsOf(cfg, e)
	saved, err := w.inLift(e.Place, "artifact-"+f.Slug, "Restored", func(capsule string) (string, string, string, bool, error) {
		dir := filepath.Join(capsule, filepath.FromSlash(e.Folder))
		if _, err := os.Lstat(dir); err == nil {
			return "", "", "", false, fmt.Errorf("there's a folder at %s:%s/ again: ws artifact list says whose", e.Place, e.Folder)
		}
		if out, err := git.Combined(capsule, "checkout", e.Discarded.Commit, "--", e.Folder); err != nil {
			return "", "", "", false, fmt.Errorf("taking %s from %s: %v: %s", e.Folder, shortSHA(e.Discarded.Commit), err, clipString(out, 300))
		}
		data, err := os.ReadFile(filepath.Join(dir, artifact.RecordFile))
		if err != nil {
			return "", "", "", false, err
		}
		a, err := artifact.Parse(data)
		if err != nil {
			return "", "", "", false, fmt.Errorf("its record, as %s has it, can't be read: %w", shortSHA(e.Discarded.Commit), err)
		}
		r.At = r.At.UTC().Truncate(time.Second)
		a.Status, a.Discarded, a.Restored = artifact.Live, nil, &r
		if a.Promoted != nil {
			a.Status = artifact.Promoted
		}
		if err := writeArtifactRecord(dir, a); err != nil {
			return "", "", "", false, err
		}
		body := fmt.Sprintf("Restored by %s, from %s, where history kept it.", r.By, shortSHA(e.Discarded.Commit))
		return e.Folder, f.Subject(cfg.Messages.Restore), body, true, nil
	})
	if err != nil {
		return ArtifactDone{}, err
	}
	out := ArtifactDone{Commit: saved.Commit, Landed: strings.TrimPrefix(saved.Message, "Restored: ")}
	out.Entry, err = store.At(ctx, e.Place, e.Folder)
	if err != nil {
		out.Entry, err = e, nil // landed, but the ground hasn't it yet
	}
	return out, err
}

// ArtifactsCleared is what clearing the expired did: those on this machine
// discarded, and those in the repo taken away in one commit (Commit,
// landed as Landed says).
type ArtifactsCleared struct {
	Local, Repo    []artifact.Entry
	Commit, Landed string
}

// ClearExpiredArtifacts clears the expired artifacts, as you: those on
// this machine are discarded, "expired" their reason, as the daemon would;
// those in the repo are taken away in one commit, with [artifacts]' clear
// message and a list of what it took, landed on its default branch.
func (w *Workspace) ClearExpiredArtifacts(by Actor) (ArtifactsCleared, error) {
	if err := youOnly(by, "clears expired"); err != nil {
		return ArtifactsCleared{}, err
	}
	store, cfg := w.Artifacts(), w.ArtifactsConfig()
	ctx := context.Background()
	list, err := store.List(ctx)
	if err != nil {
		return ArtifactsCleared{}, err
	}
	now := w.now()
	var out ArtifactsCleared
	var inRepo []artifact.Entry
	for _, e := range list {
		if !cfg.Expired(e, now) {
			continue
		}
		if e.InRepo() {
			if store.Repo != nil && store.Repo.Name == e.Place {
				inRepo = append(inRepo, e)
			}
			continue
		}
		d, err := store.Discard(ctx, e.ID, artifact.Discard{At: now, By: by.By(), Reason: "expired"})
		if err != nil {
			return out, fmt.Errorf("discarding %s: %w", e.ID, wrapArtifact(err))
		}
		out.Local = append(out.Local, d)
	}
	if len(inRepo) == 0 {
		return out, nil
	}
	r := store.Repo
	var taken []artifact.Entry
	saved, err := w.inLift(r.Name, "artifacts-clear", "Cleared", func(capsule string) (string, string, string, bool, error) {
		for _, e := range inRepo {
			if _, err := os.Stat(filepath.Join(capsule, filepath.FromSlash(e.Folder))); err != nil {
				continue // taken away meanwhile
			}
			if out, err := git.Combined(capsule, "rm", "-r", "-q", "--", e.Folder); err != nil {
				return "", "", "", false, fmt.Errorf("taking %s away: %v: %s", e.Folder, err, clipString(out, 300))
			}
			taken = append(taken, e)
		}
		if len(taken) == 0 {
			return "", "", "", false, errNothingToClear
		}
		rel := cmpStr(r.Dir, ".")
		return rel, artifact.Fields{Date: now.Local().Format("2006-01-02")}.Subject(cfg.Messages.Clear), artifact.ClearBody(by.By(), cfg.ExpiryWords(), taken), false, nil
	})
	if errors.Is(err, errNothingToClear) {
		return out, nil
	}
	if err != nil {
		return out, err
	}
	out.Commit, out.Landed = saved.Commit, strings.TrimPrefix(saved.Message, "Cleared: ")
	was := w.before(r.Name, saved.Commit)
	for _, e := range taken {
		d, err := store.MarkDiscarded(ctx, e.ID, artifact.Discard{At: now, By: by.By(), Reason: "expired", Commit: was})
		if err != nil {
			return out, err
		}
		out.Repo = append(out.Repo, d)
	}
	return out, nil
}

var errNothingToClear = errors.New("nothing to clear")

// ExpiredArtifacts is what the daemon's sweep did: the artifacts on this
// machine it discarded, expired, and those it deleted for good, discarded
// 30 days before.
type ExpiredArtifacts struct {
	Discarded, Erased []artifact.Entry
}

// ExpireArtifacts is the daemon's sweep of the artifacts on this machine:
// each expired one is discarded, by ws, "expired" its reason, and each
// folder that's been in .discarded for 30 days is deleted for good. Those
// in a repo are left to you (Clear expired): ws commits nothing to a
// shared branch on its own.
func (w *Workspace) ExpireArtifacts() (ExpiredArtifacts, error) {
	store, cfg := w.Artifacts(), w.ArtifactsConfig()
	ctx := context.Background()
	list, err := store.List(ctx)
	if err != nil {
		return ExpiredArtifacts{}, err
	}
	now := w.now()
	var out ExpiredArtifacts
	for _, e := range list {
		if e.InRepo() || !cfg.Expired(e, now) {
			continue
		}
		d, err := store.Discard(ctx, e.ID, artifact.Discard{At: now, By: AsWS.By(), Reason: "expired"})
		if err != nil {
			return out, fmt.Errorf("discarding %s: %w", e.ID, err)
		}
		out.Discarded = append(out.Discarded, d)
	}
	out.Erased, err = store.Erase(ctx, now.Add(-artifact.KeepDiscarded))
	return out, err
}
