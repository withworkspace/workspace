package ops

import (
	"context"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/ref"
	"github.com/withworkspace/workspace/internal/store"
)

// What's been said of a thing, in any mission: what points at it in the
// graph. It's how ws show tells an agent what's known of a file or a thread
// beyond the mission it's in: a review comment on the lines it's about to
// change, a decision about the file, and whatever named it: a talk, a
// mission's brief or a handoff, a decision, a document (store.Words links
// what words name as they're written).

// Said is something that points at a thing: where it's read (a mission's
// slug, a pull request, or "" for the workspace's), its ref there, what it
// is and how it stands, and how it points (on, about, of, in, from,
// mentions, by), at which lines.
type Said struct {
	Scope, Ref   string
	Kind         string // comment, decision, review, thread-ask, talk, pass, mission…
	Title, State string
	How          string
	Lines        string
	At           time.Time
}

// Standing is whether it still stands: open, current, in play.
func (s Said) Standing() bool {
	switch s.State {
	case "open", "current", "in play", "draft", "changes":
		return true
	}
	return false
}

// SaidOf is what points at any of keys, standing first, then latest first.
// What's edited outside ws is read again first where it's moved, as a
// search reads it (the briefs and files of missions in play, the pages,
// each space's documents), so that what they name now is what's said:
// what can't be read just now is as it was last read.
func (w *Workspace) SaidOf(keys ...store.Key) ([]Said, error) {
	db, err := w.Records()
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	// Read again behind them: what reads them next says if they can't be.
	_, _ = w.Missions.Reread(ctx)
	_, _ = w.pages().Reread(ctx)
	_, _, _ = w.Spaces(ctx)
	var out []Said
	err = db.Read(context.Background(), func(tx *store.Tx) error {
		links, err := tx.LinksTo(keys...)
		if err != nil {
			return err
		}
		for _, l := range links {
			out = append(out, Said{Scope: l.Src.Scope, Ref: l.Src.Name, Kind: l.From.Kind, Title: l.From.Title, State: l.From.State, How: l.Kind, Lines: l.Lines, At: l.At})
		}
		return nil
	})
	slices.SortStableFunc(out, func(a, b Said) int {
		switch {
		case a.Standing() && !b.Standing():
			return -1
		case b.Standing() && !a.Standing():
			return 1
		}
		return b.At.Compare(a.At)
	})
	return out, err
}

// missionRef is the shape of what's named in a mission: a thread, a
// decision, crew, a stage, a review and its comments.
var missionRef = regexp.MustCompile(`^(?:[tdcsr][1-9][0-9]*)(?:\.c[1-9][0-9]*)?$`)

// KeysOf is what a ref, as said in mission slug ("" for none), is in the
// graph, and the lines it names (0 for none). A file is the repo's file,
// and a capsule's file is that as well: what's said of it is said of the
// file, wherever it was said.
func KeysOf(slug, s string) (keys []store.Key, line, end int) {
	s = strings.TrimSpace(s)
	if !strings.Contains(s, "://") && strings.ContainsAny(s, ":/") {
		s, line, end = ref.SplitLines(s)
	}
	switch {
	case missionRef.MatchString(s), strings.HasPrefix(s, "mission:"):
		return []store.Key{{Scope: slug, Name: s}}, line, end
	case strings.Contains(s, "://"):
		return []store.Key{{Name: s}}, line, end
	}
	r, ok := ref.Parse(s)
	if !ok {
		return []store.Key{{Name: s}}, line, end
	}
	keys = []store.Key{{Name: r.Name()}}
	if r.InCapsule() {
		r.Worktree = ref.Ground
		keys = append(keys, store.Key{Name: r.Name()})
	}
	return keys, line, end
}

// OnLines is whether something said at lines (17, 17-20, or none, about
// the whole) bears on line to end: it's on them, or on no lines.
func OnLines(lines string, line, end int) bool {
	if line == 0 || lines == "" {
		return true
	}
	if end == 0 {
		end = line
	}
	a, b, _ := strings.Cut(lines, "-")
	from, to := atoi(a), atoi(b)
	if to == 0 {
		to = from
	}
	return from <= end && to >= line
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}
