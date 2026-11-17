package review

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/withworkspace/workspace/internal/git"
)

// Fingerprints are each changed file's change between base and head, by
// path, as an id that doesn't depend on where the change sits (git
// patch-id): the same change rebased onto a newer base has the same one.
// Looking at a pull request again after a rebase compares these with what
// was reviewed, file by file, so a rebase that changes nothing shows
// nothing to re-read.
func Fingerprints(dir, base, head string) (map[string]string, error) {
	names, err := git.Read(context.Background(), dir, "diff", "-z", "--name-status", "-M", base, head)
	if err != nil {
		return nil, err
	}
	var paths []string
	fields := strings.Split(strings.TrimSuffix(string(names), "\x00"), "\x00")
	for i := 0; i+1 < len(fields); i += 2 {
		path := fields[i+1]
		if c := fields[i][0]; c == 'R' || c == 'C' {
			if i+2 >= len(fields) {
				break
			}
			path = fields[i+2]
			i++
		}
		paths = append(paths, path)
	}
	patch, err := git.Read(context.Background(), dir, "diff", "--no-color", "--no-ext-diff", "--full-index", "-M", "-U3", base, head)
	if err != nil {
		return nil, err
	}
	sections := splitSections(string(patch))
	if len(sections) != len(paths) {
		return nil, fmt.Errorf("git listed %d changed files but patched %d", len(paths), len(sections))
	}
	out := map[string]string{}
	if len(paths) == 0 {
		return out, nil
	}
	// One patch-id run for them all: each file's patch goes in as a commit
	// of its own, named by its place in the list.
	var in bytes.Buffer
	for i, sec := range sections {
		fmt.Fprintf(&in, "commit %040x\n", i+1)
		for _, l := range sec {
			// The index line names blobs, which a rebase doesn't change
			// but an edit elsewhere in the file would: the id is of the
			// change, so it's left to patch-id (which skips it).
			in.WriteString(l + "\n")
		}
	}
	res, err := git.Cmd{Dir: dir, Args: []string{"patch-id", "--stable"}, Stdin: &in}.Output()
	if err != nil {
		return nil, err
	}
	for _, l := range strings.Split(strings.TrimSpace(string(res)), "\n") {
		id, commit, ok := strings.Cut(l, " ")
		if !ok {
			continue
		}
		var i int
		if _, err := fmt.Sscanf(commit, "%x", &i); err != nil || i < 1 || i > len(paths) {
			continue
		}
		out[paths[i-1]] = id
	}
	// A file whose patch has no lines to hash (a pure rename, a mode
	// change, a binary file) gets no id from patch-id: its blobs stand in.
	for i, p := range paths {
		if _, ok := out[p]; !ok {
			out[p] = "blob:" + blobsOf(sections[i])
		}
	}
	return out, nil
}

// blobsOf is a file section's index line ("index a..b"), or its header.
func blobsOf(section []string) string {
	for _, l := range section {
		if m := indexLine.FindStringSubmatch(l); m != nil {
			return m[1] + ".." + m[2]
		}
	}
	return strings.Join(section, "\n")
}
