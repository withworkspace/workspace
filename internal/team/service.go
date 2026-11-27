package team

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/withworkspace/workspace/internal/github"
	"github.com/withworkspace/workspace/internal/ref"
	"github.com/withworkspace/workspace/internal/store"
)

// What ws keeps of the team is in the workspace's records (records.go):
// every event as last worked out, the pull requests as GitHub last said
// them (so only what's changed is asked for again), and when it last
// looked, and how far back.

// Dir is where ws kept what it knew of the team, as files, before the
// records.
func Dir(root string) string { return filepath.Join(root, ".ws", "team") }

// Repo is a repo to read: its clone, and its default branch.
type Repo struct {
	Name   string
	Bare   string
	Branch string
	// GitHub is where it is on GitHub, as asked for its pull requests.
	GitHub ref.Remote
}

// Snapshot is the events as last worked out.
type Snapshot struct {
	// At is when git and GitHub were looked at, and Since how far back.
	At     time.Time `json:"at"`
	Since  time.Time `json:"since"`
	Events []Event   `json:"-"`
	// GitHub: pull requests were asked for, and Asked is when.
	GitHub   bool      `json:"github"`
	Asked    time.Time `json:"asked,omitzero"`
	Problems []string  `json:"problems,omitempty"`
	// Form is which form of what's kept this is (keptForm): GitHub is
	// asked for everything again when what's kept of a pull request has
	// grown since.
	Form int `json:"form,omitempty"`
}

// keptForm counts the times what's kept of a pull request has grown: 2
// added its description and people's names.
const keptForm = 2

// Service works out the team's events: it fetches each repo's default
// branch and reads its commits, asks GitHub for the pull requests that
// have changed, and keeps what it found between looks.
type Service struct {
	Root string
	// GitHub is asked for pull requests. nil for none: the events are then
	// what git knows.
	GitHub github.Client
	// Repos are the repos read: those cloned here, outside hidden groups.
	Repos func() []Repo
	// People is [team.people]: who someone is, said outright.
	People func() map[string]PersonConfig
	// Web is where a repo is on the web (https://github.com/acme/api), by
	// its name here, for a link to what has no page here: a commit. nil,
	// or "" for a repo, for nowhere.
	Web func(repo string) string
	// Window is how far back a look goes, and so what's kept (default 14
	// days): a look asked to go further keeps that until the day's out.
	Window time.Duration
	// Now is the clock (tests set it), and Fresh how long a look answers
	// before git and GitHub are looked at again (default two minutes).
	Now   func() time.Time
	Fresh time.Duration

	// Writer has an agent write a gist from a change, and answers its
	// reply (gist.go). nil for no agent: NoWriter then says why, if
	// there's a reason to give.
	Writer   func(Ask) (string, error)
	NoWriter string
	// Watch is [team.watch]: the workspace's own paths whose changes are
	// changes to how things are done (watch.go), beside the built-in ones.
	Watch func() []Watch
	// Summariser has an agent write a period's account from its changes,
	// and answers its reply (summary.go). nil for no agent.
	Summariser func(SummaryAsk) (string, error)
	// Capsules are your capsules in a repo, for what's near you (near.go).
	// nil for none.
	Capsules func(repo string) []Capsule

	// Moved, if set, is told when a look behind a read finds something
	// other than the read was given (near.go): whoever tells pages looks
	// now, not at its next poll. It mustn't wait.
	Moved func()

	// near is what your capsules have changed, as last read (near.go), by
	// repo/name; nearRepos the repos a page has asked about; nearAt when
	// they were last looked at, and nearLooking whether they're being.
	// nearRead asks git what a capsule has changed (tests set it).
	nearMu      sync.Mutex
	near        map[string]*nearCapsule
	nearRepos   map[string]bool
	nearAt      time.Time
	nearLooking bool
	nearRead    func(Capsule) ([]string, error)

	mu sync.Mutex
	// snap is the last look, and prs the pull requests it kept, as this
	// process has them; snapAt is when it was, as the records keep it: a
	// newer look by another process is read in their place.
	snap    *Snapshot
	prs     map[string]PR
	snapAt  string
	loading *lookCall
	// writing are the gists being written now, and summarising the
	// periods' accounts; version counts each start and end of either.
	writing     map[string]bool
	summarising map[string]bool
	version     int64
	sem         chan struct{}
	semOnce     sync.Once
}

type lookCall struct {
	done chan struct{}
	snap *Snapshot
	err  error
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s *Service) window() time.Duration {
	if s.Window > 0 {
		return s.Window
	}
	return 14 * 24 * time.Hour
}

