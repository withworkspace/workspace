package team

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/github"
)

func TestParseGist(t *testing.T) {
	files := []FileChange{{Path: "api/bundles.tsp"}, {Path: "src/bundle/wire.py"}, {Path: "src/other/wire.py"}, {Path: "README.md"}}
	gist, know, err := ParseGist("Here you go.\n\ngist: Bundles say what they contain.\nknow: contract | `api/bundles.tsp` | `Bundle` has `type` and `contents`: clients built before this ignore them.\n"+
		"know: action | bundles.tsp | Regenerate the SDK.\n"+ // a file named by its end
		"know: behavior | README.md | Old bundles show no total.\n"+ // spelt the other way
		"know: contract | wire.py | Two files end so: which isn't said.\n"+
		"know: contract | nope.go | Not a file of the change.\n"+
		"know: opinion | README.md | Not a kind.\n", files)
	if err != nil || gist != "Bundles say what they contain." {
		t.Fatalf("gist = %q, %v", gist, err)
	}
	if len(know) != 3 || know[0].Kind != KnowContract || know[0].File != "api/bundles.tsp" || !strings.HasPrefix(know[0].Text, "`Bundle` has") {
		t.Fatalf("know = %+v", know)
	}
	if know[1].File != "api/bundles.tsp" || know[2].Kind != KnowBehaviour {
		t.Errorf("know = %+v", know)
	}

	gist, know, err = ParseGist("gist: A typo in the README is fixed.\nknow: none", files)
	if err != nil || gist == "" || len(know) != 0 {
		t.Errorf("none: %q, %+v, %v", gist, know, err)
	}
	if _, _, err := ParseGist("I couldn't tell what this does.", files); err == nil {
		t.Error("a reply with no gist was taken")
	}
}

func TestDependenciesOnly(t *testing.T) {
	files := func(paths ...string) []FileChange {
		var out []FileChange
		for _, p := range paths {
			out = append(out, FileChange{Path: p})
		}
		return out
	}
	for _, c := range []struct {
		files []FileChange
		want  bool
	}{
		{files("package.json", "pnpm-lock.yaml"), true},
		{files("pyproject.toml", "uv.lock", "packages/api/pyproject.toml"), true},
		{files("go.mod", "go.sum"), true},
		{files("pnpm-lock.yaml"), true},
		{files("pyproject.toml"), false}, // a manifest alone may be a lint rule
		{files("package.json", "pnpm-lock.yaml", "src/app.ts"), false},
		{files("README.md"), false},
	} {
		if got := DependenciesOnly(c.files); got != c.want {
			t.Errorf("DependenciesOnly(%v) = %v", c.files, got)
		}
	}
	// A bump has a gist, and nothing worth knowing, whatever was said.
	m := Material{Files: files("package.json", "pnpm-lock.yaml")}
	gist, know, err := read("gist: glob is 14.\nknow: behaviour | package.json | Check your glob calls.", m)
	if err != nil || gist == "" || len(know) != 0 {
		t.Errorf("a bump: %q, %+v, %v", gist, know, err)
	}
}

