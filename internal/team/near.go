package team

import (
	"context"
	"maps"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/withworkspace/workspace/internal/git"
)

// Capsule is one of your capsules, for what's near you: its repo and
// name, where it is, and the ref its changes are from.
type Capsule struct {
	Repo, Name string
	Dir, Base  string
	// Stamp names what git would say its changes from, as it is now (its
	// HEAD, what's staged, the ref it left), read without asking git:
	// while it hasn't moved, what the capsule has changed is taken as
	// read. "" where that can't be said.
	Stamp string
}

// What your capsules have changed is kept, and a read answers from it:
// git is asked behind the answer, never in it (two processes a capsule
// was five seconds of a read, in a workspace of sixty). A page that was
// answered before a look found something else reads again (the version
// moves, and Moved is told).
const (
	// nearEvery is how often the capsules are listed, and their stamps
	// looked at, while a page asks: no process is started for that.
	nearEvery = 2 * time.Second
	// nearAge is how long a capsule that hasn't moved is taken as read. A
	// file edited and not staged moves nothing git keeps, so it's only by
	// asking again that it's found.
	nearAge = 5 * time.Minute
	// nearFresh is that, for a capsule with no stamp to go by.
	nearFresh = 30 * time.Second
	// nearAtOnce is how many capsules are read at a time.
	nearAtOnce = 4
)

// nearCapsule is what one capsule has changed, as last read.
type nearCapsule struct {
	repo, name string
	stamp      string
	read       time.Time
	files      []string
}

// NearYou marks the rows of t whose landed changes touch files a capsule
// of yours has changed too: what you'll meet when you next rebase. It's
// from what's kept of your capsules, and doesn't wait: they're read again
// behind it when that's due.
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
// left its repo's default branch, committed or not, as last read: by
// repo, then path, the capsules that changed it. Only the repos asked for
// are answered, and from now on read. It starts a look behind it when one
// is due, and doesn't wait for it.
func (s *Service) capsuleChanges(in map[string]bool) map[string]map[string][]string {
	s.nearMu.Lock()
	defer s.nearMu.Unlock()
	if s.nearRepos == nil {
		s.nearRepos = map[string]bool{}
	}
	// A repo that's never been asked about is read now, not at the next
	// look.
	first := false
	for repo := range in {
		if !s.nearRepos[repo] {
			s.nearRepos[repo], first = true, true
		}
	}
	if !s.nearLooking && len(s.nearRepos) > 0 && (first || s.nearAt.IsZero() || s.now().Sub(s.nearAt) >= nearEvery) {
		s.nearLooking, s.nearAt = true, s.now()
		go s.lookNear(slices.Sorted(maps.Keys(s.nearRepos)))
	}
	out := map[string]map[string][]string{}
	for _, key := range slices.Sorted(maps.Keys(s.near)) {
		c := s.near[key]
		if !in[c.repo] {
			continue
		}
		for _, path := range c.files {
			if out[c.repo] == nil {
				out[c.repo] = map[string][]string{}
			}
			out[c.repo][path] = append(out[c.repo][path], c.name)
		}
	}
	return out
}

// lookNear brings what's kept of your capsules in repos up to date: one
// that's moved, or hasn't been read for a while, is read again, and one
// that's gone is forgotten. If what's kept is other than it was, whoever
// was answered from it is told.
func (s *Service) lookNear(repos []string) {
	now := s.now()
	var due []Capsule
	here := map[string]bool{}
	for _, repo := range repos {
		for _, c := range s.Capsules(repo) {
			if c.Base == "" {
				continue
			}
			key := c.Repo + "/" + c.Name
			here[key] = true
			age := nearAge
			if c.Stamp == "" {
				age = nearFresh
			}
			s.nearMu.Lock()
			was := s.near[key]
			s.nearMu.Unlock()
			if was == nil || was.stamp != c.Stamp || now.Sub(was.read) >= age {
				due = append(due, c)
			}
		}
	}

	read := s.nearRead
	if read == nil {
		read = capsuleChanged
	}
	files := make([][]string, len(due))
	failed := make([]bool, len(due))
	var wg sync.WaitGroup
	slots := make(chan struct{}, nearAtOnce)
	for i, c := range due {
		wg.Go(func() {
			slots <- struct{}{}
			defer func() { <-slots }()
			var err error
			files[i], err = read(c)
			failed[i] = err != nil
		})
	}
	wg.Wait()

	moved := false
	s.nearMu.Lock()
	if s.near == nil {
		s.near = map[string]*nearCapsule{}
	}
	for key, c := range s.near {
		if !here[key] {
			delete(s.near, key)
			moved = moved || len(c.files) > 0
		}
	}
	for i, c := range due {
		key := c.Repo + "/" + c.Name
		was := s.near[key]
		kept := &nearCapsule{repo: c.Repo, name: c.Name, stamp: c.Stamp, read: now, files: files[i]}
		// One git couldn't read (it has no such base, or the machine's
		// busy) stays as it was, and is asked again when it's next due.
		if failed[i] && was != nil {
			kept.files = was.files
		}
		if was == nil || !slices.Equal(was.files, kept.files) {
			moved = moved || was != nil || len(kept.files) > 0
		}
		s.near[key] = kept
	}
	s.nearLooking = false
	s.nearMu.Unlock()
	if !moved {
		return
	}
	s.mu.Lock()
	s.version++
	s.mu.Unlock()
	if s.Moved != nil {
		s.Moved()
	}
}

// capsuleChanged asks git for the files a capsule has changed since it
// left its base, committed or not, sorted.
func capsuleChanged(c Capsule) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	// One process where git can find where it left by itself (2.30 on).
	names, err := git.Read(ctx, c.Dir, "diff", "--name-only", "--merge-base", c.Base, "--")
	if err != nil {
		base, err := git.ReadLine(ctx, c.Dir, "merge-base", c.Base, "HEAD")
		if err != nil {
			return nil, err
		}
		if names, err = git.Read(ctx, c.Dir, "diff", "--name-only", base, "--"); err != nil {
			return nil, err
		}
	}
	var out []string
	for path := range strings.SplitSeq(strings.TrimSpace(string(names)), "\n") {
		if path != "" {
			out = append(out, path)
		}
	}
	slices.Sort(out)
	return out, nil
}
