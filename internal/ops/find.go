package ops

import (
	"cmp"
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/artifact"
	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/page"
	"github.com/withworkspace/workspace/internal/plugin"
	"github.com/withworkspace/workspace/internal/pulls"
	"github.com/withworkspace/workspace/internal/ref"
	"github.com/withworkspace/workspace/internal/review"
	"github.com/withworkspace/workspace/internal/space"
	"github.com/withworkspace/workspace/internal/store"
)

// Find over what ws records: what's been said in any mission, in the
// workspace's conversations, pages and artifacts (their titles and
// versions' notes), of and in reviews of pull requests, and by the things
// plugins have told ws of (a Jira issue's title and words), ranked as the
// documents' search is, a thing found once and led to the place in it
// that says it best. The spaces' documents are a search of their own
// (SearchDocuments): their scores are against their own rows, and don't
// compare.

// FindQuery is a search of what ws records.
type FindQuery struct {
	Words string
	// Mission keeps to what's said in one mission (its slug, or a ref that
	// names it), the mission among it.
	Mission string
	// Gone is what's been withdrawn too: it's left out unless asked for.
	Gone  bool
	Limit int
	// Now is when it's asked, as what's found ages: now, if it's zero.
	Now time.Time
}

// Found is a thing found in the records, as ws show names it.
type Found struct {
	store.Found
	// Scope is where it's read: a mission's slug, a pull request (demo#12)
	// for its reviews, or "" for the workspace's.
	Scope string
	// Ref is what it's called where it's read (d1, t3, r1.c2, w4,
	// mission:handoffs/c1.md, a mission's slug), and Show the command that
	// shows it.
	Ref, Show string
	// On is where a comment is: its file, at its lines. Of is what it's
	// of, for a page to go to: a review's (or a comment's review's)
	// capsule or pull request, the document a thread is on (its ref, as
	// the thread has it), the page a conversation is a cell or a pass of,
	// or a page's own.
	On, Of string
	// Named is the plugin's thing it is (a Jira issue, as its plugin told
	// ws of it): whose, its name and where it is. Called is what one of
	// its kind is called (Jira issue), and Opens where it opens in the
	// plugin's app (/x/jira/issue/RAM-123), "" for nowhere in ws.
	Named         *ref.Ref
	Called, Opens string
}

// Find is the things in the records whose words best match q, best first,
// and how many more matched than are given. The briefs and files of
// missions in play, and pages, are read again first where they've moved:
// they're edited outside ws.
func (w *Workspace) Find(ctx context.Context, q FindQuery) ([]Found, int, error) {
	db, err := w.Records()
	if err != nil {
		return nil, 0, err
	}
	if _, err := w.Missions.Reread(ctx); err != nil {
		return nil, 0, fmt.Errorf("reading the missions' briefs and files: %w", err)
	}
	if _, err := w.pages().Reread(ctx); err != nil {
		return nil, 0, fmt.Errorf("reading the pages: %w", err)
	}
	if _, err := w.Artifacts().Reread(ctx); err != nil {
		return nil, 0, fmt.Errorf("reading the artifacts: %w", err)
	}
	// A plugin's things that live in a repo are found in the spaces: what's
	// new there is read. A space that can't be read is as it was last read.
	w.Spaces(ctx) //nolint:errcheck // its documents' search says so
	found, more, err := db.Find(ctx, store.Said{Words: q.Words, Mission: q.Mission, Gone: q.Gone, Limit: cmp.Or(q.Limit, 20), Now: q.Now})
	if err != nil {
		return nil, 0, err
	}
	out := make([]Found, 0, len(found))
	for _, f := range found {
		out = append(out, named(f))
	}
	if err := db.Read(ctx, func(tx *store.Tx) error { return whereOf(tx, out) }); err != nil {
		return nil, 0, err
	}
	w.nameThings(out)
	for i := range out { // a thread's document, as it has it
		if f := &out[i]; strings.HasPrefix(f.Thing.Kind, "thread") {
			if c, err := conversation.For(w.Root(), f.Scope).Load(f.Ref); err == nil {
				f.Of = c.Anchor.Doc
			}
		}
	}
	return out, more, nil
}

