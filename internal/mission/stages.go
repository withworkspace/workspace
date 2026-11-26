package mission

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"
)

// Stages. A mission's route is its plan's steps (plan.md, plan.go), or
// for a mission without a plan, its brief's "## Stages": a numbered list,
// written by the user or a briefing agent, each item one stage for
// separate crew to do in turn. Either way ws keeps the progress
// (StageRecord) in the mission's record: which crew worked on a stage or
// step, when it started and when it was accepted. A brief without stages
// is one stage, the whole brief.

// Stage is one stage of the plan, as the brief lists it.
type Stage struct {
	N    int    // its position, from 1
	Text string // the item as written, without its number
	// CheckIn: the item says to check in with the user before the next
	// stage starts.
	CheckIn bool
	// Implicit: the brief lists no stages, so this is the whole brief.
	Implicit bool
	// Step is the plan's step this is, for a mission with a plan; nil for
	// the brief's stage.
	Step *Step
}

// Route is the mission's route, as stages: its plan's steps, or without a
// plan, its brief's stages (ParseStages).
func (m *Mission) Route(brief string) []Stage {
	if m.Plan != nil && len(m.Plan.Steps) > 0 {
		return m.Plan.Stages()
	}
	return ParseStages(brief)
}

// Stages is the plan's steps, as the stages ws keeps progress of.
func (p *Plan) Stages() []Stage {
	out := make([]Stage, len(p.Steps))
	for i := range p.Steps {
		st := p.Steps[i]
		out[i] = Stage{N: st.N, Text: st.Text, Step: &st}
	}
	return out
}

// Who is who's on the stage: the step's, or crew for a brief's stage.
func (s Stage) Who() string {
	if s.Step != nil {
		return s.Step.Who
	}
	return WhoCrew
}

// Noun is what the route's parts are called: steps of a plan, or stages
// of a brief.
func (s Stage) Noun() string {
	if s.Step != nil {
		return "step"
	}
	return "stage"
}

// Whole is the stage as crew are given it: a step says what it makes too,
// what has to be true for it to be done (its checks), and how long they
// have.
func (s Stage) Whole() string {
	if s.Step == nil {
		return s.Text
	}
	out := s.Text
	if s.Step.Makes != "" {
		out += "\n\nIt makes: " + strings.TrimSuffix(s.Step.Makes, ".") + "."
	}
	switch {
	case s.Step.Type == StepVerify:
		out += "\n\nIt shows whether these are true (ws runs the commands itself; one marked yours is the user's to settle, with what's left to help them):\n"
		for i, c := range s.Step.Checks {
			out += fmt.Sprintf("\n%d. %s", i+1, c.Text)
			if c.Yours {
				out += " (yours)"
			}
		}
	case len(s.Step.Checks) > 0:
		out += "\n\nIt's done when these are true, which ws checks once you report done (the commands it runs in your capsule; the rest an agent who didn't do the work judges):\n"
		for i, c := range s.Step.Checks {
			out += fmt.Sprintf("\n%d. %s", i+1, c.Text)
		}
	}
	if s.Step.LimitText != "" {
		out += "\n\nIts limit: " + strings.TrimSuffix(s.Step.LimitText, ".") + ". ws holds the clock from when you start; ws mission limit <45m> sets yourself less time."
	}
	return out
}