// Start is where a look that isn't asked to go further back starts: the
// window ago, at the start of that day.
func (s *Service) Start() time.Time {
	t := s.now().Add(-s.window())
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// Peek is the events as last worked out, without looking: nil if they
// never have been.
func (s *Service) Peek() *Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.kept()
}

// kept is the last look in hand, or the records' when they hold a newer
// one (ws team looked, or the daemon did). Called with the lock held.
func (s *Service) kept() *Snapshot {
	if snap, prs, at, err := s.keptLook(s.snapAt); err == nil && snap != nil {
		s.snap, s.prs, s.snapAt = snap, prs, at
	}
	return s.snap
}

// forcedFresh is how long a look answers even when asked to look again.
const forcedFresh = 5 * time.Second

// Look is the team's events, looking at git and GitHub if that hasn't
// been done lately (or, with force, in the last few seconds), from since
// on (the zero time: from the window ago). When it can't look, the events
// as last known come back with the error.
func (s *Service) Look(force bool, since time.Time) (*Snapshot, error) {
	fresh := s.Fresh
	if fresh == 0 {
		fresh = 2 * time.Minute
	}
	if force {
		fresh = forcedFresh
	}
	// A look always goes back the window, however short the period asked
	// for: what's kept is then enough for any period within it.
	if start := s.Start(); since.IsZero() || since.After(start) {
		since = start
	}
	s.mu.Lock()
	// What a ws that kept less wrote is never fresh: it's looked at again.
	if snap := s.kept(); snap != nil && snap.Form >= keptForm && s.now().Sub(snap.At) < fresh && !since.Before(snap.Since) {
		s.mu.Unlock()
		return snap, nil
	}
	if c := s.loading; c != nil {
		s.mu.Unlock()
		<-c.done
		return c.snap, c.err
	}
	c := &lookCall{done: make(chan struct{})}
	s.loading = c
	prev := s.kept()
	s.mu.Unlock()

	snap, err := s.look(prev, since)
	s.mu.Lock()
	s.loading = nil
	if err == nil {
		s.snap = snap
	}
	c.snap, c.err = s.snap, err
	s.mu.Unlock()
	close(c.done)
	return c.snap, c.err
}

// lookAtOnce is how many repos are read at a time.
const lookAtOnce = 6

// askAgain is how far before the last ask GitHub is asked from, so what
// changed while it was answering isn't missed.
const askAgain = 10 * time.Minute

func (s *Service) look(prev *Snapshot, since time.Time) (*Snapshot, error) {
	now := s.now()
	snap := &Snapshot{At: now, Since: since, GitHub: s.GitHub != nil, Form: keptForm}
	// How far back is kept is the window, or as far as a look was asked
	// to go today: so a page open on a longer period isn't cut back by
	// the looks that keep it fresh, and what's kept doesn't grow for good.
	if prev != nil && prev.Since.Before(since) && sameDay(prev.At, now) {
		snap.Since = prev.Since
	}
	// GitHub is asked only for what's changed since it last was, unless
	// this goes further back, or what's kept is of an older form.
	further := prev == nil || since.Before(prev.Since) || prev.Form < keptForm
	prs := s.readPRs()
	askFrom := snap.Since
	if !further && !prev.Asked.IsZero() {
		askFrom = prev.Asked.Add(-askAgain)
	}

	repos := s.Repos()
	commits := make([][]Commit, len(repos))
	changed := make([][]github.PRActivity, len(repos))
	problems := make([][]string, len(repos))
	asked := make([]bool, len(repos))
	var wg sync.WaitGroup
	sem := make(chan struct{}, lookAtOnce)
	for i, repo := range repos {
		wg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			// A fetch that fails (offline) leaves the clone as it was,
			// which is still worth reading.
			if err := Fetch(repo.Bare, repo.Branch); err != nil {
				problems[i] = append(problems[i], repo.Name+": "+err.Error())
			}
			list, err := Commits(repo.Bare, repo.Branch, snap.Since)
			if err != nil {
				problems[i] = append(problems[i], repo.Name+": "+err.Error())
			}
			commits[i] = list
			if s.GitHub == nil {
				return
			}
			acts, err := s.GitHub.ChangedPRs(repo.GitHub.Owner, repo.GitHub.Name, askFrom)
			if err != nil {
				problems[i] = append(problems[i], repo.Name+": "+err.Error())
				return
			}
			changed[i], asked[i] = acts, true
		})
	}
	wg.Wait()

	byRepo := map[string][]Commit{}
	branches := map[string]string{}
	allAsked := s.GitHub != nil
	for i, repo := range repos {
		byRepo[repo.Name] = commits[i]
		branches[repo.Name] = repo.Branch
		for _, a := range changed[i] {
			pr := PR{Repo: repo.Name, PRActivity: a}
			prs[pr.Key()] = pr
		}
		snap.Problems = append(snap.Problems, problems[i]...)
		allAsked = allAsked && asked[i]
	}
	// A pull request that hasn't moved since how far back is kept goes,
	// and its events with it. Asked about again, it's said in full.
	for key, pr := range prs {
		if last := lastMoved(pr); !last.IsZero() && last.Before(snap.Since) {
			delete(prs, key)
		}
	}
	sort.Strings(snap.Problems)
	// GitHub is asked from the last ask on only once every repo has
	// answered: one that didn't is asked from further back next time.
	switch {
	case allAsked:
		snap.Asked = now
	case prev != nil && !further:
		snap.Asked = prev.Asked
	}

	list := make([]PR, 0, len(prs))
	for _, pr := range prs {
		list = append(list, pr)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Key() < list[j].Key() })
	snap.Events = Build(list, byRepo, branches)
	var at string
	if err := s.write(func(tx *store.Tx) error {
		var err error
		at, err = keepLook(tx, snap, list)
		return err
	}); err != nil {
		return nil, fmt.Errorf("could not save team activity: %w", err)
	}
	s.mu.Lock()
	s.snap, s.prs, s.snapAt = snap, prs, at
	s.mu.Unlock()
	return snap, nil
}