// nameThings says, of each plugin's thing found, whose it is and where it
// goes, from how ws keeps the plugin's things named, or, for one ws found
// in a space, from whose kind it was kept as: one whose plugin has gone
// since is left as its name.
func (w *Workspace) nameThings(found []Found) {
	var kept []plugin.KeptRef
	read := false
	for i := range found {
		f := &found[i]
		if f.Thing.Kind != plugin.ThingKind || f.Scope != "" {
			continue
		}
		if !read {
			kept, _ = plugin.Refs(w.WS().Root)
			read = true
		}
		for _, k := range kept {
			n := k.Named()
			if !n.Names(f.Ref) || !plugin.IsInstalled(w.WS().Root, k.Plugin) {
				continue
			}
			r, _ := (&ref.Known{Things: []ref.Named{n}}).At(f.Ref)
			f.Named, f.Called = &r, cmp.Or(k.Title, k.Plugin+" "+strings.ReplaceAll(k.Kind, "-", " "))
			if k.Path != "" {
				f.Opens = "/x/" + k.Plugin + strings.Replace(k.Path, ref.Placeholder, url.PathEscape(f.Ref), 1)
			}
			break
		}
		if f.Named != nil {
			continue
		}
		// One ws found in a space is named by its folder, not as its kind
		// names things: it's the kind's it was kept for.
		fd, ok, _ := plugin.FoundNamed(w.WS().Root, f.Ref)
		if !ok || !plugin.IsInstalled(w.WS().Root, fd.Plugin) {
			continue
		}
		for _, k := range kept {
			if k.Plugin != fd.Plugin || k.Kind != fd.Kind {
				continue
			}
			f.Named = &ref.Ref{Kind: ref.Thing, Plugin: k.Plugin, What: k.Kind, ID: f.Ref}
			f.Called = cmp.Or(k.Title, k.Plugin+" "+strings.ReplaceAll(k.Kind, "-", " "))
			if k.Path != "" {
				f.Opens = "/x/" + k.Plugin + strings.Replace(k.Path, ref.Placeholder, url.PathEscape(f.Ref), 1)
			}
			break
		}
	}
}

// named is a thing found as ws show names it.
func named(f store.Found) Found {
	k := f.Thing.Key
	out := Found{Found: f, Scope: k.Scope, Ref: k.Name}
	switch {
	case k.Scope == "" && f.Thing.Kind == "mission":
		out.Scope, out.Show = k.Name, "ws show --mission "+k.Name
	case k.Scope == "" && f.Thing.Kind == pulls.ThingKind:
		out.Scope, out.Show = k.Name, "ws show "+k.Name // read as its reviews are
	case k.Scope == "":
		out.Show = "ws show " + k.Name
	case isPRScope(k.Scope):
		out.Show = "ws show " + k.Scope // its reviews are its own view's
	default:
		out.Show = "ws show " + k.Name + " --mission " + k.Scope
	}
	return out
}

func isPRScope(s string) bool {
	_, _, ok := ref.ParsePR(s)
	return ok
}

// pageID is a page's id, as a link names one.
var pageID = regexp.MustCompile(`^p[1-9][0-9]*$`)

// whereOf says, of each thing found, where it is (On: a comment's file, at
// its lines, the repo's as it stands rather than a capsule's) and what
// it's of (Of), as their links have them.
func whereOf(tx *store.Tx, found []Found) error {
	for i := range found {
		f := &found[i]
		k := f.Thing.Key
		switch f.Thing.Kind {
		case "decision", "mission", pulls.ThingKind, plugin.ThingKind, "", "handoff", "file":
			continue // a mission's file among them: it's in its mission, and of nothing else
		case page.ThingKind:
			f.Of = k.Name // the page itself, for a page to go to
			continue
		case "comment":
			if review, _, ok := strings.Cut(k.Name, "."); ok {
				links, err := tx.Links(store.Key{Scope: k.Scope, Name: review}, true)
				if err != nil {
					return err
				}
				f.Of = linked(links, store.Of)
			}
		}
		links, err := tx.Links(k, true)
		if err != nil {
			return err
		}
		switch f.Thing.Kind {
		case "comment":
			for _, l := range links {
				if l.Kind != store.On {
					continue
				}
				f.On = l.Dst.Name
				if l.Lines != "" {
					f.On += ":" + l.Lines
				}
				if !strings.Contains(l.Dst.Name, ":") { // the repo's file, not a capsule's
					break
				}
			}
		case "review":
			f.Of = linked(links, store.Of)
		default: // a conversation: the page it's a cell or a pass of (a thread's document is its own)
			for _, l := range links {
				if (l.Kind == store.In || l.Kind == store.About) && l.Dst.Scope == "" && pageID.MatchString(l.Dst.Name) {
					f.Of = l.Dst.Name
					break
				}
			}
		}
	}
	return nil
}

