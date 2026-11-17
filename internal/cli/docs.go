package cli

import (
	"bytes"
	"context"
	"fmt"
	wsref "github.com/withworkspace/workspace/internal/ref"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/space"
	"github.com/withworkspace/workspace/internal/ui"
	"github.com/withworkspace/workspace/internal/workspace"
)

// Documents anywhere: any markdown file in one of the workspace's repos is a document to read, comment on and edit, whether
// or not a mission recorded it (saving one is in doc_save.go: where an edit is written depends on where the file is, and
// how its repo lands). It's named as a mission's artifact is: repo:path for a file on the default branch (as the ground has
// it), repo/capsule:path for one in a capsule. Its threads are the workspace's own conversations (w12), anchored to that
// ref, kept in the workspace's records with the versions of the document they were made on. A document a mission records
// keeps its threads with the mission.

// showDoc is ws show <repo:path>: a document outside a mission, and the
// threads on it.
func showDoc(ctx *ops.Workspace, ref, format string, full bool) error {
	name, line, end := fileName(ref)
	d, err := ctx.FindDoc(name)
	if err != nil {
		return err
	}
	v := &view{head: fmt.Sprintf("Document %s (on %s)", d.Ref, d.Where())}
	if d.Worktree != workspace.GroundDir {
		v.head = fmt.Sprintf("Document %s (in %s)", d.Ref, d.Where())
	}
	v.body = append(v.body, "The file: "+d.Path)
	if data, err := os.ReadFile(d.Path); err == nil {
		lines := bytes.Count(data, []byte("\n"))
		v.body[0] += fmt.Sprintf(" (%d lines)", lines)
	}
	if k, _ := ctx.WS().KindOf(d.Repo, d.Rel); k != nil {
		said := fmt.Sprintf("Of the kind %s (%s), one of the workspace's kinds of document: ws show kinds.", k.Title, k.Name)
		if guide := ctx.WS().KindGuide(k); guide != "" {
			said += " How one's written: " + guide
		}
		v.body = append(v.body, said)
	}
	if m, mref, ok := ctx.RecordedBy(d); ok {
		v.body = append(v.body, fmt.Sprintf("Mission %s (%q) records it as %s: its history, and the threads on it, are the mission's (ws show -m %s %s).", m.Slug, m.Title, mref, m.Slug, mref))
	}
	if d.Worktree == workspace.GroundDir {
		if _, ok := ctx.WS().SpaceOf(d.Repo); ok {
			citedBy(ctx, v, d.Repo, d.Rel)
		}
	}
	threads, _ := conversation.ForWorkspace(ctx.WS().Root).OnDoc(d.Ref)
	open := 0
	for _, c := range threads {
		if c.Status == conversation.Open {
			open++
		}
		said := ""
		if len(c.Turns) > 0 {
			said = ": " + clipString(strings.Join(strings.Fields(c.Turns[0].Body), " "), 100)
		}
		v.add(c.ID, fmt.Sprintf("a thread on it (%s, %s)%s", c.Kind, threadStanding(c), said))
	}
	switch {
	case len(threads) == 0:
		v.body = append(v.body, "No threads on it.")
	default:
		v.body = append(v.body, fmt.Sprintf("%d %s on it, %d open.", len(threads), plural(len(threads), "thread"), open))
	}
	// What's been said of the file anywhere: in any mission, a capsule's
	// file as the repo's. (The workspace's threads on it are above.)
	keys, _, _ := ops.KeysOf("", d.Ref)
	if said, err := ctx.SaidOf(keys...); err == nil {
		said = slices.DeleteFunc(said, func(x ops.Said) bool {
			return x.Scope == "" && x.How == "about" && strings.HasPrefix(x.Kind, "thread-")
		})
		v.addSaid(saidSection{title: "Said of it, in any mission", said: said, line: line, end: end, elsewhere: d.Ref})
	}
	if format == "llm" {
		_, err = os.Stdout.WriteString(v.render(false, full))
	} else {
		_, err = io.WriteString(ui.Stdout, v.render(true, full))
	}
	return err
}

// isFileName is whether ref names a file in a repo, as a document's ref
// is stored (repo:path, repo/capsule:path) or as refs name one (repo/path),
// lines and all.
func isFileName(ref string) bool {
	name, _, _ := fileName(ref)
	return ops.IsFileRef(name)
}

// fileName is a file's ref as a document's is stored, and the lines it
// names (0 for none).
func fileName(ref string) (name string, line, end int) {
	name, line, end = wsref.SplitLines(strings.TrimSpace(ref))
	if ops.IsFileRef(name) {
		return name, line, end
	}
	if r, ok := wsref.Parse(name); ok && r.Kind == wsref.File && r.Repo != "" && r.Path != "" {
		if r.InCapsule() {
			return r.Repo + "/" + r.Worktree + ":" + r.Path, line, end
		}
		return r.Repo + ":" + r.Path, line, end
	}
	return ref, 0, 0
}

// threadStanding is where a thread stands, in a few words.
func threadStanding(c *conversation.Conversation) string {
	switch {
	case c.Status == conversation.Resolved:
		return "resolved"
	case c.Answering:
		return "open, an agent answering it now"
	case c.WaitingOnYou():
		return "open, waiting on the user"
	case c.Kind == conversation.Note:
		return "open, a note"
	}
	return "open, waiting for an answer"
}

// workspaceThreadView is ws show w12 for a thread on a document outside a
// mission: the text it's on, what's been said, and where the document is.
func workspaceThreadView(ctx *ops.Workspace, c *conversation.Conversation, full bool) *view {
	v := threadSaid(c, func(text string) string { return clipText(text, full) }, full)
	if d, err := ctx.FindDoc(c.Anchor.Doc); err == nil {
		v.body = append(v.body, "", fmt.Sprintf("The document is a file %s, not part of a mission: %s", d.In(), d.Path))
	} else {
		v.body = append(v.body, "", "The document it's on isn't there any more.")
	}
	v.add(c.Anchor.Doc, "the document it's on, and its other threads")
	return v
}

// isDocThread reports whether a workspace conversation is a thread on a
// document.
func isDocThread(c *conversation.Conversation) bool {
	return c.Anchor.Doc != "" && c.Anchor.PR == "" && c.Anchor.Page == "" && (c.Kind == conversation.Ask || c.Kind == conversation.Change || c.Kind == conversation.Note)
}

// citedBy adds to a document's view what cites it and what it cites, in
// the workspace's spaces, as the records have them read from the default
// branch.
func citedBy(ctx *ops.Workspace, v *view, repo, rel string) {
	db, _, err := ctx.Spaces(context.Background())
	if db == nil {
		v.body = append(v.body, "What cites it isn't known: "+err.Error())
		return
	}
	by, _ := space.CitedBy(context.Background(), db, repo, rel)
	cites, _ := space.Cites(context.Background(), db, repo, rel)
	for _, c := range by {
		v.add(c.Repo+":"+c.Path, fmt.Sprintf("cites it (%q, line %d)", c.Title, c.Line))
	}
	for _, c := range cites {
		what := "it cites"
		if c.Title != "" {
			what = fmt.Sprintf("it cites %q", c.Title)
		}
		v.add(c.Repo+":"+c.Path, fmt.Sprintf("%s (line %d)", what, c.Line))
	}
	switch {
	case len(by) == 0 && len(cites) == 0:
		v.body = append(v.body, "Nothing in the workspace's spaces cites it, and it cites none of their documents.")
	default:
		v.body = append(v.body, fmt.Sprintf("Cited by %d %s; it cites %d.", len(by), plural(len(by), "document"), len(cites)))
	}
}
