package artifact

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// A version is a page and the files beside it: index.html, and whatever
// it loads (a stylesheet, scripts, images, fonts, data), each a real file
// at a path of its own. One made of the page alone is index.html and
// nothing else.

// IndexFile is a version's page: what its frame opens.
const IndexFile = "index.html"

// MaxSize is the most a version's files may come to, and MaxFiles how
// many it may have.
const (
	MaxSize  = 10 << 20
	MaxFiles = 200
)

// File is one of a version's files: its path in the version, with
// forward slashes (img/logo.png), and what's in it.
type File struct {
	Path string
	Data []byte
}

// Files is a version's files, by path.
type Files []File

// FileInfo is one of a version's files as it's listed: its path and its
// size.
type FileInfo struct {
	Path string `json:"path"`
	Size int    `json:"size"`
}

// Changed is a file that differs between two versions: added, removed or
// changed.
type Changed struct {
	Path   string
	Status string // "added", "removed", "changed"
}

// Differ is the files that differ between two versions (a to b), by
// path.
func Differ(a, b Files) []Changed {
	was := map[string][]byte{}
	for _, f := range a {
		was[f.Path] = f.Data
	}
	var out []Changed
	is := map[string]bool{}
	for _, f := range b {
		is[f.Path] = true
		switch data, ok := was[f.Path]; {
		case !ok:
			out = append(out, Changed{f.Path, "added"})
		case !bytes.Equal(data, f.Data):
			out = append(out, Changed{f.Path, "changed"})
		}
	}
	for _, f := range a {
		if !is[f.Path] {
			out = append(out, Changed{f.Path, "removed"})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

// Page is a version that's its page alone: index.html.
func Page(html []byte) Files { return Files{{Path: IndexFile, Data: html}} }

// Index is the version's page, and false where it has none.
func (f Files) Index() ([]byte, bool) { return f.File(IndexFile) }

// File is the file at path, and false where there's none.
func (f Files) File(p string) ([]byte, bool) {
	for _, x := range f {
		if x.Path == p {
			return x.Data, true
		}
	}
	return nil, false
}

// Size is what the files come to, in bytes.
func (f Files) Size() int {
	n := 0
	for _, x := range f {
		n += len(x.Data)
	}
	return n
}

// Infos lists the files, by path.
func (f Files) Infos() []FileInfo {
	out := make([]FileInfo, 0, len(f))
	for _, x := range f {
		out = append(out, FileInfo{Path: x.Path, Size: len(x.Data)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

// FilesWords says how much a version is: "3 files, 240 KB", or its size
// alone for one that's its page only.
func FilesWords(files, size int) string {
	if files <= 1 {
		return Size(size)
	}
	return fmt.Sprintf("%d files, %s", files, Size(size))
}

// CleanPath is a file's path as a version keeps it (img/logo.png), or
// why it can't be one: it stays inside the version, names no hidden file
// (.git, .DS_Store), and isn't the record's name, which the folder a
// promoted one lives in holds beside its files.
func CleanPath(p string) (string, error) {
	if p == "" {
		return "", errors.New("a file needs a path")
	}
	if !utf8.ValidString(p) || strings.ContainsRune(p, '\\') || strings.IndexFunc(p, unicode.IsControl) >= 0 {
		return "", fmt.Errorf("%q isn't a path a version can have: forward slashes, and no control characters", p)
	}
	if strings.HasPrefix(p, "/") {
		return "", fmt.Errorf("%q starts at the root: a file's path is inside its version (img/logo.png)", p)
	}
	if len(p) > 240 {
		return "", fmt.Errorf("%q is too long a path (240 at most)", clip(p))
	}
	for seg := range strings.SplitSeq(p, "/") {
		switch {
		case seg == "" || seg == "." || seg == "..":
			return "", fmt.Errorf("%q leads out of the version, or nowhere in it", p)
		case strings.HasPrefix(seg, "."):
			return "", fmt.Errorf("%q is hidden (%s): a version keeps no hidden files", p, seg)
		}
	}
	if strings.EqualFold(p, RecordFile) {
		return "", fmt.Errorf("%s is the artifact's record's name: call the file something else", RecordFile)
	}
	return p, nil
}

// ReadFolder reads a folder as a version's files: every file in it and
// under it, by its path from the folder. Hidden files and folders (.git,
// .DS_Store) are left out. A link is refused, since what it points at
// isn't in the folder.
func ReadFolder(dir string) (Files, error) {
	var out Files
	size := 0
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == dir {
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		switch {
		case d.IsDir():
			return nil
		case d.Type()&fs.ModeSymlink != 0:
			return fmt.Errorf("%s is a link: put the file itself in the folder", rel)
		case !d.Type().IsRegular():
			return fmt.Errorf("%s isn't a file", rel)
		}
		if len(out) >= MaxFiles {
			return fmt.Errorf("it has more than %d files, the most a version can: leave out what the page doesn't load", MaxFiles)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if size += int(info.Size()); size > MaxSize {
			return tooBig(size)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		out = append(out, File{Path: rel, Data: data})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func tooBig(size int) error {
	return fmt.Errorf("its files come to %s or more, and a version can be at most %s: use smaller images, or fewer", Size(size), Size(MaxSize))
}

// WriteFolder writes a version's files into dir, each at its path, making
// the folders between.
func (f Files) WriteFolder(dir string) error {
	for _, x := range f {
		p, err := CleanPath(x.Path)
		if err != nil {
			return err
		}
		to := filepath.Join(dir, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(to, x.Data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// checkFiles checks a version's files are ones it can keep: a page, each
// at a path of its own, within the limits. It's them by path.
func checkFiles(f Files) (Files, error) {
	if len(f) == 0 {
		return nil, errors.New("it has nothing in it")
	}
	if len(f) > MaxFiles {
		return nil, fmt.Errorf("it has %d files, and a version can have at most %d: leave out what the page doesn't load", len(f), MaxFiles)
	}
	out := make(Files, 0, len(f))
	seen := map[string]string{}
	for _, x := range f {
		p, err := CleanPath(x.Path)
		if err != nil {
			return nil, err
		}
		// One file to a path, whatever its case: a promoted one's files
		// are checked out on disks that don't tell them apart.
		if was, ok := seen[strings.ToLower(p)]; ok {
			return nil, fmt.Errorf("%s and %s are the same path but for their case: name one of them something else", was, p)
		}
		seen[strings.ToLower(p)] = p
		out = append(out, File{Path: p, Data: x.Data})
	}
	for p := range seen {
		// A file where another's path has a folder (img, and img/logo.png).
		for dir := path.Dir(p); dir != "."; dir = path.Dir(dir) {
			if was, ok := seen[dir]; ok {
				return nil, fmt.Errorf("%s is a file, and a folder too (%s): name one of them something else", was, seen[p])
			}
		}
	}
	if size := out.Size(); size > MaxSize {
		return nil, tooBig(size)
	}
	if _, ok := out.Index(); !ok {
		return nil, fmt.Errorf("it has no %s: that's a version's page, at the top of its files, which its frame opens", IndexFile)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}
