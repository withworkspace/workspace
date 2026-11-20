package ops

import (
	"bytes"
	"cmp"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/doc"
	"github.com/withworkspace/workspace/internal/job"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/review"
	"github.com/withworkspace/workspace/internal/workspace"
)

// Documents anywhere: any markdown file in one of the workspace's repos is a document to read, comment on and edit, whether
// or not a mission recorded it (saving one is in doc_save.go: where an edit is written depends on where the file is, and
// how its repo lands). It's named as a mission's artifact is: repo:path for a file on the default branch (as the ground has
// it), repo/capsule:path for one in a capsule. Its threads are the workspace's own conversations (w12), anchored to that
// ref, kept in the workspace's records with the versions of the document they were made on. A document a mission records
// keeps its threads with the mission.

// What reading, saving and filing a document says, for web to map to what
// its API serves (each has a twin there, with its fields in this order).

// DocText is a document as it's read: to show, comment on and edit.
type DocText struct {
	Ref, Kind, Label, State, URL, Path, By string
	Added                                  time.Time
	Text                                   string
	Markdown                               bool
	// Stage is the stage of its mission it was made for, if it was.
	Stage *DocStage
	// Editable, or Uneditable: why not.
	Editable   bool
	Uneditable string
	// In is where it is, for a file in a repo; Mission, the mission that
	// records one outside a mission; Saves, how an edit to one is saved;
	// DocKind, the kind of document it is; Page, the page it's edited in.
	In      *DocIn
	Mission *DocMission
	Saves   *DocSaves
	DocKind *Kind
	Page    string
	// Media is its content type, for an image or a video (crew's proof,
	// say): it's shown as it is, not read as text, and Text is empty.
	Media string
}

// mediaTypes are the files a mission's artifact is shown as rather than
// read as: images and videos, by their extension.
var mediaTypes = map[string]string{
	".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".gif": "image/gif",
	".webp": "image/webp", ".svg": "image/svg+xml", ".avif": "image/avif",
	".mp4": "video/mp4", ".m4v": "video/mp4", ".webm": "video/webm", ".mov": "video/quicktime",
}

// MediaType is the content type a file is shown as, an image's or a
// video's, by its extension: "" for anything else, which is read as text.
func MediaType(path string) string { return mediaTypes[strings.ToLower(filepath.Ext(path))] }

// DocStage is the stage a document was made for, and where it stands.
type DocStage struct {
	N     int
	Title string
	State StepState
}

// StepState is where a step of a mission's plan stands.
type StepState string

const (
	StepTodo     StepState = "todo"
	StepCurrent  StepState = "current"
	StepAccepted StepState = "accepted"
)

// DocIn is where a document in a repo is: .ground, or a capsule.
type DocIn struct{ Repo, Worktree, Path string }

// DocMission is the mission that records a document, and its ref there.
type DocMission struct{ Slug, Title, Ref string }

// DocSaves is how an edit to a document outside a mission is saved: How is
// savesInCapsule, savesByLanding or savesByLifting.
type DocSaves struct {
	How                     SaveHow
	Branch, Verify, Subject string
}

// DocSaved is what saving a document did: Conflict, an edit that clashes
// with what changed meanwhile (nothing's written); Ref, where it's saved,
// when that's somewhere new.
type DocSaved struct {
	Message, Version, Commit string
	Merged                   bool
	Conflict                 *DocConflict
	Ref                      string
}

// DocConflict is an edit merged with what changed meanwhile, its clashes
// marked.
type DocConflict struct {
	Text    string
	Clashes int
}

// PageTied is where a page tied to a file stands with it.
type PageTied struct {
	Ref     string
	In      *DocIn
	Changed bool
	Gone    string
	Saves   *DocSaves
}

// PageBroughtIn is what bringing a file's changes into its page did.
type PageBroughtIn struct {
	Changed bool
	Clashes int
}

// PageFile asks for a page to be filed as a document of one of the
// workspace's kinds (Dry: only say what it would do).
type PageFile struct {
	Kind, Title, Message string
	// To is where it's filed when it's of no kind (Kind ""): a folder of a
	// repo (repo:folder/), its name made from its title, or a file in one.
	// None: where the page says it's to go.
	To  string
	Dry bool
}

