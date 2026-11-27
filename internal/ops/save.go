package ops

import (
	"cmp"
	"errors"
	"fmt"
	"github.com/withworkspace/workspace/internal/doc"
	"os"
	"path/filepath"
	"strings"

	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/git"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/slug"
	"github.com/withworkspace/workspace/internal/workspace"
)

// Saving a document edited in the web UI.
//
// You edit from the version of the document you opened (base). If it's
// still what's there, your text is written. If it's changed since (crew
// committed to it while you edited), your edit is merged into what's
// there now, three ways, as git merges: changes to different parts of it
// both stand. Where yours and theirs change the same lines, nothing's
// written; the merge comes back with the clash marked, for you to settle
// and save again.
//
// It's written where accepting a thread's suggestion writes
// (acceptSuggestion): in a capsule it's committed as you, just that file,
// unless the file has uncommitted changes of its own (crew's work in
// progress), which it's written alongside; in the mission's own files it's
// written. A document on a default branch isn't changed in place.

// docSave is where an edit is written, and what's kept about it: the
// document's ref, where its file can be changed, the conversations its
// versions are kept with, the commit's body, and how its draft is dropped.
// subject is the commit's first line when you give none: its kind's
// message, for a document of one ("": "Edit <path>").
type docSave struct {
	ref     string
	place   docPlace
	store   conversation.Store
	body    string
	subject string
	drop    func()
}

// saveDocument writes text, edited from version base, to the mission's
// document ref. message is the commit's subject, if you gave one.
func (w *Workspace) saveDocument(m *mission.Mission, ref, base, text, message string) (*DocSaved, error) {
	place, err := w.DocWhere(m, ref)
	if err != nil {
		return nil, err
	}
	return saveInPlace(docSave{
		ref: ref, place: place, store: conversation.For(w.WS().Root, m.Slug),
		body:    fmt.Sprintf("Edited in ws's web UI, on mission %s.", m.Slug),
		subject: place.kindSubject(w.WS(), []byte(text)),
		drop:    func() { logFailed(doc.Drafts{Root: w.WS().Root, Scope: doc.OfMission(m.Slug)}.Drop(ref)) },
	}, base, text, message)
}

// kindSubject is the first line of the commit that changes the document
// here, from its kind's message: "" for one of no kind, or not in a
// capsule.
func (p docPlace) kindSubject(ws *workspace.Workspace, text []byte) string {
	repo, _, ok := strings.Cut(p.Capsule, "/")
	if !ok || p.CapsuleDir == "" {
		return ""
	}
	return kindSubject(ws, repo, p.Rel, text, false)
}

// saveInPlace writes text, edited from version base, into the document's
// file where it is, merged with what's changed there since: committed as
// you, in a capsule, unless the file has uncommitted changes of its own.
func saveInPlace(d docSave, base, text, message string) (*DocSaved, error) {
	place, store, ref := d.place, d.store, d.ref
	info, err := os.Stat(place.File)
	if err != nil {
		return nil, err
	}
	now, err := os.ReadFile(place.File)
	if err != nil {
		return nil, err
	}
	nowV := doc.Version(now)
	out, merged := []byte(text), false
	if nowV != base {
		was, err := store.VersionText(base)
		if errors.Is(err, conversation.ErrNotFound) {
			return nil, fmt.Errorf("%s changed since you started and could not be merged: copy your edit, reload, and make it again", ref)
		}
		if err != nil {
			return nil, err
		}
		got, clashes, err := mergeText(was, []byte(text), now)
		if err != nil {
			return nil, err
		}
		if clashes > 0 {
			// To settle and save again, from what's there now.
			logFailed(store.KeepVersion(now))
			return &DocSaved{Version: nowV, Conflict: &DocConflict{Text: string(got), Clashes: clashes}}, nil
		}
		out, merged = got, true
	}
	if string(out) == string(now) {
		d.drop()
		return &DocSaved{Message: "Nothing to save: no changes", Version: nowV}, nil
	}

	commitIt := false
	if place.CapsuleDir != "" {
		st, err := git.Line(place.CapsuleDir, "status", "--porcelain", "--", place.Rel)
		if err != nil {
			return nil, fmt.Errorf("could not check %s in %s: %w", place.Rel, place.Capsule, err)
		}
		commitIt = st == ""
	}
	if err := os.WriteFile(place.File, out, info.Mode().Perm()); err != nil {
		return nil, err
	}
	rel := filepath.ToSlash(place.Rel)
	saved := &DocSaved{Merged: merged}
	switch {
	case commitIt:
		subject := cmp.Or(strings.TrimSpace(oneLine(message)), d.subject, "Edit "+rel)
		body := d.body
		if merged {
			body += " Merged with the changes made to it while it was being edited."
		}
		if cmd, err := git.Combined(place.CapsuleDir, "commit", "-q", "-m", subject, "-m", body, "--", place.Rel); err != nil {
			_ = os.WriteFile(place.File, now, info.Mode().Perm()) // as it was
			return nil, fmt.Errorf("could not commit in %s: %v: %s", place.Capsule, err, clipString(cmd, 300))
		}
		saved.Commit, _ = git.Line(place.CapsuleDir, "rev-parse", "HEAD")
		saved.Message = fmt.Sprintf("Saved: committed %s in %s", shortSHA(saved.Commit), place.Capsule)
	case place.CapsuleDir != "":
		saved.Message = fmt.Sprintf("Saved, not committed: %s already had uncommitted changes in %s", rel, place.Capsule)
	default:
		saved.Message = "Saved"
	}
	if merged {
		saved.Message += ", merged with changes made while you edited"
	}
	saved.Version, _ = store.KeepVersion(out)
	d.drop()
	return saved, nil
}