// linked is what the first link of a kind points at.
func linked(links []store.Link, kind string) string {
	for _, l := range links {
		if l.Kind == kind {
			return l.Dst.Name
		}
	}
	return ""
}

// Where says where a thing found is read: in a mission, a pull request,
// or the workspace.
func (f Found) Where() string {
	switch {
	case f.Named != nil:
		return "the " + f.Named.Plugin + " plugin"
	case f.Scope == "":
		return "the workspace"
	case isPRScope(f.Scope):
		return "pull request " + f.Scope
	}
	return "mission " + f.Scope
}

// What says what a thing found is, and where in it it was found: a turn
// of a thread, a decision, a mission's brief (its heading is Heading).
func (f Found) What() string {
	if f.Named != nil {
		return f.Called
	}
	switch f.Place {
	case "brief":
		if f.At != "" {
			return "its brief"
		}
		return "the mission"
	case "handoff":
		return "a handoff"
	case "file":
		return "a file"
	case "turn":
		if f.At != "" {
			return fmt.Sprintf("%s, turn %s", thingKind(f.Thing.Kind), f.At)
		}
	case "finding":
		if f.At != "" {
			return fmt.Sprintf("%s, a finding (%s)", thingKind(f.Thing.Kind), f.At)
		}
	}
	return thingKind(f.Thing.Kind)
}

// Title is a thing found as a line names it: its title, and the heading
// of the section it's led to.
func (f Found) Title() string {
	title := strings.Join(strings.Fields(f.Thing.Title), " ")
	if f.Heading != "" {
		title += " › " + f.Heading
	}
	return title
}

// thingKind is a thing's kind in words.
func thingKind(kind string) string {
	switch kind {
	case "":
		return "something"
	case "thread-ask", "thread-change", "thread-note":
		return "a thread (" + strings.TrimPrefix(kind, "thread-") + ")"
	case "pr":
		return "a talk about a pull request"
	case "doc":
		return "a talk about a document" // an ask, kept
	case pulls.ThingKind:
		return "a pull request"
	case "review-pass":
		return "a pass over a review"
	case "ask":
		return "a question in a page"
	case artifact.ThingKind:
		return "an artifact"
	case plugin.ThingKind:
		return "something a plugin had" // its plugin's gone, or names it no more
	}
	return "a " + kind
}

// Reindexed is how many of each store's records said their words again.
type Reindexed struct{ Missions, Conversations, Reviews, PRs, Pages, Artifacts, Documents int }

// SaidWords is what ws doctor --reindex asks: every store says its words
// again from its records (missions' files read again), a transaction at a
// time, and each space's documents are read again. What their words name
// is linked again with them, as things are named now: it's what links
// what was written before a plugin named its things.
func (w *Workspace) SaidWords(ctx context.Context) (n Reindexed, err error) {
	if n.Missions, err = w.Missions.SayWords(ctx); err != nil {
		return
	}
	if n.Conversations, err = conversation.SayWords(ctx, w.Root()); err != nil {
		return
	}
	if n.Reviews, err = review.SayWords(ctx, w.Root()); err != nil {
		return
	}
	if n.PRs, err = pulls.SayWords(ctx, w.Root()); err != nil {
		return
	}
	if n.Pages, err = page.SayWords(ctx, w.Root()); err != nil {
		return
	}
	if _, err = w.Artifacts().Reread(ctx); err != nil {
		return
	}
	if n.Artifacts, err = artifact.SayWords(ctx, w.Root()); err != nil {
		return
	}
	db, err := w.Records()
	if err != nil {
		return
	}
	if err = space.ReadAgain(ctx, db); err != nil {
		return
	}
	_, st, err := w.Spaces(ctx)
	n.Documents = st.Read
	return
}