func TestSubjects(t *testing.T) {
	open := pr(20, "fix: evict on revoke", func(a *github.PRActivity) { a.Body, a.BaseRefName = "Evicts.", "main" })
	landed := pr(12, "feat: cache tokens", func(a *github.PRActivity) { a.State, a.MergedAt = "MERGED", stamp(4, 10) })
	unseen := pr(13, "feat: merged, and not fetched", func(a *github.PRActivity) { a.State, a.MergedAt = "MERGED", stamp(4, 11) })
	closed := pr(14, "spike", func(a *github.PRActivity) { a.State, a.ClosedAt = "CLOSED", stamp(4, 12) })
	bot := pr(15, "chore: bump", func(a *github.PRActivity) { a.Author, a.Bot = "dependabot", true })
	commits := map[string][]Commit{"api": {
		{SHA: "aaa", Name: "Ana Diaz", Email: "ana@acme.example", Subject: "feat: cache tokens (#12)", At: at(4, 10), Parents: 1},
		{SHA: "r2", Name: "Cal Reyes", Email: "cal@acme.example", Subject: "docs: amend adr", At: at(4, 12), Parents: 1, Paths: []string{"adr.md"}},
		{SHA: "r1", Name: "Cal Reyes", Email: "cal@acme.example", Subject: "docs: add adr", At: at(4, 9), Parents: 1, Paths: []string{"adr.md"}},
	}}
	prs := []PR{open, landed, unseen, closed, bot}
	byKey := map[string]PR{}
	for _, p := range prs {
		byKey[p.Key()] = p
	}
	subs := subjects(Build(prs, commits, map[string]string{"api": "main"}), byKey, nil, utc)
	by := map[string]Subject{}
	for _, s := range subs {
		by[s.Strand] = s
	}
	if s := by["pr:api#20"]; s.Of != OfOpen || s.From != OfOpen || s.Body != "Evicts." || s.Base != "main" || s.Branch != "branch" || len(s.Commits) != 0 {
		t.Errorf("open = %+v", s)
	}
	if s := by["pr:api#12"]; s.Of != OfLanded || s.From != "aaa" || len(s.Commits) != 1 || s.Title != "feat: cache tokens" {
		t.Errorf("landed = %+v", s)
	}
	// Merged, but git hasn't its commit; and closed: nothing to write of.
	if _, ok := by["pr:api#13"]; ok {
		t.Error("a merge git hasn't seen is a subject")
	}
	if _, ok := by["pr:api#14"]; ok {
		t.Error("a closed pull request is a subject")
	}
	if !by["pr:api#15"].Bot {
		t.Error("the bot's isn't a bot's")
	}
	run := by["c:api:cal-reyes:2026-11-04:r1"]
	if run.Of != OfCommits || len(run.Commits) != 2 || len(run.Subjects) != 2 || run.From == "" || run.Bot || run.Title != "docs: add adr" {
		t.Errorf("commits = %+v", run)
	}
	// Another commit that day is another thing to write of.
	commits["api"] = append(commits["api"], Commit{SHA: "r3", Name: "Cal Reyes", Email: "cal@acme.example", Subject: "docs: amend adr again", At: at(4, 15), Parents: 1, Paths: []string{"adr.md"}})
	more := 0
	for _, s := range subjects(Build(prs, commits, nil), byKey, nil, utc) {
		if s.Strand == run.Strand {
			more = len(s.Commits)
			if s.From == run.From {
				t.Error("more commits, and it's from the same")
			}
		}
	}
	if more != 3 {
		t.Errorf("the strand has %d commits, want 3", more)
	}
}

func TestMaterial(t *testing.T) {
	w := newWorld(t)
	big := strings.Repeat("a line of this file\n", 600)
	w.write("big.txt", big)
	w.write("package-lock.json", "{}\n")
	sha := w.commit("Ana Diaz", "ana@acme.example", "feat: one (#3)", at(8, 9), "src/a.go")
	Fetch(w.bare, "main")

	bare := func(string) string { return w.bare }
	m, err := material(bare, Subject{Of: OfLanded, Commits: []string{sha}})
	if err != nil {
		t.Fatal(err)
	}
	paths := map[string]FileChange{}
	for _, f := range m.Files {
		paths[f.Path] = f
	}
	if len(m.Files) != 3 || paths["src/a.go"].Additions != 1 || paths["big.txt"].Additions != 600 {
		t.Fatalf("files = %+v", m.Files)
	}
	// What a tool wrote isn't given, and a long file's change is cut.
	if strings.Contains(m.Diff, "package-lock.json") || !strings.Contains(m.Diff, "+feat: one (#3)") {
		t.Errorf("diff = %s", clip(m.Diff, 300))
	}
	if !m.Clipped || len(m.Diff) > diffPerFile+2048 || !strings.Contains(m.Diff, "the rest of this file's change is left out") {
		t.Errorf("a long file wasn't cut: %d bytes, clipped %v", len(m.Diff), m.Clipped)
	}

	// An open pull request's change is its branch's, since it left its base.
	gitT(t, w.remote, nil, "checkout", "-q", "-b", "evict")
	head := w.commit("Ben Okoro", "ben@acme.example", "wip", at(9, 9), "evict.go")
	gitT(t, w.remote, nil, "update-ref", "refs/pull/7/head", head)
	gitT(t, w.remote, nil, "checkout", "-q", "main")
	w.commit("Cal Reyes", "cal@acme.example", "docs: meanwhile", at(9, 10), "docs.md")
	m, err = material(bare, Subject{Of: OfOpen, PR: 7, Branch: "evict", Base: "main"})
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Files) != 1 || m.Files[0].Path != "evict.go" || !strings.Contains(m.Diff, "+wip") || strings.Contains(m.Diff, "docs.md") {
		t.Errorf("open: files %+v, diff %s", m.Files, clip(m.Diff, 200))
	}
	if got := renamedTo("src/{old => new}/a.go"); got != "src/new/a.go" {
		t.Errorf("renamedTo = %q", got)
	}
	if got := renamedTo("a.go => b.go"); got != "b.go" {
		t.Errorf("renamedTo = %q", got)
	}
}

