package store

import (
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/withworkspace/workspace/internal/ref"
)

// What words name: where something's said (Words), the things its words
// name are found by ws's one grammar (internal/ref) and linked from what
// says them, as mentions, for the record that said them. So what names a
// thing is in the graph wherever it was written (a brief, a handoff, a
// decision, a comment, a pull request's words), and not only where it was
// said in a conversation, whose turns pin their own.
//
// What there is to name isn't the store's to know: whoever holds a
// workspace says how it's found (Name). Where nobody has, nothing is
// linked.

var (
	namingMu sync.Mutex
	naming   = map[string]func() *ref.Known{} // by root, made absolute
	kept     = map[string]bool{}              // roots named by KeepName
)

// Name says how what there is to name in the workspace at root is found.
// known is asked once in a transaction, when the first words that could
// name something are written in it: it's inside a Write then, so it may
// read the records and mustn't write them. nil takes it back. A way kept
// for the process's life (KeepName) isn't replaced by it.
func Name(root string, known func() *ref.Known) {
	name(root, known, false)
}

// KeepName is Name for whoever holds the workspace at root for the
// process's life (the daemon): what it says stands, whoever names it
// after, until it's taken back (nil).
func KeepName(root string, known func() *ref.Known) {
	name(root, known, true)
}

func name(root string, known func() *ref.Known, keep bool) {
	if abs, err := filepath.Abs(root); err == nil {
		root = abs
	}
	namingMu.Lock()
	defer namingMu.Unlock()
	switch {
	case known == nil:
		delete(naming, root)
		delete(kept, root)
	case keep:
		naming[root], kept[root] = known, true
	case !kept[root]:
		naming[root] = known
	}
}

// names is what there is to name, as the workspace's holder finds it: nil
// where nobody's said how, or in a step, which runs before there's a
// workspace to ask.
func (tx *Tx) names() *ref.Known {
	if !tx.asked {
		tx.asked = true
		namingMu.Lock()
		known := naming[tx.root]
		namingMu.Unlock()
		if known != nil {
			tx.known = known()
		}
	}
	return tx.known
}

// KeyOf is what a ref is in the graph, and the lines of it named: its
// name, less its lines, and an artifact's version as the artifact (what's
// said of a3.v2 is said of a3).
func KeyOf(r ref.Ref) (k Key, lines string) {
	if r.Line > 0 {
		lines = strconv.Itoa(r.Line)
		if r.EndLine > r.Line {
			lines += "-" + strconv.Itoa(r.EndLine)
		}
	}
	r.Line, r.EndLine, r.Version = 0, 0, 0
	return Key{Name: r.Name()}, lines
}

// Mention links from to what r names, for the record at origin: each
// thing once, at its lines. A repo named by its name alone isn't linked
// (workspace, records: a word as often as a name), nor is from itself.
func (tx *Tx) Mention(origin Origin, from Key, r ref.Ref, at time.Time) error {
	if r.Kind == ref.Repo {
		return nil
	}
	k, lines := KeyOf(r)
	if k.Name == "" || k == from {
		return nil
	}
	var n int
	if err := tx.QueryRow(`SELECT count(*) FROM links WHERE origin = ? AND src_scope = ? AND src_name = ? AND kind = ?
			AND dst_scope = ? AND dst_name = ? AND lines = ?`,
		string(origin), from.Scope, from.Name, Mentions, k.Scope, k.Name, lines).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	return tx.Link(origin, Link{Src: from, Kind: Mentions, Dst: k, Lines: lines, At: at})
}

// Named links from to each thing text names, by its name or by where it
// is (an address), for the record at origin: nothing, where there's no
// saying what there is to name.
func (tx *Tx) Named(origin Origin, from Key, text string, at time.Time) error {
	return tx.namedBut(origin, from, Key{}, text, at)
}

// namedBut is Named, less one thing that's not to be linked.
func (tx *Tx) namedBut(origin Origin, from, but Key, text string, at time.Time) error {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	known := tx.names()
	if known == nil {
		return nil
	}
	var named []ref.Ref
	for _, f := range known.Find(text) {
		named = append(named, f.Ref)
	}
	// And what's named by where it is: an address written out, or a link's.
	for _, r := range append(named, known.Addressed(text)...) {
		if k, _ := KeyOf(r); but != (Key{}) && k == but {
			continue
		}
		if err := tx.Mention(origin, from, r, at); err != nil {
			return err
		}
	}
	return nil
}

// named links what a place's words name: its heading's and its text's from
// what says them (By, or the thing it's in), and its title's from the
// thing it's in, whose title it is. The thing it's in isn't linked from
// what says its words (a brief that names its own mission is in it
// already). A place whose record pins its own names links nothing here.
func (tx *Tx) named(origin Origin, p Place) error {
	if p.Pinned {
		return nil
	}
	from := p.Thing
	if p.By != (Key{}) {
		from = p.By
	} else if err := tx.Named(origin, p.Thing, p.Title, p.Said); err != nil {
		return err
	}
	return tx.namedBut(origin, from, p.Thing, p.Heading+"\n"+p.Text, p.Said)
}
