package mission

import (
	"strings"
	"testing"
	"time"
)

// A Wait says who it's on, and whether ws can watch for it: on you (yours
// to do), or on someone else; with checks (watched), or without.
func TestParsePlanReadsAWait(t *testing.T) {
	p, err := ParsePlan("## Z\n- Released.\n## Stops early\n- None.\n## Steps\n"+
		"1. Wait: **Release.** The user tags a release.\n   - on: you\n"+
		"2. Wait: **On npm.** The version is on npm.\n   - on: npm\n   - in: `ui-lib`\n   - check: `npm view pkg@1.0.0 version` passes\n   - check: `gh release view v1.0.0` passes\n"+
		"3. Wait: **Deploy.** The platform team deploys it.\n   - on: The platform team.\n"+
		"4. Wait: **Tag.** The tag is cut.\n   - on: You\n   - check: `git ls-remote --exit-code --tags origin v1.0.0` passes\n"+
		"5. Wait: Something happens.\n", nil)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		who, whom              string
		onYou, watched, unsaid bool
		checkpoint             bool
	}{
		{who: WhoYou, onYou: true, checkpoint: true},
		{who: WhoOutside, whom: "npm", watched: true},
		{who: WhoOutside, whom: "The platform team"},
		{who: WhoYou, onYou: true, watched: true, checkpoint: true},
		{who: WhoOutside, unsaid: true},
	}
	for i, c := range cases {
		st := p.Steps[i]
		if st.Type != StepWait || st.Who != c.who || st.Whom != c.whom || st.WaitOnYou() != c.onYou || st.Watched() != c.watched || st.WaitUnsaid() != c.unsaid || st.Checkpoint() != c.checkpoint {
			t.Errorf("step %d = %s on %q/%q: on you %v, watched %v, unsaid %v, checkpoint %v", st.N, st.Type, st.Who, st.Whom, st.WaitOnYou(), st.Watched(), st.WaitUnsaid(), st.Checkpoint())
		}
	}
	if st := p.Steps[1]; st.Capsule != "ui-lib" || len(st.Checks) != 2 || st.Checks[0].Command != "npm view pkg@1.0.0 version" || st.Checks[0].Expect != CheckPasses {
		t.Errorf("step 2's checks = %+v in %q", st.Checks, st.Capsule)
	}
	// A plan written before kinds: its who: is who it waits on, still.
	old, err := ParsePlan("## Z\n- x\n## Stops early\n- None.\n## Steps\n1. Merged.\n   - who: outside: a teammate's review\n2. Green.\n   - who: CI\n", nil)
	if err != nil {
		t.Fatal(err)
	}
	if st := old.Steps[0]; st.Type != StepWait || st.Whom != "a teammate's review" || st.WaitUnsaid() || st.Watched() {
		t.Errorf("a step from before kinds = %+v", st)
	}
	if st := old.Steps[1]; st.Type != StepWait || st.WaitUnsaid() {
		t.Errorf("CI's step from before kinds = %+v", st)
	}
}

// A Wait's check is a command: ws runs it to see whether it's happened,
// with nobody to judge words.
func TestParsePlanRefusesAWaitsCheckInWords(t *testing.T) {
	for written, want := range map[string]string{
		"1. Wait: The release is out.\n   - on: you\n   - check: the release is published\n": "but a Wait's check is a command",
		"1. Wait: The release is out.\n   - on:\n":                                           "on: is empty",
		"1. Wait: The release is out.\n   - makes: a release\n":                              "makes: does not apply to a Wait step, which takes - on:, - in:, - check:, - until:, - only if:",
	} {
		_, err := ParsePlan("## Z\n- x\n## Stops early\n- None.\n## Steps\n"+written, nil)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: %v, want it to say %q", written, err, want)
		}
	}
}

