package ref

import (
	"regexp"
	"strings"
)

// A plugin's things are named as the plugin registered: data ws is given
// (Named), not code, so the web UI's finder reads the same names from
// the same data (web/src/refs/refs.ts).

// Named is a kind of thing a plugin has, and how one is named: one of
// its prefixes and its tail (RAM-123), as written.
type Named struct {
	Plugin string // whose it is: jira
	What   string // its kind there: issue
	// Prefixes are what a name starts with: RAM-, RAP-.
	Prefixes []string
	// Tail is what follows the prefix: TailNumber, or "" for it. One
	// that isn't known is a newer ws's: none of the kind's names are read.
	Tail string
	// Address is where one is outside ws, with Placeholder where its
	// name goes: https://acme.atlassian.net/browse/{ref}. "" for nowhere.
	Address string
}

// Placeholder is where a thing's name goes in a Named's Address.
const Placeholder = "{ref}"

// TailNumber is a whole number that doesn't start with 0, of up to nine
// digits: the tail of RAM-123.
const TailNumber = "number"

// tail is the pattern of what follows a prefix, or "" for a tail that
// isn't known.
func (n Named) tail() string {
	switch n.Tail {
	case "", TailNumber:
		return `[1-9]\d{0,8}`
	}
	return ""
}

// pattern finds the kind's names, or nil if it has none that are found.
func (n Named) pattern() *regexp.Regexp {
	tail := n.tail()
	if tail == "" || len(n.Prefixes) == 0 {
		return nil
	}
	return regexp.MustCompile(`(?:` + alt(n.Prefixes) + `)` + tail)
}

// Names is whether name is, whole, one of the kind's.
func (n Named) Names(name string) bool {
	re := n.pattern()
	if re == nil {
		return false
	}
	loc := re.FindStringIndex(name)
	return loc != nil && loc[0] == 0 && loc[1] == len(name)
}

// ref is the thing of this kind called name.
func (n Named) ref(name string) Ref {
	r := Ref{Kind: Thing, Plugin: n.Plugin, What: n.What, ID: name}
	if n.Address != "" {
		r.URL = strings.Replace(n.Address, Placeholder, name, 1)
	}
	return r
}

// thingNamed is the plugin's thing a name is, whole: the first kind that
// names it.
func (k *Known) thingNamed(name string) (Ref, bool) {
	if k == nil {
		return Ref{}, false
	}
	for _, n := range k.Things {
		if n.Names(name) {
			return n.ref(name), true
		}
	}
	return Ref{}, false
}

// thingAt is the plugin's thing an address is the place of: one made as
// its kind's Address is.
func (k *Known) thingAt(addr string) (Ref, bool) {
	if k == nil || !strings.Contains(addr, "://") {
		return Ref{}, false
	}
	// What follows an address after a ? or a # is beside the point
	// (?focusedCommentId=…), unless the kind's address has one itself.
	bare := addr
	if i := strings.IndexAny(addr, "?#"); i >= 0 {
		bare = addr[:i]
	}
	for _, n := range k.Things {
		before, after, ok := strings.Cut(n.Address, Placeholder)
		if !ok {
			continue
		}
		at := bare
		if strings.ContainsAny(n.Address, "?#") {
			at = addr
		}
		if len(at) < len(before)+len(after) || !strings.HasPrefix(at, before) || !strings.HasSuffix(at, after) {
			continue
		}
		if name := at[len(before) : len(at)-len(after)]; n.Names(name) {
			return n.ref(name), true
		}
	}
	return Ref{}, false
}
