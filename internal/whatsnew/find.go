package whatsnew

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/withworkspace/workspace/internal/release"
)

// EntriesDir is where entries are written, from the repo's root.
const EntriesDir = "internal/whatsnew/entries"

// Find reads the entries in a clone of ws (root) and works out which
// release each is in: the first whose tag holds the commit that added it.
// One that isn't committed, or is in no release yet, is unreleased. An
// entry that doesn't read is an error: a build doesn't leave one out.
func Find(root string) ([]Entry, error) {
	dir := filepath.Join(root, EntriesDir)
	files, err := filepath.Glob(filepath.Join(dir, "*.md"))
	if err != nil {
		return nil, err
	}
	var out []Entry
	for _, path := range files {
		name := filepath.Base(path)
		if name == "README.md" {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		e, err := Parse(strings.TrimSuffix(name, ".md"), data)
		if err != nil {
			return nil, err
		}
		if e.Picture != "" {
			if _, err := os.Stat(filepath.Join(dir, e.Picture)); err != nil {
				return nil, fmt.Errorf("%s: its picture %s isn't beside it in %s", e.Slug, e.Picture, EntriesDir)
			}
		}
		if e.Version, err = releasedIn(root, filepath.Join(EntriesDir, name)); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, nil
}

// releasedIn is the first release whose tag holds the commit that added a
// file: "" for none yet.
func releasedIn(root, path string) (string, error) {
	added, err := git(root, "log", "--diff-filter=A", "--format=%H", "--", path)
	if err != nil {
		return "", err
	}
	commits := strings.Fields(added)
	if len(commits) == 0 {
		return "", nil // not committed
	}
	tags, err := git(root, "tag", "--contains", commits[len(commits)-1])
	if err != nil {
		return "", err
	}
	var first string
	for _, tag := range strings.Fields(tags) {
		if release.IsTag(tag) && (first == "" || release.Newer(first, tag)) {
			first = tag
		}
	}
	return first, nil
}

func git(root string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	out, err := cmd.Output()
	if err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(string(exit.Stderr)))
		}
		return "", err
	}
	return string(out), nil
}

// Pictures is the pictures of a release's entries, as files to publish
// with it: the file in entries/ for each name it's published as.
func Pictures(root string, entries []Entry, version string) map[string]string {
	out := map[string]string{}
	for _, e := range entries {
		if e.Version == version && e.Picture != "" {
			out[AssetName(e.Picture)] = filepath.Join(root, EntriesDir, e.Picture)
		}
	}
	return out
}
