package artifact

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/withworkspace/workspace/internal/git"
)

// Landing a capsule that adds artifacts, or versions of them, to the repo
// they're kept in. Two people can add v3 to the same artifact, or make
// folders of the same name, and the second to land clashes: its rebase
// stops on the artifact's files. Settle settles it as numberTaken does a
// numbered document's, but itself: the capsule's version takes the next
// number (v3 lands as v4, based on what it revised), or its folder the
// next name (-2), and the landing says so. The rest of the capsule's
// commits are rebased as they are. Check, before the push, makes sure
// every artifact the landing touches is whole.

// Settler settles a landing's clashes in the artifacts kept in a repo:
// Dir is the folder they're in, in it, and Name the shape of a folder's
// name.
type Settler struct {
	Dir, Name string
}

// conflict is a file a rebase stopped at, in an artifact's folder.
type inFolder struct {
	folder string // its path in the repo: artifacts/2026-11-12-horizon-glow
	file   string // artifact.toml, v3.html
}

// folderOf is the artifact's folder a path in the repo is in, if it's
// one: artifacts/<name>/<file>.
func (s Settler) folderOf(rel string) (inFolder, bool) {
	rel = filepath.ToSlash(rel)
	dir, file := path.Split(rel)
	folder := strings.TrimSuffix(dir, "/")
	if folder == "" || path.Dir(folder) != cmpOr(s.Dir, ".") {
		return inFolder{}, false
	}
	if _, ok := IsVersionFile(file); !ok && file != RecordFile {
		return inFolder{}, false
	}
	return inFolder{folder: folder, file: file}, true
}

// Settle settles the clashes a rebase stopped at in the worktree dir
// (conflicted: the paths, in the repo), where they're all in artifacts'
// folders: each clashing version takes the next number, and a folder made
// under a name that's been taken the next name. It's what it did, as
// said to people, and false when there's a clash it can't settle: the
// rebase is then the caller's to abort.
func (s Settler) Settle(dir string, conflicted []string) ([]string, bool) {
	folders := map[string]bool{}
	var order []string
	for _, rel := range conflicted {
		f, ok := s.folderOf(rel)
		if !ok {
			return nil, false
		}
		if !folders[f.folder] {
			folders[f.folder] = true
			order = append(order, f.folder)
		}
	}
	var said []string
	for _, folder := range order {
		got, err := s.settleFolder(dir, folder)
		if err != nil {
			return nil, false
		}
		said = append(said, got...)
	}
	if left, _ := git.Output(dir, "diff", "--name-only", "--diff-filter=U"); strings.TrimSpace(left) != "" {
		return nil, false
	}
	return said, true
}

// show is a file at a commit (rev:path), and whether it's there.
func show(dir, rev, rel string) ([]byte, bool) {
	out, err := git.Cmd{Dir: dir, Args: []string{"show", rev + ":" + rel}}.Output()
	return out, err == nil
}

// recordAt is the artifact record in folder at rev, if there's one that
// can be read.
func recordAt(dir, rev, folder string) *Artifact {
	data, ok := show(dir, rev, folder+"/"+RecordFile)
	if !ok {
		return nil
	}
	a, err := Parse(data)
	if err != nil {
		return nil
	}
	return a
}

// sameArtifact reports whether two records are the same artifact: made at
// the same time, with the same first version.
func sameArtifact(a, b *Artifact) bool {
	if a == nil || b == nil || !a.Made.Equal(b.Made) {
		return false
	}
	av, aok := a.Version(1)
	bv, bok := b.Version(1)
	return aok == bok && (!aok || SameVersion(av, bv))
}

