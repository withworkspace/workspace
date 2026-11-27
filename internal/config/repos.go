package config

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// repoKeyOrder is the order a repo's keys are written in: as MANUAL.md
// lists them, what it's called first.
var repoKeyOrder = []string{"github", "display_name", "aliases", "color", "group", "after_create", "landing", "verify", "documents"}

// repoLines are a repo's keys as lines of TOML, in repoKeyOrder: what it
// says, and none of what it leaves unset.
func repoLines(name string, rc RepoConfig) ([]string, error) {
	rc.Yours = false
	ls, err := leaves(LocalConfig{Repos: map[string]RepoConfig{name: rc}})
	if err != nil {
		return nil, err
	}
	type kv struct {
		key  string
		line string
	}
	var out []kv
	for _, l := range ls {
		val, err := encodeValue(l.value)
		if err != nil {
			return nil, err
		}
		key := l.path[len(l.path)-1]
		out = append(out, kv{key, quoteKey(key) + " = " + val})
	}
	rank := func(k string) int {
		if i := slices.Index(repoKeyOrder, k); i >= 0 {
			return i
		}
		return len(repoKeyOrder)
	}
	slices.SortFunc(out, func(a, b kv) int {
		if d := rank(a.key) - rank(b.key); d != 0 {
			return d
		}
		return strings.Compare(a.key, b.key)
	})
	lines := make([]string, len(out))
	for i, x := range out {
		lines[i] = x.line
	}
	return lines, nil
}

// AddTeamRepo writes a repo into ws.toml, with what rc says of it: after
// the repos it has, keeping the rest of the file as it was. ws.toml is
// the team's, in the workspace's config repo: the caller commits it.
func AddTeamRepo(root, name string, rc RepoConfig) error {
	path := filepath.Join(root, FileName)
	team, err := Parse(path)
	if err != nil {
		return err
	}
	if _, ok := team.Repos[name]; ok {
		return fmt.Errorf("%s already has [repos.%s]", FileName, name)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines, err := repoLines(name, rc)
	if err != nil {
		return err
	}
	d := parseDoc(string(data))
	secs, order, err := d.sections()
	if err != nil {
		return err
	}
	// After the last repo's table, before the blank lines that part it
	// from what follows; at the end where there are none.
	at := len(d.lines)
	for _, k := range order {
		if strings.HasPrefix(k, "repos\x00") {
			at = secs[k].end
		}
	}
	for at > 0 && at < len(d.lines) && strings.TrimSpace(d.lines[at-1]) == "" {
		at--
	}
	block := append([]string{"", "[" + joinKey([]string{"repos", name}) + "]"}, lines...)
	if at == 0 {
		block = block[1:]
	}
	d.lines = slices.Insert(d.lines, at, block...)
	out := d.String()
	if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
		return err
	}
	// Read back: what was written is the repo, and the file still reads.
	again, err := Parse(path)
	if err == nil {
		if _, ok := again.Repos[name]; !ok {
			err = fmt.Errorf("could not verify [repos.%s] after writing", name)
		}
	}
	if err != nil {
		_ = os.WriteFile(path, data, 0o644) // put back as it was: the error says what went wrong
		return fmt.Errorf("could not write %s: %w", FileName, err)
	}
	return nil
}

// DropLocalRepo takes a repo out of ws.local.toml whole: its keys, and its
// table, which a repo that's yours has even with no keys (it's the table
// that makes it one).
func DropLocalRepo(root, name string) error {
	return Turn(root, func() error {
		if err := updateLocal(root, func(l *LocalConfig) { delete(l.Repos, name) }, true); err != nil {
			return err
		}
		path := filepath.Join(root, LocalFileName)
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		d := parseDoc(string(data))
		secs, _, err := d.sections()
		if err != nil {
			return err
		}
		sec, ok := secs[pathKey([]string{"repos", name})]
		if !ok {
			return nil
		}
		from, to := sec.header, sec.end
		if from > 0 && strings.TrimSpace(d.lines[from-1]) == "" {
			from--
		}
		d.lines = slices.Delete(d.lines, from, to)
		return os.WriteFile(path, []byte(d.String()), 0o644)
	})
}

// AddLocalRepo writes a repo into ws.local.toml: yours alone, cloned here
// and not in a teammate's workspace.
func AddLocalRepo(root, name string, rc RepoConfig) error {
	return Turn(root, func() error {
		local, err := ReadLocal(root)
		if err != nil {
			return err
		}
		if _, ok := local.Repos[name]; ok {
			return fmt.Errorf("%s already has [repos.%s]", LocalFileName, name)
		}
		path := filepath.Join(root, LocalFileName)
		data, err := os.ReadFile(path)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		lines, err := repoLines(name, rc)
		if err != nil {
			return err
		}
		d := parseDoc(string(data))
		block := []string{"[" + joinKey([]string{"repos", name}) + "]"}
		if n := len(d.lines); n > 0 && strings.TrimSpace(d.lines[n-1]) != "" {
			block = append([]string{""}, block...)
		}
		d.lines = append(d.lines, append(block, lines...)...)
		if err := os.WriteFile(path, []byte(d.String()), 0o644); err != nil {
			return err
		}
		if again, err := ReadLocal(root); err != nil || !hasRepo(again.Repos, name) {
			_ = os.WriteFile(path, data, 0o644) // put back as it was: the error says what went wrong
			return fmt.Errorf("could not write %s: [repos.%s] did not read back (%v)", LocalFileName, name, err)
		}
		return nil
	})
}

func hasRepo(repos map[string]RepoConfig, name string) bool {
	_, ok := repos[name]
	return ok
}