// waitNarrated waits until nothing's being written.
func waitNarrated(t *testing.T, s *Service) {
	t.Helper()
	for range 400 {
		s.mu.Lock()
		n := len(s.writing)
		s.mu.Unlock()
		if n == 0 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("gists were still being written")
}

func TestNarrate(t *testing.T) {
	w := newWorld(t)
	sha := w.commit("Ana Diaz", "ana@acme.example", "feat: cache tokens (#12)", at(9, 9), "cache.go")
	w.commit("Cal Reyes", "cal@acme.example", "docs: add adr", at(10, 10), "adr.md")
	w.commit("Dee Old", "dee@acme.example", "chore: long ago", at(3, 10), "old.md")
	w.prs = []github.PRActivity{{
		Number: 12, Title: "feat: cache tokens", State: "MERGED", Author: "ana-acme", CreatedAt: stamp(8, 9), MergedAt: stamp(9, 9),
		Body: "Caches tokens per tenant.", MergeCommit: sha,
	}}
	var mu sync.Mutex
	var asks []Ask
	fail := false
	w.svc.Writer = func(a Ask) (string, error) {
		mu.Lock()
		defer mu.Unlock()
		asks = append(asks, a)
		if fail {
			return "", errors.New("the model isn't there")
		}
		if a.Subject.Kind == "commits" {
			return "gist: An ADR is recorded.\nknow: convention | adr.md | Cache keys are per tenant from now on.", nil
		}
		return "gist: Tokens are cached per tenant.\nknow: none", nil
	}
	snap, err := w.svc.Look(false, time.Time{})
	if err != nil {
		t.Fatal(err)
	}

	// Unasked, only what's happened since yesterday is written.
	if n := w.svc.Narrate(snap, nil, time.Time{}); n != 2 {
		t.Fatalf("started %d, want 2 (not the commit from a week ago, nor the first)", n)
	}
	waitNarrated(t, w.svc)
	asked := map[string]Ask{}
	for _, a := range asks {
		asked[a.Subject.Strand] = a
	}
	cal := ""
	for strand := range asked {
		if strings.HasPrefix(strand, "c:api:cal-reyes:2026-11-10:") {
			cal = strand
		}
	}
	if cal == "" || len(asks) != 2 {
		t.Fatalf("asked of %d, and not of cal's commit", len(asks))
	}
	if a := asked["pr:api#12"]; a.Subject.Body != "Caches tokens per tenant." || len(a.Material.Files) != 1 || !strings.Contains(a.Material.Diff, "cache.go") {
		t.Errorf("the pull request's ask = %+v", a)
	}
	tl := w.svc.Timeline(snap, at(9, 0))
	today := tl.Days[0].Items[0]
	if today.Gist != "An ADR is recorded." || len(today.Know) != 1 || today.Narrative != "" {
		t.Errorf("today = %+v", today)
	}
	if len(tl.Know) != 1 || tl.Know[0].Kind != KnowConvention || tl.Know[0].Item != today.Key || tl.Know[0].File != "adr.md" {
		t.Errorf("worth knowing = %+v", tl.Know)
	}
	if !tl.Narrative.On || tl.Narrative.Writing != 0 || tl.Narrative.Unwritten != 0 {
		t.Errorf("narrative = %+v", tl.Narrative)
	}
	// Asked again, nothing's written again; the older ones can be asked for.
	// And nothing's news: an open Team page asks each tick, and mustn't
	// read it all again when nothing started.
	v := w.svc.NarrativeVersion()
	if n := w.svc.Narrate(snap, nil, time.Time{}); n != 0 {
		t.Errorf("a second asking started %d", n)
	}
	if got := w.svc.NarrativeVersion(); got != v {
		t.Errorf("an asking that started nothing moved the version from %d to %d", v, got)
	}
	all := w.svc.Timeline(snap, time.Time{})
	if all.Narrative.Unwritten != 2 {
		t.Errorf("%d unwritten, want 2", all.Narrative.Unwritten)
	}

	// A failure is kept, shown, and tried again only when asked for by name.
	fail = true
	old := ""
	for _, d := range all.Days {
		for _, it := range d.Items {
			if strings.HasPrefix(it.Strand, "c:api:dee-old:") {
				old = it.Strand
			}
		}
	}
	if n := w.svc.Narrate(snap, []string{old}, time.Time{}); n != 1 {
		t.Fatalf("asked for one, started %d", n)
	}
	waitNarrated(t, w.svc)
	find := func(tl *Timeline, strand string) Item {
		for _, d := range tl.Days {
			for _, it := range d.Items {
				if it.Strand == strand {
					return it
				}
			}
		}
		t.Fatalf("no item of %s", strand)
		return Item{}
	}
	if it := find(w.svc.Timeline(snap, time.Time{}), old); it.Narrative != "failed" || !strings.Contains(it.GistError, "isn't there") {
		t.Errorf("failed = %+v", it)
	}
	if n := w.svc.Narrate(snap, nil, snap.Since); n != 1 {
		t.Errorf("the period's started %d, want 1 (the one never tried, not the one that failed)", n)
	}
	waitNarrated(t, w.svc)
	fail = false
	if n := w.svc.Narrate(snap, []string{old}, time.Time{}); n != 1 {
		t.Errorf("trying again started %d", n)
	}
	waitNarrated(t, w.svc)
	if it := find(w.svc.Timeline(snap, time.Time{}), old); it.Narrative != "" || it.Gist == "" {
		t.Errorf("tried again = %+v", it)
	}

	// What's written is kept: a new service has it.
	again := w.service()
	if it := find(again.Timeline(again.Peek(), time.Time{}), "pr:api#12"); it.Gist != "Tokens are cached per tenant." {
		t.Errorf("kept = %+v", it)
	}
	// And with nothing to write them, the page says so.
	if n := again.Timeline(again.Peek(), time.Time{}).Narrative; n.On || n.Off == "" {
		t.Errorf("no writer: %+v", n)
	}
}

// sweep is the same change in three repos, by ana-acme on one branch
// name: api's merged (in a commit git has), bff's and ui's open.
func sweep() ([]PR, map[string][]Commit) {
	in := func(repo string, number int, set func(*github.PRActivity)) PR {
		p := pr(number, "chore: pin undici", func(a *github.PRActivity) {
			a.HeadRefName, a.BaseRefName, a.CreatedAt, a.Body = "pin-undici", "main", stamp(3, 9), "Pins undici in "+repo+"."
			if set != nil {
				set(a)
			}
		})
		p.Repo = repo
		return p
	}
	prs := []PR{
		in("api", 12, func(a *github.PRActivity) { a.State, a.MergedAt = "MERGED", stamp(4, 10) }),
		in("bff", 7, func(a *github.PRActivity) {
			a.Reviews = []github.PRReview{{Author: "ben-acme", State: "APPROVED", SubmittedAt: stamp(4, 9)}}
		}),
		in("ui", 30, func(a *github.PRActivity) {
			a.CreatedAt = stamp(5, 9)
			a.Reviews = []github.PRReview{{Author: "ben-acme", State: "COMMENTED", SubmittedAt: stamp(5, 10)}, {Author: "ben-acme", State: "APPROVED", SubmittedAt: stamp(5, 11)}}
		}),
		// The same branch name by someone else, and by her a month later:
		// neither is part of it.
		in("docs", 2, func(a *github.PRActivity) { a.Author = "cal-acme" }),
		in("docs", 9, func(a *github.PRActivity) { a.CreatedAt = "2026-12-20T09:00:00Z" }),
	}
	commits := map[string][]Commit{"api": {{SHA: "aaa", Name: "Ana Diaz", Email: "ana@acme.example", Subject: "chore: pin undici (#12)", At: at(4, 10), Parents: 1}}}
	return prs, commits
}

func TestJoined(t *testing.T) {
	prs, commits := sweep()
	cfg := map[string]PersonConfig{"ben": {Name: "Ben Okoro", Logins: []string{"ben-acme"}}}
	tl := View(Build(prs, commits, nil), cfg, at(3, 0), utc)
	by := map[string]Item{}
	for _, d := range tl.Days {
		for _, it := range d.Items {
			by[it.Key] = it
		}
	}
	strand := "b:pin-undici:api#12"
	first := by[strand+"@2026-11-03"]
	if first.Name != "Pin undici" || first.PR != 0 || first.Repo != "api" || strings.Join(first.Repos, " ") != "api bff ui" || len(first.PRs) != 3 {
		t.Fatalf("the day it opened = %+v", first)
	}
	// It's in review while any of it is, and each pull request says where
	// it stands itself.
	if first.State != InReview || first.PRs[0].State != Done || first.PRs[1].State != InReview || first.PRs[2].Number != 30 {
		t.Errorf("standing = %s, %+v", first.State, first.PRs)
	}
	if want := "Ana opened it in 2 repos."; first.Words != want {
		t.Errorf("day one says %q, want %q", first.Words, want)
	}
	second := by[strand+"@2026-11-04"]
	if want := "Ben approved it in bff. Ana landed it in api."; second.Words != want || strings.Join(second.People, " ") != "ana-acme ben" {
		t.Errorf("day two says %q (%v), want %q", second.Words, second.People, want)
	}
	// He commented and then approved: that's approving.
	if want := "Ana opened it in ui. Ben approved it in ui."; by[strand+"@2026-11-05"].Words != want {
		t.Errorf("day three says %q, want %q", by[strand+"@2026-11-05"].Words, want)
	}
	// Where its pull requests are called different things, it's called by
	// the branch they share.
	prs[1].Title = "chore: pin undici in the bff"
	for _, d := range View(Build(prs, commits, nil), cfg, at(3, 0), utc).Days {
		for _, it := range d.Items {
			if it.Strand == strand && it.Name != "Pin undici" {
				t.Errorf("with titles that differ it's called %q", it.Name)
			}
		}
	}
	for in, want := range map[string]string{"pin-undici": "Pin undici", "deps-security-sweep-2026-11-09": "Deps security sweep 2026-11-09", "ana/typespec-1-17": "Typespec 1-17", "custom_actions-create": "Custom actions create"} {
		if got := branchName(in); got != want {
			t.Errorf("branchName(%q) = %q, want %q", in, got, want)
		}
	}
	prs[1].Title = "chore: pin undici"

	// Someone else's, and hers from much later, are their own.
	if it := by["pr:docs#2@2026-11-03"]; it.PR != 2 || len(it.PRs) != 0 {
		t.Errorf("another's pull request on the same branch name = %+v", it)
	}
	if it := by["pr:docs#9@2026-12-20"]; it.PR != 9 {
		t.Errorf("hers from a month on = %+v", it)
	}
	if n := len(by); n != 5 {
		t.Errorf("%d rows, want 5", n)
	}
}

func TestSubjects_Joined(t *testing.T) {
	prs, commits := sweep()
	byKey := map[string]PR{}
	for _, p := range prs {
		byKey[p.Key()] = p
	}
	whole := func() Subject {
		for _, s := range subjects(Build(prs, commits, nil), byKey, nil, utc) {
			if s.Strand == "b:pin-undici:api#12" {
				return s
			}
		}
		t.Fatal("the change in several repos isn't a subject")
		return Subject{}
	}
	open := whole()
	if open.Of != OfOpen || len(open.Parts) != 3 || open.Repo != "api" || open.Title != "chore: pin undici" || open.Parts[0].Of != OfLanded || open.Parts[1].Branch != "pin-undici" {
		t.Fatalf("while some are open = %+v", open)
	}
	// Its pull requests aren't also subjects of their own.
	for _, s := range subjects(Build(prs, commits, nil), byKey, nil, utc) {
		if s.Strand == "pr:api#12" || s.Strand == "pr:bff#7" {
			t.Errorf("%s is a subject of its own", s.Strand)
		}
	}
	// One more landing, with one still open: it's of the same.
	prs[1].State, prs[1].MergedAt = "MERGED", stamp(6, 9)
	commits["bff"] = []Commit{{SHA: "bbb", Name: "Ana Diaz", Email: "ana@acme.example", Subject: "chore: pin undici (#7)", At: at(6, 9), Parents: 1}}
	if got := whole(); got.Of != OfOpen || got.From != open.From {
		t.Errorf("one more landed: of %s, from the same %v", got.Of, got.From == open.From)
	}
	// The last merged, but git hasn't its commit: still not all landed.
	prs[2].State, prs[2].MergedAt = "MERGED", stamp(7, 9)
	if got := whole(); got.Of != OfOpen || len(got.Parts) != 2 {
		t.Errorf("merged and not fetched: of %s, %d parts", got.Of, len(got.Parts))
	}
	// Fetched: it's landed, and to be written of again.
	commits["ui"] = []Commit{{SHA: "ccc", Name: "Ana Diaz", Email: "ana@acme.example", Subject: "chore: pin undici (#30)", At: at(7, 9), Parents: 1}}
	if got := whole(); got.Of != OfLanded || got.From == open.From || len(got.Parts) != 3 {
		t.Errorf("all landed: of %s, %d parts", got.Of, len(got.Parts))
	}
}

func TestMaterial_Joined(t *testing.T) {
	w := newWorld(t)
	one := w.commit("Ana Diaz", "ana@acme.example", "chore: pin undici (#12)", at(8, 9), "package.json")
	two := w.commit("Ana Diaz", "ana@acme.example", "chore: pin undici (#7)", at(8, 10), "src/pin.ts")
	Fetch(w.bare, "main")
	// Two repos' clones, which here are one.
	sub := Subject{Of: OfLanded, Parts: []Subject{
		{Repo: "api", PR: 12, Of: OfLanded, Commits: []string{one}},
		{Repo: "bff", PR: 7, Of: OfLanded, Commits: []string{two}},
	}}
	m, err := material(func(string) string { return w.bare }, sub)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Files) != 2 || m.Files[0].Name() != "api:package.json" || m.Files[1].Name() != "bff:src/pin.ts" {
		t.Fatalf("files = %+v", m.Files)
	}
	if !strings.Contains(m.Diff, "=== in api (#12)") || !strings.Contains(m.Diff, "=== in bff (#7)") {
		t.Errorf("diff = %s", clip(m.Diff, 300))
	}
	// What's worth knowing says which repo its file is in.
	_, know, err := ParseGist("gist: undici is pinned.\nknow: action | bff:src/pin.ts | Re-run the pin.\nknow: action | package.json | By its path alone.", m.Files)
	if err != nil || len(know) != 2 || know[0].In != "bff" || know[0].File != "src/pin.ts" || know[1].In != "api" {
		t.Errorf("know = %+v, %v", know, err)
	}
	// A repo that isn't cloned doesn't stop the rest being read.
	m, err = material(func(repo string) string {
		if repo == "api" {
			return w.bare
		}
		return ""
	}, sub)
	if err != nil || len(m.Files) != 1 {
		t.Errorf("with one not cloned: %+v, %v", m.Files, err)
	}
}

