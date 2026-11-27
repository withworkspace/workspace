package runs

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

// A task that names its inputs is cached: once it has passed, a run of it
// asked for again with the same inputs (the same files, command and
// variables) is that pass, reused, with its outputs put back, rather than
// run again. Key says what a run's inputs come to.

// skipped are folders a pattern doesn't reach into unless it names them:
// what's installed, and git's and ws's own.
var skipped = []string{"node_modules", ".git", ".ws"}

// KeyOf is what a task's key is made of.
type KeyOf struct {
	// Repo is the repo it's a recipe of ("" for the workspace's own), and
	// Command what it runs, as it's run.
	Repo, Command string
	// Inputs are the files it reads, as its recipe names them, found in
	// Dir, where it runs.
	Dir    string
	Inputs []string
	// Outputs are what it writes that's kept, and Env the variables it
	// reads, by name, with what each is set to ("" for unset).
	Outputs []string
	Env     map[string]string
}

// keyVersion is said in every key: a change to what's in one makes every
// key kept before it a miss.
const keyVersion = "ws-run-cache-1"

// Key is what o's run comes to: the same key, the same run. It reads the
// files o's inputs name where it runs, which has to be in git: git says
// the files it has as they're committed, and the rest (changed, new and
// ignored files) are read.
func Key(o KeyOf) (string, error) {
	ins, err := cleanPatterns(o.Inputs)
	if err != nil {
		return "", fmt.Errorf("inputs: %w", err)
	}
	if len(ins) == 0 {
		return "", errors.New("it names no inputs")
	}
	outs, err := cleanPatterns(o.Outputs)
	if err != nil {
		return "", fmt.Errorf("outputs: %w", err)
	}
	files, err := inputFiles(o.Dir, ins)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	// The recipe as it's written where it's asked for (a capsule's
	// ws.repo.toml may say it otherwise), then what its inputs come to.
	fmt.Fprintf(h, "%s\nrepo %q\ncommand %q\n", keyVersion, o.Repo, o.Command)
	for _, p := range ins {
		fmt.Fprintf(h, "input %q\n", p)
	}
	for _, p := range outs {
		fmt.Fprintf(h, "output %q\n", p)
	}
	for _, name := range slices.Sorted(mapKeys(o.Env)) {
		fmt.Fprintf(h, "env %q=%q\n", name, o.Env[name])
	}
	for _, name := range slices.Sorted(mapKeys(files)) {
		fmt.Fprintf(h, "file %q %s\n", name, files[name])
	}
	return hex.EncodeToString(h.Sum(nil))[:32], nil
}

func mapKeys[V any](m map[string]V) func(func(string) bool) {
	return func(yield func(string) bool) {
		for k := range m {
			if !yield(k) {
				return
			}
		}
	}
}

// pathspecs is ps as git's: a path as itself and what's below it, a
// pattern as a glob, with what's skipped left out unless one names it.
func pathspecs(ps []string) []string {
	var out []string
	for _, p := range ps {
		if hasMeta(p) {
			out = append(out, ":(glob)"+p)
		} else {
			out = append(out, ":(literal)"+p)
		}
	}
	for _, s := range skipped {
		named := false
		for _, p := range ps {
			if slices.Contains(strings.Split(p, "/"), s) {
				named = true
			}
		}
		if !named {
			out = append(out, ":(exclude,glob)**/"+s+"/**")
		}
	}
	return out
}

// inputFiles is each file the patterns name in dir, slashed from dir, with
// what it is: its mode and its contents' hash, as git has them where it's
// unchanged, else as it's read.
func inputFiles(dir string, ps []string) (map[string]string, error) {
	said, err := gitIn(dir, "rev-parse", "--show-prefix")
	if err != nil {
		return nil, fmt.Errorf("inputs are read through git, and %s isn't in a git repo", dir)
	}
	prefix := strings.TrimSpace(string(said))
	specs := pathspecs(ps)
	out := map[string]string{}
	rel := func(name string) (string, bool) {
		if !strings.HasPrefix(name, prefix) {
			return "", false
		}
		return strings.TrimPrefix(name, prefix), true
	}

	// What git has, as it has it: mode and blob.
	listed, err := gitIn(dir, append([]string{"ls-files", "-z", "-s", "--full-name", "--"}, specs...)...)
	if err != nil {
		return nil, err
	}
	for _, line := range strings.Split(string(listed), "\x00") {
		meta, name, ok := strings.Cut(line, "\t")
		if !ok {
			continue
		}
		f := strings.Fields(meta) // mode blob stage
		if len(f) < 2 {
			continue
		}
		if r, ok := rel(name); ok {
			out[r] = f[0] + " " + f[1]
		}
	}

	// What's changed since, new, or ignored: read as it is now.
	status, err := gitIn(dir, append([]string{"status", "-z", "--porcelain=v1", "--untracked-files=all", "--ignored=traditional", "--no-renames", "--"}, specs...)...)
	if err != nil {
		return nil, err
	}
	for _, entry := range strings.Split(string(status), "\x00") {
		if len(entry) < 4 {
			continue
		}
		r, ok := rel(entry[3:])
		if !ok {
			continue
		}
		what, err := readFile(filepath.Join(dir, filepath.FromSlash(r)))
		switch {
		case errors.Is(err, fs.ErrNotExist):
			delete(out, r)
		case err != nil:
			return nil, err
		default:
			out[r] = what
		}
	}
	return out, nil
}

// readFile is what the file at p is, as inputFiles says it: link and
// where it points, or file, whether it runs, and its contents' hash.
func readFile(p string) (string, error) {
	fi, err := os.Lstat(p)
	if err != nil {
		return "", err
	}
	switch {
	case fi.Mode()&fs.ModeSymlink != 0:
		to, err := os.Readlink(p)
		return "link " + to, err
	case fi.IsDir():
		return "", fs.ErrNotExist // a submodule, or a folder git lists whole: its files are what's read
	}
	f, err := os.Open(p)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	mode := "file"
	if fi.Mode()&0o111 != 0 {
		mode = "exec"
	}
	return mode + " " + hex.EncodeToString(h.Sum(nil)), nil
}

// gitIn runs git in dir, reading only: it takes no lock another git could
// be waiting on.
func gitIn(dir string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", append([]string{"--no-optional-locks", "-C", dir}, args...)...) //nolint:noctx // a read of git as a run is started, with no context to end it with
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git %s: %s", args[0], strings.TrimSpace(stderr.String()))
	}
	return out, nil
}