// PageFiling is what filing a page would do, or did.
type PageFiling struct {
	Kind, Title, Ref, Subject string
	How                       SaveHow
	Branch, Verify            string
	Taken                     bool
	Commit, Message           string
}

// DocAdd asks for a new document of one of the workspace's kinds (Kind,
// its name), added where the kind says: its text, whose first heading is
// its title, and the commit's body (Why: where it came from). Dry: only
// say what it would do.
type DocAdd struct {
	Kind, Text, Why string
	Dry             bool
}

// DocAdding is what adding a document would do, or did: its kind and
// title, where it goes (Ref, repo:path; repo/capsule:path once it's left in
// a capsule), the commit's first line, how it's saved (How, onto Branch,
// checked by Verify), whether the path is Taken already, and, once it's
// added, its commit and what was done. (PageFiling's twin.)
type DocAdding struct {
	Kind, Title, Ref, Subject string
	How                       SaveHow
	Branch, Verify            string
	Taken                     bool
	Commit, Message           string
}

// ErrChangedSince is a page's save refused because its file has changed
// since the page last took it in.
var ErrChangedSince = errors.New("the file has changed since the page last took it in")

// DocFile is a document in a repo: where it is.
type DocFile struct {
	Ref      string // repo:path, or repo/capsule:path
	Repo     string
	Worktree string // .ground, or the capsule it's read from
	Rel      string // its path in the repo, slash-separated
	Path     string // the file
}

// IsFileRef reports whether ref has the shape of a file in a repo.
func IsFileRef(ref string) bool {
	r := ParseArtifactRef(ref)
	return r.Loc == LocGround || r.Loc == LocCapsule
}

func isMarkdown(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".md", ".markdown", ".mdx":
		return true
	}
	return false
}

