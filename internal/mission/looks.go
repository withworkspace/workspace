package mission

import (
	"slices"
	"strings"
)

// Look is one thing a step of yours looks at: a verification bundle, a
// capsule's diff, or a document made at the steps before it. A Review
// step looks at all three (Looks); a step whose checks are yours to say,
// at the bundles that show them (ChecksLooks).
type Look struct {
	Bundle  string // a bundle's ref (b3)
	Capsule string // repo/name
	Doc     string // an artifact's ref
	Name    string // what to call it: the bundle, the capsule, or the document's path
	// Read is a document to read beside the review, not to approve: one
	// the plan doesn't name, such as crew's notes and handoffs. What the
	// plan put under review is the work, and approving that passes the
	// step.
	Read bool
}

// Looks is what step n, a Review step, looks at: the capsules crew worked
// in at the steps since your last (back to the step before that's yours, or
// the planner's), and the documents made there. What the plan says is
// under review is yours to approve: those capsules, where its steps work,
// and a document it names, by its path, in the Review step or the steps
// it looks at. The other documents are to read (Read), after them. None
// (the findings are in words) leaves the crew's handoffs to read, on the
// mission's page.
func (m *Mission) Looks(states []StageState, n int) []Look {
	var steps []int
	for i := n - 1; i >= 1 && i <= len(states); i-- {
		st := states[i-1]
		if st.Step != nil && (st.Step.Who == WhoYou || st.Step.Who == WhoPlanner) {
			break
		}
		steps = append(steps, st.N)
	}
	slices.Reverse(steps)
	said := planSays(states, append([]int{n}, steps...))
	var out []Look
	seen := map[string]bool{}
	add := func(l Look) {
		key := l.Bundle + "|" + l.Capsule + "|" + l.Doc
		if !seen[key] {
			seen[key] = true
			out = append(out, l)
		}
	}
	// What was handed over to show the work comes first: it's the thing
	// to approve, with the work itself beside it.
	for _, s := range steps {
		for _, b := range m.StandingBundles(states[s-1].Text) {
			add(Look{Bundle: b.ID, Name: b.ID})
		}
	}
	for _, s := range steps {
		if r := states[s-1].Record; r != nil {
			for _, id := range r.Crew {
				if c, ok := m.CrewByID(id); ok && c.Capsule != "" && c.Role != RoleAnswer {
					add(Look{Capsule: c.Capsule, Name: c.Capsule})
				}
			}
		}
	}
	for _, a := range m.Artifacts {
		if !a.IsDocument() {
			continue
		}
		if st, ok := ArtifactStage(states, a); ok && slices.Contains(steps, st.N) {
			_, path, _ := strings.Cut(a.Ref, ":")
			add(Look{Doc: a.Ref, Name: path, Read: !strings.Contains(said, path)})
		}
	}
	// What's yours to approve first: the first look is the one to open.
	slices.SortStableFunc(out, func(a, b Look) int {
		switch {
		case a.Read == b.Read:
			return 0
		case b.Read:
			return -1
		}
		return 1
	})
	return out
}

// ChecksLooks is what shows step n's checks when they're yours to say
// (its checks task): the bundles handed over for the step that nothing
// has taken the place of. Approving them passes the step, as passing it
// as it is does.
func (m *Mission) ChecksLooks(states []StageState, n int) []Look {
	if n < 1 || n > len(states) {
		return nil
	}
	var out []Look
	for _, b := range m.StandingBundles(states[n-1].Text) {
		out = append(out, Look{Bundle: b.ID, Name: b.ID})
	}
	return out
}

// planSays is what the plan says of steps ns: each step's text, what it
// makes and its checks, where a document it puts under review is named.
func planSays(states []StageState, ns []int) string {
	var b strings.Builder
	for _, n := range ns {
		if n < 1 || n > len(states) || states[n-1].Step == nil {
			continue
		}
		st := states[n-1].Step
		b.WriteString(st.Text + "\n" + st.Makes + "\n")
		for _, c := range st.Checks {
			b.WriteString(c.Text + "\n")
		}
	}
	return b.String()
}

// IsDocument reports whether the artifact is a document, to read and
// comment on: a file (in a capsule, on a repo's default branch, or the
// mission's own), not a pull request or a link.
func (a Artifact) IsDocument() bool {
	switch a.Kind {
	case "pr", "issue", "link":
		return false
	}
	where, path, ok := strings.Cut(a.Ref, ":")
	return ok && path != "" && !strings.HasPrefix(path, "//") && where != ""
}