func TestParseSummary(t *testing.T) {
	list := []Change{{Number: 1, Strand: "pr:api#12"}, {Number: 2, Strand: "pr:bff#7"}, {Number: 3, Strand: "c:records:cal:2026-11-04:r1"}}
	topics, err := ParseSummary("Here it is.\n- topic: **Token cache** | 1, 2 | Tokens are cached per tenant in the API; the BFF's half is in review.\n"+
		"topic: Records | 3 and 2 | ADR 0007 is recorded.\n"+ // 2 is taken
		"topic: Nothing real | 9 | Names no change it was given.\n"+
		"topic: No text | 3 |\n", list)
	if err != nil || len(topics) != 2 {
		t.Fatalf("topics = %+v, %v", topics, err)
	}
	if got := topics[0]; got.Name != "Token cache" || strings.Join(got.Strands, " ") != "pr:api#12 pr:bff#7" || !strings.HasPrefix(got.Text, "Tokens are cached") {
		t.Errorf("first = %+v", got)
	}
	if got := topics[1]; got.Name != "Records" || len(got.Strands) != 1 || got.Strands[0] != "c:records:cal:2026-11-04:r1" {
		t.Errorf("second = %+v", got)
	}
	if _, err := ParseSummary("I can't tell.", list); err == nil {
		t.Error("a reply with no topics was taken")
	}
}

