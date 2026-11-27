package ops

import (
	"cmp"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/doc"
	"github.com/withworkspace/workspace/internal/events"
	"github.com/withworkspace/workspace/internal/page"
	"github.com/withworkspace/workspace/internal/workspace"
)

// Pages, in and out: a page can be started as one of the workspace's kinds
// of document, and filed as one. Filed, the page as it reads (its cells'
// words kept as text, its questions gone) is a new file in the kind's
// repo, where the kind says, named from its title, in a commit of yours on
// the default branch: landed, in a repo with no pull requests, or left in
// a capsule lifted for it. The page stays, with its cells and passes, and
// says where it was filed.

// filePage files page id as a document of the kind r names, or, r.Dry,
// says what that would do: where it would go (as the ground has it now:
// the number a numbered kind takes is worked out again when it's filed),
// the commit's first line, and how it's saved.
func (w *Workspace) filePage(id string, r PageFile, at time.Time) (*PageFiling, error) {
	ps := page.Store{Root: w.WS().Root}
	text, p, err := ps.Read(id)
	if err != nil {
		return nil, err
	}
	if p.Filed != nil {
		return nil, fmt.Errorf("%s already filed as %s: edit it there", id, p.Filed.Ref)
	}
	// Of a kind, where the kind says; else where it's said to go, a folder
	// of a repo (or a file in one), named from its title.
	var k *workspace.Kind
	var repo, plain string
	if r.Kind != "" {
		var ok bool
		if k, ok = w.WS().KindNamed(r.Kind); !ok {
			return nil, fmt.Errorf("no document kind %q (ws show kinds)", r.Kind)
		}
		repo = k.Repo
	} else {
		to := cmp.Or(r.To, p.To)
		if to == "" {
			return nil, fmt.Errorf("choose where %s goes: a document kind, or a repo folder (repo:folder/)", id)
		}
		if to, err = w.plainPlace(to); err != nil {
			return nil, err
		}
		repo, plain, _ = strings.Cut(to, ":")
	}
	if busy := w.cellBeingWritten(id); busy != "" {
		return nil, fmt.Errorf("a cell of %s is still being written (%s): file once done", id, busy)
	}
	body := page.AsText(text)
	if strings.TrimSpace(string(body)) == "" {
		return nil, fmt.Errorf("%s is empty: nothing to file", id)
	}
	heading := workspace.DocTitle(body)
	title := cmp.Or(strings.Join(strings.Fields(r.Title), " "), heading)
	if title == "" {
		return nil, fmt.Errorf("add a title")
	}
	if heading == "" {
		body = append([]byte("# "+title+"\n\n"), body...)
	}

	added, err := w.addDoc(newDoc{
		kind: k, repo: repo, plain: plain, title: title, body: body, message: r.Message,
		why: fmt.Sprintf("Filed from page %s in ws's web UI.", id), what: "Filed", dry: r.Dry,
	}, at)
	if err != nil {
		return nil, err
	}
	out := (*PageFiling)(added)
	if r.Dry {
		return out, nil
	}
	// The page is tied to it from now on: saving it writes there. A heading
	// filing gave it is the page's too.
	if heading == "" {
		if _, err := ps.Write(id, doc.Version(text), append([]byte("# "+title+"\n\n"), text...)); err != nil {
			out.Message += fmt.Sprintf("; could not add the title heading to the page (%v)", err)
		}
	}
	meta := ps.Meta(id)
	meta.Filed = &page.Filed{Ref: out.Ref, Kind: out.Kind, Commit: out.Commit, Landed: out.How == savesByLanding, At: at, Version: doc.Version(text)}
	if k != nil {
		meta.Kind = k.Name
	}
	if err := ps.SetMeta(id, meta); err != nil {
		out.Message += fmt.Sprintf("; could not save the filed location on the page (%v)", err)
	}
	if err := tiePage(w.WS(), ps, id, out.Ref, body, at); err != nil {
		out.Message += fmt.Sprintf("; could not link the page to the file (%v)", err)
	}
	return out, nil
}

// newDoc is a new document to add to a repo, on its default branch: of a
// kind (where it says), or of none, at a plain place (a folder of the
// repo, named from its title, or a file in it). Its body has its title as
// its heading already. message is the commit's first line, if one's given
// (else its kind's); why is the commit's body; what ("Filed", "Added") is
// how what it says it did starts.
type newDoc struct {
	kind           *workspace.Kind
	repo, plain    string
	title, message string
	body           []byte
	why, what      string
	dry            bool
}

