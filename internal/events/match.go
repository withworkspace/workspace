package events

import (
	"fmt"
	"path"
	"slices"
	"strings"
)

// A Match is which events something waits on, written as a plan or an
// automation writes it: the kind, then what it's of, a field and a value
// each (pr.merged pr=api#212; thing.changed thing=RAM-881 status=Done).
// A value may be a pattern (status=failed*, path=docs/*). Every field has
// to match.
//
// The fields are what an event says: who did it (by, which a person's key
// or its first part matches: brendan for person:brendan-ash), where (repo,
// capsule, branch, mission), what of (pr, doc, thing, kind for a thing's
// kind, run, conversation), how it stands (status, title), and a path any
// of its paths matches. Any other field is a thing's own, for thing.changed
// and thing.added: status=Done matches a thing whose status changed to
// Done.

// Match is which events something waits on.
type Match struct {
	Kind   string
	Fields []MatchField
}

// MatchField is one field a Match asks of an event, and its value.
type MatchField struct {
	Key, Value string
}

// matchKeys are the fields a Match asks of what an event says.
var matchKeys = []string{"by", "repo", "capsule", "branch", "mission", "pr", "doc", "thing", "kind", "run", "conversation", "status", "title", "path", "origin"}

// ParseMatch reads a Match: its kind, then field=value pairs. It says
// what's wrong with one it can't read.
func ParseMatch(s string) (Match, error) {
	words := strings.Fields(strings.TrimSpace(s))
	if len(words) == 0 {
		return Match{}, fmt.Errorf("say which event: a kind (%s), then what it's of (pr=api#212)", strings.Join(Kinds, ", "))
	}
	m := Match{Kind: words[0]}
	if !Known(m.Kind) {
		return Match{}, fmt.Errorf("%q isn't a kind of event: they're %s", m.Kind, strings.Join(Kinds, ", "))
	}
	ofThing := m.Kind == ThingChanged || m.Kind == ThingAdded
	for _, w := range words[1:] {
		key, value, ok := strings.Cut(w, "=")
		key = strings.ToLower(strings.TrimSpace(key))
		if !ok || key == "" || value == "" {
			return Match{}, fmt.Errorf("%q: after the kind, say what it's of as field=value (pr=api#212)", w)
		}
		if !slices.Contains(matchKeys, key) && !ofThing {
			return Match{}, fmt.Errorf("%q isn't a field %s events say: they're %s (and a thing's own fields, for thing.changed)", key, m.Kind, strings.Join(matchKeys, ", "))
		}
		if _, err := path.Match(value, ""); err != nil {
			return Match{}, fmt.Errorf("%q: %s's value isn't a pattern ws can read", w, key)
		}
		m.Fields = append(m.Fields, MatchField{Key: key, Value: value})
	}
	return m, nil
}

// String is the Match as it's written.
func (m Match) String() string {
	out := m.Kind
	for _, f := range m.Fields {
		out += " " + f.Key + "=" + f.Value
	}
	return out
}

// Matches reports whether e is one m waits on.
func (m Match) Matches(e Event) bool {
	if e.Kind != m.Kind {
		return false
	}
	for _, f := range m.Fields {
		if !fieldMatches(f, e) {
			return false
		}
	}
	return true
}

func fieldMatches(f MatchField, e Event) bool {
	switch f.Key {
	case "by":
		return byMatches(f.Value, e.By)
	case "repo":
		return like(f.Value, e.Repo)
	case "capsule":
		return like(f.Value, e.Capsule) || like(f.Value, e.Repo+"/"+e.Capsule)
	case "branch":
		return like(f.Value, e.Branch)
	case "mission":
		return like(f.Value, e.Mission)
	case "pr":
		return like(f.Value, e.PR)
	case "doc":
		return like(f.Value, e.Doc) || like(f.Value, strings.TrimPrefix(e.Doc, e.Repo+":"))
	case "thing":
		return like(f.Value, e.Thing) || like(f.Value, strings.TrimSuffix(e.Thing, "/"))
	case "kind":
		return like(f.Value, e.ThingKind) || like(f.Value, e.ProposalKind)
	case "run":
		return like(f.Value, e.Run) || like(f.Value, e.Title)
	case "conversation":
		return like(f.Value, e.Conversation)
	case "status":
		if e.Status != "" || len(e.Changes) == 0 {
			return like(f.Value, e.Status)
		}
	case "title":
		return like(f.Value, e.Title)
	case "path":
		return slices.ContainsFunc(e.Paths, func(p string) bool { return like(f.Value, p) }) || like(f.Value, strings.TrimPrefix(e.Doc, e.Repo+":"))
	case "origin":
		return like(f.Value, e.Origin) || (f.Value == "did" && e.Origin == Did)
	}
	// A thing's own field: changed to what's asked.
	return slices.ContainsFunc(e.Changes, func(c FieldChange) bool { return strings.EqualFold(c.Field, f.Key) && like(f.Value, c.To) })
}

// byMatches is whether by (as an event says it) is who's asked: the same,
// or a person whose key, or its first part, is it (brendan for
// person:brendan-ash), or you, as me.
func byMatches(want, by string) bool {
	want = strings.ToLower(want)
	switch {
	case like(want, by):
		return true
	case want == "me" || want == ByYou:
		return by == ByYou
	}
	key, ok := PersonOf(by)
	if !ok {
		return false
	}
	first, _, _ := strings.Cut(key, "-")
	return like(want, key) || like(want, first)
}

// like is whether s is as pattern asks, ignoring case.
func like(pattern, s string) bool {
	if s == "" {
		return false
	}
	ok, _ := path.Match(strings.ToLower(pattern), strings.ToLower(s))
	return ok
}

// FieldOf is e's value for a field as a Match names it (pr, thing, repo,
// by…), or for a thing's own field, what it changed to: "" for none.
func FieldOf(e Event, key string) string {
	switch strings.ToLower(key) {
	case "by":
		return e.By
	case "repo":
		return e.Repo
	case "capsule":
		if e.Capsule == "" {
			return ""
		}
		return e.Repo + "/" + e.Capsule
	case "branch":
		return e.Branch
	case "mission":
		return e.Mission
	case "pr":
		return e.PR
	case "doc":
		return e.Doc
	case "thing":
		return e.Thing
	case "kind":
		return firstOf(e.ThingKind, e.ProposalKind)
	case "run":
		return e.Run
	case "conversation":
		return e.Conversation
	case "status":
		return e.Status
	case "title":
		return e.Title
	case "origin":
		return e.Origin
	}
	for _, c := range e.Changes {
		if strings.EqualFold(c.Field, key) {
			return c.To
		}
	}
	return ""
}

func firstOf(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
