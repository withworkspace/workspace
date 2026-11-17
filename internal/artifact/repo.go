package artifact

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/git"
	"github.com/withworkspace/workspace/internal/store"
)

// Artifacts kept in a repo ([artifacts] path names one). The layout is the
// same as on this machine: a folder each, artifact.toml and its versions.
// ws reads them from the repo's ground, so the list is as fresh as the
// last fetch, and from the mission capsules of the repo, where crew's
// versions wait until they land (in flight). Each is a row in the same
// index, its place the repo's name and its folder its path in the repo,
// and has a ref (a3) like any other.
//
// A version in a capsule that the ground has too, the same one (its
// number, time and who), has landed. One the ground hasn't is in flight:
// it's listed with its capsule, and where its number has been taken on
// the default branch since (by someone else's version, landed first),
// under the number it'll land as, one past the latest, as landing will
// number it (Settle). A capsule's folder holding another artifact than
// the ground's of that name (made another day, by someone else) isn't
// listed until it lands, under another name.

// Repo is the repo a workspace keeps its artifacts in, as the store reads
// it.
type Repo struct {
	Name   string // records
	Ground string // its .ground worktree: the default branch, as last fetched
	Dir    string // the folder they're in, in it: artifacts ("" for its top)
	// Capsules are the mission capsules of the repo, where crew's
	// versions wait until they land.
	Capsules []Capsule
}

// Capsule is a mission's capsule of the repo artifacts are kept in.
type Capsule struct {
	Ref     string // records/glow
	Path    string // its worktree
	Mission string // the mission it's of
}

// FolderPath is an artifact's folder's path in the repo, from its name:
// artifacts/2026-11-12-horizon-glow.
func (r *Repo) FolderPath(name string) string { return path.Join(r.Dir, name) }

// folder is an artifact's folder in the ground, or in capsule (its ref).
func (r *Repo) folder(e Entry, capsule string) string {
	if r == nil || r.Name != e.Place {
		return ""
	}
	if capsule != "" {
		for _, c := range r.Capsules {
			if c.Ref == capsule {
				return filepath.Join(c.Path, filepath.FromSlash(e.Folder))
			}
		}
	}
	return filepath.Join(r.Ground, filepath.FromSlash(e.Folder))
}

// CapsuleOf is the capsule of the repo whose ref is ref, if it's one.
func (r *Repo) CapsuleOf(ref string) (Capsule, bool) {
	if r != nil {
		for _, c := range r.Capsules {
			if c.Ref == ref {
				return c, true
			}
		}
	}
	return Capsule{}, false
}

// source is an artifact.toml found in the ground or a capsule.
type source struct {
	capsule string // its ref; "" for the ground
	path    string
	info    os.FileInfo
}

// flown is an artifact in the repo whose records have moved since its row
// was written: its folder (its path in the repo), its row (nil for none),
// what's read of it now (nil: it's gone), its ground's record's size (0
// for none), and the signature of all its records.
type flown struct {
	folder string
	k      *kept
	a      *Artifact
	size   int64
	sig    string
	// removed is, for one gone, the commit on the default branch that took
	// its folder away: it was discarded there.
	removed *Discard
}