// settleFolder settles the clash in one artifact's folder: the commit
// being rebased (REBASE_HEAD) adds versions, or the folder, that what
// it's rebased onto (HEAD) has taken since.
func (s Settler) settleFolder(dir, folder string) ([]string, error) {
	theirs := recordAt(dir, "REBASE_HEAD", folder)
	if theirs == nil {
		return nil, errors.New("the commit's record isn't there, or can't be read")
	}
	ours := recordAt(dir, "HEAD", folder)
	var said []string
	target := folder
	if ours != nil && !sameArtifact(ours, theirs) {
		// Another artifact by the same name landed first: this one lands
		// under the next name, or where an earlier commit of the capsule
		// put it.
		var err error
		if target, err = s.elsewhere(dir, folder, theirs); err != nil {
			return nil, err
		}
		said = append(said, fmt.Sprintf("%s/ lands as %s/: there's another artifact of that name on the branch", folder, target))
		ours = recordAt(dir, "HEAD", target)
	}
	merged := theirs
	renumbered := map[int]int{}
	if ours != nil {
		merged = &Artifact{}
		*merged = *ours
		merged.Versions = slices.Clone(ours.Versions)
		if was := recordAt(dir, "REBASE_HEAD^", folder); was != nil && was.Title != theirs.Title {
			merged.Title = theirs.Title // retitled in the capsule
		}
		renumbered = AddFlying(merged, theirs, "")
	}
	var ns []int
	for n := range renumbered {
		ns = append(ns, n)
	}
	slices.Sort(ns)
	for _, n := range ns {
		if v, ok := merged.Version(renumbered[n]); ok && v.Was == n {
			said = append(said, fmt.Sprintf("v%d of %s/ lands as v%d: v%d was taken on the branch", n, target, renumbered[n], n))
		}
	}
	// The folder as it lands: each version's file from where it was, and
	// its record.
	files := map[string][]byte{}
	for _, v := range merged.Versions {
		var text []byte
		var ok bool
		if have, mine := ours.sameIn(v); mine {
			text, ok = show(dir, "HEAD", target+"/"+VersionFile(have.N))
		} else {
			text, ok = show(dir, "REBASE_HEAD", folder+"/"+VersionFile(cmpInt(v.Was, v.N)))
		}
		if !ok {
			return nil, fmt.Errorf("v%d of %s has no file", v.N, folder)
		}
		files[VersionFile(v.N)] = text
	}
	for i := range merged.Versions {
		merged.Versions[i].Capsule, merged.Versions[i].Was = "", 0
	}
	record, err := Encode(merged)
	if err != nil {
		return nil, err
	}
	files[RecordFile] = record
	if target != folder {
		if err := restore(dir, folder); err != nil {
			return nil, err
		}
	}
	if err := s.write(dir, target, files); err != nil {
		return nil, err
	}
	return said, nil
}

// sameIn is the version of a (nil: none) that's v.
func (a *Artifact) sameIn(v Version) (Version, bool) {
	if a == nil {
		return Version{}, false
	}
	return a.sameAs(v)
}

// elsewhere is where the capsule's artifact in folder lands, there being
// another by that name on the branch: where an earlier commit of the
// capsule put it, or the next name.
func (s Settler) elsewhere(dir, folder string, theirs *Artifact) (string, error) {
	parent := cmpOr(s.Dir, ".")
	names := git.Lines(dir, "ls-tree", "--name-only", "HEAD", parent+"/")
	for _, n := range names {
		other := path.Join(s.Dir, path.Base(n))
		if other != folder && sameArtifact(recordAt(dir, "HEAD", other), theirs) {
			return other, nil
		}
	}
	var bases []string
	for _, n := range names {
		bases = append(bases, path.Base(n))
	}
	name := path.Base(folder)
	taken := func(n string) bool { return slices.Contains(bases, n) }
	if f, ok := MatchName(s.Name, name); ok && f.Number != "" {
		// Numbered: the next number past the highest.
		high := 0
		for _, b := range bases {
			if g, ok := MatchName(s.Name, b); ok {
				if v, err := strconv.Atoi(g.Number); err == nil && v > high {
					high = v
				}
			}
		}
		for i := 1; i <= 100; i++ {
			f.Number = fmt.Sprintf("%04d", high+i)
			if n := f.Fill(s.Name); !taken(n) {
				return path.Join(s.Dir, n), nil
			}
		}
	}
	for i := 2; i <= 100; i++ {
		if n := name + "-" + strconv.Itoa(i); !taken(n) {
			return path.Join(s.Dir, n), nil
		}
	}
	return "", fmt.Errorf("couldn't name a folder for %s", folder)
}

// restore puts folder back as HEAD has it, in the worktree and the index:
// what the commit added to it goes elsewhere.
func restore(dir, folder string) error {
	full := filepath.Join(dir, filepath.FromSlash(folder))
	have := map[string]bool{}
	for _, n := range git.Lines(dir, "ls-tree", "--name-only", "HEAD", folder+"/") {
		have[path.Base(n)] = true
	}
	entries, _ := os.ReadDir(full)
	for _, e := range entries {
		if !have[e.Name()] {
			rel := folder + "/" + e.Name()
			git.Cmd{Dir: dir, Args: []string{"rm", "-q", "--cached", "--ignore-unmatch", "--", rel}}.Combined()
			os.RemoveAll(filepath.Join(full, e.Name()))
		}
	}
	if len(have) > 0 {
		if out, err := git.Combined(dir, "checkout", "HEAD", "--", folder); err != nil {
			return fmt.Errorf("putting %s back: %v: %s", folder, err, out)
		}
	}
	return nil
}

