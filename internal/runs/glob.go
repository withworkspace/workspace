package runs

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"slices"
	"strings"
)

// Patterns of files, as a task's inputs and outputs are written: a path
// below where it runs (src, package.json), which is the file or
// everything in the folder, or a pattern of them, where * and ? match
// within a name and ** any number of folders (src/**/*.ts, dist/**).

// hasMeta is whether p is a pattern rather than a path.
func hasMeta(p string) bool { return strings.ContainsAny(p, "*?[") }

// cleanPattern is p as it's matched: slashes, no ./ in front or / behind.
// It's refused when it isn't below where the task runs.
func cleanPattern(p string) (string, error) {
	p = strings.TrimSpace(filepath.ToSlash(p))
	p = strings.TrimPrefix(p, "./")
	p = strings.TrimSuffix(p, "/")
	switch {
	case p == "" || p == ".":
		return "", errors.New("an empty path: name a file, a folder or a pattern (src/**)")
	case path.IsAbs(p) || slices.Contains(strings.Split(p, "/"), ".."):
		return "", fmt.Errorf("%s is not below where the task runs", p)
	}
	if _, err := path.Match(strings.ReplaceAll(p, "**", "*"), ""); err != nil {
		return "", fmt.Errorf("%s: %w", p, err)
	}
	return p, nil
}

// cleanPatterns is each of ps as cleanPattern has it, with what's wrong
// with the first that's refused.
func cleanPatterns(ps []string) ([]string, error) {
	out := make([]string, 0, len(ps))
	for _, p := range ps {
		c, err := cleanPattern(p)
		if err != nil {
			return nil, err
		}
		if !slices.Contains(out, c) {
			out = append(out, c)
		}
	}
	return out, nil
}

// Matches is whether the file at name (slashed, from where the task runs)
// is one pattern p names: p itself, below p where p is a path, or matched
// by p where it's a pattern.
func Matches(p, name string) bool {
	if !hasMeta(p) {
		return name == p || strings.HasPrefix(name, p+"/")
	}
	return matchParts(strings.Split(p, "/"), strings.Split(name, "/"))
}

func matchParts(ps, ns []string) bool {
	for len(ps) > 0 {
		if ps[0] == "**" {
			rest := ps[1:]
			if len(rest) == 0 {
				return len(ns) > 0 // everything inside, as git has a trailing /**
			}
			for i := 0; i <= len(ns); i++ {
				if matchParts(rest, ns[i:]) {
					return true
				}
			}
			return false
		}
		if len(ns) == 0 {
			return false
		}
		if ok, _ := path.Match(ps[0], ns[0]); !ok {
			return false
		}
		ps, ns = ps[1:], ns[1:]
	}
	return len(ns) == 0
}

// MatchesAny is whether any of ps names the file.
func MatchesAny(ps []string, name string) bool {
	for _, p := range ps {
		if Matches(p, name) {
			return true
		}
	}
	return false
}

// base is the folder a pattern's files are all below: its leading names
// before the first that's a pattern, or the path itself.
func base(p string) string {
	if !hasMeta(p) {
		return p
	}
	parts := strings.Split(p, "/")
	var keep []string
	for _, s := range parts {
		if hasMeta(s) {
			break
		}
		keep = append(keep, s)
	}
	return strings.Join(keep, "/")
}

// Files is every file below dir that one of ps names, slashed from dir,
// in order. A folder is walked only where a pattern could name what's in
// it, and what's skipped only where it's named.
func Files(dir string, ps []string) ([]string, error) {
	seen := map[string]bool{}
	var out []string
	for _, p := range ps {
		root := filepath.Join(dir, filepath.FromSlash(base(p)))
		err := filepath.WalkDir(root, func(at string, d fs.DirEntry, err error) error {
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					return nil
				}
				return err
			}
			if d.IsDir() {
				if at != root && slices.Contains(skipped, d.Name()) && !slices.Contains(strings.Split(p, "/"), d.Name()) {
					return filepath.SkipDir // what's installed, and git's and ws's own, unless it's named
				}
				return nil
			}
			rel, err := filepath.Rel(dir, at)
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			if Matches(p, rel) && !seen[rel] {
				seen[rel] = true
				out = append(out, rel)
			}
			return nil
		})
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
	}
	slices.Sort(out)
	return out, nil
}