// scan reads the repo's artifacts whose records have moved since their
// rows (all) were written, and those that have gone. Nothing for a
// workspace that keeps none in a repo.
func (r *Repo) scan(all []*kept) ([]flown, error) {
	if r == nil {
		return nil, nil
	}
	found := map[string][]source{}
	var order []string
	look := func(capsule, worktree string) {
		dir := filepath.Join(worktree, filepath.FromSlash(r.Dir))
		entries, _ := os.ReadDir(dir)
		for _, e := range entries {
			name := e.Name()
			if !e.IsDir() || strings.HasPrefix(name, ".") {
				continue
			}
			p := filepath.Join(dir, name, RecordFile)
			info, err := os.Stat(p)
			if err != nil {
				continue
			}
			folder := r.FolderPath(name)
			if _, ok := found[folder]; !ok {
				order = append(order, folder)
			}
			found[folder] = append(found[folder], source{capsule: capsule, path: p, info: info})
		}
	}
	look("", r.Ground)
	for _, c := range r.Capsules {
		look(c.Ref, c.Path)
	}
	rows := map[string]*kept{}
	for _, k := range all {
		if k.Place == r.Name {
			rows[k.Folder] = k
		}
	}
	var out []flown
	for _, folder := range order {
		srcs := found[folder]
		var size int64
		var sig []string
		for _, s := range srcs {
			if s.capsule == "" {
				size = s.info.Size()
			}
			sig = append(sig, fmt.Sprintf("%s=%d@%s", cmpOr(s.capsule, "ground"), s.info.Size(), stampOf(s.info.ModTime())))
		}
		f := flown{folder: folder, k: rows[folder], size: size, sig: strings.Join(sig, " ")}
		if f.k != nil && f.k.size == f.size && f.k.changed == f.sig {
			continue
		}
		a, ok := r.merge(srcs)
		if !ok {
			continue // can't be read: left as it was
		}
		f.a = a
		out = append(out, f)
	}
	for folder, k := range rows {
		if _, ok := found[folder]; !ok && k.Status != Discarded && k.size >= 0 && k.changed != movingStamp {
			out = append(out, flown{folder: folder, k: k, size: -1, sig: k.changed, removed: r.removed(folder)})
		}
	}
	return out, nil
}

// removed is the commit on the default branch (as the ground has it) that
// took an artifact's folder away, as a discard: when, by whom (its
// author), why (as its body says), and the commit before, where history
// keeps it. Nil for a folder no commit there took away: one that never
// landed, say.
func (r *Repo) removed(folder string) *Discard {
	out, err := git.Output(r.Ground, "log", "-1", "--diff-filter=D", "--format=%H%x1f%P%x1f%aI%x1f%an%x1f%B", "--", folder+"/"+RecordFile)
	if err != nil {
		return nil
	}
	parts := strings.SplitN(strings.TrimSpace(out), "\x1f", 5)
	if len(parts) < 5 || parts[1] == "" {
		return nil
	}
	at, err := time.Parse(time.RFC3339, parts[2])
	if err != nil {
		return nil
	}
	_, body, _ := strings.Cut(parts[4], "\n")
	return &Discard{At: at.UTC(), By: parts[3], Reason: RemovedWhy(folder, body), Commit: strings.Fields(parts[1])[0]}
}

func cmpOr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// merge reads an artifact's records, the ground's and each capsule's, as
// one: the ground's, with the versions waiting in capsules that it hasn't,
// in flight. False when the one it's read from can't be.
func (r *Repo) merge(srcs []source) (*Artifact, bool) {
	var out *Artifact
	for _, s := range srcs {
		data, err := os.ReadFile(s.path)
		if err != nil {
			if out == nil {
				return nil, false
			}
			continue
		}
		a, err := Parse(data)
		if err != nil {
			if out == nil {
				return nil, false
			}
			continue // a capsule's that can't be read waits
		}
		if out == nil {
			out = a
			for i := range out.Versions {
				out.Versions[i].Capsule = s.capsule
			}
			continue
		}
		if !a.Made.Equal(out.Made) {
			continue // another artifact by the same name: it lands under another
		}
		AddFlying(out, a, s.capsule)
	}
	return out, out != nil
}

// SameVersion reports whether two versions are the same one, whatever
// number each has: added at the same time, by the same one, with the same
// note. A version renumbered as it landed is the same version.
func SameVersion(a, b Version) bool { return a.At.Equal(b.At) && a.By == b.By && a.Note == b.Note }

// sameAs is a's version that's v, whatever its number.
func (a *Artifact) sameAs(v Version) (Version, bool) {
	if have, ok := a.Version(v.N); ok && SameVersion(have, v) {
		return have, true
	}
	for _, have := range a.Versions {
		if SameVersion(have, v) {
			return have, true
		}
	}
	return Version{}, false
}

