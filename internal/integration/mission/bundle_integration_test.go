package mission_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/withworkspace/workspace/internal/cli"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/testutil"
)

// Crew hand over a verification bundle for the step they're on: ws reads
// it against the step's checks, keeps its files in a folder of its own,
// and shows it by its ref, from its step, and in the mission.
func TestBundle_HandedOverAndShown(t *testing.T) {
	w, store := testutil.OrchestrateFixture(t)
	testutil.WithStdin(t, workPlan)
	testutil.Run(t, w, nil, "mission", "plan", "-m", "cache", "--set", "-")
	store.JoinStage("cache", "c1", 1)
	proof := filepath.Join(store.FilesDir("cache"), "proof")
	os.MkdirAll(proof, 0o755)
	os.WriteFile(filepath.Join(proof, "hits.txt"), []byte("lookup 1: miss\nlookup 2: hit\n"), 0o644)
	os.WriteFile(filepath.Join(proof, "second.png"), []byte("\x89PNG\r\n"), 0o644)

	// Only crew hand one over.
	testutil.WithStdin(t, "## Summary\nx\n")
	if r := testutil.RunCommand(t, w.Root, nil, "mission", "bundle", "-m", "cache", "--set", "-"); r.Err == nil || !strings.Contains(r.Err.Error(), "this is for crew") {
		t.Errorf("handed over by no crew: %v", r.Err)
	}
	t.Setenv(mission.EnvMission, "cache")
	t.Setenv(mission.EnvCrew, "c1")

	// Crew are told how one's written, with their step's checks by number.
	out := testutil.Run(t, w, nil, "mission", "bundle", "--format").Stdout
	for _, want := range []string{"## Not verified", "Your step (s1) has these checks, by number:", "1. `test -f cache.go` passes (ws runs it: leave it out)", "2. the lookup is cached (yours to show)"} {
		if !strings.Contains(out, want) {
			t.Errorf("--format lacks %q:\n%s", want, out)
		}
	}

	// One that isn't right is refused, with every problem and its line.
	testutil.WithStdin(t, "## Summary\nCached.\n## Checks\n1. met: the file's there\n2. met: a second lookup is a hit\n   - shows: `proof/gone.png` the second lookup\n")
	r := testutil.RunCommand(t, w.Root, nil, "mission", "bundle", "--set", "-")
	if r.Err == nil {
		t.Fatal("a bundle that isn't right was kept")
	}
	for _, want := range []string{"line 4: check 1 is a command", "line 6: shows: proof/gone.png: there's no such file", "it has no ## Not verified", "ws mission bundle --format says how one is written"} {
		if !strings.Contains(r.Err.Error(), want) {
			t.Errorf("want %q in:\n%s", want, r.Err)
		}
	}

	testutil.WithStdin(t, "## Summary\nA second lookup of the same key doesn't reach the server.\n\n## Try it\n- try: `go run ./cmd/lookup a a`\n- look for: one miss, then a hit\n\n"+
		"## Checks\n2. met: looked up the same key twice; the second was a hit.\n   - shows: `proof/hits.txt` What two lookups printed\n     - made by: `go run ./cmd/lookup a a > hits.txt`\n   - shows: `proof/second.png` The second lookup, in the page\n   - lines: `cache.go:10-24`\n\n"+
		"## Not verified\n- Eviction: nothing fills the cache here.\n")
	r = testutil.Run(t, w, nil, "mission", "bundle", "--set", "-")
	if !strings.Contains(r.Stderr, "b1 kept for step 1: 1 met") || !strings.Contains(r.Stderr, "it's of backend/cache-lookup at ") {
		t.Errorf("handing over said: %s", r.Stderr)
	}
	kept := store.BundleDir("cache", "b1")
	if data, err := os.ReadFile(filepath.Join(kept, "hits.txt")); err != nil || !strings.Contains(string(data), "lookup 2: hit") {
		t.Errorf("its text isn't kept: %q, %v", data, err)
	}
	m := testutil.LoadMission(t, w, "cache")
	if len(m.Bundles) != 1 || m.Bundles[0].Commit == "" || m.Bundles[0].Checks[0].Shows[0].MadeBy == "" {
		t.Fatalf("bundles = %+v", m.Bundles)
	}

	t.Setenv(mission.EnvCrew, "")
	show := func(args ...string) string {
		return testutil.Run(t, w, nil, append([]string{"show", "-m", "cache", "-f", "llm"}, args...)...).Stdout
	}
	out = show("b1")
	for _, want := range []string{
		"Bundle b1 (1 met): of step 1, cache the lookup",
		"by c1 (cursor), who did the work it shows, of backend/cache-lookup at ",
		"A second lookup of the same key doesn't reach the server.",
		"- `go run ./cmd/lookup a a`", "- look for: one miss, then a hit",
		"2. met: the lookup is cached", "looked up the same key twice; the second was a hit.",
		"shows: " + filepath.Join(kept, "hits.txt") + " (text): What two lookups printed",
		"made by: `go run ./cmd/lookup a a > hits.txt`",
		filepath.Join(kept, "second.png") + " (an image)", "lines: cache.go:10-24",
		"Not verified:\n- Eviction: nothing fills the cache here.",
		"s1 ", "the step it's of", "c1 ", "handed it over", "backend/cache-lookup", "the capsule it's of",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("ws show b1 lacks %q:\n%s", want, out)
		}
	}
	if out := show("s1"); !strings.Contains(out, "a bundle of it, by c1: 1 met") {
		t.Errorf("ws show s1 doesn't link its bundle:\n%s", out)
	}
	if out := show(); !strings.Contains(out, "Bundles (what was handed over") || !strings.Contains(out, "b1 step 1 (cache the lookup), by c1: 1 met") {
		t.Errorf("the mission doesn't list its bundle:\n%s", out)
	}
	if out := testutil.Run(t, w, nil, "mission", "bundle", "-m", "cache").Stdout; !strings.Contains(out, "b1 step 1 (cache the lookup), by c1: 1 met") {
		t.Errorf("ws mission bundle: %s", out)
	}
	if r := testutil.RunCommand(t, w.Root, nil, "show", "-m", "cache", "b7"); r.Err == nil || !strings.Contains(r.Err.Error(), "has no bundle b7") {
		t.Errorf("a bundle it hasn't: %v", r.Err)
	}

	// A bundle is of a commit: a capsule with changes not committed has to
	// commit them first.
	readme := filepath.Join(w.Root, "repos", "backend", "cache-lookup", "README.md")
	if err := os.WriteFile(readme, []byte("hello, cached\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(mission.EnvCrew, "c1")
	testutil.WithStdin(t, "## Summary\nx\n## Checks\n2. unmet: it isn't cached\n## Not verified\nNothing.\n")
	if r := testutil.RunCommand(t, w.Root, nil, "mission", "bundle", "--set", "-"); r.Err == nil || !strings.Contains(r.Err.Error(), "a bundle is of a commit") {
		t.Errorf("handed over with changes not committed: %v", r.Err)
	}
}

// A bundle is yours to approve or send back, from a shell too: approving
// one a Review step looks at passes that step; sending one back says what's
// not right of a check, and it's kept on the bundle.
func TestBundle_ApprovedAndSentBack(t *testing.T) {
	w, store := testutil.OrchestrateFixture(t)
	testutil.WithStdin(t, "## Z\n- Cached.\n## Stops early\n- None.\n## Steps\n1. Work: cache the lookup.\n   - in: `backend`\n2. Review: the cache.\n3. Plan\n")
	testutil.Run(t, w, nil, "mission", "plan", "-m", "cache", "--set", "-")
	store.JoinStage("cache", "c1", 1)
	proof := filepath.Join(store.FilesDir("cache"), "hits.txt")
	os.MkdirAll(filepath.Dir(proof), 0o755)
	os.WriteFile(proof, []byte("hit\n"), 0o644)
	bundle := "## Summary\nCached.\n## Checks\n1. met: a second lookup is a hit\n   - shows: `hits.txt` What it printed\n2. can't tell: whether it evicts\n## Not verified\nNothing.\n"
	t.Setenv(mission.EnvMission, "cache")
	t.Setenv(mission.EnvCrew, "c1")
	testutil.WithStdin(t, bundle)
	testutil.Run(t, w, nil, "mission", "bundle", "--set", "-")
	// Crew don't approve their own.
	if r := testutil.RunCommand(t, w.Root, nil, "mission", "bundle", "approve", "b1"); r.Err == nil || !strings.Contains(r.Err.Error(), "a bundle is the user's to approve or send back") {
		t.Errorf("crew approved it: %v", r.Err)
	}
	t.Setenv(mission.EnvCrew, "")
	store.SetCrewStatus("cache", "c1", mission.CrewDone, "cached")
	testutil.Run(t, w, nil, "mission", "accept", "cached")

	out := testutil.Run(t, w, nil, "show", "-f", "llm", "b1").Stdout
	if !strings.Contains(out, "It waits for the user: step 2 looks at it") {
		t.Errorf("ws show b1 doesn't say it waits:\n%s", out)
	}
	if out := testutil.Run(t, w, nil, "mission", "bundle").Stdout; !strings.Contains(out, "waiting for the user (step 2 looks at it)") {
		t.Errorf("ws mission bundle: %s", out)
	}

	// Sent back, with what's not right of a check and of it as a whole.
	if r := testutil.RunCommand(t, w.Root, nil, "mission", "bundle", "send-back", "b1", "--check", "evicts?"); r.Err == nil || !strings.Contains(r.Err.Error(), "say the check's number, then what's not right") {
		t.Errorf("a note of no check: %v", r.Err)
	}
	r := testutil.Run(t, w, nil, "mission", "bundle", "send-back", "b1", "--check", "2: fill it and see", "and", "say", "how", "big", "it", "gets")
	if !strings.Contains(r.Stderr, "Sent b1 back: ") {
		t.Errorf("sending back said: %s", r.Stderr)
	}
	out = testutil.Run(t, w, nil, "show", "-f", "llm", "b1").Stdout
	for _, want := range []string{"The user sent it back ", "- check 2: fill it and see", "- of it as a whole: and say how big it gets"} {
		if !strings.Contains(out, want) {
			t.Errorf("ws show b1 lacks %q:\n%s", want, out)
		}
	}
	if m := testutil.LoadMission(t, w, "cache"); m.Bundles[0].Verdict != mission.BundleSentBack || len(m.Bundles[0].Notes) != 2 {
		t.Errorf("bundle = %+v", m.Bundles[0])
	}

	// The one that comes back takes its place, and approving it passes
	// the Review.
	t.Setenv(mission.EnvCrew, "c1")
	testutil.WithStdin(t, bundle)
	testutil.Run(t, w, nil, "mission", "bundle", "--set", "-")
	t.Setenv(mission.EnvCrew, "")
	if r := testutil.RunCommand(t, w.Root, nil, "mission", "bundle", "approve", "b1"); r.Err == nil || !strings.Contains(r.Err.Error(), "b2 has taken b1's place") {
		t.Errorf("approved one that's replaced: %v", r.Err)
	}
	r = testutil.Run(t, w, nil, "mission", "bundle", "approve", "b2")
	if !strings.Contains(r.Stderr, "Approved b2: step 2 passed") {
		t.Errorf("approving said: %s", r.Stderr)
	}
	if out := testutil.Run(t, w, nil, "mission", "bundle").Stdout; !strings.Contains(out, "b2 step 1 (cache the lookup), by c1: 1 met, 1 can't tell, approved by you") || !strings.Contains(out, "b1 step 1 (cache the lookup), by c1: 1 met, 1 can't tell, replaced by b2") {
		t.Errorf("ws mission bundle: %s", out)
	}
}

// A verifier, crew at a Verify step, is told which of its step's checks
// are whose; its done is its bundle, not a handoff; and handing it over
// settles the step.
func TestBundle_AVerifiersIsItsDone(t *testing.T) {
	w, store := testutil.OrchestrateFixture(t)
	_, restore := cli.CaptureOrchestratorRuns() // the step's checks run here, as the daemon would run them
	defer restore()
	testutil.WithStdin(t, "## Z\n- Cached.\n## Stops early\n- None.\n## Steps\n1. Work: cache the lookup.\n   - in: `backend`\n"+
		"2. Verify: the cache, running.\n   - in: `backend`\n   - check: a second lookup is a hit\n   - check: `test -f README.md` passes\n   - yours: the page reads well\n3. Review: the cache.\n")
	testutil.Run(t, w, nil, "mission", "plan", "-m", "cache", "--set", "-")
	store.JoinStage("cache", "c1", 1)
	store.SetCrewStatus("cache", "c1", mission.CrewDone, "cached")
	t.Setenv(mission.EnvMission, "cache")
	testutil.Run(t, w, nil, "mission", "accept", "cached")

	out := testutil.Run(t, w, nil, "show", "-f", "llm", "s2").Stdout
	for _, want := range []string{"A Verify step. Who: a verifier, crew who didn't do the work, in `backend`", "It shows whether these are true", "3. the page reads well (yours)"} {
		if !strings.Contains(out, want) {
			t.Errorf("ws show s2 lacks %q:\n%s", want, out)
		}
	}
	// Its verifier, as ws would have started it.
	store.StartCrew("cache", "cursor", "conv-2")
	store.SetCrewProcess("cache", "c2", 0, "backend/cache-lookup")
	store.JoinStage("cache", "c2", 2)
	proof := filepath.Join(store.FilesDir("cache"), "hits.txt")
	os.MkdirAll(filepath.Dir(proof), 0o755)
	os.WriteFile(proof, []byte("hit\n"), 0o644)

	t.Setenv(mission.EnvCrew, "c2")
	testutil.WithStdin(t, "## Done\n- x\n## Learned\n- x\n## Left\n- Nothing.\n## Read first\n- Nothing.\n")
	if r := testutil.RunCommand(t, w.Root, nil, "mission", "done", "verified", "--handoff", "-"); r.Err == nil || !strings.Contains(r.Err.Error(), "you're verifying this step: your done is your bundle") {
		t.Errorf("a verifier reported done with a handoff: %v", r.Err)
	}
	out = testutil.Run(t, w, nil, "mission", "bundle", "--format").Stdout
	for _, want := range []string{"1. a second lookup is a hit (yours to show)", "2. `test -f README.md` passes (ws runs it: leave it out)", "3. the page reads well (the user's to settle: say yours:"} {
		if !strings.Contains(out, want) {
			t.Errorf("--format lacks %q:\n%s", want, out)
		}
	}
	testutil.WithStdin(t, "## Summary\nLooked it up twice.\n## Checks\n1. met: the second was a hit\n   - shows: `hits.txt` What it printed\n3. yours: the page, as it is\n   - try: open the page\n## Not verified\nNothing.\n")
	r := testutil.Run(t, w, nil, "mission", "bundle", "--set", "-")
	for _, want := range []string{"b1 kept for step 2: 1 met, 1 yours", "ws runs step 2's checks", "that's your part done: the bundle is your handoff"} {
		if !strings.Contains(r.Stderr, want) {
			t.Errorf("handing over lacks %q:\n%s", want, r.Stderr)
		}
	}
	t.Setenv(mission.EnvCrew, "")
	m := testutil.LoadMission(t, w, "cache")
	if b := m.Bundles[0]; b.Own || b.Step != 2 {
		t.Errorf("bundle = %+v", b)
	}
	// ws ran the step's command, took the rest from the bundle, and passed
	// the step: what's the user's is left for them, at the Review.
	states, _ := m.StageProgress("")
	if st := states[1]; !st.Accepted() || st.Record.AcceptedBy != "ws" {
		t.Fatalf("step 2 isn't passed: %+v", st.Record)
	}
	out = testutil.Run(t, w, nil, "show", "-f", "llm", "b1").Stdout
	for _, want := range []string{"Bundle b1 (1 met, 1 yours): of step 2", "by c2 (cursor), of backend/cache-lookup at ", "3. yours: the page reads well", "- `test -f README.md` passes: met, by ws", "It waits for the user: step 3 looks at it"} {
		if !strings.Contains(out, want) {
			t.Errorf("ws show b1 lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "who did the work it shows") {
		t.Errorf("a verifier's bundle says it's the author's own:\n%s", out)
	}
}
