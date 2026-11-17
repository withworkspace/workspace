package plugin

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// A plugin is published as a repo. Each release of it holds a build for
// each platform, <name>_<tag>_<os>_<arch>.tar.gz (its plugin.toml and what
// its command starts), and checksums.txt, as ws's own releases do. ws
// plugin add owner/repo installs this machine's build of the latest
// release. A repo with no release, or none for this machine, is installed
// from its source, which is built here.

// Releases is where plugins are published: GitHub (internal/github).
type Releases interface {
	// Release is a release of repo: the one tagged tag, or with "" the
	// latest. It's the release's tag and the names of its files; ok is
	// false if there's no such release.
	Release(repo, tag string) (name string, files []string, ok bool, err error)
	// Download fetches files of the release tagged tag into dir.
	Download(repo, tag string, files []string, dir string) error
	// Clone fetches repo's source into dir, which isn't there yet: at
	// ref (a tag or a branch), or with "" its default branch.
	Clone(repo, ref, dir string) error
}

// Source is where an installed plugin came from, kept beside it so it can
// be updated.
type Source struct {
	Repo string `json:"repo"`
	// Tag is the release it's from; "" for one built from the source of
	// the default branch.
	Tag string `json:"tag,omitempty"`
	// File is the release's file it was, "" for one built from source.
	File string    `json:"file,omitempty"`
	At   time.Time `json:"at"`
}

func sourcePath(root, name string) string { return filepath.Join(home(root, name), "source.json") }

// ReadSource is where the plugin was installed from; false for one added
// from a folder.
func ReadSource(root, name string) (Source, bool) {
	var s Source
	data, err := os.ReadFile(sourcePath(root, name))
	if err != nil || json.Unmarshal(data, &s) != nil || s.Repo == "" {
		return Source{}, false
	}
	return s, true
}

