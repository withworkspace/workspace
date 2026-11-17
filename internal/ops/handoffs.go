package ops

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/workspace"
)

// Handoffs: what crew leave for whoever comes next (mission.Handoff). Crew
// give one with ws mission done; new crew are given the latest ahead of
// the brief (leftOff), and the orchestrator the one just left.

// CanonicalReadFirst turns each "Read first" entry into the reference it
// resolves to, as ws mission artifact add would, so it still points at the
// same thing when read from somewhere else. The error names every entry
// it can't place.
func (w *Workspace) CanonicalReadFirst(m *mission.Mission, h *mission.Handoff, dir string) error {
	var bad []string
	for i, item := range h.ReadFirst {
		r, err := w.NormalizeArtifact(m, item.Ref, dir)
		if err != nil {
			bad = append(bad, fmt.Sprintf("%s: %v", item.Ref, err))
			continue
		}
		h.ReadFirst[i].Ref = r.String()
	}
	if len(bad) > 0 {
		return errors.New(`"Read first" names what ws can't find:` + "\n  " + strings.Join(bad, "\n  "))
	}
	return nil
}

// WhereToRead lists references with where to read each, now: a recorded
// artifact keeps its kind and label.
func WhereToRead(ws *workspace.Workspace, store mission.Store, m *mission.Mission, refs []string) string {
	if len(refs) == 0 {
		return ""
	}
	v := BuildMissionViewsFrom(ws, nil, []*mission.Mission{m})[0]
	var b strings.Builder
	for _, ref := range refs {
		a := mission.Artifact{Ref: ref, Kind: DefaultKind(ParseArtifactRef(ref))}
		for _, rec := range m.Artifacts {
			if rec.Ref == ref {
				a = rec
			}
		}
		b.WriteString("- " + ArtifactLine(ResolveArtifact(ws, store, v, a)) + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// HandoffLabel is the summary a crew's handoff was left with.
func HandoffLabel(m *mission.Mission, crewID string) string {
	for _, a := range m.Artifacts {
		if a.Ref == mission.HandoffRef(crewID) {
			return a.Label
		}
	}
	return ""
}

// LeftOff is where earlier crew left off, for new crew starting in
// capsule: the latest handoff from that capsule in full, or else the
// mission's latest, with where to read what it names; then where the
// other capsules' latest handoffs are. It's "" when no crew has left one.
func LeftOff(ws *workspace.Workspace, store mission.Store, m *mission.Mission, capsule string) string {
	latest := m.LatestHandoffs()
	if len(latest) == 0 {
		return ""
	}
	main := latest[0]
	for _, c := range latest {
		if c.Capsule == capsule {
			main = c
			break
		}
	}
	data, err := os.ReadFile(store.HandoffPath(m.Slug, main.ID))
	if err != nil {
		return ""
	}
	var b strings.Builder
	b.WriteString("## Where earlier crew left off\n\n")
	if main.Capsule == capsule {
		b.WriteString("The last crew in your capsule left this handoff:\n\n")
	} else {
		b.WriteString("The mission's latest handoff:\n\n")
	}
	b.WriteString(strings.TrimSpace(string(data)) + "\n")
	if h, err := mission.ParseHandoff(string(data)); err == nil && len(h.ReadFirst) > 0 {
		refs := make([]string, len(h.ReadFirst))
		for i, r := range h.ReadFirst {
			refs[i] = r.Ref
		}
		b.WriteString("\nWhere to read what it names:\n" + WhereToRead(ws, store, m, refs) + "\n")
	}
	var others []string
	for _, c := range latest {
		if c.ID == main.ID {
			continue
		}
		line := fmt.Sprintf("- %s, crew %s", firstNonEmpty(c.Capsule, "no capsule"), c.ID)
		if l := HandoffLabel(m, c.ID); l != "" {
			line += ": " + l
		}
		others = append(others, line+" — read: "+store.HandoffPath(m.Slug, c.ID))
	}
	if len(others) > 0 {
		b.WriteString("\nThe latest handoffs from the mission's other capsules:\n" + strings.Join(others, "\n") + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// ReadFirstList resolves what a dispatch names to read first (as typed,
// in dir) into lines for it, with where to read each.
func (w *Workspace) ReadFirstList(m *mission.Mission, reads []string, dir string) (string, error) {
	refs := make([]string, len(reads))
	for i, in := range reads {
		r, err := w.NormalizeArtifact(m, in, dir)
		if err != nil {
			return "", fmt.Errorf("--read %s: %w", in, err)
		}
		refs[i] = r.String()
	}
	return WhereToRead(w.WS(), w.Missions, m, refs), nil
}
