package github

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Releases reads a repo's releases and its source, for installing what's
// published there (a plugin: ws plugin add owner/repo). With gh logged
// in, it's gh that's asked, so a private repo is read as whoever's logged
// in. Without it, GitHub is asked as anyone, which a public repo answers:
// a release is then one that lists its files in a checksums.txt.
type Releases struct{}

var (
	ghOnce  sync.Once
	ghReady bool
)

// loggedIn says whether gh is here and logged in.
func loggedIn() bool {
	ghOnce.Do(func() {
		_, _, err := ghExec("auth", "status")
		ghReady = err == nil
	})
	return ghReady
}

// anyone is how GitHub's asked without gh.
var anyone = &http.Client{Timeout: 5 * time.Minute}

// Release is a release of repo ("owner/name"): the one tagged tag, or with
// "" the latest. It's the release's tag and its files' names; ok is false
// if there's no such release.
func (Releases) Release(repo, tag string) (name string, files []string, ok bool, err error) {
	if !loggedIn() {
		return releaseAsAnyone(repo, tag)
	}
	args := []string{"release", "view"}
	if tag != "" {
		args = append(args, tag)
	}
	stdOut, stdErr, err := ghExec(append(args, "--repo", repo, "--json", "tagName,assets")...)
	if err != nil {
		if strings.Contains(stdErr.String(), "release not found") {
			return "", nil, false, nil
		}
		return "", nil, false, fmt.Errorf("reading %s's releases: %s", repo, ghError(stdOut.Bytes(), stdErr.String(), err))
	}
	var rel struct {
		TagName string `json:"tagName"`
		Assets  []struct {
			Name string `json:"name"`
		} `json:"assets"`
	}
	if err := json.Unmarshal(stdOut.Bytes(), &rel); err != nil {
		return "", nil, false, fmt.Errorf("reading %s's releases: %w", repo, err)
	}
	for _, a := range rel.Assets {
		files = append(files, a.Name)
	}
	return rel.TagName, files, rel.TagName != "", nil
}

// githubHost is where GitHub's pages are: a test's stand-in, in a test.
var githubHost = "https://github.com"

// noFollow asks without following where it's sent: where the latest
// release is, is where it's sent.
var noFollow = &http.Client{
	Timeout:       time.Minute,
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
}

// releaseAsAnyone reads a release without GitHub's API, which answers
// anyone only so many times an hour: the latest release is where
// /releases/latest sends you, and a release's files are those its
// checksums.txt lists.
func releaseAsAnyone(repo, tag string) (name string, files []string, ok bool, err error) {
	if tag == "" {
		res, err := noFollow.Get(githubHost + "/" + repo + "/releases/latest")
		if err != nil {
			return "", nil, false, fmt.Errorf("reading %s's releases: %w", repo, err)
		}
		res.Body.Close()
		// To /releases/tag/<tag> where there's a release; to /releases
		// where there's none; not found for a repo anyone can't see.
		_, after, found := strings.Cut(res.Header.Get("Location"), "/releases/tag/")
		if res.StatusCode != http.StatusFound || !found || after == "" {
			return "", nil, false, nil
		}
		tag = after
	}
	res, err := anyone.Get(githubHost + "/" + repo + "/releases/download/" + tag + "/checksums.txt")
	if err != nil {
		return "", nil, false, fmt.Errorf("reading %s's release %s: %w", repo, tag, err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		// No such release, or one that lists nothing: its source is what
		// there is.
		return "", nil, false, nil
	}
	list, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return "", nil, false, fmt.Errorf("reading %s's release %s: %w", repo, tag, err)
	}
	files = []string{"checksums.txt"}
	for _, line := range strings.Split(string(list), "\n") {
		if fields := strings.Fields(line); len(fields) == 2 {
			files = append(files, strings.TrimPrefix(fields[1], "*"))
		}
	}
	return tag, files, true, nil
}

// Download fetches files of repo's release tagged tag into dir.
func (Releases) Download(repo, tag string, files []string, dir string) error {
	if loggedIn() {
		args := []string{"release", "download", tag, "--repo", repo, "--dir", dir, "--clobber"}
		for _, f := range files {
			args = append(args, "--pattern", f)
		}
		if stdOut, stdErr, err := ghExec(args...); err != nil {
			return fmt.Errorf("downloading from %s's release %s: %s", repo, tag, ghError(stdOut.Bytes(), stdErr.String(), err))
		}
		return nil
	}
	for _, f := range files {
		if err := fetchFile(githubHost+"/"+repo+"/releases/download/"+tag+"/"+f, filepath.Join(dir, f)); err != nil {
			return fmt.Errorf("downloading %s from %s's release %s: %w", f, repo, tag, err)
		}
	}
	return nil
}

func fetchFile(url, to string) error {
	res, err := anyone.Get(url)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("GitHub said %s", res.Status)
	}
	out, err := os.Create(to)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, res.Body); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// Clone fetches repo's source into dir, which isn't there yet: at ref (a
// tag or a branch), or with "" its default branch. Its history isn't
// fetched.
func (Releases) Clone(repo, ref, dir string) error {
	depth := []string{"--depth", "1"}
	if ref != "" {
		depth = append(depth, "--branch", ref)
	}
	if loggedIn() {
		args := append([]string{"repo", "clone", repo, dir, "--"}, depth...)
		if stdOut, stdErr, err := ghExec(args...); err != nil {
			return fmt.Errorf("fetching %s's source: %s", repo, ghError(stdOut.Bytes(), stdErr.String(), err))
		}
		return nil
	}
	args := append(append([]string{"clone", "--quiet"}, depth...), githubHost+"/"+repo+".git", dir)
	cmd := exec.Command("git", args...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0") // a repo that wants a login isn't asked for one
	if out, err := cmd.CombinedOutput(); err != nil {
		said := strings.TrimSpace(string(out))
		if said == "" {
			said = err.Error()
		}
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return fmt.Errorf("fetching %s's source: %s (is it there, and public? gh auth login reads a private one as you)", repo, lastLine(said))
		}
		return fmt.Errorf("fetching %s's source: %s", repo, said)
	}
	return nil
}
