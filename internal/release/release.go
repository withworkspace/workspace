// Package release is ws's own releases: finding the latest, fetching one's
// build for this machine, and putting it in place of the ws that's
// running. It does what scripts/install.sh does, for a ws that's installed
// already.
package release

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// DefaultRepo is where ws is released.
const DefaultRepo = "withworkspace/workspace"

// Source is where releases are fetched from: a repo's releases on GitHub.
type Source struct {
	// Base is the repo's page: https://github.com/withworkspace/workspace.
	Base string
	// HTTP fetches; nil is a client that gives up on a server that says
	// nothing for a minute.
	HTTP *http.Client
}

// FromEnv is the source ws upgrades from: GitHub's withworkspace/workspace,
// another repo there with $WS_REPO (as scripts/install.sh takes), or
// wherever $WS_RELEASES says a repo's page is (a mirror).
func FromEnv() Source {
	if base := os.Getenv("WS_RELEASES"); base != "" {
		return Source{Base: strings.TrimRight(base, "/")}
	}
	repo := os.Getenv("WS_REPO")
	if repo == "" {
		repo = DefaultRepo
	}
	return Source{Base: "https://github.com/" + repo}
}

func (s Source) client() *http.Client {
	if s.HTTP != nil {
		return s.HTTP
	}
	return &http.Client{Transport: &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		ResponseHeaderTimeout: time.Minute,
	}}
}

// Latest is the latest release's tag (v1.4.0), which is never a
// prerelease. It's read from where the releases' "latest" page sends you,
// which needs no API and no token.
func (s Source) Latest(ctx context.Context) (string, error) {
	c := *s.client()
	c.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.Base+"/releases/latest", nil)
	if err != nil {
		return "", err
	}
	resp, err := c.Do(req)
	if err != nil {
		return "", fmt.Errorf("finding the latest release: %w", err)
	}
	resp.Body.Close()
	_, tag, ok := strings.Cut(resp.Header.Get("Location"), "/releases/tag/")
	if !ok || tag == "" {
		if resp.StatusCode >= 400 {
			return "", fmt.Errorf("finding the latest release: %s said %s", s.Base, resp.Status)
		}
		return "", fmt.Errorf("%s has no release yet", s.Base)
	}
	return tag, nil
}

// Asset is the name of a release's build for a platform, or why there's
// none: releases are built for macOS on Apple silicon and Linux (amd64,
// arm64).
func Asset(version, goos, goarch string) (string, error) {
	switch goos + "/" + goarch {
	case "darwin/arm64", "linux/amd64", "linux/arm64":
		return fmt.Sprintf("workspace_%s_%s_%s.tar.gz", version, goos, goarch), nil
	case "darwin/amd64":
		return "", errors.New("there's no release of ws for Intel Macs: build it from a clone with just build")
	}
	return "", fmt.Errorf("there's no release of ws for %s/%s: build it from a clone with just build", goos, goarch)
}

// Fetch downloads a release's build for a platform, checks it against the
// release's checksums, and writes its ws beside dest (in dest's folder,
// so it can be renamed over it), having run it for its version. It
// returns where it wrote it, and the version it says it is. Nothing's
// left behind if it fails.
func (s Source) Fetch(ctx context.Context, version, goos, goarch, dest string) (path, says string, err error) {
	asset, err := Asset(version, goos, goarch)
	if err != nil {
		return "", "", err
	}
	base := s.Base + "/releases/download/" + version
	want, err := s.checksum(ctx, base+"/checksums.txt", asset)
	if err != nil {
		return "", "", err
	}

	dir := filepath.Dir(dest)
	archive, err := os.CreateTemp(dir, ".workspace.download-*")
	if err != nil {
		return "", "", &Unwritable{Dir: dir, Err: err}
	}
	defer os.Remove(archive.Name())
	defer archive.Close()
	sum := sha256.New()
	if err := s.get(ctx, base+"/"+asset, io.MultiWriter(archive, sum)); err != nil {
		return "", "", fmt.Errorf("downloading %s: %w", asset, err)
	}
	if got := hex.EncodeToString(sum.Sum(nil)); got != want {
		return "", "", fmt.Errorf("%s doesn't match its checksum (%s, and the release says %s): not installed", asset, got, want)
	}

	path = filepath.Join(dir, ".workspace.new")
	if _, err := archive.Seek(0, io.SeekStart); err != nil {
		return "", "", err
	}
	if err := extract(archive, path); err != nil {
		os.Remove(path)
		return "", "", fmt.Errorf("unpacking %s: %w", asset, err)
	}
	if says, err = VersionOf(path); err != nil {
		os.Remove(path)
		return "", "", fmt.Errorf("the ws in %s doesn't run here (%w): not installed", asset, err)
	}
	return path, says, nil
}

// Unwritable is a folder a ws can't be written to: the one the running
// ws is in, which an upgrade replaces.
type Unwritable struct {
	Dir string
	Err error
}