// addDoc adds a new document to its repo's default branch, as any
// document on one is saved: landed, in a repo with no pull requests, or
// committed in a capsule lifted for it (add-<slug>) and left. Nothing's
// left of one that stops. d.dry: only say what it would do, as the ground
// has it now (the number a numbered kind takes is worked out again when
// it's added).
func (w *Workspace) addDoc(d newDoc, at time.Time) (*DocAdding, error) {
	k, repo, plain := d.kind, d.repo, d.plain
	f := workspace.DocFields{Date: at.Format("2006-01-02"), Slug: workspace.Slug(d.title), Title: d.title}
	out := &DocAdding{Title: d.title, Branch: w.WS().DefaultBranch, How: savesByLifting}
	if k != nil {
		out.Kind = k.Name
	}
	if w.WS().Direct(repo) {
		out.How, out.Verify = savesByLanding, w.WS().VerifyCommand(repo)
	}
	// Where it goes, and the commit's first line, as the worktree dir has
	// the kind's documents.
	var rel string
	place := func(dir string) {
		switch {
		case k == nil && strings.HasSuffix(strings.ToLower(plain), ".md"):
			rel = plain
		case k == nil:
			rel = plain + f.Slug + ".md"
		default:
			if k.Numbered() {
				f.Number = k.NextNumber(dir)
			}
			rel = f.Fill(k.Pattern)
		}
		out.Ref = repo + ":" + rel
		kindSubject := ""
		if k != nil {
			kindSubject = k.Subject(true, f)
		}
		out.Subject = cmp.Or(strings.TrimSpace(oneLine(d.message)), kindSubject, "Add "+rel)
	}
	if d.dry {
		ground := w.WS().MainWorktree(repo)
		place(ground)
		_, err := os.Stat(filepath.Join(ground, filepath.FromSlash(rel)))
		out.Taken = err == nil
		return out, nil
	}

	var saved *DocSaved
	// A number someone else took while this was landing is taken again,
	// once, from what's on the branch then.
	for try := 1; ; try++ {
		l, err := w.liftForDefaultBranch(repo, w.freeBranch(repo, "add-"+f.Slug))
		if err != nil {
			return nil, err
		}
		place(l.dir)
		file := filepath.Join(l.dir, filepath.FromSlash(rel))
		if _, err := os.Stat(file); err == nil {
			l.undo()
			return nil, fmt.Errorf("%s already exists on %s: choose another title", rel, w.WS().DefaultBranch)
		}
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			l.undo()
			return nil, err
		}
		if err := os.WriteFile(file, d.body, 0o644); err != nil {
			l.undo()
			return nil, err
		}
		saved, err = l.commitAndLand(rel, out.Subject, d.why, d.what, true)
		var stopped *workspace.LandError
		if errors.As(err, &stopped) && stopped.Step == "number" && try < 2 && strings.TrimSpace(d.message) == "" {
			continue
		}
		if err != nil {
			return nil, err
		}
		break
	}
	out.Ref, out.Commit, out.Message = saved.Ref, saved.Commit, saved.Message
	w.emit(events.Event{Kind: events.DocAdded, Repo: repo, Doc: out.Ref, Text: d.title}, ActorFromEnv())
	return out, nil
}

// filedFrom is the page a document in repo at rel (slash-separated) is
// edited in, if one is: tied to it, or filed as it, wherever it is (on the
// default branch, or in a capsule).
func filedFrom(root, repo, rel string) string {
	list, _ := page.Store{Root: root}.List()
	for _, p := range list {
		for _, ref := range []string{refOf(p.Source), refOfFiled(p.Filed)} {
			if r := ParseArtifactRef(ref); ref != "" && r.Repo == repo && r.Path == rel {
				return p.ID
			}
		}
	}
	return ""
}

func refOf(s *page.Source) string {
	if s == nil {
		return ""
	}
	return s.Ref
}

func refOfFiled(f *page.Filed) string {
	if f == nil {
		return ""
	}
	return f.Ref
}

// pageKindPara tells an agent writing into or about a page started as one
// of the workspace's kinds of document what it's to be, and to read the
// kind's guide first: or nothing.
func (w *Workspace) PageKindPara(id string) string {
	_, p, err := page.Store{Root: w.WS().Root}.Read(id)
	if err != nil || p.Kind == "" {
		return ""
	}
	k, ok := w.WS().KindNamed(p.Kind)
	if !ok {
		return ""
	}
	note := fmt.Sprintf("The page is being written as one of the kinds of document this workspace has, %s (%s): once written, it's filed at %s.", k.Title, k.Name, k.Where())
	if guide := w.WS().KindGuide(k); guide != "" {
		if _, err := os.Stat(guide); err == nil {
			note += fmt.Sprintf(" How one's written is in %s: read it first, and write, and judge, as it says.", guide)
		}
	}
	return note + "\n\n"
}