// AddFlying adds to a the versions of other (the same artifact, as a
// capsule has it) that a hasn't, as from capsule: each numbered one past
// a's latest where its own number's taken in a, and based on the number
// the one it revises has in a. It's how an artifact's versions in flight
// are listed, and how landing numbers them (Settle). It's other's numbers
// that are another in a: each version a has already under another number,
// and each it renumbered.
func AddFlying(a, other *Artifact, capsule string) map[int]int {
	renumbered := map[int]int{}
	vs := slices.Clone(other.Versions)
	slices.SortFunc(vs, func(x, y Version) int { return x.N - y.N })
	for _, v := range vs {
		if have, ok := a.sameAs(v); ok {
			if have.N != v.N {
				renumbered[v.N] = have.N
			}
			continue
		}
		was := v.N
		if _, taken := a.Version(v.N); taken {
			latest, _ := a.Latest()
			v.N = latest.N + 1
			renumbered[was] = v.N
			if v.Was == 0 {
				v.Was = was
			}
		}
		if n, ok := renumbered[v.BasedOn]; ok {
			v.BasedOn = n
		}
		if capsule != "" {
			v.Capsule = capsule
		}
		a.Versions = append(a.Versions, v)
	}
	return renumbered
}

// save writes the rows of what scan found, in tx.
func (r *Repo) save(tx *store.Tx, fs []flown) error {
	for _, f := range fs {
		k := f.k
		if k != nil {
			now, err := keptIn(tx, k.ID)
			if err != nil {
				return err
			}
			if now == nil || now.size != k.size || now.changed != k.changed {
				continue // written since it was read: left as that write had it
			}
			k = now
		} else {
			now, err := keptAt(tx, r.Name, f.folder)
			if err != nil {
				return err
			}
			if now != nil {
				continue // given a row since it was read
			}
			n, err := nextNumber(tx)
			if err != nil {
				return err
			}
			k = &kept{Entry: Entry{ID: "a" + strconv.Itoa(n), N: n, Place: r.Name, Folder: f.folder}}
		}
		if f.a != nil {
			// Its status is its record's: discarding one in a repo is
			// done there.
			k.Artifact = *f.a
		}
		if f.removed != nil {
			k.Status, k.Discarded = Discarded, f.removed
		}
		if err := saveAs(tx, k, f.size, f.sig); err != nil {
			return err
		}
	}
	return nil
}

// At is the artifact kept at folder in place (its repo's name, or Local),
// as its folder now is.
func (s Store) At(ctx context.Context, place, folder string) (Entry, error) {
	if _, err := s.freshen(ctx); err != nil {
		return Entry{}, err
	}
	db, err := s.records()
	if err != nil {
		return Entry{}, err
	}
	var k *kept
	err = db.Read(ctx, func(tx *store.Tx) error {
		k, err = keptAt(tx, place, folder)
		return err
	})
	if err != nil {
		return Entry{}, err
	}
	if k == nil {
		return Entry{}, fmt.Errorf("%w: there's no artifact at %s:%s", ErrNotFound, place, folder)
	}
	return k.Entry, nil
}

// Named reports whether a row has the folder in place: a folder's name
// isn't used twice, as it's what's written for other people.
func (s Store) Named(place, folder string) bool {
	db, err := s.records()
	if err != nil {
		return false
	}
	var k *kept
	db.Read(context.Background(), func(tx *store.Tx) error {
		k, err = keptAt(tx, place, folder)
		return err
	})
	return k != nil
}

// --- Writing in a worktree