// sameDay reports whether two times are on the same day, here.
func sameDay(a, b time.Time) bool {
	ay, am, ad := a.In(b.Location()).Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

// lastMoved is when a pull request last moved, as far as GitHub said: the
// zero time when it said nothing that can be read.
func lastMoved(pr PR) time.Time {
	var last time.Time
	for _, s := range []string{pr.UpdatedAt, pr.CreatedAt, pr.ReadyAt, pr.MergedAt, pr.ClosedAt} {
		if t := parseTime(s); t.After(last) {
			last = t
		}
	}
	for _, r := range pr.Reviews {
		if t := parseTime(r.SubmittedAt); t.After(last) {
			last = t
		}
	}
	return last
}

// readPRs is the pull requests as GitHub last said them, as the last look
// kept them: a copy.
func (s *Service) readPRs() map[string]PR {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.kept()
	out := make(map[string]PR, len(s.prs))
	maps.Copy(out, s.prs)
	return out
}

// Timeline is a snapshot's timeline from a time on (the zero time: from
// as far back as it goes).
func (s *Service) Timeline(snap *Snapshot, since time.Time) *Timeline {
	if since.Before(snap.Since) {
		since = snap.Since
	}
	var cfg map[string]PersonConfig
	if s.People != nil {
		cfg = s.People()
	}
	t := View(snap.Events, cfg, since, s.now().Location())
	t.At, t.GitHub, t.Problems = snap.At, snap.GitHub, snap.Problems
	if seen := s.Seen(); !seen.IsZero() {
		t.Seen = &seen
	}
	s.touches(t)
	s.narrate(t, snap, cfg)
	if s.Web != nil {
		for _, d := range t.Days {
			for i := range d.Items {
				it := &d.Items[i]
				last := it.Events[len(it.Events)-1]
				for n := range it.PRs {
					if pr := &it.PRs[n]; pr.URL == "" {
						if web := s.Web(pr.Repo); web != "" {
							pr.URL = fmt.Sprintf("%s/pull/%d", web, pr.Number)
						}
					}
				}
				web := s.Web(it.Repo)
				switch {
				case web == "" || it.URL != "" || len(it.PRs) > 0:
				case it.PR != 0:
					it.URL = fmt.Sprintf("%s/pull/%d", web, it.PR)
				case len(it.Events) == 1 || last.Branch == "":
					it.URL = fmt.Sprintf("%s/commit/%s", web, last.Commit)
				default:
					it.URL = fmt.Sprintf("%s/commits/%s", web, last.Branch)
				}
			}
		}
	}
	return t
}

// ParseWindow reads [team] window: a number of days or weeks (14d, 2w).
// "" is the default.
func ParseWindow(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	n := len(s)
	if n > 1 {
		if count, err := strconv.Atoi(s[:n-1]); err == nil && count > 0 {
			switch s[n-1] {
			case 'd':
				return time.Duration(count) * 24 * time.Hour, nil
			case 'w':
				return time.Duration(count) * 7 * 24 * time.Hour, nil
			}
		}
	}
	return 0, fmt.Errorf("[team] window = %q: say a number of days or weeks, as 14d or 2w", s)
}

// Litter is Dir, if it's there: Team's feed is in the records now, so what's
// there is from before them, which nothing reads (ws doctor).
func Litter(root string) []string {
	if _, err := os.Stat(Dir(root)); err == nil {
		return []string{Dir(root)}
	}
	return nil
}
