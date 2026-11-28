package prompt

import (
	"strings"
	"testing"
)

// The decision look is told the step's checks and the user's decisions,
// and its answer is read a decision at a line: what one goes against, or
// that it fits, which is also what it is when the look said nothing of it.
func TestDecisionLook(t *testing.T) {
	p := LookAtDecisions(DecisionLook{Mission: "refs", Title: "Refs", Step: "3: Work: mission refs", Checks: []string{"each kind has a chip", "nothing new reads git on a request path"},
		Yours: []string{"d1: Capsules are written repo/capsule."}, Decisions: []string{"d3 (c1): A capsule's pin reads its branch from git."}})
	for _, want := range []string{"2. nothing new reads git on a request path", "- d1: Capsules are written repo/capsule.", "- d3 (c1): A capsule's pin", "their step's checks, or a decision the user made", "Where you aren't sure, it fits"} {
		if !strings.Contains(p, want) {
			t.Errorf("prompt missing %q:\n%s", want, p)
		}
	}
	got := ReadDecisionFindings("```\nd3: check 2: it runs git rev-parse on every pin\n**d4**: fits\n- d5 — d1: writes capsules as repo:capsule\n```", []string{"d3", "d4", "d5", "d6"})
	want := []DecisionFinding{{"d3", "check 2", "it runs git rev-parse on every pin"}, {"d4", "", ""}, {"d5", "d1", "writes capsules as repo:capsule"}, {"d6", "", ""}}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("finding %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}
