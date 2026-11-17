package team

import (
	"context"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/git"
)

// Capsule is one of your capsules, for what's near you: its repo and
// name, where it is, and the ref its changes are from.
type Capsule struct {
	Repo, Name string
	Dir, Base  string
}

// nearFresh is how long what your capsules have changed is kept before
// it's read again.
const nearFresh = 30 * time.Second

// NearYou marks the rows of t whose landed changes touch files a capsule
// of yours has changed too: what you'll meet when you next rebase.
func (s *Service) NearYou(t *Timeline) {
	if s.Capsules == nil {
		return
	}
	in := map[string]bool{}
	for _, d := range t.Days {
		for _, it := range d.Items {
			for _, e := range it.Events {
				if len(e.Paths) > 0 {
					in[e.Repo] = true
				}
			}
		}
	}
	changed := s.capsuleChanges(in)
	if len(changed) == 0 {
		return
	}
	for _, d := range t.Days {
		for i := range d.Items {
			it := &d.Items[i]
			files := map[string][]string{}
			var order []string
			for _, e := range it.Events {
				for _, path := range e.Paths {
					for _, capsule := range changed[e.Repo][path] {
						name := e.Repo + "/" + capsule
						if files[name] == nil {
							order = append(order, name)
						}
						if !slices.Contains(files[name], path) {
							files[name] = append(files[name], path)
						}
					}
				}
			}
			for _, name := range order {
				it.Near = append(it.Near, Near{Capsule: name, Files: files[name]})
			}
		}
	}
}

// capsuleChanges is the files each of your capsules has changed since it
// left its repo's default branch, committed or not: by repo, then path,
// the capsules that changed it. Only the repos asked for are read, and
// what's read is kept for nearFresh.
func (s *Service) capsuleChanges(in map[string]bool) map[string]map[string][]string {
	s.nearMu.Lock()
	defer s.nearMu.Unlock()
	if s.near != nil && s.now().Sub(s.nearAt) < nearFresh {
		return s.near
	}
	ctx := context.Background()
	out := map[string]map[string][]string{}
	for _, repo := range slices.Sorted(maps.Keys(in)) {
		for _, c := range s.Capsules(repo) {
			if c.Base == "" {
				continue
			}
			base, err := git.ReadLine(ctx, c.Dir, "merge-base", c.Base, "HEAD")
			if err != nil {
				continue
			}
			names, err := git.Read(ctx, c.Dir, "diff", "--name-only", base, "--")
			if err != nil {
				continue
			}
			for _, path := range strings.Split(strings.TrimSpace(string(names)), "\n") {
				if path == "" {
					continue
				}
				if out[repo] == nil {
					out[repo] = map[string][]string{}
				}
				out[repo][path] = append(out[repo][path], c.Name)
			}
		}
	}
	s.near, s.nearAt = out, s.now()
	return out
}