// StageRecord is ws's record of a stage's progress. It's matched to its
// stage by text, or by position while the number of stages is unchanged
// (see stageRecordsFor), so rewording a stage keeps its progress.
type StageRecord struct {
	Text string `toml:"text" json:"text"`
	// N and Of are the stage's position and the number of stages when it
	// was last matched.
	N          int        `toml:"n" json:"n"`
	Of         int        `toml:"of" json:"of"`
	Crew       []string   `toml:"crew,omitempty" json:"crew,omitempty"`
	Started    *time.Time `toml:"started,omitempty" json:"started,omitempty"`
	Accepted   *time.Time `toml:"accepted,omitempty" json:"accepted,omitempty"`
	AcceptedBy string     `toml:"accepted_by,omitempty" json:"accepted_by,omitempty"`
	Why        string     `toml:"why,omitempty" json:"why,omitempty"` // why it was accepted
	// Checks are the step's checks, each round they were run and judged
	// (once a round its crew reported done), latest last.
	Checks []CheckRound `toml:"checks,omitempty" json:"checks,omitempty"`
	// VerifyHead, for a Verify step, is the commit its capsule was at when
	// its verifier last started: a verifier changes nothing, so its bundle
	// is of that commit.
	VerifyHead string `toml:"verify_head,omitempty" json:"verify_head,omitempty"`
	// Deadline is when the step's limit runs out: from when crew first
	// started on it, or when it was last given more time. OutOfTime is
	// when ws said it had.
	Deadline  *time.Time `toml:"deadline,omitempty" json:"deadline,omitempty"`
	OutOfTime *time.Time `toml:"out_of_time,omitempty" json:"out_of_time,omitempty"`
	// Rounds is how many times an Open Pull Request step's pull request
	// has come back for fixing: its checks gone red, or a conflict.
	Rounds int `toml:"rounds,omitempty" json:"rounds,omitempty"`
	// Go is when a step that waits for someone's call to start (it
	// applies only if, or runs in each of several repos) was started, and
	// GoBy who by.
	Go   *time.Time `toml:"go,omitempty" json:"go,omitempty"`
	GoBy string     `toml:"go_by,omitempty" json:"go_by,omitempty"`
	// Skipped: passed without being done, its only if not holding (Why
	// says why).
	Skipped bool `toml:"skipped,omitempty" json:"skipped,omitempty"`
	// Attempts is how many times the step was started again, fresh.
	Attempts int `toml:"attempts,omitempty" json:"attempts,omitempty"`
	// Instances are the step's runs, one per repo, for a step that runs
	// in each of several.
	Instances []StepInstance `toml:"instances,omitempty" json:"instances,omitempty"`
	// Watch, for a Wait step, is since when it's been waited for, and what
	// ws's last look at its checks came to (wait.go).
	Watch *Watch `toml:"watch,omitempty" json:"watch,omitempty"`
}

// StageState is a stage with its progress.
type StageState struct {
	Stage
	Record  *StageRecord // nil until crew start on it or it's accepted
	Current bool         // the first stage not yet accepted
}

// Accepted reports whether the stage has been accepted.
func (s StageState) Accepted() bool { return s.Record != nil && s.Record.Accepted != nil }

var (
	stagesHeading = regexp.MustCompile(`(?i)^(#{1,6})\s+stages\s*:?\s*#*\s*$`)
	anyHeading    = regexp.MustCompile(`^(#{1,6})\s`)
	stageItem     = regexp.MustCompile(`^(?:\d+[.)]|[-*+])\s+(.*)$`)
	checkIn       = regexp.MustCompile(`(?i)\bcheck[\s-]?in\b`)
)

// ParseStages reads the brief's stages: the list under a "Stages"
// heading, numbered or bulleted, with each item's indented lines joined
// to it. It always returns at least one: without a list, the whole brief
// is one implicit stage.
func ParseStages(brief string) []Stage {
	var stages []Stage
	in, level := false, 0
	for line := range strings.SplitSeq(strings.ReplaceAll(brief, "\r\n", "\n"), "\n") {
		if g := stagesHeading.FindStringSubmatch(line); g != nil {
			in, level = true, len(g[1])
			continue
		}
		if !in {
			continue
		}
		if g := anyHeading.FindStringSubmatch(line); g != nil && len(g[1]) <= level {
			break
		}
		if g := stageItem.FindStringSubmatch(line); g != nil && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			stages = append(stages, Stage{N: len(stages) + 1, Text: strings.TrimSpace(g[1])})
			continue
		}
		if t := strings.TrimSpace(line); t != "" && len(stages) > 0 {
			st := &stages[len(stages)-1]
			st.Text += "\n" + t
		}
	}
	for i := range stages {
		stages[i].CheckIn = checkIn.MatchString(stages[i].Text)
	}
	if len(stages) == 0 {
		return []Stage{{N: 1, Implicit: true}}
	}
	return stages
}

// Title is the stage's first line, for lists.
func (s Stage) Title() string {
	if s.Implicit {
		return "the whole brief"
	}
	first, _, _ := strings.Cut(s.Text, "\n")
	return first
}

// ShortMax is the longest a stage's short form gets.
const ShortMax = 60

