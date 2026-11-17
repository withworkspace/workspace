package mission

import (
	"strings"
	"testing"
	"time"
)

// Tasks are a queue, oldest first: a step's task takes the place of that
// step's, a question the place of its asker's, and neither of another's.
func TestTasks_AQueue(t *testing.T) {
	s := owned(t)
	s.Create("Cache")
	s.AskAtStep("cache", TaskReview, "Step 2 is yours: is it right?", 2)
	s.Ask("cache", "CI is red: fix it?", "orchestrator", "Fix it", "Leave it")
	s.Ask("cache", "Which key?", "c1")
	m, _ := s.Load("cache")
	if len(m.Tasks) != 3 || m.Asking().Step != 2 || m.Asking().Verb != TaskReview || m.Tasks[1].N != 2 || m.Tasks[2].By != "c1" {
		t.Fatalf("the queue = %+v", m.Tasks)
	}
	// Asking again takes the place of your own; a step's again, the step's.
	s.Ask("cache", "CI is still red: fix it?", "orchestrator")
	s.AskAtStep("cache", TaskReview, "Step 2 is yours, again", 2)
	m, _ = s.Load("cache")
	var texts []string
	for _, x := range m.Tasks {
		texts = append(texts, x.Text)
	}
	if strings.Join(texts, " | ") != "Which key? | CI is still red: fix it? | Step 2 is yours, again" || m.TaskSeq != 5 {
		t.Errorf("after asking again: %q (seq %d)", texts, m.TaskSeq)
	}
	if q, ok := m.Question(); !ok || q.Text != "Which key?" {
		t.Errorf("the oldest question = %+v", q)
	}
	if st, ok := m.StepTask(2); !ok || st.N != 5 {
		t.Errorf("step 2's task = %+v", st)
	}

	// Answered and kept (a step of yours), then closed with its step.
	n := m.Tasks[2].N
	if _, _, err := s.AnswerTask("cache", n, "change the key first", "you", true); err != nil {
		t.Fatal(err)
	}
	if m, _ = s.Load("cache"); len(m.Tasks) != 3 || m.Answer.Text != "change the key first" {
		t.Errorf("kept: %+v, answer %+v", m.Tasks, m.Answer)
	}
	s.CloseStepTasks("cache", 2)
	s.Ask("cache", "", "c1") // c1 takes its question back
	if m, _ = s.Load("cache"); len(m.Tasks) != 1 || m.Asking().By != "orchestrator" {
		t.Errorf("after closing: %+v", m.Tasks)
	}
	if err := s.CloseTask("cache", 99, "you", "x"); err == nil {
		t.Error("closed a task that isn't open")
	}
}

// A mission kept before tasks had one question, its flag: a question, or
// ws's check-in at a step. It's a task once it's loaded, and kept so once
// anything's written.
func TestTasks_AFlagFromBefore(t *testing.T) {
	for _, c := range []struct {
		name, record string
		want         Task
	}{
		{"a question", `{"title":"Cache","created":"2026-11-14T10:00:00Z","flag":{"text":"CI is red","by":"orchestrator","at":"2026-11-14T10:00:00Z","options":["Fix it"]}}`,
			Task{N: 1, Kind: TaskYours, Verb: TaskAnswer, Text: "CI is red", By: "orchestrator", Options: []string{"Fix it"}}},
		{"a check-in", `{"title":"Cache","created":"2026-11-14T10:00:00Z","flag":{"text":"Step 2 is yours","by":"ws","at":"2026-11-14T10:00:00Z","options":["Go on","Not yet"],"check_in":2}}`,
			Task{N: 1, Kind: TaskYours, Verb: TaskApprove, Step: 2, Text: "Step 2 is yours", By: "ws", Options: []string{"Go on", "Not yet"}}},
	} {
		t.Run(c.name, func(t *testing.T) {
			m, err := decode("cache", []byte(c.record))
			if err != nil {
				t.Fatal(err)
			}
			got := m.Asking()
			if m.OldFlag != nil || got == nil || got.N != c.want.N || got.Kind != c.want.Kind || got.Verb != c.want.Verb || got.Step != c.want.Step ||
				got.Text != c.want.Text || got.By != c.want.By || strings.Join(got.Options, "|") != strings.Join(c.want.Options, "|") || got.At.IsZero() {
				t.Errorf("task = %+v, flag %+v", got, m.OldFlag)
			}
		})
	}
}

// The court has a headline for each task, each naming the task its answer
// answers: a Review step's is a review, of what the steps before made; a
// question is asked.
func TestTasks_EachAHeadline(t *testing.T) {
	m := &Mission{Slug: "s", Tasks: []Task{
		{N: 3, Verb: TaskReview, Text: "Step 2 is yours, to review: the fix", Step: 2, By: "ws", Options: []string{CheckInGoOn}},
		{N: 4, Verb: TaskAnswer, Text: "CI is red", By: "orchestrator"},
	}}
	m.Plan, _ = ParsePlan("## Z\n- Done.\n## Stops early\n- None.\n## Steps\n1. Work: fix it.\n   - in: `api`\n2. Review: the fix.\n", nil)
	then := time.Now().Add(-time.Hour)
	m.Stages = []StageRecord{{Text: "fix it.", N: 1, Of: 2, Crew: []string{"c1"}, Accepted: &then}}
	m.Crew = []Crew{{ID: "c1", Capsule: "api/fix", Status: CrewDone, Ended: &then}}
	m.Artifacts = []Artifact{{Ref: "api/fix:docs/why.md", Crew: "c1"}, {Ref: "api#4", Kind: "pr", Crew: "c1"}}
	var review, asked *Headline
	for _, h := range Resolve(Snapshot{M: m, State: Active, Now: time.Now()}) {
		switch h.Kind {
		case KindStepReview:
			review = &h
		case KindFlag:
			asked = &h
		}
	}
	if review == nil || review.Action.Verb != VerbReviewStep || review.Action.Target != "3" || review.Text != "Step 2 is yours, to review: the fix" ||
		len(review.Action.Look) != 2 || review.Action.Look[0] != (Look{Capsule: "api/fix", Name: "api/fix"}) || review.Action.Look[1] != (Look{Doc: "api/fix:docs/why.md", Name: "docs/why.md", Read: true}) {
		t.Errorf("the review: %+v", review)
	}
	if asked == nil || asked.Action.Verb != VerbAnswer || asked.Action.Target != "4" || !strings.HasSuffix(asked.Text, "CI is red") {
		t.Errorf("the question: %+v", asked)
	}
}
