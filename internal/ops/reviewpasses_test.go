package ops

import (
	"github.com/withworkspace/workspace/internal/store/storetest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/config"
	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/doc"
	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/job"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/page"
	"github.com/withworkspace/workspace/internal/prompt"
	"github.com/withworkspace/workspace/internal/review"
	"github.com/withworkspace/workspace/internal/workspace"
)

// changed is a repo with two commits, the change between them, for a pass
// to have read.
func changed(t *testing.T) (dir, base, head string) {
	t.Helper()
	dir = t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@t", "-c", "commit.gpgsign=false"}, args...)...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	write := func(name, text string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git("init", "-q", "--initial-branch=main")
	write("key.go", "package cache\n\nfunc key(id string) string {\n\treturn id\n}\n")
	write("gone.go", "package cache\n\nvar old = 1\n")
	git("add", "-A")
	git("commit", "-q", "-m", "base")
	base = git("rev-parse", "HEAD")
	write("key.go", "package cache\n\n// key names an entry.\nfunc key(tenant, id string) string {\n\tk := tenant + id\n\treturn k\n}\n")
	os.Remove(filepath.Join(dir, "gone.go"))
	git("add", "-A")
	git("commit", "-q", "-m", "change")
	return dir, base, git("rev-parse", "HEAD")
}

// What a review's pass's agent says is read into findings on the change's
// lines as they read: at its head, or, for lines it removes, before it. A
// finding on lines that aren't there is one on the change as a whole, and
// a kind the pass hasn't is one that doesn't block.
func TestReviewPassRead(t *testing.T) {
	dir, base, head := changed(t)
	w := New(&config.Config{}, &workspace.Workspace{Root: storetest.Own(t, t.TempDir()).Root()}, nil, Options{})
	def, _ := page.PassNamed(page.ReviewPasses([]page.PassDef{{Name: "security", Looks: "x", Kinds: []string{"vulnerability", "hardening"}, Blocks: []string{"vulnerability"}}}), "security")
	a := &answering{w: w, rpass: prompt.ReviewPass{Def: def, Clone: dir, Base: base, Head: head}, around: head}
	out := reviewPassRead(a, harness.Report{Output: "Not as it is.\n\n" +
		"## vulnerability\n@ key.go:5-6\nThe tenant and the id run together.\n\n```suggestion\n\tk := tenant + \"/\" + id\n\treturn k\n```\n\n" +
		"## hardening\n@ gone.go:3 old\nIts callers still read it.\n\n" +
		"## hardening\n@ key.go:400\nToo long.\n\n" +
		"## praise\nNicely named.\n"}, nil)
	if out.err != nil || len(out.turns) != 1 {
		t.Fatalf("read: %+v", out)
	}
	turn := out.turns[0]
	if turn.Body != "Not as it is." || turn.Around != head || len(turn.Findings) != 4 {
		t.Fatalf("turn: %+v", turn)
	}
	f := turn.Findings
	if f[0].Commit != head || strings.Join(f[0].Excerpt, "|") != "\tk := tenant + id|\treturn k" || f[0].Suggest != "\tk := tenant + \"/\" + id\n\treturn k" || f[0].Key == "" {
		t.Errorf("on its lines: %+v", f[0])
	}
	if f[1].Commit != base || f[1].Side != "old" || strings.Join(f[1].Excerpt, "|") != "var old = 1" {
		t.Errorf("on lines it removes: %+v", f[1])
	}
	if f[2].Placed() || !strings.HasPrefix(f[2].Text, "(On key.go:400, not in the change.) Too long.") {
		t.Errorf("on lines that aren't there: %+v", f[2])
	}
	if f[3].Kind != "hardening" {
		t.Errorf("a kind the pass hasn't: %q", f[3].Kind)
	}

	// Its lines are found where they've moved to, and not if they've changed.
	if at, err := linesNow(dir, head, f[0]); err != nil || at != 5 {
		t.Errorf("where it was: %d, %v", at, err)
	}
	moved := f[0]
	moved.Line, moved.EndLine = 2, 3
	if at, err := linesNow(dir, head, moved); err != nil || at != 5 {
		t.Errorf("moved: %d, %v", at, err)
	}
	gone := f[0]
	gone.Excerpt = []string{"\tk := tenant", "\treturn k"}
	if _, err := linesNow(dir, head, gone); err == nil {
		t.Error("lines that have changed were found")
	}

	// It couldn't say anything.
	if out := reviewPassRead(a, harness.Report{}, os.ErrDeadlineExceeded); out.err == nil || out.turns[0].By != conversation.WS {
		t.Errorf("failed: %+v", out)
	}
}