// SaveHow is how a document outside a mission is saved (DocSaves.How):
// committed in the capsule it's in, landed straight away, or committed in
// a capsule lifted for it.
type SaveHow string

const (
	savesInCapsule SaveHow = "capsule"
	savesByLanding SaveHow = "land"
	savesByLifting SaveHow = "lift"
)

// docSaves is how an edit to a document outside a mission is saved: where
// it's written depends on where it is, and how its repo lands. text is the
// document's, as it is.
func (w *Workspace) docSaves(d *DocFile, text []byte) *DocSaves {
	out := &DocSaves{How: savesInCapsule, Branch: w.WS().DefaultBranch, Subject: firstNonEmpty(kindSubject(w.WS(), d.Repo, d.Rel, text, false), "Edit "+d.Rel)}
	switch {
	case d.Worktree != workspace.GroundDir:
	case w.WS().Direct(d.Repo):
		out.How, out.Verify = savesByLanding, w.WS().VerifyCommand(d.Repo)
	default:
		out.How = savesByLifting
	}
	return out
}

// SaveWorkspaceDoc writes text, edited from version base, to a document
// outside any mission: a file in one of the workspace's repos.
//
//   - In a capsule, it's saved there as a mission's is: committed as you.
//   - On the default branch of a repo with no pull requests, a capsule is
//     lifted for it, your edit committed there, and landed (ws land): one
//     commit, straight to the default branch. A landing that stops (the
//     repo's check fails, the push is refused) saves nothing.
//   - On the default branch of a repo that takes pull requests, a capsule
//     named for the document is lifted with your commit in it, and left:
//     it's yours, to add to or open a pull request from.
//
// subject is the commit's first line (none: "Edit <path>"), and body the
// rest of its message.
func (w *Workspace) SaveWorkspaceDoc(ref, base, text, subject, body string) (*DocSaved, error) {
	return w.saveWorkspaceDoc(ref, base, text, subject, body, true)
}

// saveWorkspaceDoc is SaveWorkspaceDoc, which drops your draft of the
// document (dropDraft) when the edit saved is yours from its page: an edit
// accepted from a conversation isn't that draft, which stays.
func (w *Workspace) saveWorkspaceDoc(ref, base, text, subject, body string, dropDraft bool) (*DocSaved, error) {
	d, err := w.FindDoc(ref)
	if err != nil {
		return nil, err
	}
	if m, mref, ok := w.RecordedBy(d); ok {
		return nil, fmt.Errorf("recorded by mission %s (%s): edit it there", m.Slug, mref)
	}
	if !isMarkdown(d.Path) {
		return nil, fmt.Errorf("%s cannot be edited: only markdown can", d.Ref)
	}
	store := conversation.ForWorkspace(w.WS().Root)
	drop := func() { logFailed(doc.Drafts{Root: w.WS().Root, Scope: doc.Workspace}.Drop(d.Ref)) }
	if !dropDraft {
		drop = func() {}
	}
	if d.Worktree != workspace.GroundDir {
		dir := filepath.Join(w.WS().RepoDir(d.Repo), d.Worktree)
		return saveInPlace(docSave{
			ref: d.Ref, store: store, body: body, drop: drop,
			subject: kindSubject(w.WS(), d.Repo, d.Rel, []byte(text), false),
			place:   docPlace{File: d.Path, Capsule: d.Repo + "/" + d.Worktree, CapsuleDir: dir, Rel: filepath.FromSlash(d.Rel)},
		}, base, text, subject)
	}
	return w.saveOnDefaultBranch(d, store, base, text, subject, body, drop)
}

