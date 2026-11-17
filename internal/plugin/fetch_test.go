package plugin_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/withworkspace/workspace/internal/plugin"
)

// published is a repo's releases and its source, as a test has them.
type published struct {
	tag    string            // its latest release's; "" for none
	files  map[string][]byte // that release's files
	source map[string]string // its source, for a clone
	cloned []string          // the refs its source was fetched at
	err    error             // what reading its releases fails with
}

func (p *published) Release(repo, tag string) (string, []string, bool, error) {
	if p.err != nil {
		return "", nil, false, p.err
	}
	if p.tag == "" || (tag != "" && tag != p.tag) {
		return "", nil, false, nil
	}
	var names []string
	for name := range p.files {
		names = append(names, name)
	}
	return p.tag, names, true, nil
}

func (p *published) Download(repo, tag string, files []string, dir string) error {
	for _, f := range files {
		data, ok := p.files[f]
		if !ok {
			return errors.New("no such file: " + f)
		}
		if err := os.WriteFile(filepath.Join(dir, f), data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func (p *published) Clone(repo, ref, dir string) error {
	p.cloned = append(p.cloned, ref)
	if p.source == nil {
		return errors.New("no such repo")
	}
	for name, text := range p.source {
		to := filepath.Join(dir, name)
		os.MkdirAll(filepath.Dir(to), 0o755)
		if err := os.WriteFile(to, []byte(text), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// entry is a file in an archive a test makes.
type entry struct {
	name string
	text string
	mode int64
	kind byte // tar's type; 0 for a file
}

func archive(t *testing.T, entries ...entry) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, e := range entries {
		h := &tar.Header{Name: e.name, Mode: e.mode, Size: int64(len(e.text)), Typeflag: tar.TypeReg}
		if e.mode == 0 {
			h.Mode = 0o644
		}
		if e.kind != 0 {
			h.Typeflag, h.Size, h.Linkname = e.kind, 0, e.text
		}
		if err := tw.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		if e.kind == 0 {
			tw.Write([]byte(e.text))
		}
	}
	tw.Close()
	gz.Close()
	return buf.Bytes()
}

func sums(files map[string][]byte) []byte {
	var out strings.Builder
	for name, data := range files {
		sum := sha256.Sum256(data)
		out.WriteString(hex.EncodeToString(sum[:]) + "  " + name + "\n")
	}
	return []byte(out.String())
}

const jiraToml = "name = \"jira\"\nversion = \"0.3.0\"\ncommand = [\"./ws-jira\"]\n"

func fetch(t *testing.T, p *published, tag string) (dir string, src plugin.Source, said string, err error) {
	t.Helper()
	dir = t.TempDir()
	var lines []string
	src, err = plugin.Fetch(p, "acme/plugin-jira", tag, "linux", "arm64", dir, func(s string) { lines = append(lines, s) })
	return dir, src, strings.Join(lines, "\n"), err
}

func TestFetchRelease(t *testing.T) {
	files := map[string][]byte{
		"jira_v0.3.0_linux_arm64.tar.gz":  archive(t, entry{name: "plugin.toml", text: jiraToml}, entry{name: "ws-jira", text: "#!/bin/sh\n", mode: 0o755}),
		"jira_v0.3.0_darwin_arm64.tar.gz": archive(t, entry{name: "plugin.toml", text: "wrong one"}),
	}
	files["checksums.txt"] = sums(files)
	p := &published{tag: "v0.3.0", files: files}

	dir, src, said, err := fetch(t, p, "")
	if err != nil {
		t.Fatal(err)
	}
	if src.Repo != "acme/plugin-jira" || src.Tag != "v0.3.0" || src.File != "jira_v0.3.0_linux_arm64.tar.gz" {
		t.Errorf("where it's from: %+v", src)
	}
	if !strings.Contains(said, "downloading jira_v0.3.0_linux_arm64.tar.gz") {
		t.Errorf("it said: %s", said)
	}
	// This machine's build: its plugin.toml, and what it starts, to run.
	if m, err := plugin.ReadManifest(dir); err != nil || m.Name != "jira" {
		t.Errorf("what was fetched: %+v, %v", m, err)
	}
	if info, err := os.Stat(filepath.Join(dir, "ws-jira")); err != nil || info.Mode()&0o111 == 0 {
		t.Errorf("its executable: %v, %v", info, err)
	}
	if len(p.cloned) != 0 {
		t.Errorf("its source was fetched too: %v", p.cloned)
	}

	// The same release, asked for by its tag.
	if _, src, _, err := fetch(t, p, "v0.3.0"); err != nil || src.File == "" {
		t.Errorf("by its tag: %+v, %v", src, err)
	}
}

func TestFetchChecksum(t *testing.T) {
	good := archive(t, entry{name: "plugin.toml", text: jiraToml})
	files := map[string][]byte{"jira_v0.3.0_linux_arm64.tar.gz": good}
	files["checksums.txt"] = sums(files)
	// What's downloaded isn't what the release said it published.
	files["jira_v0.3.0_linux_arm64.tar.gz"] = archive(t, entry{name: "plugin.toml", text: jiraToml}, entry{name: "extra", text: "x"})
	dir, _, _, err := fetch(t, &published{tag: "v0.3.0", files: files}, "")
	if err == nil || !strings.Contains(err.Error(), "isn't what the release's checksums.txt says") {
		t.Errorf("a build that doesn't match its checksum: %v", err)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Errorf("it was unpacked anyway: %v", entries)
	}

	// A build the checksums don't list isn't taken on trust.
	files["checksums.txt"] = []byte("abc  something_else.tar.gz\n")
	if _, _, _, err := fetch(t, &published{tag: "v0.3.0", files: files}, ""); err == nil || !strings.Contains(err.Error(), "isn't in the release's checksums.txt") {
		t.Errorf("a build its checksums don't list: %v", err)
	}

	// A release with no checksums is installed, and it's said.
	delete(files, "checksums.txt")
	files["jira_v0.3.0_linux_arm64.tar.gz"] = good
	if _, _, said, err := fetch(t, &published{tag: "v0.3.0", files: files}, ""); err != nil || !strings.Contains(said, "lists no checksums") {
		t.Errorf("a release with no checksums: %v, said %q", err, said)
	}
}

func TestFetchSource(t *testing.T) {
	source := map[string]string{"plugin.toml": jiraToml, "main.go": "package main\n", ".git/HEAD": "ref: refs/heads/main\n"}

	// No releases: its source, at its default branch.
	p := &published{source: source}
	dir, src, said, err := fetch(t, p, "")
	if err != nil {
		t.Fatal(err)
	}
	if src.Tag != "" || src.File != "" || !strings.Contains(said, "has no releases") {
		t.Errorf("from source: %+v, said %q", src, said)
	}
	if _, err := os.Stat(filepath.Join(dir, "main.go")); err != nil {
		t.Errorf("its source wasn't fetched: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
		t.Error("what git keeps of it was installed with it")
	}

	// A release with no build for this machine: its source, at the release.
	files := map[string][]byte{"jira_v0.3.0_darwin_arm64.tar.gz": archive(t, entry{name: "plugin.toml", text: jiraToml})}
	p = &published{tag: "v0.3.0", files: files, source: source}
	if _, src, said, err := fetch(t, p, ""); err != nil || src.Tag != "v0.3.0" || src.File != "" || !strings.Contains(said, "no build for linux/arm64") {
		t.Errorf("no build for this machine: %+v, %v, said %q", src, err, said)
	}
	if len(p.cloned) != 1 || p.cloned[0] != "v0.3.0" {
		t.Errorf("its source was fetched at %v", p.cloned)
	}

	// Something that isn't a release: a branch or a tag of its source.
	p = &published{tag: "v0.3.0", files: files, source: source}
	if _, src, _, err := fetch(t, p, "try-this"); err != nil || src.Tag != "try-this" || len(p.cloned) != 1 || p.cloned[0] != "try-this" {
		t.Errorf("a branch: %+v, %v, cloned %v", src, err, p.cloned)
	}

	// Neither a release nor a repo to fetch.
	if _, _, _, err := fetch(t, &published{}, ""); err == nil {
		t.Error("a repo that isn't there was fetched")
	}
	if _, _, _, err := fetch(t, &published{err: errors.New("GitHub is down")}, ""); err == nil || !strings.Contains(err.Error(), "GitHub is down") {
		t.Errorf("releases that can't be read: %v", err)
	}
}

func TestFetchRefusesWhatIsntAPlugins(t *testing.T) {
	for what, bad := range map[string]entry{
		"a file outside its folder": {name: "../../outside", text: "x"},
		"an absolute path":          {name: "/etc/outside", text: "x"},
		"a link":                    {name: "link", text: "/etc/passwd", kind: tar.TypeSymlink},
	} {
		files := map[string][]byte{"jira_v0.3.0_linux_arm64.tar.gz": archive(t, entry{name: "plugin.toml", text: jiraToml}, bad)}
		files["checksums.txt"] = sums(files)
		if _, _, _, err := fetch(t, &published{tag: "v0.3.0", files: files}, ""); err == nil || !strings.Contains(err.Error(), "not installed") {
			t.Errorf("%s: %v", what, err)
		}
	}
	files := map[string][]byte{"jira_v0.3.0_linux_arm64.tar.gz": []byte("not an archive")}
	files["checksums.txt"] = sums(files)
	if _, _, _, err := fetch(t, &published{tag: "v0.3.0", files: files}, ""); err == nil || !strings.Contains(err.Error(), "isn't a .tar.gz") {
		t.Errorf("something that isn't an archive: %v", err)
	}
}

func TestParseRepo(t *testing.T) {
	for in, want := range map[string][2]string{
		"withworkspace/plugin-jira":                         {"withworkspace/plugin-jira", ""},
		"withworkspace/plugin-jira@v0.3.0":                  {"withworkspace/plugin-jira", "v0.3.0"},
		"github.com/withworkspace/plugin-jira":              {"withworkspace/plugin-jira", ""},
		"https://github.com/withworkspace/plugin-jira.git":  {"withworkspace/plugin-jira", ""},
		"https://github.com/withworkspace/plugin-jira/":     {"withworkspace/plugin-jira", ""},
		"https://github.com/withworkspace/plugin-jira@main": {"withworkspace/plugin-jira", "main"},
	} {
		repo, tag, ok := plugin.ParseRepo(in)
		if !ok || repo != want[0] || tag != want[1] {
			t.Errorf("%s: %q, %q, %v", in, repo, tag, ok)
		}
	}
	for _, in := range []string{"plugin-jira", "./plugin-jira", "a/b/c", "/abs/folder", "../up/one", "owner/repo@--upload-pack=x", "", "own er/repo"} {
		if repo, _, ok := plugin.ParseRepo(in); ok {
			t.Errorf("%q was read as the repo %s", in, repo)
		}
	}
}

func TestSource(t *testing.T) {
	root, _ := workspace(t)
	os.MkdirAll(filepath.Join(plugin.Dir(root), "jira"), 0o755)
	if _, ok := plugin.ReadSource(root, "jira"); ok {
		t.Error("a plugin from nowhere has a source")
	}
	want := plugin.Source{Repo: "acme/plugin-jira", Tag: "v0.3.0", File: "jira_v0.3.0_linux_arm64.tar.gz"}
	if err := plugin.WriteSource(root, "jira", want); err != nil {
		t.Fatal(err)
	}
	if got, ok := plugin.ReadSource(root, "jira"); !ok || got.Repo != want.Repo || got.Tag != want.Tag || got.File != want.File {
		t.Errorf("read back: %+v, %v", got, ok)
	}
	// Added from a folder since: it's from nowhere again.
	if err := plugin.WriteSource(root, "jira", plugin.Source{}); err != nil {
		t.Fatal(err)
	}
	if _, ok := plugin.ReadSource(root, "jira"); ok {
		t.Error("its source wasn't forgotten")
	}
}