func TestReviewPassDefs(t *testing.T) {
	w := New(&config.Config{Reviews: config.ReviewsConfig{Passes: map[string]config.ReviewPassConfig{
		"security": {Looks: "Find what could leak a secret.", Repos: []string{"api"}},
		"skilled":  {Skill: "no-such-skill"},
	}}}, &workspace.Workspace{Root: storetest.Own(t, t.TempDir()).Root()}, nil, Options{})
	if defs := w.ReviewPasses("web"); len(defs) != len(page.ReviewBuiltin)+1 || defs[len(defs)-1].Unavailable == "" {
		t.Errorf("for another repo: %+v", defs)
	}
	if defs := w.ReviewPasses("api"); len(defs) != len(page.ReviewBuiltin)+2 {
		t.Errorf("for its repo: %+v", defs)
	}
	if n := noteKindOf(page.PassDef{Blocks: []string{"vulnerability"}}, "vulnerability"); n != "blocking" {
		t.Errorf("a kind that blocks is a %s comment", n)
	}
	if n := noteKindOf(page.PassDef{Blocks: []string{"vulnerability"}}, "hardening"); n != "suggestion" {
		t.Errorf("a kind that doesn't is a %s comment", n)
	}
	if n := noteKindOf(page.PassDef{}, "nit"); n != "nit" {
		t.Errorf("a comment's own kind is a %s comment", n)
	}
}

// A follow-up's findings are each on the comment it's about, where that
// comment is. A blocking comment not dealt with blocks; what's said of
// anything else is left out.
func TestFollowedFindings(t *testing.T) {
	def, _ := page.PassNamed(page.ReviewPasses(nil), page.FollowUp)
	rp := prompt.ReviewPass{Def: def, Threads: []prompt.FollowThread{
		{Label: "T1", ID: "PRRT_1", Path: "key.go", Side: "new", Line: 41, EndLine: 42, Commit: "abc", Kind: "blocking", Excerpt: []string{"x", "y"}},
		{Label: "T2", ID: "PRRT_2", Path: "old.go", Side: "old", Line: 3, EndLine: 3, Kind: "nit"},
		{Label: "T3", ID: "PRRT_3", Path: "c.go", Side: "new", Line: 9, EndLine: 9, Kind: "blocking"},
	}}
	_, fs := conversation.ParseReviewFindings("One of three is dealt with.\n\n" +
		"## not addressed\n@ T1\nNothing that's been pushed touches the key.\n\n" +
		"## addressed\n@ T3\nThe nil check is at c.go:12 now.\n\n" +
		"## maybe\n@ T2.\nThe author says it's for a follow-up.\n\n" +
		"## addressed\n@ T9\nNo such comment.\n\n" +
		"## partly\nAbout nothing.\n")
	got := followedFindings(rp, fs)
	conversation.KeyFindings(got)
	if len(got) != 3 {
		t.Fatalf("findings: %+v", got)
	}
	if f := got[0]; f.Thread != "PRRT_1" || f.Kind != page.FollowedNot || !f.Blocks || f.Path != "key.go" || f.Line != 41 || f.EndLine != 42 || len(f.Excerpt) != 2 || f.Key == "" {
		t.Errorf("a blocking comment not dealt with: %+v", f)
	}
	if f := got[1]; f.Thread != "PRRT_3" || f.Kind != page.FollowedAddressed || f.Blocks {
		t.Errorf("one dealt with: %+v", f)
	}
	if f := got[2]; f.Thread != "PRRT_2" || f.Kind != page.FollowedPartly || f.Blocks || f.Side != "old" {
		t.Errorf("a kind it hasn't, on a nit: %+v", f)
	}
}