// saveOnDefaultBranch saves an edit to a document that's on its repo's
// default branch, which is never changed in place: in a capsule lifted
// from the branch as it is on the remote now, and then landed (a repo
// with no pull requests) or left to you (one with).
func (w *Workspace) saveOnDefaultBranch(d *DocFile, store conversation.Store, base, text, subject, body string, drop func()) (*DocSaved, error) {
	repo, onto := d.Repo, w.WS().DefaultBranch
	l, err := w.liftForDefaultBranch(repo, w.editBranch(repo, d.Rel))
	if err != nil {
		return nil, err
	}
	file := filepath.Join(l.dir, filepath.FromSlash(d.Rel))
	info, err := os.Stat(file)
	if err != nil {
		l.undo()
		return nil, fmt.Errorf("%s is no longer on %s: moved or removed", d.Rel, onto)
	}
	now, err := os.ReadFile(file)
	if err != nil {
		l.undo()
		return nil, err
	}
	nowV := doc.Version(now)
	out, merged := []byte(text), false
	if nowV != base {
		was, err := store.VersionText(base)
		if err != nil {
			l.undo()
			return nil, fmt.Errorf("%s changed on %s since you started and could not be merged: copy your edit, reload, and make it again", d.Ref, onto)
		}
		got, clashes, err := mergeText(was, []byte(text), now)
		if err != nil {
			l.undo()
			return nil, err
		}
		if clashes > 0 {
			logFailed(store.KeepVersion(now))
			l.undo()
			return &DocSaved{Version: nowV, Conflict: &DocConflict{Text: string(got), Clashes: clashes}}, nil
		}
		out, merged = got, true
	}
	if string(out) == string(now) {
		l.undo()
		drop()
		return &DocSaved{Message: "Nothing to save: same as on " + onto, Version: nowV}, nil
	}
	if err := os.WriteFile(file, out, info.Mode().Perm()); err != nil {
		l.undo()
		return nil, err
	}
	subject = cmp.Or(strings.TrimSpace(oneLine(subject)), kindSubject(w.WS(), repo, d.Rel, out, false), "Edit "+d.Rel)
	if merged {
		body += " Merged with the changes made to it while it was being edited."
	}
	saved, err := l.commitAndLand(d.Rel, subject, body, "Saved", false)
	if err != nil {
		return nil, err
	}
	saved.Merged = merged
	if merged {
		saved.Message += ", merged with changes on " + onto + " meanwhile"
	}
	saved.Version, _ = store.KeepVersion(out)
	drop()
	return saved, nil
}

// notLanded is a save or a filing whose landing stopped: what it says, and
// why the landing stopped.
type notLanded struct {
	msg     string
	stopped *workspace.LandError
}

func (e *notLanded) Error() string { return e.msg }
func (e *notLanded) Unwrap() error { return e.stopped }

// lifted is a capsule lifted from a repo's default branch as it is on the
// remote, to change something there (a document saved, a page filed): the
// change is committed in it as you, then landed, or left to you.
type lifted struct {
	w                          *Workspace
	repo, branch, capsule, dir string
	// land is how it's landed: quietly, as the web UI may be asking.
	land workspace.LandOptions
}

// liftForDefaultBranch lifts a capsule on a new branch (named branch) from
// the repo's default branch, fetched.
func (w *Workspace) liftForDefaultBranch(repo, branch string) (*lifted, error) {
	c, err := w.Lift(repo, branch, "", CapsuleOptions{Bare: true, By: AsYou})
	if err != nil {
		return nil, fmt.Errorf("could not lift a capsule of %s: %w", repo, err)
	}
	return &lifted{w: w, repo: repo, branch: branch, capsule: c.Name, dir: c.Path, land: workspace.LandOptions{Quiet: true}}, nil
}

// undo leaves nothing of it: no capsule, no branch.
func (l *lifted) undo() {
	_, _ = l.w.Burn(l.repo, l.capsule, BurnOptions{Force: true, DropBranch: true, quiet: true}) // undoing: the error that called for it is the one returned
}

