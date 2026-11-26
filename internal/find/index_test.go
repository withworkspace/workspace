package find

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/git"
	"github.com/withworkspace/workspace/internal/testgit"
)

func TestMain(m *testing.M) {
	cleanup := testgit.NoSigning()
	code := m.Run()
	cleanup()
	os.Exit(code)
}

func gitT(t testing.TB, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func write(t testing.TB, dir, path, text string) {
	t.Helper()
	full := filepath.Join(dir, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// space is a workspace as the index sees one: a repo, demo, with its
// ground and a capsule, feature, branched from it.
type space struct {
	t       *testing.T
	root    string
	ground  string
	capsule string
	ix      *Index
	mu      sync.Mutex
	now     time.Time
	wts     []Worktree
}

func (s *space) clock() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.now
}

// later moves the time on, past how long anything is taken as read.
func (s *space) later() {
	s.mu.Lock()
	s.now = s.now.Add(10 * time.Second)
	s.mu.Unlock()
}

// commit commits everything in dir. In the ground, origin's default
// branch follows, as a fetch would leave it.
func (s *space) commit(dir, subject string) {
	gitT(s.t, dir, "add", "-A")
	gitT(s.t, dir, "commit", "-q", "-m", subject)
	if dir == s.ground {
		gitT(s.t, dir, "update-ref", "refs/remotes/origin/main", "HEAD")
	}
}

func newSpace(t *testing.T) *space {
	t.Helper()
	root := t.TempDir()
	// Where macOS keeps temp folders is a link: name them as git will.
	if r, err := filepath.EvalSymlinks(root); err == nil {
		root = r
	}
	s := &space{t: t, root: root, now: time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)}
	s.ground = filepath.Join(root, "repos", "demo", Ground)
	s.capsule = filepath.Join(root, "repos", "demo", "feature")
	os.MkdirAll(s.ground, 0o755)
	gitT(t, s.ground, "init", "-q", "--initial-branch=main")
	gitT(t, s.ground, "config", "user.name", "Test")
	gitT(t, s.ground, "config", "user.email", "test@example.com")
	write(t, s.ground, "main.go", "package main\n")
	write(t, s.ground, "main_test.go", "package main\n")
	write(t, s.ground, "internal/token/refresh.go", "package token\n")
	write(t, s.ground, "internal/token/cache.go", "package token\n")
	write(t, s.ground, "old.go", "package main\n")
	write(t, s.ground, "moved.go", "package main\n\n// moved is long enough to be followed when it's renamed.\nfunc moved() {}\n")
	write(t, s.ground, "docs/guide.md", "# Guide\n")
	write(t, s.ground, ".gitignore", "*.log\n")
	s.commit(s.ground, "base")
	gitT(t, s.ground, "worktree", "add", "-q", "-b", "feature", s.capsule)
	s.wts = []Worktree{
		{Repo: "demo", Name: Ground, Dir: s.ground},
		{Repo: "demo", Name: "feature", Dir: s.capsule, BaseRef: "origin/main"},
	}
	s.ix = &Index{
		RepoAt: func(owner, name string) (string, bool) { return name, strings.EqualFold(owner, "acme") },
		Now:    s.clock,
		Worktrees: func() ([]Worktree, error) {
			s.mu.Lock()
			defer s.mu.Unlock()
			return append([]Worktree(nil), s.wts...), nil
		},
	}
	return s
}

// find is what a search answers, by kind.
func (s *space) find(q string, kinds ...string) map[string]Group {
	s.t.Helper()
	out := map[string]Group{}
	if err := s.ix.Search(context.Background(), Request{Q: q, Kinds: kinds, Limit: 20}, func(g Group) { out[g.Kind] = g }); err != nil {
		s.t.Fatalf("search %q: %v", q, err)
	}
	return out
}

func (s *space) files(q string) []Result { return s.find(q, KindFiles)[KindFiles].Results }

func refs(rs []Result) []string {
	out := []string{}
	for _, r := range rs {
		out = append(out, r.Ref)
	}
	return out
}

func byRef(rs []Result, ref string) *Result {
	for i := range rs {
		if rs[i].Ref == ref {
			return &rs[i]
		}
	}
	return nil
}

func TestFilesOfTheDefaultBranch(t *testing.T) {
	s := newSpace(t)
	if _, err := s.ix.Warm(context.Background()); err != nil {
		t.Fatal(err)
	}
	got := s.files("main")
	if len(got) < 2 || got[0].Ref != "demo/main.go" || got[1].Ref != "demo/main_test.go" {
		t.Fatalf("main: %v", refs(got))
	}
	r := got[0]
	if r.Kind != KindFile || r.Repo != "demo" || r.Worktree != Ground || r.Path != "main.go" || r.Title != "demo/main.go" {
		t.Errorf("main.go: %+v", r)
	}
	if want := []int{5, 6, 7, 8}; !reflect.DeepEqual(r.At, want) {
		t.Errorf("main.go marked at %v, want %v", r.At, want)
	}
	// A repo's name narrows, and a few letters of a path find it.
	if got := s.files("demo/tokref"); len(got) == 0 || got[0].Ref != "demo/internal/token/refresh.go" {
		t.Errorf("demo/tokref: %v", refs(got))
	}
	// Lines asked for go with every result.
	if got := s.files("refresh:12-20"); len(got) == 0 || got[0].Ref != "demo/internal/token/refresh.go:12-20" || got[0].Line != 12 || got[0].EndLine != 20 || got[0].Title != "demo/internal/token/refresh.go" {
		t.Errorf("refresh:12-20: %+v", got)
	}
	if got := s.files("zzzz"); len(got) != 0 {
		t.Errorf("zzzz: %v", refs(got))
	}
	// What git ignores isn't a file to find, and nor is what isn't committed
	// to the default branch.
	write(t, s.ground, "debug.log", "x")
	s.later()
	if got := s.files("debug"); len(got) != 0 {
		t.Errorf("debug.log found: %v", refs(got))
	}
}

func TestMoreThanFit(t *testing.T) {
	s := newSpace(t)
	var got Group
	s.ix.Search(context.Background(), Request{Q: "o", Kinds: []string{KindFiles}, Limit: 2}, func(g Group) { got = g })
	if len(got.Results) != 2 || got.More < 3 {
		t.Errorf("2 of many: %d and %d more", len(got.Results), got.More)
	}
}

func TestACapsuleHoldsOnlyWhatItChanged(t *testing.T) {
	s := newSpace(t)
	write(t, s.capsule, "main.go", "package main\n\nfunc main() {}\n") // changed, committed
	write(t, s.capsule, "internal/token/store.go", "package token\n")  // new, committed
	os.Remove(filepath.Join(s.capsule, "old.go"))                      // removed
	gitT(t, s.capsule, "mv", "moved.go", "renamed.go")                 // renamed
	s.commit(s.capsule, "work")
	write(t, s.capsule, "scratch.txt", "notes\n")                 // new, untracked
	write(t, s.capsule, "internal/token/cache.go", "package x\n") // changed, not committed
	write(t, s.capsule, "trace.log", "ignored\n")

	// A file of the default branch is one line, saying who's changed it.
	got := s.files("main.go")
	if n := len(got); n == 0 || got[0].Ref != "demo/main.go" {
		t.Fatalf("main.go: %v", refs(got))
	}
	if want := []Changed{{Capsule: "feature", Status: Modified}}; !reflect.DeepEqual(got[0].In, want) {
		t.Errorf("main.go changed in %v, want %v", got[0].In, want)
	}
	if r := byRef(got, "demo/feature:main.go"); r != nil {
		t.Errorf("main.go listed for the capsule too: %+v", r)
	}
	if r := byRef(s.files("cache.go"), "demo/internal/token/cache.go"); r == nil || len(r.In) != 1 || r.In[0].Status != Modified {
		t.Errorf("cache.go, changed and not committed: %+v", r)
	}
	// One it removed is still the default branch's.
	if r := byRef(s.files("old.go"), "demo/old.go"); r == nil || len(r.In) != 1 || r.In[0].Status != Removed {
		t.Errorf("old.go: %+v", r)
	}
	// What only the capsule has is its own line, named as the capsule's.
	for _, c := range []struct{ q, ref string }{
		{"store.go", "demo/feature:internal/token/store.go"},
		{"scratch", "demo/feature:scratch.txt"},
		{"renamed", "demo/feature:renamed.go"},
	} {
		r := byRef(s.files(c.q), c.ref)
		if r == nil || r.Worktree != "feature" || r.Status != Added || r.Repo != "demo" {
			t.Errorf("%s: %+v in %v", c.q, r, refs(s.files(c.q)))
		}
	}
	if r := byRef(s.files("moved.go"), "demo/moved.go"); r == nil || len(r.In) != 1 || r.In[0].Status != Removed {
		t.Errorf("moved.go, renamed in the capsule: %+v", r)
	}
	if got := s.files("trace.log"); len(got) != 0 {
		t.Errorf("an ignored file found: %v", refs(got))
	}
	// A file nobody's touched says nothing of capsules.
	if r := byRef(s.files("guide"), "demo/docs/guide.md"); r == nil || r.In != nil {
		t.Errorf("guide.md: %+v", r)
	}
	// Its name isn't letters its files are found by: only said of them.
	if got := s.files("feature"); len(got) != 0 {
		t.Errorf("feature finds %v", refs(got))
	}
	// With a colon it's the capsule that's asked for, and what's its own.
	if got := s.files("feature:"); len(got) != 3 {
		t.Errorf("feature: finds %v", refs(got))
	}
	if got := s.files("feature:sto"); len(got) != 1 || got[0].Ref != "demo/feature:internal/token/store.go" {
		t.Errorf("feature:sto finds %v", refs(got))
	}
	// A letter matched in its path is marked where the path is in its name.
	if r := byRef(s.files("store.go"), "demo/feature:internal/token/store.go"); r == nil || !reflect.DeepEqual(r.At, []int{28, 29, 30, 31, 32, 33, 34, 35}) {
		t.Errorf("store.go marked at %+v", r)
	}
	// The capsule is found by its name, with what it's changed.
	caps := s.find("feat", KindCapsules)[KindCapsules].Results
	if len(caps) != 1 || caps[0].Kind != KindCapsule || caps[0].Ref != "demo/feature" || caps[0].Hint != "5 files changed" || caps[0].Worktree != "feature" {
		t.Errorf("feat as a capsule: %+v", caps)
	}

	// With nothing typed: what capsules have changed, as it is there.
	got = s.files("")
	want := []string{"demo/feature:internal/token/cache.go", "demo/feature:internal/token/store.go", "demo/feature:main.go", "demo/feature:renamed.go", "demo/feature:scratch.txt"}
	if !reflect.DeepEqual(refs(got), want) {
		t.Errorf("nothing typed: %v, want %v", refs(got), want)
	}
}

func TestACapsuleBehindHasntChangedWhatsMergedSince(t *testing.T) {
	s := newSpace(t)
	write(t, s.capsule, "main.go", "package main\n\nfunc main() {}\n")
	s.commit(s.capsule, "work")
	// The default branch moves on after the capsule branched.
	write(t, s.ground, "docs/guide.md", "# Guide\n\nMore.\n")
	write(t, s.ground, "later.go", "package main\n")
	s.commit(s.ground, "merged since")

	if r := byRef(s.files("guide"), "demo/docs/guide.md"); r == nil || r.In != nil {
		t.Errorf("guide.md, changed on the default branch only: %+v", r)
	}
	if r := byRef(s.files("later"), "demo/later.go"); r == nil || r.In != nil {
		t.Errorf("later.go, added on the default branch only: %+v", r)
	}
	if r := byRef(s.files("main.go"), "demo/main.go"); r == nil || len(r.In) != 1 {
		t.Errorf("main.go, changed in the capsule: %+v", r)
	}
	if got := s.files(""); !reflect.DeepEqual(refs(got), []string{"demo/feature:main.go"}) {
		t.Errorf("what the capsule changed: %v", refs(got))
	}
}

func TestReadAgainOnlyWhenItsMoved(t *testing.T) {
	s := newSpace(t)
	if got := s.files("newly"); len(got) != 0 {
		t.Fatalf("newly: %v", refs(got))
	}
	write(t, s.ground, "newly.go", "package main\n")
	s.commit(s.ground, "add")
	write(t, s.capsule, "newly_made.txt", "x\n")
	// Asked again at once, it answers from what it holds.
	if got := s.files("newly"); len(got) != 0 {
		t.Errorf("found before it was due a look: %v", refs(got))
	}
	s.later()
	got := s.files("newly")
	if byRef(got, "demo/newly.go") == nil || byRef(got, "demo/feature:newly_made.txt") == nil {
		t.Errorf("after a while: %v", refs(got))
	}
	// A ground that hasn't moved is the same paths, not read again.
	s.ix.mu.Lock()
	was := s.ix.grounds["demo"].data
	s.ix.mu.Unlock()
	s.later()
	s.files("newly")
	s.ix.mu.Lock()
	is := s.ix.grounds["demo"].data
	s.ix.mu.Unlock()
	if was != is {
		t.Error("the ground was read again though its head hadn't moved")
	}
	// A capsule that's gone is dropped.
	s.mu.Lock()
	s.wts = s.wts[:1]
	s.mu.Unlock()
	s.later()
	if got := s.files("newly"); byRef(got, "demo/feature:newly_made.txt") != nil {
		t.Errorf("a capsule that's gone still answers: %v", refs(got))
	}
}

// waitSeen waits for the index's Seen to pass n.
func (s *space) waitSeen(n int64) {
	s.t.Helper()
	for i := 0; i < 500 && s.ix.Seen() <= n; i++ {
		time.Sleep(10 * time.Millisecond)
	}
	if s.ix.Seen() <= n {
		s.t.Fatalf("Seen stayed at %d", s.ix.Seen())
	}
}

// waitIdle waits for every ground read under way to finish.
func (s *space) waitIdle() {
	s.t.Helper()
	for range 500 {
		s.ix.mu.Lock()
		busy := false
		for _, e := range s.ix.grounds {
			busy = busy || e.job != nil
		}
		s.ix.mu.Unlock()
		if !busy {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	s.t.Fatal("a ground read never finished")
}

func TestGroundsKeptNeverWait(t *testing.T) {
	s := newSpace(t)
	// Never read: nothing to answer, and a read's started behind it.
	if got := s.ix.GroundsKept(); len(got) != 0 {
		t.Fatalf("answered before anything was read: %v", got)
	}
	s.waitSeen(0)
	seen := s.ix.Seen()
	got := s.ix.GroundsKept()["demo"]
	if !got.Has("main.go") || !got.HasDir("internal/token") || got.Has("nope.go") || got.Head() == "" {
		t.Errorf("kept ground: has main.go %v, internal/token %v, nope.go %v, head %q", got.Has("main.go"), got.HasDir("internal/token"), got.Has("nope.go"), got.Head())
	}
	// Asked again before it's due a look: from memory, no git.
	before := git.Started()
	for range 5 {
		s.ix.GroundsKept()
	}
	if n := git.Started() - before; n != 0 {
		t.Errorf("answering what's kept started %d git processes", n)
	}
	// The ground moves: what's kept is answered while it's read again,
	// and Seen moves when the read finds it.
	write(t, s.ground, "newly.go", "package main\n")
	s.commit(s.ground, "add")
	s.later()
	if s.ix.GroundsKept()["demo"].Has("newly.go") {
		t.Error("answered after waiting on git")
	}
	s.waitSeen(seen)
	if !s.ix.GroundsKept()["demo"].Has("newly.go") {
		t.Error("the read behind didn't keep what it found")
	}
	// A look that finds it hasn't moved doesn't move Seen.
	seen = s.ix.Seen()
	s.later()
	s.ix.GroundsKept()
	s.waitIdle()
	if s.ix.Seen() != seen {
		t.Errorf("Seen moved from %d to %d though nothing had", seen, s.ix.Seen())
	}
}

func TestWarm(t *testing.T) {
	s := newSpace(t)
	write(t, s.capsule, "scratch.txt", "x\n")
	st, err := s.ix.Warm(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if st.Repos != 1 || st.Files != 8 || st.Capsules != 1 || st.Changed != 1 {
		t.Errorf("warm: %+v", st)
	}
}

func TestADroppedRequestStops(t *testing.T) {
	s := newSpace(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	n := 0
	err := s.ix.Search(ctx, Request{Q: "main"}, func(Group) { n++ })
	if err == nil || n != 0 {
		t.Errorf("a cancelled search answered %d groups, err %v", n, err)
	}
	// What it started reading is still read, for whoever asks next.
	if got := s.files("main"); len(got) == 0 {
		t.Error("nothing found after a dropped request")
	}
}

var somePulls = []Pull{
	{Repo: "demo", Number: 12, Title: "Refresh the token before it expires", Author: "sam", Branch: "token-refresh", State: "needs-review", Words: "asked of you"},
	{Repo: "demo", Number: 128, Title: "Key the cache by tenant", Author: "priya", Branch: "cache/tenant", State: "approved", Checks: "success", Stack: "2 of 3: Cache by tenant"},
	{Repo: "api", Number: 12, Title: "Retry on 429", Author: "sam", Branch: "retry", State: "drafts", Draft: true},
	{Repo: "api", Number: 89, Title: "Token refresh: retry on 401", Author: "you", Branch: "fix-token-refresh", State: "ready", Mine: true},
}

func TestPulls(t *testing.T) {
	find := func(q string) []string {
		rs, _ := searchPulls(somePulls, q, 10)
		return refs(rs)
	}
	for _, c := range []struct {
		q    string
		want []string
	}{
		{"", []string{"demo#12", "demo#128", "api#12", "api#89"}}, // as the list has them
		{"12", []string{"demo#12", "api#12", "demo#128"}},         // the number whole, then its start
		{"#12", []string{"demo#12", "api#12", "demo#128"}},
		{"demo#12", []string{"demo#12", "demo#128"}},
		{"api#", []string{"api#12", "api#89"}},
		{"#89", []string{"api#89"}},
		{"429", []string{"api#12"}}, // a number in a title
		{"@sam", []string{"demo#12", "api#12"}},
		{"priya", []string{"demo#128"}},
		{"token refresh", []string{"api#89", "demo#12"}},
		{"sam retry", []string{"api#12"}}, // each term on its own part
		{"cache/tenant", []string{"demo#128"}},
		{"nothing-like-it", []string{}},
	} {
		if got := find(c.q); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%q: %v, want %v", c.q, got, c.want)
		}
	}
	rs, more := searchPulls(somePulls, "", 3)
	if len(rs) != 3 || more != 1 {
		t.Errorf("3 of 4: %d and %d more", len(rs), more)
	}
	rs, _ = searchPulls(somePulls, "tenant", 3)
	if len(rs) != 1 || rs[0].Kind != KindPull || rs[0].Number != 128 || rs[0].Stack == "" || rs[0].Checks != "success" || len(rs[0].At) != 6 {
		t.Errorf("tenant: %+v", rs)
	}
}

func TestWhatsTypedReadAsARef(t *testing.T) {
	s := newSpace(t)
	write(t, s.capsule, "internal/token/store.go", "package token\n")
	s.commit(s.capsule, "work")
	head := gitT(t, s.ground, "rev-parse", "HEAD")
	s.ix.Pulls = func() ([]Pull, time.Time) { return somePulls, s.clock() }
	exact := func(q string) *Result {
		t.Helper()
		g := s.find(q, KindRef)[KindRef]
		if len(g.Results) == 0 {
			return nil
		}
		if r := g.Results[0]; !r.Exact {
			t.Errorf("%q: not marked exact: %+v", q, r)
		}
		return &g.Results[0]
	}
	for _, c := range []struct {
		q, kind, ref string
	}{
		{"demo/main.go", KindFile, "demo/main.go"},
		{"demo/main.go:12", KindFile, "demo/main.go:12"},
		{"demo/internal/token/refresh.go:12-20", KindFile, "demo/internal/token/refresh.go:12-20"},
		{"demo/internal/token/", KindDir, "demo/internal/token/"},
		{"demo/internal", KindDir, "demo/internal/"},
		{"demo/feature:internal/token/store.go", KindFile, "demo/feature:internal/token/store.go"},
		{"demo/feature:main.go:3", KindFile, "demo/feature:main.go:3"},
		// Where it is on disk, whole or from the workspace's root.
		{s.capsule + "/internal/token/store.go", KindFile, "demo/feature:internal/token/store.go"},
		{"repos/demo/feature/internal/token/store.go:4", KindFile, "demo/feature:internal/token/store.go:4"},
		{"repos/demo/.ground/main.go", KindFile, "demo/main.go"},
		// Its address on GitHub.
		{"https://github.com/acme/demo/blob/main/main.go", KindFile, "demo/main.go"},
		{"https://github.com/acme/demo/blob/" + head + "/internal/token/refresh.go#L12-L20", KindFile, "demo/internal/token/refresh.go:12-20"},
		{"https://github.com/acme/demo/blob/main/main.go?plain=1#L7", KindFile, "demo/main.go:7"},
		{"https://github.com/acme/demo/blob/feature/internal/token/store.go", KindFile, "demo/feature:internal/token/store.go"},
		{"https://github.com/acme/demo/blob/some/branch/docs/guide.md", KindFile, "demo/docs/guide.md"},
		{"https://github.com/acme/demo/tree/main/internal/token", KindDir, "demo/internal/token/"},
		{"https://github.com/acme/demo/pull/12", KindPull, "demo#12"},
		{"https://github.com/acme/demo/pull/128/files#diff-abc", KindPull, "demo#128"},
		{"https://github.com/acme/demo/issues/12", KindPull, "demo#12"},
		{"demo#12", KindPull, "demo#12"},
		{"api#89", KindPull, "api#89"},
	} {
		r := exact(c.q)
		if r == nil || r.Kind != c.kind || r.Ref != c.ref {
			t.Errorf("%q: %+v, want %s %s", c.q, r, c.kind, c.ref)
		}
	}
	// One that isn't among the open ones is offered, to ask GitHub for.
	if r := exact("demo#812"); r == nil || !r.Asks || r.Number != 812 || r.Title != "Open demo#812" {
		t.Errorf("demo#812: %+v", r)
	}
	if r := exact("https://github.com/acme/demo/pull/812"); r == nil || !r.Asks {
		t.Errorf("an address of one not in hand: %+v", r)
	}
	// What isn't there, or isn't the workspace's, names nothing.
	for _, q := range []string{
		"demo/nope.go", "demo/", "demo", "main.go", "and/or", "nope/main.go", "demo/../main.go", "demo/feature:nope.go", "demo/nope:main.go",
		"nope#12", "demo#0", "demo/main.go and more",
		"https://github.com/other/demo/blob/main/main.go", "https://github.com/acme/nope/pull/3", "https://github.com/acme/demo",
		"https://github.com/acme/demo/blob/main/nope.go", "https://github.com/acme/demo/issues/812", "https://example.com/acme/demo/pull/12",
	} {
		if r := exact(q); r != nil {
			t.Errorf("%q named %+v", q, r)
		}
	}
	// A file named exactly says which capsules have changed it, too.
	write(t, s.capsule, "main.go", "package main\n\nfunc main() {}\n")
	s.later()
	if r := exact("demo/main.go"); r == nil || len(r.In) != 1 {
		t.Errorf("demo/main.go, changed in a capsule: %+v", r)
	}
	if r := exact("demo/feature:main.go"); r == nil || r.Status != Modified {
		t.Errorf("demo/feature:main.go: %+v", r)
	}
}

func TestSearchAnswersAKindAtATime(t *testing.T) {
	s := newSpace(t)
	s.ix.Pulls = func() ([]Pull, time.Time) { return somePulls, s.clock() }
	var order []string
	s.ix.Search(context.Background(), Request{Q: "token"}, func(g Group) { order = append(order, g.Kind) })
	if want := []string{KindPulls, KindRef, KindCapsules, KindFiles}; !reflect.DeepEqual(order, want) {
		t.Errorf("groups came %v, want %v", order, want)
	}
	got := s.find("token")
	if len(got[KindPulls].Results) != 2 || got[KindPulls].At == nil || len(got[KindRef].Results) != 0 || len(got[KindFiles].Results) != 2 {
		t.Errorf("token: %+v", got)
	}
	// Only the kinds asked for.
	if got := s.find("token", KindPulls); len(got) != 1 {
		t.Errorf("pulls only: %v", got)
	}
	// With no GitHub there's no group of pull requests at all.
	s.ix.Pulls = nil
	if _, ok := s.find("token")[KindPulls]; ok {
		t.Error("a group of pull requests with no GitHub")
	}
}

// A workspace's worth of paths, made up: how long a letter typed takes.
func bigSnapshot(repos, each int) *snapshot {
	dirs := []string{"internal", "cmd", "pkg", "web/src", "docs", "scripts", "test/fixtures", "api/v1", "api/v2", "vendor/lib"}
	names := []string{"server", "client", "token", "cache", "refresh", "handler", "router", "store", "index", "config", "model", "view", "page", "review", "mission"}
	exts := []string{".go", "_test.go", ".ts", ".tsx", ".md", ".json"}
	snap := &snapshot{}
	for r := range repos {
		g := &groundData{repo: fmt.Sprintf("repo%d", r)}
		g.repoMask = Mask(g.repo) | maskOf('/')
		for i := range each {
			p := fmt.Sprintf("%s/%s%d/%s/%s%d%s", dirs[i%len(dirs)], names[(i/7)%len(names)], i%97, names[(i/3)%len(names)], names[i%len(names)], i, exts[i%len(exts)])
			g.paths = append(g.paths, p)
			g.masks = append(g.masks, Mask(p))
		}
		snap.grounds = append(snap.grounds, g)
	}
	return snap
}

func BenchmarkFiles(b *testing.B) {
	snap := bigSnapshot(10, 50_000) // half a million paths
	for _, q := range []string{"s", "tok", "tokref", "server.go", "repo3/handler", "zqxj"} {
		b.Run(q, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				searchFiles(context.Background(), snap, q, DefaultLimit)
			}
		})
	}
}
