package prompt

import (
	"testing"
)

// The judge is told the step, its checks, what to answer with, and what
// shows the work: the handoff, what ws ran, the documents and the diff.
func TestJudge_Golden(t *testing.T) {
	golden(t, "judge", Judge(Judging{
		Mission: "invoice-penny", Title: "Invoice totals off by a penny",
		Step: "3: Work: reproduce it as a failing test", Makes: "the failing test", Goal: "Find why invoice totals are off by a penny.",
		Checks: []string{"the test exercises the cause in the findings", "the findings say where the rounding happens"},
		Crew:   "c2", Capsule: "api/invoice-penny", Commit: "0123456789abcdef0123", Dir: "/ws/repos/api/invoice-penny",
		Handoff:   "## Done\nAdded TestTotalRounding.\n",
		Diff:      " invoice/total_test.go | 12 ++++++++++++\n\n+func TestTotalRounding(t *testing.T) {",
		Ran:       []string{"`go test ./invoice -run TestTotalRounding` fails: met (exited 1)"},
		Artifacts: []string{"`mission:findings.md`: the findings (c1)"},
	}))
}

// The judge's answer is read a line a check, in any order, its words
// going on over lines; what it said nothing of is can't tell.
func TestReadVerdicts(t *testing.T) {
	got := ReadVerdicts("Here's my judgement:\n```\n2. **unmet**: the test asserts the total,\n   not where it rounds.\n1. met: invoice/total_test.go:12 calls Total()\n4. met: nothing\n```\n", 3)
	want := []Verdict{
		{"met", "invoice/total_test.go:12 calls Total()"},
		{"unmet", "the test asserts the total, not where it rounds."},
		{"can't tell", "the judge said nothing of it"},
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("check %d = %+v, want %+v", i+1, got[i], want[i])
		}
	}
	if v := ReadVerdicts("1. Can't tell — it needs the staging data", 1); v[0].Result != "can't tell" || v[0].Why != "it needs the staging data" {
		t.Errorf("can't tell: %+v", v[0])
	}
	// One that corrects itself gives its answer last.
	said := "1. unmet: there's no VERSION file. But d1 drops it, so it's met as d1 leaves it.\n\nCorrected answer, a single line as asked:\n\n1. met: main.go:5 has greeting(), and the VERSION file isn't asked, as d1 has it."
	if v := ReadVerdicts(said, 1); v[0].Result != "met" || v[0].Why != "main.go:5 has greeting(), and the VERSION file isn't asked, as d1 has it." {
		t.Errorf("corrected: %+v", v[0])
	}
}

// A verifier is told its step and its checks (which are its own, which ws
// runs and which are the user's), where the work is and who made it, to
// change nothing, and to hand over a bundle; and, when the work came back,
// what went back before.
func TestVerifier_Golden(t *testing.T) {
	golden(t, "verifier", Verifier(Verifying{
		Slug: "accent-picker", Title: "One accent colour", Goal: "Let the user pick one accent colour.",
		StepN: 5, StepOf: 7, Step: "**Browser check.** The picker, running, shown with the shots script.", Limit: "30m",
		Checks:  []VerifyCheck{{Text: "a pick applies at once, light and dark"}, {Text: "`npm test` passes", Command: true}, {Text: "it reads well at phone width", Yours: true}},
		Capsule: "web/accent-picker", Commit: "3f9c2ab1c0de4f5a6b7c", Worker: "c3", Handoff: "mission:handoffs/c3.md",
		Before: "b2", Unmet: []string{"check 1 (a pick applies at once, light and dark): dark kept the old accent"},
		Files: "/ws/.ws/missions/accent-picker/files", Guide: "The workspace's guide for agents is at /ws/AGENTS.md: read it first.",
	}))
}