// commitAndLand commits the file rel (slash-separated; add: a new one) as
// you, and lands it on the default branch (a repo with no pull requests,
// with ws land) or leaves it in the capsule, for you. what ("Saved",
// "Filed") is how what it says it did starts. Nothing's left of one that
// stops: no capsule, no branch.
func (l *lifted) commitAndLand(rel, subject, body, what string, add bool) (*DocSaved, error) {
	w, onto, path := l.w, l.w.WS().DefaultBranch, filepath.FromSlash(rel)
	if add {
		if cmd, err := git.Combined(l.dir, "add", "--", path); err != nil {
			l.undo()
			return nil, fmt.Errorf("could not add: %v: %s", err, clipString(cmd, 300))
		}
	}
	if cmd, err := git.Combined(l.dir, "commit", "-q", "-m", subject, "-m", body, "--", path); err != nil {
		l.undo()
		return nil, fmt.Errorf("could not commit: %v: %s", err, clipString(cmd, 300))
	}
	saved := &DocSaved{Commit: workspace.GitRevParse(l.dir, "HEAD")}
	if !w.WS().Direct(l.repo) {
		saved.Ref = l.repo + "/" + l.capsule + ":" + rel
		saved.Message = fmt.Sprintf("%s in a new capsule, %s/%s (%s): add to it or open a pull request", what, l.repo, l.capsule, shortSHA(saved.Commit))
		return saved, nil
	}
	res, err := w.land(l.repo, l.capsule, AsYou, l.land, true)
	if err != nil {
		l.undo()
		if stopped, ok := errors.AsType[*workspace.LandError](err); ok {
			msg := fmt.Sprintf("not %s, nothing pushed: %s", strings.ToLower(what), stopped.Msg)
			if stopped.Output != "" {
				msg += "\n" + clipString(stopped.Output, 600)
			}
			return nil, &notLanded{msg: msg, stopped: stopped}
		}
		return nil, err
	}
	// The capsule's burned: its branch has nothing left to say.
	logFailed(git.Run(w.WS().BareDir(l.repo), "branch", "-q", "-D", l.branch))
	saved.Commit, saved.Ref = res.Landed.Head, l.repo+":"+rel
	saved.Message = fmt.Sprintf("%s: landed on %s (%s)", what, onto, shortSHA(saved.Commit))
	if res.Landed.Ground != nil {
		saved.Message += fmt.Sprintf("; could not update the ground (%v)", res.Landed.Ground)
	}
	if res.BurnErr != nil {
		saved.Message += fmt.Sprintf("; capsule %s/%s was not removed (%v)", l.repo, l.capsule, res.BurnErr)
	}
	return saved, nil
}

// editBranch names the capsule lifted for an edit to a document: edit-
// and the file's name, numbered if there's one already.
func (w *Workspace) editBranch(repo, rel string) string {
	name := strings.TrimSuffix(filepath.Base(rel), filepath.Ext(rel))
	return w.freeBranch(repo, "edit-"+slug.Make(name, 0, "doc"))
}

// freeBranch is base, or base-2, base-3…: the first that's neither a
// branch of repo nor a capsule's folder.
func (w *Workspace) freeBranch(repo, base string) string {
	taken := func(b string) bool {
		if _, err := os.Stat(filepath.Join(w.WS().RepoDir(repo), b)); err == nil {
			return true
		}
		return git.Ok(w.WS().BareDir(repo), "show-ref", "--verify", "--quiet", "refs/heads/"+b)
	}
	branch := base
	for n := 2; taken(branch); n++ {
		branch = fmt.Sprintf("%s-%d", base, n)
	}
	return branch
}

// mergeText merges ours (an edit made from base) into theirs (what's there
// now), as git merge-file does, and says how many places the two clash;
// those are marked in what comes back, yours first.
func mergeText(base, ours, theirs []byte) (merged []byte, clashes int, err error) {
	dir, err := os.MkdirTemp("", "ws-merge-*")
	if err != nil {
		return nil, 0, err
	}
	defer os.RemoveAll(dir)
	paths := []string{filepath.Join(dir, "yours"), filepath.Join(dir, "base"), filepath.Join(dir, "theirs")}
	for i, b := range [][]byte{ours, base, theirs} {
		if err := os.WriteFile(paths[i], b, 0o600); err != nil {
			return nil, 0, err
		}
	}
	out, err := git.Cmd{Args: []string{"merge-file", "-p", "-L", "your edit", "-L", "original", "-L", "current", paths[0], paths[1], paths[2]}}.Output()
	switch code := git.ExitCode(err); {
	case err == nil:
		return out, 0, nil
	case code > 0 && code < 128: // how many clashes
		return out, code, nil
	default:
		return nil, 0, fmt.Errorf("could not merge: %w", err)
	}
}