// WriteSource records where the plugin was installed from. A zero Source
// forgets it: it's been added from a folder since.
func WriteSource(root, name string, s Source) error {
	if s.Repo == "" {
		err := os.Remove(sourcePath(root, name))
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(sourcePath(root, name), append(data, '\n'), 0o644)
}

var repoRE = regexp.MustCompile(`^([A-Za-z0-9][A-Za-z0-9-]*)/([A-Za-z0-9._-]+)$`)

// ParseRepo reads what names a published plugin: owner/repo, with @tag for
// a release that isn't the latest, or its address on GitHub. ok is false
// for anything else (a folder).
func ParseRepo(s string) (repo, tag string, ok bool) {
	s = strings.TrimSuffix(strings.TrimSpace(s), "/")
	for _, prefix := range []string{"https://github.com/", "http://github.com/", "github.com/"} {
		s = strings.TrimPrefix(s, prefix)
	}
	s, tag, _ = strings.Cut(s, "@")
	s = strings.TrimSuffix(s, ".git")
	if !repoRE.MatchString(s) || strings.HasPrefix(tag, "-") {
		return "", "", false
	}
	return s, tag, true
}

// Fetch fetches the plugin published at repo into the folder into, which
// is empty, ready to be added: this platform's build of the release tagged
// tag (with "", the latest), or, where there's no such build, the repo's
// source, to be built. say is told what it's doing.
func Fetch(rs Releases, repo, tag, goos, goarch, into string, say func(string)) (Source, error) {
	src := Source{Repo: repo, At: time.Now().UTC()}
	name, files, ok, err := rs.Release(repo, tag)
	if err != nil {
		return src, err
	}
	if !ok && tag != "" {
		// Not a release: a tag or a branch of the source, perhaps.
		say(fmt.Sprintf("%s has no release tagged %s: fetching its source there", repo, tag))
		src.Tag = tag
		return src, cloneInto(rs, repo, tag, into)
	}
	file := ""
	if ok {
		suffix := "_" + goos + "_" + goarch + ".tar.gz"
		for _, f := range files {
			if strings.HasSuffix(f, suffix) {
				file = f
			}
		}
	}
	if file == "" {
		if ok {
			say(fmt.Sprintf("%s %s has no build for %s/%s: fetching its source, to build here", repo, name, goos, goarch))
			src.Tag = name
			return src, cloneInto(rs, repo, name, into)
		}
		say(fmt.Sprintf("%s has no releases: fetching its source, to build here", repo))
		return src, cloneInto(rs, repo, "", into)
	}

	say(fmt.Sprintf("downloading %s from %s", file, repo))
	tmp, err := os.MkdirTemp("", "ws-plugin-download-")
	if err != nil {
		return src, err
	}
	defer os.RemoveAll(tmp)
	want := []string{file}
	sums := false
	for _, f := range files {
		if f == "checksums.txt" {
			want, sums = append(want, f), true
		}
	}
	if err := rs.Download(repo, name, want, tmp); err != nil {
		return src, err
	}
	archive := filepath.Join(tmp, file)
	if sums {
		// The release lists every build's checksum: this one's is checked
		// before anything of it runs.
		if err := checksum(archive, filepath.Join(tmp, "checksums.txt")); err != nil {
			return src, err
		}
	} else {
		say(fmt.Sprintf("%s %s lists no checksums: installed as it was downloaded", repo, name))
	}
	if err := extract(archive, into); err != nil {
		return src, fmt.Errorf("%s: %w", file, err)
	}
	src.Tag, src.File = name, file
	return src, nil
}

// cloneInto fetches a plugin's source into a folder that's there and
// empty.
func cloneInto(rs Releases, repo, ref, into string) error {
	// A clone is into a folder that isn't there.
	if err := os.Remove(into); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := rs.Clone(repo, ref, into); err != nil {
		return err
	}
	// It's installed as a copy: what git keeps of it isn't the plugin's.
	return os.RemoveAll(filepath.Join(into, ".git"))
}

// checksum checks the file at path against its line in the sums file:
// "<sha256>  <name>", as sha256sum writes it.
func checksum(path, sums string) error {
	list, err := os.ReadFile(sums)
	if err != nil {
		return err
	}
	name, want := filepath.Base(path), ""
	for _, line := range strings.Split(string(list), "\n") {
		if fields := strings.Fields(line); len(fields) == 2 && strings.TrimPrefix(fields[1], "*") == name {
			want = fields[0]
		}
	}
	if want == "" {
		return fmt.Errorf("%s isn't in the release's checksums.txt", name)
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	if got := hex.EncodeToString(h.Sum(nil)); !strings.EqualFold(got, want) {
		return fmt.Errorf("%s isn't what the release's checksums.txt says it is (it's %s, and should be %s): not installed", name, got, want)
	}
	return nil
}

// maxPluginSize is more than any plugin's build is: an archive that
// unpacks to more isn't one.
const maxPluginSize = 1 << 30

// extract unpacks the .tar.gz at archive into dir: its files and folders,
// and nothing that would land outside dir.
func extract(archive, dir string) error {
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("it isn't a .tar.gz: %w", err)
	}
	tr := tar.NewReader(gz)
	var size int64
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		name := filepath.Clean(filepath.FromSlash(h.Name))
		if name == "." {
			continue
		}
		if filepath.IsAbs(name) || name == ".." || strings.HasPrefix(name, ".."+string(filepath.Separator)) {
			return fmt.Errorf("it has a file outside the plugin's folder (%s): not installed", h.Name)
		}
		to := filepath.Join(dir, name)
		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(to, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if size += h.Size; size > maxPluginSize {
				return errors.New("it unpacks to more than a plugin is: not installed")
			}
			if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
				return err
			}
			mode := os.FileMode(0o644)
			if h.FileInfo().Mode()&0o111 != 0 {
				mode = 0o755
			}
			out, err := os.OpenFile(to, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, io.LimitReader(tr, h.Size)); err != nil {
				out.Close()
				return err
			}
			if err := out.Close(); err != nil {
				return err
			}
		default:
			// A link, a device: nothing a plugin's build has.
			return fmt.Errorf("it has something that isn't a file or a folder (%s): not installed", h.Name)
		}
	}
}