var (
	boldLead    = regexp.MustCompile(`^(?:\*\*|__)(.+?)(?:\*\*|__)`)
	sentenceEnd = regexp.MustCompile(`[.:;!?](?:\s|$)`)
	trailParens = regexp.MustCompile(`\s*\([^()]*\)\s*$`)
	mdLink      = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	mdEmphasis  = regexp.MustCompile("\\*\\*|__|`")
)

// Short is the stage in a few words, for lists: the bold lead it starts
// with, a plan step's name ("**Proposal (in `records/x`).** Write…" is
// "Proposal"), else its first sentence, without markdown or a closing
// aside in brackets, and cut at ShortMax. A step written as its kind
// alone is its kind.
func (s Stage) Short() string {
	if s.Implicit {
		return s.Title()
	}
	if s.Step != nil && s.Step.Bare {
		return s.Step.Type
	}
	t := strings.TrimSpace(s.Title())
	if g := boldLead.FindStringSubmatch(t); g != nil {
		t = g[1]
	} else if loc := sentenceEnd.FindStringIndex(t); loc != nil {
		t = t[:loc[0]]
	}
	t = mdEmphasis.ReplaceAllString(mdLink.ReplaceAllString(t, "$1"), "")
	t = strings.TrimRight(strings.TrimSpace(t), ".:;")
	if short := trailParens.ReplaceAllString(t, ""); short != "" {
		t = short
	}
	if r := []rune(t); len(r) > ShortMax {
		// At a word, when there's one in the second half.
		cut := string(r[:ShortMax-1])
		if i := strings.LastIndex(cut, " "); i > len(cut)/2 {
			cut = cut[:i]
		}
		t = strings.TrimRight(cut, " ,;:") + "…"
	}
	return t
}

// Plain is the stage's text on one line without markdown, for showing whole.
func (s Stage) Plain() string {
	if s.Implicit {
		return s.Title()
	}
	return strings.Join(strings.Fields(mdEmphasis.ReplaceAllString(mdLink.ReplaceAllString(s.Text, "$1"), "")), " ")
}

var capsuleToken = regexp.MustCompile("`([A-Za-z0-9][A-Za-z0-9._-]*/[A-Za-z0-9._-]+)`")

// NamedCapsules is every repo/name the stage names in backticks, the way
// briefs name capsules: a step's own capsule first.
func (s Stage) NamedCapsules() []string {
	var out []string
	if s.Step != nil && s.Step.Capsule != "" {
		out = append(out, s.Step.Capsule)
	}
	for _, g := range capsuleToken.FindAllStringSubmatch(s.Text, -1) {
		out = append(out, g[1])
	}
	return out
}

// CapsuleIn is the first of caps the stage names, as repo/name, or "".
func (s Stage) CapsuleIn(caps []Capsule) string {
	for _, c := range caps {
		name := c.Repo + "/" + c.Name
		for _, ok := range []string{"`" + name + "`", " " + name + " ", " " + name + ".", " " + name + ","} {
			if strings.Contains(" "+s.Text+" ", ok) {
				return name
			}
		}
	}
	return ""
}

// normStage is a stage's text as matching compares it: case, spacing,
// markdown emphasis and a closing full stop don't count.
func normStage(s string) string {
	s = strings.NewReplacer("*", "", "_", "", "`", "").Replace(strings.ToLower(s))
	return strings.TrimRight(strings.Join(strings.Fields(s), " "), ".")
}

// stageRecordsFor matches records to stages: by text first, then any
// record still unmatched to the stage at its position, if the number of
// stages is what it was. It returns each stage's record index (-1 for
// none) and the records that match no stage.
func stageRecordsFor(stages []Stage, recs []StageRecord) ([]int, []int) {
	byStage := make([]int, len(stages))
	used := make([]bool, len(recs))
	for i := range byStage {
		byStage[i] = -1
	}
	for i, st := range stages {
		for j, r := range recs {
			if !used[j] && normStage(r.Text) == normStage(st.Text) {
				byStage[i], used[j] = j, true
				break
			}
		}
	}
	for j, r := range recs {
		if i := r.N - 1; !used[j] && r.Of == len(stages) && i >= 0 && i < len(stages) && byStage[i] < 0 {
			byStage[i], used[j] = j, true
		}
	}
	var orphans []int
	for j := range recs {
		if !used[j] {
			orphans = append(orphans, j)
		}
	}
	return byStage, orphans
}

