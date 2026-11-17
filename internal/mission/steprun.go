package mission

import (
	"fmt"
	"slices"
	"strings"
	"time"
)

// The orchestrator's decisions on the plan, as ws records them: a step
// that applies only if something holds waits for its call to start, or to
// skip it; a step that runs in each of several repos waits for it to name
// them, and runs once in each (its instances), done when every one is; a
// step can be started again, fresh (an attempt), within max_rounds. You
// may make any of these calls yourself.

// StepInstance is one of a step's runs, in one repo.
type StepInstance struct {
	Repo string `toml:"repo" json:"repo"`
	// Capsule is where it runs (repo/name), once there's one.
	Capsule string `toml:"capsule,omitempty" json:"capsule,omitempty"`
	// Done is when it was done: its crew reported done (with its checks
	// met, where it has them), or its pull request merged.
	Done *time.Time `toml:"done,omitempty" json:"done,omitempty"`
	// Rounds is how many times its pull request came back for fixing.
	Rounds int `toml:"rounds,omitempty" json:"rounds,omitempty"`
}

// Held reports whether the step waits for someone's call to start: it
// applies only if, or runs in each of several repos, and nobody's started
// it.
func (s StageState) Held() bool {
	return s.Step != nil && (s.Step.OnlyIf != "" || s.Step.InEach != "") && !s.Accepted() && (s.Record == nil || s.Record.Go == nil)
}

// Skipped reports whether the step was passed without being done.
func (s StageState) Skipped() bool { return s.Record != nil && s.Record.Skipped }

// Instances are the step's runs, one per repo, if it runs in several.
func (s StageState) Instances() []StepInstance {
	if s.Record == nil {
		return nil
	}
	return s.Record.Instances
}

// Instance is the step's run in a capsule (repo/name) or a repo.
func (s StageState) Instance(where string) (StepInstance, bool) {
	repo, _, _ := strings.Cut(where, "/")
	for _, in := range s.Instances() {
		if in.Capsule == where || in.Repo == repo && (in.Capsule == "" || !strings.Contains(where, "/")) {
			return in, true
		}
	}
	return StepInstance{}, false
}

// InstancesLeft are the repos of the step's runs not done yet.
func (s StageState) InstancesLeft() []string {
	var out []string
	for _, in := range s.Instances() {
		if in.Done == nil {
			out = append(out, in.Repo)
		}
	}
	return out
}

// stepRecord is the record of the step the mission's at, numbered n, for
// a change to it: refused when the mission isn't at it.
func stepRecord(m *Mission, brief string, n int) (StageState, *StageRecord, error) {
	states, _ := m.StageProgress(brief)
	cur, ok := CurrentStage(states)
	if !ok || cur.N != n {
		return StageState{}, nil, fmt.Errorf("the mission isn't at step %d", n)
	}
	return cur, recordFor(m, m.Route(brief), cur.Stage), nil
}

// StartStep records step n, which waited for someone's call, started by
// by: in repos, one run in each, for a step that runs in several.
func (s Store) StartStep(slug string, n int, by string, repos []string) error {
	brief, _ := s.ReadBrief(slug)
	if _, err := s.Update(slug, func(m *Mission) error {
		_, r, err := stepRecord(m, brief, n)
		if err != nil {
			return err
		}
		t := now()
		r.Go, r.GoBy = &t, by
		r.Instances = nil
		for _, repo := range repos {
			r.Instances = append(r.Instances, StepInstance{Repo: repo})
		}
		return nil
	}); err != nil {
		return err
	}
	text := fmt.Sprintf("step %d started", n)
	if len(repos) > 0 {
		text += " in " + strings.Join(repos, ", ")
	}
	return s.AppendLog(slug, Event{Kind: EventStage, Crew: by, Text: text})
}

// SkipStep passes step n without its being done, for by, saying why: a
// step that applies only if, and doesn't.
func (s Store) SkipStep(slug string, n int, by, why string) error {
	brief, _ := s.ReadBrief(slug)
	why = strings.TrimSpace(why)
	if _, err := s.Update(slug, func(m *Mission) error {
		_, r, err := stepRecord(m, brief, n)
		if err != nil {
			return err
		}
		t := now()
		r.Skipped, r.Accepted, r.AcceptedBy, r.Why = true, &t, by, why
		return nil
	}); err != nil {
		return err
	}
	return s.AppendLog(slug, Event{Kind: EventStage, Crew: by, Text: fmt.Sprintf("step %d skipped · %s", n, why)})
}

// RepeatStep counts another attempt at step n, for by, saying why, and
// says which attempt it is (the first is 1). Its runs in repos (or the
// one named) aren't done any more.
func (s Store) RepeatStep(slug string, n int, repo, by, why string) (int, error) {
	brief, _ := s.ReadBrief(slug)
	attempt := 0
	if _, err := s.Update(slug, func(m *Mission) error {
		_, r, err := stepRecord(m, brief, n)
		if err != nil {
			return err
		}
		r.Attempts++
		attempt = r.Attempts + 1
		for i := range r.Instances {
			if repo == "" || r.Instances[i].Repo == repo {
				r.Instances[i].Done = nil
			}
		}
		return nil
	}); err != nil {
		return 0, err
	}
	where := ""
	if repo != "" {
		where = " in " + repo
	}
	return attempt, s.AppendLog(slug, Event{Kind: EventStage, Crew: by, Text: fmt.Sprintf("step %d again%s, attempt %d · %s", n, where, attempt, strings.TrimSpace(why))})
}

// SetInstance records where step n's run in repo runs, or that it's done
// (done), and reports whether every run of the step is done now.
func (s Store) SetInstance(slug string, n int, repo, capsule string, done bool) (bool, error) {
	brief, _ := s.ReadBrief(slug)
	all := false
	if _, err := s.Update(slug, func(m *Mission) error {
		_, r, err := stepRecord(m, brief, n)
		if err != nil {
			return err
		}
		i := slices.IndexFunc(r.Instances, func(in StepInstance) bool { return in.Repo == repo })
		if i < 0 {
			return fmt.Errorf("step %d doesn't run in %s", n, repo)
		}
		if capsule != "" {
			r.Instances[i].Capsule = capsule
		}
		if done && r.Instances[i].Done == nil {
			t := now()
			r.Instances[i].Done = &t
		}
		all = !slices.ContainsFunc(r.Instances, func(in StepInstance) bool { return in.Done == nil })
		return nil
	}); err != nil {
		return false, err
	}
	if done {
		return all, s.AppendLog(slug, Event{Kind: EventStage, Text: fmt.Sprintf("step %d done in %s", n, repo)})
	}
	return all, nil
}

// HeldWhy is why a step waits for a call, in words: "applies only if …",
// "runs in each …".
func (st StageState) HeldWhy() string {
	var why []string
	if st.Step.OnlyIf != "" {
		why = append(why, "applies only if "+strings.TrimSuffix(st.Step.OnlyIf, "."))
	}
	if st.Step.InEach != "" {
		why = append(why, "runs in each "+strings.TrimSuffix(st.Step.InEach, "."))
	}
	return strings.Join(why, ", and ")
}