// FindDoc finds the file a ref names: in the repo's ground, or in
// one of its capsules (or the ground, once that capsule's gone: what it
// held has landed, or hasn't). A repo may be named by an alias.
func (w *Workspace) FindDoc(ref string) (*DocFile, error) {
	r := ParseArtifactRef(strings.TrimSpace(ref))
	if r.Loc != LocGround && r.Loc != LocCapsule {
		return nil, fmt.Errorf("%w: %q isn't a file in a repo: one is named repo:path, or repo/capsule:path", ErrNotFound, ref)
	}
	repo, ok := w.WS().ResolveAlias(r.Repo)
	if !ok {
		return nil, fmt.Errorf("%w: the workspace has no repo %s", ErrNotFound, r.Repo)
	}
	r.Repo = repo
	d := &DocFile{Repo: repo, Worktree: workspace.GroundDir, Rel: filepath.ToSlash(filepath.Clean(filepath.FromSlash(r.Path)))}
	dir := w.WS().MainWorktree(repo)
	if r.Loc == LocCapsule && r.Capsule != workspace.GroundDir && r.Capsule != workspace.SiloDir && !strings.ContainsAny(r.Capsule, `/\`) {
		if info, err := os.Stat(filepath.Join(w.WS().RepoDir(repo), r.Capsule)); err == nil && info.IsDir() {
			d.Worktree, dir = r.Capsule, filepath.Join(w.WS().RepoDir(repo), r.Capsule)
		}
	}
	r.Path = d.Rel
	d.Ref = r.String()
	full, err := review.ResolvePath(dir, d.Rel)
	if errors.Is(err, review.ErrOutside) {
		return nil, fmt.Errorf("%w: %s leads out of %s", ErrNotFound, r.Path, repo)
	}
	if info, serr := os.Stat(full); err != nil || serr != nil || info.IsDir() {
		return nil, fmt.Errorf("%w: there's no %s %s", ErrNotFound, d.Rel, d.In())
	}
	d.Path = full
	return d, nil
}

// kindSubject is the first line of the commit that changes a document of
// one of the workspace's kinds (adding: that adds one): the kind's message,
// filled in from the document's path and text. "" for a file of no kind,
// or of a kind with no such message.
func kindSubject(ws *workspace.Workspace, repo, rel string, text []byte, adding bool) string {
	k, f := ws.KindOf(repo, filepath.ToSlash(rel))
	if k == nil {
		return ""
	}
	f.Title = workspace.DocTitle(text)
	return k.Subject(adding, f)
}

// docKindOf is the kind a document in a repo is, or nil.
func docKindOf(ws *workspace.Workspace, repo, rel string, text []byte) *Kind {
	k, _ := ws.KindOf(repo, filepath.ToSlash(rel))
	if k == nil {
		return nil
	}
	return &Kind{Name: k.Name, Title: k.Title, Where: k.Where(), Guide: ws.KindGuide(k), Subject: kindSubject(ws, repo, rel, text, false), Passes: k.Passes}
}

// kindNote tells an agent that writes into or about a document what kind
// of document it is, and to read the kind's guide first. "" for a file of
// no kind.
func kindNote(ws *workspace.Workspace, repo, rel string) string {
	k, _ := ws.KindOf(repo, filepath.ToSlash(rel))
	if k == nil {
		return ""
	}
	note := fmt.Sprintf("The document is of a kind this workspace has: %s (%s, at %s).", k.Title, k.Name, k.Where())
	if guide := ws.KindGuide(k); guide != "" {
		if _, err := os.Stat(guide); err == nil {
			note += fmt.Sprintf(" How one's written is in %s: read it before you answer, and judge the document, and anything you suggest, by it.", guide)
		}
	}
	return note
}

// KindPara is kindNote, for the document a ref names (a mission's artifact,
// or a file in a repo), as a paragraph of a prompt: or nothing.
func KindPara(ws *workspace.Workspace, ref string) string {
	r := ParseArtifactRef(ref)
	if r.Loc != LocGround && r.Loc != LocCapsule {
		return ""
	}
	repo, ok := ws.ResolveAlias(r.Repo)
	if !ok {
		return ""
	}
	if note := kindNote(ws, repo, r.Path); note != "" {
		return note + "\n\n"
	}
	return ""
}

// Where says which of a repo's worktrees a document is in.
func (d *DocFile) Where() string {
	if d.Worktree == workspace.GroundDir {
		return "the default branch of " + d.Repo
	}
	return "the capsule " + d.Repo + "/" + d.Worktree
}

// In says where a document is, as "on the default branch of records" or
// "in the capsule records/note".
func (d *DocFile) In() string {
	if d.Worktree == workspace.GroundDir {
		return "on " + d.Where()
	}
	return "in " + d.Where()
}

// RecordedBy is the mission that records a document, if one that isn't
// archived does: its threads are that mission's, and so is its history.
func (w *Workspace) RecordedBy(d *DocFile) (m *mission.Mission, ref string, ok bool) {
	store := w.Missions
	missions, _ := store.InPlay()
	here, err := os.Stat(d.Path)
	if err != nil {
		return nil, "", false
	}
	for _, m := range missions {
		// Only missions with an artifact of that path in that repo are
		// worth working out the whereabouts of.
		named := false
		for _, a := range m.Artifacts {
			r := ParseArtifactRef(a.Ref)
			named = named || ((r.Loc == LocGround || r.Loc == LocCapsule) && r.Repo == d.Repo && r.Path == d.Rel)
		}
		if !named {
			continue
		}
		// What git says of its capsules as kept: a page reads this each
		// time a document opens.
		v := buildMissionViews(&w.git, w.WS(), nil, []*mission.Mission{m})[0]
		for _, av := range ResolveArtifacts(w.WS(), store, v) {
			if av.Path == "" || av.State == "missing" {
				continue
			}
			if there, err := os.Stat(av.Path); err == nil && os.SameFile(here, there) {
				return m, av.Ref, true
			}
		}
	}
	return nil, "", false
}

// Doc is a document outside a mission, any markdown file in one of the
// workspace's repos by its ref: read as a mission's artifact is, to
// comment on and edit. ErrNotFound for what isn't a file there.
func (w *Workspace) Doc(ref string) (*DocText, error) {
	{
		d, err := w.FindDoc(ref)
		if err != nil {
			return nil, err
		}
		info, err := os.Stat(d.Path)
		if err != nil {
			return nil, err
		}
		if info.Size() > artifactMax {
			return nil, fmt.Errorf("%s is too large to show (%d bytes)", d.Ref, info.Size())
		}
		data, err := os.ReadFile(d.Path)
		if err != nil {
			return nil, err
		}
		if bytes.IndexByte(data, 0) >= 0 {
			return nil, fmt.Errorf("%s isn't text", d.Ref)
		}
		state := "landed"
		if d.Worktree != workspace.GroundDir {
			state = "draft"
		}
		out := &DocText{
			Ref: d.Ref, Kind: "doc", State: state, Path: d.Path, Added: info.ModTime(),
			Text: string(data), Markdown: isMarkdown(d.Path),
			In:      &DocIn{Repo: d.Repo, Worktree: d.Worktree, Path: d.Rel},
			DocKind: docKindOf(w.WS(), d.Repo, d.Rel, data),
			Page:    filedFrom(w.WS().Root, d.Repo, d.Rel),
		}
		switch m, ref, ok := w.RecordedBy(d); {
		case ok:
			out.Mission = &DocMission{Slug: m.Slug, Title: m.Title, Ref: ref}
			out.Uneditable = "it's mission " + m.Slug + "'s document: it's edited there"
		case !out.Markdown:
			out.Uneditable = "only markdown is edited here"
		default:
			out.Editable, out.Saves = true, w.docSaves(d, data)
		}
		return out, nil
	}
}

// docPlace is where a document can be changed: its file, and, for one in a
// capsule, the capsule's folder and the file's path in it.
type docPlace struct {
	File, Capsule, CapsuleDir, Rel string
}

// DocWhere is where a mission's document can be changed, or why it can't.
func (w *Workspace) DocWhere(m *mission.Mission, ref string) (docPlace, error) {
	v := BuildMissionViewsFrom(w.WS(), nil, []*mission.Mission{m})[0]
	for _, av := range ResolveArtifacts(w.WS(), w.Missions, v) {
		if av.Ref == ref {
			return placeOf(av, v)
		}
	}
	return docPlace{}, fmt.Errorf("mission %s has no document %s", m.Slug, ref)
}

// placeOf is where a mission's resolved document can be changed, or why it
// can't.
func placeOf(av ArtifactView, v MissionView) (docPlace, error) {
	switch {
	case av.Path == "" || av.State == "missing":
		return docPlace{}, fmt.Errorf("%s isn't a file here", av.Ref)
	case av.Parsed.Loc == LocMission:
		return docPlace{File: av.Path}, nil
	case av.Parsed.Loc == LocCapsule && av.State != "landed":
		for _, c := range v.Capsules {
			if c.Repo == av.Parsed.Repo && c.Name == av.Parsed.Capsule && c.Exists {
				return docPlace{File: av.Path, Capsule: c.Repo + "/" + c.Name, CapsuleDir: c.Path, Rel: filepath.FromSlash(av.Parsed.Path)}, nil
			}
		}
	}
	return docPlace{}, fmt.Errorf("%s is on %s's default branch, which isn't changed in place: send it to crew to change in a capsule", av.Ref, av.Parsed.Repo)
}

// acceptSuggestion writes a thread's waiting suggestion into its document,
// committing just that file (as the user) if it's in a capsule and has no
// uncommitted changes of its own, and resolves the thread. It says what it
// did.
func (w *Workspace) acceptSuggestion(m *mission.Mission, id string, at time.Time) (string, error) {
	store := conversation.For(w.WS().Root, m.Slug)
	c, err := store.Load(id)
	if err != nil {
		return "", err
	}
	p := c.Pending()
	if p == nil {
		return "", fmt.Errorf("%w: %s has no suggestion waiting", conversation.ErrNotFound, id)
	}
	place, err := w.DocWhere(m, c.Anchor.Doc)
	if err != nil {
		return "", err
	}
	old, err := os.ReadFile(place.File)
	if err != nil {
		return "", err
	}
	if n := strings.Count(string(old), p.From); n != 1 {
		return "", fmt.Errorf("the text it replaces is in the document %d times now, not once: it's changed since. Ask in the thread for a new suggestion", n)
	}
	// Saved as an edit you made from the document as it is: committed as
	// you in a capsule, unless the file has uncommitted changes of its own
	// (crew's draft, say), which it's written in alongside.
	updated := strings.Replace(string(old), p.From, p.To, 1)
	base, _ := store.KeepVersion(old)
	saved, err := saveInPlace(docSave{
		ref: c.Anchor.Doc, place: place, store: store,
		body:    fmt.Sprintf("A suggested edit, accepted from thread %s on mission %s: %s", id, m.Slug, clipString(oneLine(c.Turns[0].Body), 200)),
		subject: cmp.Or(place.kindSubject(w.WS(), []byte(updated)), fmt.Sprintf("Accept suggestion on %s (%s)", filepath.ToSlash(place.Rel), id)),
		drop:    func() { doc.Drafts{Root: w.WS().Root, Scope: doc.OfMission(m.Slug)}.Drop(c.Anchor.Doc) },
	}, base, updated, "")
	if err != nil {
		return "", err
	}
	commit, said := saved.Commit, "Accepted"+strings.TrimPrefix(saved.Message, "Saved")
	if _, err := store.SettleSuggestion(id, conversation.You, conversation.SuggestAccepted, commit, at); err != nil {
		return said, err
	}
	c, err = store.SetStatus(id, conversation.You, conversation.Resolved, at)
	if err == nil && c.ShouldDraft() {
		w.answer(job.Draft, doc.OfMission(m.Slug), c.ID)
	}
	return said + ", and resolved " + id + ".", err
}

// artifactMax is the largest artifact the web UI shows.
const artifactMax = 2 << 20

// Artifact is a mission's readable artifact, by its reference among the
// mission's artifacts, resolved as mission.Resolve has it: to show,
// comment on and edit.
func (w *Workspace) Artifact(slug, ref string) (*DocText, error) {
	{
		store := w.Missions
		m, err := w.load(slug)
		if err != nil {
			return nil, err
		}
		v := BuildMissionViewsFrom(w.WS(), nil, []*mission.Mission{m})[0]
		for _, av := range ResolveArtifacts(w.WS(), store, v) {
			if av.Ref != ref {
				continue
			}
			if av.Path == "" || av.State == "missing" {
				return nil, fmt.Errorf("%s isn't a file here to show", ref)
			}
			info, err := os.Stat(av.Path)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", ref, err)
			}
			// An image or a video is shown as it is (the web UI's raw
			// route serves it), whatever its size: it isn't read here.
			media := MediaType(av.Path)
			var data []byte
			if media == "" {
				if info.IsDir() {
					return nil, fmt.Errorf("%s is a folder, not a file to show", ref)
				}
				if info.Size() > artifactMax {
					return nil, fmt.Errorf("%s is too large to show (%d bytes)", ref, info.Size())
				}
				if data, err = os.ReadFile(av.Path); err != nil {
					return nil, err
				}
				if bytes.IndexByte(data, 0) >= 0 {
					return nil, fmt.Errorf("%s isn't text, or an image or a video", ref)
				}
			}
			out := &DocText{Ref: av.Ref, Kind: av.Kind, Label: av.Label, State: av.State, URL: av.URL, Path: av.Path, By: av.Crew, Added: av.Added,
				Text: string(data), Markdown: media == "" && isMarkdown(av.Path), Media: media}
			if av.Parsed.Loc == LocGround || av.Parsed.Loc == LocCapsule {
				out.In = &DocIn{Repo: av.Parsed.Repo, Worktree: av.Parsed.Capsule, Path: av.Parsed.Path}
				// One that's landed (or whose capsule is gone) is read from the ground.
				if ground := w.WS().MainWorktree(av.Parsed.Repo); av.Parsed.Loc == LocGround || strings.HasPrefix(av.Path, ground+string(filepath.Separator)) {
					out.In.Worktree = filepath.Base(ground)
				}
				if media == "" {
					out.DocKind = docKindOf(w.WS(), av.Parsed.Repo, av.Parsed.Path, data)
				}
			}
			if media != "" {
				out.Uneditable = "it's an image or a video, shown as it is"
			} else if _, err := placeOf(av, v); err == nil {
				out.Editable = true
			} else {
				out.Uneditable = err.Error()
			}
			brief, _ := store.ReadBrief(slug)
			states, _ := m.StageProgress(brief)
			if st, ok := mission.ArtifactStage(states, av.Artifact); ok {
				out.Stage = &DocStage{N: st.N, Title: st.Short(), State: "todo"}
				switch {
				case st.Accepted():
					out.Stage.State = "accepted"
				case st.Current:
					out.Stage.State = "current"
				}
			}
			return out, nil
		}
		return nil, fmt.Errorf("%w: mission %s has no artifact %s", ErrNotFound, slug, ref)
	}
}

// AcceptSuggestion writes the edit a thread on a mission's document
// suggests into it (committing it, in a capsule), and resolves the thread
// (its agent drafts what was decided, if anything was said worth it). It
// says what it did.
func (w *Workspace) AcceptSuggestion(slug, id string) (string, error) {
	m, err := w.load(slug)
	if err != nil {
		return "", err
	}
	return w.acceptSuggestion(m, id, w.now())
}

// SaveDocument saves a mission's document ref, edited from version base,
// merged with what's changed in it meanwhile: a clash comes back as the
// DocSaved's Conflict, with nothing written. message is the commit's first
// line, if you gave one.
func (w *Workspace) SaveDocument(slug, ref, base, text, message string) (*DocSaved, error) {
	m, err := w.load(slug)
	if err != nil {
		return nil, err
	}
	return w.saveDocument(m, ref, base, text, message)
}

// AddDocument adds a document of one of the workspace's kinds where its
// kind says, named from its title (its first heading), with its kind's
// commit message, on its repo's default branch: landed, in a repo with no
// pull requests, or committed in a capsule lifted for it (add-<slug>) and
// left. Nothing's left of one that stops. r.Dry: only say what it would
// do.
func (w *Workspace) AddDocument(r DocAdd) (*DocAdding, error) {
	k, err := w.kindNamed(r.Kind)
	if err != nil {
		return nil, err
	}
	text := strings.TrimSpace(r.Text)
	if text == "" {
		return nil, &Invalid{Reason: "the document has nothing in it"}
	}
	body := []byte(text + "\n")
	if bytes.IndexByte(body, 0) >= 0 {
		return nil, &Invalid{Reason: "a document is markdown: this isn't text"}
	}
	// Its title is its first heading, as a kind's names say.
	title := workspace.DocTitle(body)
	if title == "" {
		return nil, &Invalid{Reason: "give it a title: start it with a heading (# Its title), which is what it's named from"}
	}
	return w.addDoc(newDoc{kind: k, repo: k.Repo, title: title, body: body, why: r.Why, what: "Added", dry: r.Dry}, w.now())
}

// DocAmend is a document of one of the workspace's kinds as it is now, to
// edit: where it is (Ref), its kind (by name, and its guide), its title
// (its first heading) and text, the version that text is, and how an edit
// to it is saved (Saves: its Subject is the commit's first line, from its
// kind's amend message).
type DocAmend struct {
	Ref, Kind, Title, Text, Version string
	Guide                           *DocGuide
	Saves                           *DocSaves
}

// DocToAmend is the document ref names, as it is now, for an edit to it
// that's saved as an edit on its page is (SaveWorkspaceDoc): one of the
// workspace's kinds, which no mission records (a mission's document is
// changed in the mission).
func (w *Workspace) DocToAmend(ref string) (*DocAmend, error) {
	d, err := w.FindDoc(ref)
	if err != nil {
		return nil, err
	}
	if m, mref, ok := w.RecordedBy(d); ok {
		return nil, &Refused{Code: "mission", Reason: fmt.Sprintf("mission %s records %s (as %s): it's changed in the mission, not here", m.Slug, d.Ref, mref)}
	}
	k, _ := w.WS().KindOf(d.Repo, d.Rel)
	if k == nil {
		return nil, &Invalid{Reason: fmt.Sprintf("%s isn't a document of one of the workspace's kinds (ws show kinds): only those are changed this way", d.Ref)}
	}
	text, err := os.ReadFile(d.Path)
	if err != nil {
		return nil, err
	}
	g, err := w.KindGuideOf(k.Name)
	if err != nil {
		return nil, err
	}
	return &DocAmend{
		Ref: d.Ref, Kind: k.Name, Guide: g,
		Title: cmp.Or(workspace.DocTitle(text), filepath.Base(d.Rel)),
		Text:  string(text), Version: doc.Version(text),
		Saves: w.docSaves(d, text),
	}, nil
}

// AmendDocument saves text, the whole of a document of one of the
// workspace's kinds as it should read, made from was (the whole of it as
// DocToAmend gave it), where the document is, as an edit saved on its page
// is (SaveWorkspaceDoc), with body as the rest of the commit's message. If
// it's changed since was, the edit's merged into what's there: a clash
// comes back as the DocSaved's Conflict, with nothing written. Your draft
// of it on its page, if there is one, isn't this edit, and stays.
func (w *Workspace) AmendDocument(ref, was, text, body string) (*DocSaved, error) {
	if _, err := w.DocToAmend(ref); err != nil {
		return nil, err
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, &Invalid{Reason: "the document has nothing in it"}
	}
	if strings.IndexByte(text, 0) >= 0 {
		return nil, &Invalid{Reason: "a document is markdown: this isn't text"}
	}
	// What a merge starts from, kept with the versions the workspace's
	// documents are merged from.
	base, err := conversation.ForWorkspace(w.Root()).KeepVersion([]byte(was))
	if err != nil {
		return nil, err
	}
	return w.saveWorkspaceDoc(ref, base, text+"\n", "", body, false)
}

// kindNamed is the workspace's kind of document called name, or why
// there's none.
func (w *Workspace) kindNamed(name string) (*workspace.Kind, error) {
	k, ok := w.WS().KindNamed(strings.TrimSpace(name))
	if ok {
		return k, nil
	}
	var names []string
	for _, k := range w.WS().Kinds {
		names = append(names, k.Name)
	}
	if len(names) == 0 {
		return nil, &Invalid{Reason: "the workspace has no kinds of document: [documents.<name>] in ws.toml says one"}
	}
	return nil, &Invalid{Reason: fmt.Sprintf("the workspace has no kind of document named %q: it has %s (ws show kinds)", name, strings.Join(names, ", "))}
}

// DocGuide is how a kind's documents are written: the kind, where one
// goes (Where, repo:path) and the first line of the commit that adds one
// (Add), whether they're numbered; and its guide, if it has one: its file
// as the workspace names it (Name: records:README.md) and where it is, its
// text (Clipped: cut short at guideMax), and its version, which a
// conversation's agent proposes one with, to show it's read this text.
// Version is "" for a kind with no guide.
type DocGuide struct {
	Kind, Title, Where, Add   string
	Numbered                  bool
	Name, Path, Text, Version string
	Clipped                   bool
}

// guideMax is as much of a guide as is shown at once.
const guideMax = 24 << 10

// KindGuideOf is how the kind called name is written: where one goes, and
// its guide, if it has one whose file is there (ws doctor says when it
// isn't).
func (w *Workspace) KindGuideOf(name string) (*DocGuide, error) {
	k, err := w.kindNamed(name)
	if err != nil {
		return nil, err
	}
	g := &DocGuide{Kind: k.Name, Title: k.Title, Where: k.Where(), Add: k.Add, Numbered: k.Numbered()}
	path := w.WS().KindGuide(k)
	if path == "" {
		return g, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return g, nil
	}
	g.Name, g.Path, g.Text, g.Version = k.Guide, path, string(data), doc.Version(data)
	if len(g.Text) > guideMax {
		g.Text, g.Clipped = g.Text[:guideMax], true
	}
	return g, nil
}

// FilePage files a page as a document of one of the workspace's kinds,
// where its kind says, on its repo's default branch (landed, or in a
// capsule lifted for it), or, r.Dry, says what it would do.
func (w *Workspace) FilePage(id string, r PageFile) (*PageFiling, error) {
	return w.filePage(id, r, w.now())
}

// OpenFile is the page a file in a repo is edited in (made if there's
// none), or the mission that records the file.
func (w *Workspace) OpenFile(ref string) (string, *DocMission, error) {
	return w.openFilePage(ref, w.now())
}

// PageTied is where a page tied to a file stands with it; nil for a page
// that isn't.
func (w *Workspace) PageTied(id string) *PageTied { return w.pageTied(id) }

// SavePageToFile writes a page tied to a file back to it: ErrChangedSince
// when the file's moved on since the page last took it in.
func (w *Workspace) SavePageToFile(id, message string) (*DocSaved, error) {
	saved, err := w.savePageToFile(id, message, w.now())
	if isFileMoved(err) {
		err = fileMoved{err}
	}
	return saved, err
}

// BringIn brings what's changed in a page's file into the page.
func (w *Workspace) BringIn(id string) (*PageBroughtIn, error) { return w.bringIn(id, w.now()) }