// ws looks at a watched step often at first, then less: what it waits for
// is likeliest to be about to happen when the mission's just got there.
func TestWatchLooksLessOftenAsItWaits(t *testing.T) {
	since := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	at := func(d time.Duration) *time.Time { t := since.Add(d); return &t }
	if next := (&Watch{Since: since}).NextLook(); !next.IsZero() {
		t.Errorf("before any look, the next is %v, want at once", next)
	}
	if next := (*Watch)(nil).NextLook(); !next.IsZero() {
		t.Errorf("before it's reached, the next is %v, want at once", next)
	}
	for _, c := range []struct{ looked, every time.Duration }{
		{0, time.Minute}, {10 * time.Minute, time.Minute}, {15 * time.Minute, 5 * time.Minute}, {time.Hour, 5 * time.Minute}, {2 * time.Hour, 15 * time.Minute}, {30 * time.Hour, 15 * time.Minute},
	} {
		w := &Watch{Since: since, Looked: at(c.looked)}
		if got := w.NextLook().Sub(*w.Looked); got != c.every {
			t.Errorf("looked %v in: next in %v, want %v", c.looked, got, c.every)
		}
	}
}

func TestWatchSummary(t *testing.T) {
	now := time.Now()
	for _, c := range []struct {
		results []CheckResult
		want    string
		met     bool
	}{
		{[]CheckResult{{Result: CheckMet}}, "met", true},
		{[]CheckResult{{Result: CheckUnmet}}, "not yet", false},
		{[]CheckResult{{Result: CheckMet}, {Result: CheckUnmet}}, "not yet (1 of 2 met)", false},
		{[]CheckResult{{Result: CheckCantTell}}, "could not tell", false},
	} {
		w := &Watch{Since: now, Looked: &now, Results: c.results}
		if got := w.Summary(); got != c.want || w.Met() != c.met {
			t.Errorf("%+v: %q, met %v; want %q, %v", c.results, got, w.Met(), c.want, c.met)
		}
	}
	if got := (&Watch{Since: now}).Summary(); got != "" {
		t.Errorf("before any look: %q", got)
	}
}

// Done marks a Wait done, alone or with what was done after it.
func TestSaysDone(t *testing.T) {
	for answer, want := range map[string]string{
		"Done": "", "done.": "", " DONE ": "", "Done: tagged v0.2.0": "tagged v0.2.0", "done, tagged v0.2.0": "tagged v0.2.0", "Done - it's v0.2.0, not v0.1.0": "it's v0.2.0, not v0.1.0",
	} {
		if note, ok := SaysDone(answer); !ok || note != want {
			t.Errorf("%q: %q, %v; want %q", answer, note, ok, want)
		}
	}
	for _, answer := range []string{"Not yet", "which version do I tag?", "donetsk", "I'm done with this", ""} {
		if note, ok := SaysDone(answer); ok {
			t.Errorf("%q marks it done (%q)", answer, note)
		}
	}
}

// A Wait's until is an event ws hears of, read as the plan is: one that
// isn't is refused, saying what there is.
func TestParsePlanReadsAWaitsUntil(t *testing.T) {
	parse := func(steps string) (*Plan, error) {
		return ParsePlan("## Z\n- x\n## Stops early\n- None.\n## Steps\n"+steps, nil)
	}
	p, err := parse("1. Wait: The API change merges.\n   - on: alice\n   - until: pr.merged pr=api#212\n")
	if err != nil {
		t.Fatal(err)
	}
	steps := p.Steps
	if c := steps[0].Checks; len(c) != 1 || !c[0].Heard() || c[0].Until != "pr.merged pr=api#212" || c[0].Run() || !steps[0].Watched() {
		t.Errorf("checks = %+v", c)
	}
	for plan, want := range map[string]string{
		"1. Wait: x\n   - until: pr.mergd pr=api#212\n":  `"pr.mergd" is not an event kind`,
		"1. Wait: x\n   - until: pr.merged colour=red\n": `"colour" is not a field of pr.merged events`,
		"1. Work: x\n   - until: pr.merged pr=api#1\n":   "until: does not apply to a Work step",
	} {
		if _, err := parse(plan); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: %v, want it to say %q", plan, err, want)
		}
	}
}