func (e *Unwritable) Error() string { return fmt.Sprintf("can't write to %s: %v", e.Dir, e.Err) }
func (e *Unwritable) Unwrap() error { return e.Err }

// checksum is the SHA-256 a release's checksums.txt gives a build.
func (s Source) checksum(ctx context.Context, url, asset string) (string, error) {
	var b strings.Builder
	if err := s.get(ctx, url, &b); err != nil {
		return "", fmt.Errorf("fetching the release's checksums: %w", err)
	}
	sc := bufio.NewScanner(strings.NewReader(b.String()))
	for sc.Scan() {
		if f := strings.Fields(sc.Text()); len(f) == 2 && strings.TrimPrefix(f[1], "*") == asset {
			return strings.ToLower(f[0]), nil
		}
	}
	return "", fmt.Errorf("%s isn't in the release's checksums.txt", asset)
}

func (s Source) get(ctx context.Context, url string, w io.Writer) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := s.client().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("%s isn't there: is there such a release?", url)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s said %s", url, resp.Status)
	}
	_, err = io.Copy(w, resp.Body)
	return err
}

// extract writes the workspace in a release's archive to path, to be run.
func extract(archive io.Reader, path string) error {
	gz, err := gzip.NewReader(archive)
	if err != nil {
		return err
	}
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return errors.New("it has no workspace in it")
		}
		if err != nil {
			return err
		}
		if h.Typeflag != tar.TypeReg || filepath.Base(h.Name) != "workspace" {
			continue
		}
		f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
		if err != nil {
			return err
		}
		if _, err := io.Copy(f, tr); err != nil {
			f.Close()
			return err
		}
		return f.Close()
	}
}

// VersionOf runs a ws binary for its version: "workspace version v1.4.0"
// is v1.4.0.
func VersionOf(exe string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, exe, "--version").Output()
	if err != nil {
		return "", err
	}
	fields := strings.Fields(string(out))
	if len(fields) == 0 {
		return "", fmt.Errorf("%s --version said nothing", exe)
	}
	return fields[len(fields)-1], nil
}

// PrevPath is where the ws an install replaced is kept, beside it, for
// putting back.
func PrevPath(exe string) string { return filepath.Join(filepath.Dir(exe), ".workspace.prev") }

// Install puts the ws at path (from Fetch) in place of exe, keeping the
// one that was there (PrevPath). It's renamed over it: a ws that's
// running, the daemon, keeps the file it started from rather than having
// it changed underneath.
func Install(path, exe string) error {
	prev := PrevPath(exe)
	os.Remove(prev)
	if err := os.Link(exe, prev); err != nil {
		// Kept if it can be: an upgrade doesn't fail for want of the old one.
		os.Remove(prev)
	}
	if err := os.Rename(path, exe); err != nil {
		os.Remove(path)
		return err
	}
	return nil
}

// Rollback puts back the ws the last Install replaced, and says which it
// is. The one it replaces isn't kept: installing it again gets it back.
func Rollback(exe string) (version string, err error) {
	prev := PrevPath(exe)
	if _, err := os.Stat(prev); err != nil {
		return "", errors.New("there's no ws kept from before an upgrade to put back")
	}
	if version, err = VersionOf(prev); err != nil {
		return "", fmt.Errorf("the ws kept from before the last upgrade doesn't run (%w): left as it is", err)
	}
	return version, os.Rename(prev, exe)
}

// Newer is whether release a is later than b, both tags (v1.4.0,
// v1.4.0-rc.1): by their numbers, and a prerelease before its release.
// One that isn't a tag is no later than anything.
func Newer(a, b string) bool {
	an, apre, aok := parse(a)
	bn, bpre, bok := parse(b)
	if !aok || !bok {
		return aok && !bok
	}
	for i := range an {
		if an[i] != bn[i] {
			return an[i] > bn[i]
		}
	}
	switch {
	case apre == bpre:
		return false
	case apre == "" || bpre == "":
		return apre == ""
	}
	return apre > bpre
}

// IsTag is whether v is a release's tag: v1.4.0, or a prerelease's
// v1.4.0-rc.1.
func IsTag(v string) bool {
	_, _, ok := parse(v)
	return ok
}

func parse(v string) (nums [3]int, pre string, ok bool) {
	v, found := strings.CutPrefix(v, "v")
	if !found {
		return nums, "", false
	}
	v, _, _ = strings.Cut(v, "+")
	v, pre, _ = strings.Cut(v, "-")
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return nums, "", false
	}
	for i, p := range parts {
		n := 0
		if p == "" {
			return nums, "", false
		}
		for _, r := range p {
			if r < '0' || r > '9' {
				return nums, "", false
			}
			n = n*10 + int(r-'0')
		}
		nums[i] = n
	}
	return nums, pre, true
}