// StageProgress is the brief's stages with their progress, and the
// records that no longer match a stage (it was removed or rewritten).
func (m *Mission) StageProgress(brief string) ([]StageState, []StageRecord) {
	stages := m.Route(brief)
	byStage, orphanIdx := stageRecordsFor(stages, m.Stages)
	out := make([]StageState, len(stages))
	current := false
	for i, st := range stages {
		out[i].Stage = st
		if j := byStage[i]; j >= 0 {
			r := m.Stages[j]
			out[i].Record = &r
		}
		if !current && !out[i].Accepted() {
			out[i].Current, current = true, true
		}
	}
	var orphans []StageRecord
	for _, j := range orphanIdx {
		orphans = append(orphans, m.Stages[j])
	}
	return out, orphans
}

// CurrentStage is the first stage not yet accepted, if any.
func CurrentStage(states []StageState) (StageState, bool) {
	for _, s := range states {
		if s.Current {
			return s, true
		}
	}
	return StageState{}, false
}

// stageMadeFor is the text of the stage something crew (or, with no crew,
// the user) makes now is for: the crew's stage, else the current one; ""
// when the brief lists no stages.
func stageMadeFor(m *Mission, brief, crewID string) string {
	states, _ := m.StageProgress(brief)
	st, ok := StageFor(states, crewID)
	if crewID == "" || !ok {
		st, ok = CurrentStage(states)
	}
	if !ok || st.Implicit {
		return ""
	}
	return st.Text
}

// ArtifactStage is the stage an artifact was made for, among the brief's
// stages now: the one it recorded, else the stage its crew worked on.
func ArtifactStage(states []StageState, a Artifact) (StageState, bool) {
	if a.Stage != "" {
		for _, st := range states {
			if st.Text == a.Stage && !st.Implicit {
				return st, true
			}
		}
	}
	if a.Crew != "" {
		if st, ok := StageFor(states, a.Crew); ok && !st.Implicit {
			return st, true
		}
	}
	return StageState{}, false
}

// CurrentStageArtifacts is the current stage and the artifacts made for it:
// those linked to it, or for a brief without stages (one stage, the whole
// brief), all of them. None when every stage is accepted.
func (m *Mission) CurrentStageArtifacts(brief string) (StageState, []Artifact) {
	states, _ := m.StageProgress(brief)
	cur, ok := CurrentStage(states)
	if !ok {
		return StageState{}, nil
	}
	if cur.Implicit {
		return cur, m.Artifacts
	}
	var out []Artifact
	for _, a := range m.Artifacts {
		if st, ok := ArtifactStage(states, a); ok && st.N == cur.N {
			out = append(out, a)
		}
	}
	return cur, out
}

// StageFor is the stage crew is for, from its record, if it joined one:
// the latest, for crew who carried on from one step to the next.
func StageFor(states []StageState, crewID string) (StageState, bool) {
	for _, s := range slices.Backward(states) {
		if s.Record != nil && contains(s.Record.Crew, crewID) {
			return s, true
		}
	}
	return StageState{}, false
}

func contains(list []string, s string) bool {
	return slices.Contains(list, s)
}

// recordFor finds or adds the record for stage st in m (matched as
// StageProgress does), keeping its text and position current.
func recordFor(m *Mission, stages []Stage, st Stage) *StageRecord {
	byStage, _ := stageRecordsFor(stages, m.Stages)
	j := byStage[st.N-1]
	if j < 0 {
		m.Stages = append(m.Stages, StageRecord{})
		j = len(m.Stages) - 1
	}
	r := &m.Stages[j]
	r.Text, r.N, r.Of = st.Text, st.N, len(stages)
	return r
}

// JoinStage records crew as working on stage n of the brief's stages (0
// for the current one), and returns it.
func (s Store) JoinStage(slug, crewID string, n int) (Stage, error) {
	brief, _ := s.ReadBrief(slug)
	var joined Stage
	if _, err := s.Update(slug, func(m *Mission) error {
		states, _ := m.StageProgress(brief)
		st, err := PickStage(states, n)
		if err != nil {
			return err
		}
		joined = st.Stage
		r := recordFor(m, m.Route(brief), st.Stage)
		if !contains(r.Crew, crewID) {
			r.Crew = append(r.Crew, crewID)
		}
		if r.Started == nil {
			t := now()
			r.Started = &t
			if st.Step != nil && st.Step.Limit > 0 {
				// ws holds the step's clock from when crew start on it.
				d := t.Add(st.Step.Limit)
				r.Deadline = &d
			}
		}
		return nil
	}); err != nil {
		return Stage{}, err
	}
	return joined, s.AppendLog(slug, Event{Kind: EventStage, Crew: crewID, Text: fmt.Sprintf("on %s %d: %s", joined.Noun(), joined.N, joined.Title())})
}