func TestSummarise(t *testing.T) {
	w := newWorld(t)
	sha := w.commit("Ana Diaz", "ana@acme.example", "feat: cache tokens (#12)", at(9, 9), "cache.go")
	w.commit("Cal Reyes", "cal@acme.example", "docs: add adr", at(10, 10), "adr.md")
	w.prs = []github.PRActivity{{
		Number: 12, Title: "feat: cache tokens", State: "MERGED", Author: "ana-acme", CreatedAt: stamp(8, 9), MergedAt: stamp(9, 9),
		MergeCommit: sha, Reviews: []github.PRReview{{Author: "ben-acme", State: "APPROVED", SubmittedAt: stamp(8, 15)}},
	}}
	w.svc.Writer = func(a Ask) (string, error) { return "gist: " + a.Subject.Title + " is done.\nknow: none", nil }
	var asked []SummaryAsk
	reply := "topic: Token cache | 1, 2 | Tokens are cached, and an ADR says how."
	w.svc.Summariser = func(a SummaryAsk) (string, error) {
		asked = append(asked, a)
		return reply, nil
	}
	snap, err := w.svc.Look(false, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	wait := func() {
		t.Helper()
		for range 400 {
			w.svc.mu.Lock()
			n := len(w.svc.summarising) + len(w.svc.writing)
			w.svc.mu.Unlock()
			if n == 0 {
				return
			}
			time.Sleep(5 * time.Millisecond)
		}
		t.Fatal("still writing")
	}
	view := func() *SummaryView {
		tl := w.svc.Timeline(snap, time.Time{})
		w.svc.Account(tl, "")
		return tl.Summary
	}

	// With nothing read, there's nothing to write it from.
	if w.svc.Summarise(snap, time.Time{}, "") || view().Can != 0 {
		t.Fatal("an account was started from changes nobody's read")
	}
	w.svc.Narrate(snap, nil, time.Time{})
	wait()
	if v := view(); v.Can != 2 || v.At != nil {
		t.Fatalf("before: %+v", v)
	}
	if !w.svc.Summarise(snap, time.Time{}, "") {
		t.Fatal("it didn't start")
	}
	wait()
	// It's given the changes that have been read, the latest first, with
	// who's on each.
	if len(asked) != 1 || len(asked[0].Changes) != 2 || asked[0].Changes[0].Number != 1 || asked[0].Changes[0].Gist != "docs: add adr is done." {
		t.Fatalf("asked = %+v", asked)
	}
	if got := asked[0].Changes[1]; got.Strand != "pr:api#12" || strings.Join(got.People, " ") != "Ana ben-acme" || got.State != Done {
		t.Errorf("the pull request as told = %+v", got)
	}
	v := view()
	if len(v.Topics) != 1 || v.Stale || v.Failed != "" || v.Topics[0].Name != "Token cache" || len(v.Topics[0].Strands) != 2 {
		t.Fatalf("written = %+v", v)
	}
	// Who's on it is whose its changes are, then who else was on them.
	if got := strings.Join(v.Topics[0].People, " "); got != "cal-reyes ana-acme ben-acme" {
		t.Errorf("people = %s", got)
	}

	// Something more lands and is read: it's out of date.
	w.now = w.now.Add(time.Hour)
	w.commit("Cal Reyes", "cal@acme.example", "docs: add a note", at(10, 12), "note.md")
	snap, _ = w.svc.Look(false, time.Time{})
	w.svc.Narrate(snap, nil, time.Time{})
	wait()
	if v := view(); !v.Stale || v.Can != 3 || len(v.Topics) != 1 {
		t.Errorf("after more: %+v", v)
	}
	// Written again and failing, it keeps what it had, with why.
	reply = "I'd rather not."
	w.svc.Summarise(snap, time.Time{}, "")
	wait()
	if v := view(); len(v.Topics) != 1 || !strings.Contains(v.Failed, "no topics") {
		t.Errorf("after a failure: %+v", v)
	}
	// It's kept: a new service has it, under what the period's called.
	again := w.service()
	tl := again.Timeline(again.Peek(), time.Time{})
	again.Account(tl, "")
	if tl.Summary == nil || len(tl.Summary.Topics) != 1 {
		t.Errorf("kept = %+v", tl.Summary)
	}
	other := again.Timeline(again.Peek(), time.Time{})
	again.Account(other, "7d")
	if other.Summary != nil {
		t.Errorf("another period has this one's account: %+v", other.Summary)
	}
}