// A pass runs over a mission's capsule as it does over a pull request:
// its conversation is the workspace's, anchored to the capsule; its agent
// is told the mission's brief is what the work is meant to do, and where
// the capsule is; and a finding taken goes into the mission's draft
// review of the capsule. Follow-up, which is of comments on GitHub, isn't
// one of a capsule's.
func TestCapsulePass(t *testing.T) {
	root := t.TempDir()
	storetest.Own(t, root)
	dir := filepath.Join(root, "repos", "api", "fix")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	git := func(args ...string) string {
		t.Helper()
		out, err := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@t", "-c", "commit.gpgsign=false"}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-q", "--initial-branch=main")
	os.WriteFile(filepath.Join(dir, "key.go"), []byte("package cache\n\nfunc key(id string) string {\n\treturn id\n}\n"), 0o644)
	git("add", "-A")
	git("commit", "-q", "-m", "base")
	git("update-ref", "refs/remotes/origin/main", "HEAD")
	base := git("rev-parse", "HEAD")
	os.WriteFile(filepath.Join(dir, "key.go"), []byte("package cache\n\nfunc key(tenant, id string) string {\n\treturn tenant + id\n}\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "go.sum"), []byte("example.com/x v1.0.0 h1:x\n"), 0o644)
	git("add", "-A")
	git("commit", "-q", "-m", "by tenant")
	head := git("rev-parse", "HEAD")

	var asked []string
	w := New(&config.Config{
		Conversation: config.ConversationConfig{AgentConfig: config.AgentConfig{Harness: "claude"}},
		Reviews: config.ReviewsConfig{Passes: map[string]config.ReviewPassConfig{
			"check": {Looks: "Find what's wrong."},
			"quick": {Looks: "Anything odd.", Cold: true},
		}},
	}, &workspace.Workspace{Root: root, DefaultBranch: "main"}, nil, Options{Jobs: JobsOf(func(j job.Job) { asked = append(asked, j.ID) }, job.Answer, job.Thread, job.Draft)})
	m, err := w.Missions.Create("Key by tenant")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Missions.Update(m.Slug, func(m *mission.Mission) error {
		m.Capsules = append(m.Capsules, mission.Capsule{Repo: "api", Name: "fix", Branch: "fix"})
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	w.Missions.WriteBrief(m.Slug, "## Goal\n\nKey the cache by tenant.\n")
	of := OfCapsule(m.Slug, "api", "fix")

	var names []string
	for _, d := range w.ReviewPassesOf(of) {
		names = append(names, d.Name)
	}
	if strings.Join(names, " ") != "check quick" {
		t.Fatalf("a capsule's passes: %v", names)
	}
	if _, _, err := w.RunReviewPass(OfCapsule(m.Slug, "api", "nope"), "check"); err == nil {
		t.Error("a pass was run over a capsule the mission hasn't got")
	}

	// Run: its conversation is the workspace's, over the capsule.
	_, c, err := w.RunReviewPass(of, "check")
	if err != nil || c == nil || len(asked) != 1 {
		t.Fatalf("run: %v %+v %v", err, c, asked)
	}
	if c.What != conversation.WhatReviewPass || c.Anchor.Mission != m.Slug || c.Anchor.Capsule != "api/fix" || c.Anchor.Reviewed() != m.Slug+":api/fix" {
		t.Errorf("anchored: %+v", c.Anchor)
	}

	// What its agent's told: the capsule, the mission, and where it is.
	a := &answering{w: w, store: w.cells(), c: c, here: w.here(doc.Scope{}, c)}
	if stop, err := reviewPassReady(a); stop || err != nil {
		t.Fatalf("ready: %v %v", stop, err)
	}
	if a.rpass.Base != base || a.rpass.Head != head || a.rpass.Clone != dir || a.rpass.Capsule != dir || a.around != head {
		t.Errorf("the change: %+v", a.rpass)
	}
	p := reviewPassFirst(a)
	for _, want := range []string{"the work in the capsule api/fix, of the mission \"Key by tenant\"", "ws show -f llm -m " + m.Slug, "git -C " + dir + " diff " + base + " " + head, "It's checked out at " + dir, "Find what's wrong."} {
		if !strings.Contains(p, want) {
			t.Errorf("its prompt lacks %q:\n%s", want, p)
		}
	}
	if strings.Contains(p, "reviewing the pull request") || strings.Contains(p, "ws show -f llm api") {
		t.Errorf("its prompt speaks of a pull request:\n%s", p)
	}

	// Cold, it's given the brief as what the work is meant to do.
	_, q, err := w.RunReviewPass(of, "quick")
	if err != nil {
		t.Fatal(err)
	}
	b := &answering{w: w, store: w.cells(), c: q, here: w.here(doc.Scope{}, q)}
	if stop, err := reviewPassReady(b); stop || err != nil {
		t.Fatalf("ready, cold: %v %v", stop, err)
	}
	p = reviewPassFirst(b)
	for _, want := range []string{"What the work is meant to do (the mission's brief)", "Key the cache by tenant.", "=== key.go (modified)", "+\treturn tenant + id", "aren't shown: go.sum (lockfile)."} {
		if !strings.Contains(p, want) {
			t.Errorf("its cold prompt lacks %q:\n%s", want, p)
		}
	}
	// The guide keeps what it was given while it writes, for its view to
	// say: the files it may place, and those it wasn't shown. Another cold
	// pass doesn't.
	if w.cells().GuideGiven(q.ID) != nil {
		t.Error("a cold pass that isn't the guide kept what it was given")
	}
	_, gc, err := w.RunReviewPass(of, page.Guide)
	if err != nil {
		t.Fatal(err)
	}
	if stop, err := reviewPassReady(&answering{w: w, store: w.cells(), c: gc, here: w.here(doc.Scope{}, gc)}); stop || err != nil {
		t.Fatalf("ready, the guide: %v %v", stop, err)
	}
	if given := w.cells().GuideGiven(gc.ID); given == nil || strings.Join(given.Files, " ") != "key.go go.sum" || len(given.Left) != 1 || given.Left[0] != (conversation.GuideLeft{Path: "go.sum", Why: review.SkimLockfile}) {
		t.Errorf("what the guide was given: %+v", given)
	}
	if w.cells().ClearRun(gc.ID); w.cells().GuideGiven(gc.ID) != nil {
		t.Error("what it was given outlived its run")
	}

	// What it found, taken: a comment in the mission's draft review.
	out := reviewPassRead(a, harness.Report{Output: "Not yet.\n\n## blocking\n@ key.go:4\nThe tenant and the id run together.\n\n## nit\nSay why in the commit.\n"}, nil)
	if out.err != nil || len(out.turns[0].Findings) != 2 {
		t.Fatalf("read: %+v", out)
	}
	c, _ = w.cells().Say(c.ID, out.turns[0])
	fs := c.Written().Findings
	def, _ := w.reviewPassNamed(of, "check")
	if st := page.ReviewStanding(def, c); st.Status != page.StandingNoGo || st.Blocking != 1 {
		t.Errorf("standing: %+v", st)
	}
	if _, _, err := w.TakeReviewFinding(of, "check", fs[0].Key); err != nil {
		t.Fatal(err)
	}
	if _, _, err := w.TakeReviewFinding(of, "check", fs[1].Key); err != nil {
		t.Fatal(err)
	}
	store, key := w.Reviews(of)
	draft, err := store.Draft(key)
	if err != nil || draft == nil || len(draft.Comments) != 2 {
		t.Fatalf("the draft: %+v %v", draft, err)
	}
	if n := draft.Comments[0]; n.Path != "key.go" || n.Line != 4 || n.Kind != "blocking" || n.By != conversation.Agent || n.Excerpt[0] != "\treturn tenant + id" {
		t.Errorf("on its line: %+v", n)
	}
	if n := draft.Comments[1]; !n.General() || n.Kind != "nit" {
		t.Errorf("on the change as a whole: %+v", n)
	}
	_, c, _ = w.ReviewPass(of, "check")
	if st := page.ReviewStanding(def, c); st.Status != page.StandingGo {
		t.Errorf("once both are taken: %+v", st)
	}
	// Crew read the one on no lines under the change as a whole.
	rv, err := store.Submit(key, review.Diffed{Dir: dir, BaseRef: "origin/main", Base: base, Head: head}, review.Changes, "", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if text := review.Render(rv); !strings.Contains(text, "## The change as a whole") || !strings.Contains(text, "Say why in the commit.") {
		t.Errorf("as crew read it:\n%s", text)
	}
}