// MakeIn writes a new artifact's folder in parent (a worktree's folder of
// artifacts): named by name's shape from its title and the day (taken
// says which names are), v1.html, then artifact.toml. It's the folder's
// name and the record. Nothing's left of one that stops.
func MakeIn(parent, name string, n New, taken func(string) bool) (string, *Artifact, error) {
	title := CleanTitle(n.Title)
	if title == "" {
		return "", nil, errors.New("an artifact needs a title")
	}
	at := n.At.UTC().Truncate(time.Second)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return "", nil, err
	}
	var numbers []string
	if entries, err := os.ReadDir(parent); err == nil {
		for _, e := range entries {
			numbers = append(numbers, e.Name())
		}
	}
	folder, err := NameFolder(name, at, title, numbers, func(f string) bool {
		if taken != nil && taken(f) {
			return true
		}
		_, err := os.Lstat(filepath.Join(parent, f))
		return err == nil
	})
	if err != nil {
		return "", nil, err
	}
	dir := filepath.Join(parent, folder)
	if err := os.Mkdir(dir, 0o755); err != nil {
		return "", nil, err
	}
	if err := writeNew(filepath.Join(dir, VersionFile(1)), n.HTML); err != nil {
		os.RemoveAll(dir)
		return "", nil, err
	}
	v := n.Version
	v.N, v.BasedOn, v.At, v.Note, v.Capsule, v.Was = 1, 0, at, CleanNote(v.Note), "", 0
	a := &Artifact{Title: title, Status: Live, Made: at, Versions: []Version{v}}
	if _, err := writeRecord(dir, a); err != nil {
		os.RemoveAll(dir)
		return "", nil, err
	}
	return folder, a, nil
}

// ReviseIn writes a new version of the artifact whose folder is dir (in a
// worktree): numbered one past its latest, and past past (the highest
// number elsewhere: what the ground has, where dir is a capsule's), its
// file, then artifact.toml naming it. It's the record, and the version's
// number.
func ReviseIn(dir, id string, r Revision, past int) (*Artifact, int, error) {
	a, n, made, err := revise(dir, id, r, past)
	if err != nil && made != "" {
		os.Remove(made)
	}
	return a, n, err
}

// revise is ReviseIn, saying which file it made (so a caller whose row
// isn't written can take it away again): "" once artifact.toml names it.
func revise(dir, id string, r Revision, past int) (*Artifact, int, string, error) {
	at := r.At.UTC().Truncate(time.Second)
	data, err := os.ReadFile(filepath.Join(dir, RecordFile))
	if errors.Is(err, os.ErrNotExist) {
		return nil, 0, "", fmt.Errorf("%s's folder has gone (%s): add a new artifact", id, dir)
	}
	if err != nil {
		return nil, 0, "", err
	}
	a, err := Parse(data) // the file wins
	if err != nil {
		return nil, 0, "", fmt.Errorf("%s's record can't be read (%s): %w", id, filepath.Join(dir, RecordFile), err)
	}
	if a.Status == Discarded {
		return nil, 0, "", fmt.Errorf("%w: %s was discarded, so it takes no new versions: add a new artifact", ErrDiscarded, id)
	}
	latest, _ := a.Latest()
	on := r.On
	if on == 0 {
		on = latest.N
	} else if _, ok := a.Version(on); !ok {
		return nil, 0, "", fmt.Errorf("%s has no v%d to base it on: %s", id, on, versionsSaid(*a))
	}
	// One past the latest; past a file no version names, too (what a write
	// that didn't finish left): a version's number is never used for two
	// files.
	n := max(latest.N, past) + 1
	var made string
	for ; ; n++ {
		if n > max(latest.N, past)+100 {
			return nil, 0, "", fmt.Errorf("couldn't number a new version of %s", id)
		}
		p := filepath.Join(dir, VersionFile(n))
		err := writeNew(p, r.HTML)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		if err != nil {
			return nil, 0, "", err
		}
		made = p
		break
	}
	v := r.Version
	v.N, v.BasedOn, v.At, v.Note, v.Capsule, v.Was = n, on, at, CleanNote(v.Note), "", 0
	a.Versions = append(a.Versions, v)
	if t := CleanTitle(r.Title); t != "" {
		a.Title = t
	}
	if _, err := writeRecord(dir, a); err != nil {
		return nil, 0, made, err
	}
	return a, n, "", nil
}