// PickStage is stage n (from 1), or for 0 the current stage, or the last
// when every stage is accepted.
func PickStage(states []StageState, n int) (StageState, error) {
	if n < 0 || n > len(states) {
		if len(states) > 0 && states[0].Step != nil {
			return StageState{}, fmt.Errorf("there's no step %d: the plan has %d", n, len(states))
		}
		return StageState{}, fmt.Errorf("there's no stage %d: the brief lists %d", n, len(states))
	}
	if n > 0 {
		return states[n-1], nil
	}
	if st, ok := CurrentStage(states); ok {
		return st, nil
	}
	return states[len(states)-1], nil
}

// ErrStageBusy is refusing to accept a stage its crew are still on.
var ErrStageBusy = errors.New("the stage's crew are still on it")

// ErrAllAccepted is accept with no stage left to accept.
var ErrAllAccepted = errors.New("every stage is accepted")

// AcceptStage accepts the current stage: its work is complete and good
// enough. It's refused while any of the stage's crew are still working
// (running, not reported done) or asking the user something. Crew who
// reported done and are in a turn begun since (a message sent to them
// after it) hold it for ws and the orchestrator until that turn ends,
// and not for you: it's yours to pass over them. So is a step whose crew
// handed it over, had it checked, and were sent back since without
// reporting again, once they've stopped: what you were asked of its
// checks is yours to answer. It returns the stage accepted and the stages
// as they are now.
func (s Store) AcceptStage(slug, by, why string) (StageState, []StageState, error) {
	return s.AcceptStageAt(slug, 0, by, why)
}

// ErrStageMoved is accepting a stage the mission isn't at any more.
var ErrStageMoved = errors.New("the mission has moved on from that stage")

// AcceptStageAt is AcceptStage for stage n, refused (ErrStageMoved) when
// the mission isn't at it: for a pass decided from what was read a moment
// before, which someone else may have made meanwhile. 0 is whichever it's
// at.
func (s Store) AcceptStageAt(slug string, n int, by, why string) (StageState, []StageState, error) {
	brief, _ := s.ReadBrief(slug)
	var accepted StageState
	m, err := s.Update(slug, func(m *Mission) error {
		states, _ := m.StageProgress(brief)
		cur, ok := CurrentStage(states)
		if !ok {
			return ErrAllAccepted
		}
		if n != 0 && cur.N != n {
			return fmt.Errorf("%w: it's at %d, not %d", ErrStageMoved, cur.N, n)
		}
		if cur.Record != nil {
			for _, id := range cur.Record.Crew {
				for _, c := range m.Crew {
					switch {
					case c.ID != id || c.Role != "":
					case c.Status == CrewAsking:
						return fmt.Errorf("%w: %s is asking the user something", ErrStageBusy, id)
					case c.Ended != nil:
					case c.Status == CrewDone:
						if by != "you" && c.InTurnSinceDone() {
							return fmt.Errorf("%w: %s is in a turn begun since it reported done (a message sent to it): the step passes when that ends", ErrStageBusy, id)
						}
					case by == "you" && c.Activity != ActivityWorking && cur.Record.checkedFor(id):
					default:
						return fmt.Errorf("%w: %s is still working on it (it reports done when it's finished)", ErrStageBusy, id)
					}
				}
			}
		}
		r := recordFor(m, m.Route(brief), cur.Stage)
		t := now()
		r.Accepted, r.AcceptedBy, r.Why = &t, by, strings.TrimSpace(why)
		accepted = cur
		accepted.Record = r
		return nil
	})
	if err != nil {
		return StageState{}, nil, err
	}
	text := fmt.Sprintf("%s %d accepted: %s", accepted.Noun(), accepted.N, accepted.Title())
	if why != "" {
		text += " · " + strings.TrimSpace(why)
	}
	if err := s.AppendLog(slug, Event{Kind: EventStage, Crew: by, Text: text}); err != nil {
		return StageState{}, nil, err
	}
	states, _ := m.StageProgress(brief)
	return accepted, states, nil
}