// write writes an artifact's folder as it lands, files by name, and stages
// it: a version file it doesn't name is taken out.
func (s Settler) write(dir, folder string, files map[string][]byte) error {
	full := filepath.Join(dir, filepath.FromSlash(folder))
	if err := os.MkdirAll(full, 0o755); err != nil {
		return err
	}
	entries, _ := os.ReadDir(full)
	for _, e := range entries {
		if _, ok := files[e.Name()]; !ok {
			if _, isVersion := IsVersionFile(e.Name()); isVersion {
				os.Remove(filepath.Join(full, e.Name()))
			}
		}
	}
	for name, text := range files {
		if err := os.WriteFile(filepath.Join(full, name), text, 0o644); err != nil {
			return err
		}
	}
	if out, err := git.Combined(dir, "add", "-A", "--", folder); err != nil {
		return fmt.Errorf("staging %s: %v: %s", folder, err, out)
	}
	return nil
}

// Check checks the artifacts a capsule's commits since base change (in
// the worktree dir) are whole: each one's record can be read, has each
// version once, each with its file, and no version file it doesn't name.
// It's why one isn't, for a landing to stop at.
func (s Settler) Check(dir, base string) error {
	parent := cmpOr(s.Dir, ".")
	changed := git.Lines(dir, "diff", "--name-only", base, "HEAD", "--", parent+"/")
	seen := map[string]bool{}
	for _, rel := range changed {
		f, ok := s.folderOf(rel)
		if !ok || seen[f.folder] {
			continue
		}
		seen[f.folder] = true
		full := filepath.Join(dir, filepath.FromSlash(f.folder))
		data, err := os.ReadFile(filepath.Join(full, RecordFile))
		if errors.Is(err, os.ErrNotExist) {
			if _, err := os.Stat(full); errors.Is(err, os.ErrNotExist) {
				continue // taken away whole
			}
			return fmt.Errorf("%s/ has no %s", f.folder, RecordFile)
		}
		if err != nil {
			return err
		}
		a, err := Parse(data)
		if err != nil {
			return fmt.Errorf("%s/%s can't be read: %v", f.folder, RecordFile, err)
		}
		named := map[int]bool{}
		for _, v := range a.Versions {
			if named[v.N] {
				return fmt.Errorf("%s/%s names v%d twice", f.folder, RecordFile, v.N)
			}
			named[v.N] = true
			if _, err := os.Stat(filepath.Join(full, VersionFile(v.N))); err != nil {
				return fmt.Errorf("%s/%s names v%d, which has no file", f.folder, RecordFile, v.N)
			}
		}
		entries, _ := os.ReadDir(full)
		for _, e := range entries {
			if n, ok := IsVersionFile(e.Name()); ok && !named[n] {
				return fmt.Errorf("%s/%s is a version %s doesn't name", f.folder, e.Name(), RecordFile)
			}
		}
	}
	return nil
}

func cmpInt(a, b int) int {
	if a != 0 {
		return a
	}
	return b
}

// Body is what a commit that keeps a version of an artifact says of where
// it came from, for its body: a conversation (named, with its title and
// the proposal), a mission, or by hand; then the version's note.
func Body(v Version) string {
	var b bytes.Buffer
	title := ""
	if v.FromTitle != "" {
		title = fmt.Sprintf(", %q", v.FromTitle)
	}
	proposal := ""
	if v.Proposal != 0 {
		proposal = fmt.Sprintf(" (proposal %d)", v.Proposal)
	}
	switch {
	case missionTalk.MatchString(v.From):
		slug, id, _ := strings.Cut(v.From, "/")
		fmt.Fprintf(&b, "Proposed in conversation %s on mission %s%s%s, and added by %s.", id, slug, title, proposal, v.By)
	case conversation.MatchString(v.From):
		fmt.Fprintf(&b, "Proposed in conversation %s%s%s, and added by %s.", v.From, title, proposal, v.By)
	case v.From != "":
		fmt.Fprintf(&b, "Made for mission %s%s, by %s.", v.From, title, v.By)
	default:
		fmt.Fprintf(&b, "Added by hand, by %s.", v.By)
	}
	if v.Note != "" {
		b.WriteString("\n\n" + v.Note)
	}
	return b.String()
}

var (
	conversation = regexp.MustCompile(`^w[1-9][0-9]*$`)
	missionTalk  = regexp.MustCompile(`^[^/\s]+/t[1-9][0-9]*$`)
)

// IsConversation reports whether a version's from is a conversation: a
// workspace's (w9) or a mission's (glow/t3).
func IsConversation(from string) bool {
	return conversation.MatchString(from) || missionTalk.MatchString(from)
}
