package mission

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

// Tasks: what ws knows how to do, each open until what completes it
// happens. A task's kind says whose it is: yours, or ws's. So far a
// mission keeps yours, what it waits on you for: a question to answer, a
// step of yours to go on from, a capsule to land. A mission is your turn
// while any task of yours is open; they're a queue, oldest first, and the
// oldest is its headline. A step's task can't be lost to a question asked
// after it: each has its own.

// Whose a task is.
const (
	TaskYours = "you"
	TaskWS    = "ws"
)

// What a task of yours asks of you.
const (
	// TaskAnswer is a question, with the answers it suggests: the
	// orchestrator's (or crew's) for you, or a Decide step's.
	TaskAnswer = "answer"
	// TaskReview is a Review step: look at what the steps before made, and
	// go on.
	TaskReview = "review"
	// TaskApprove is going on: a stage to start, or a step ws waits on you
	// for (a pull request to open, the steps after the planner's to write).
	TaskApprove = "approve"
	// TaskLand is a Land step: land its capsule.
	TaskLand = "land"
	// TaskChecks is a Work step whose checks weren't all met, or the
	// judge couldn't tell: pass it as it is, or send it back.
	TaskChecks = "checks"
	// TaskTime is a Work step whose time has run out: more time, or wrap
	// up.
	TaskTime = "time"
	// TaskReply is a reply drafted on a pull request, to a person: post
	// it, or not (Task.Reply says which).
	TaskReply = "reply"
)

// The answers a Work step's tasks suggest.
const (
	ChecksPass     = "Pass it"
	ChecksSendBack = "Send it back"
	TimeMore       = "More time"
	TimeWrapUp     = "Wrap up"
	ReplyPost      = "Post it"
	ReplyDont      = "Don't post"
	StepSkip       = "Skip it"
)

// Task is one thing a mission waits on you for.
type Task struct {
	N    int    `json:"n"`
	Kind string `json:"kind"`
	Verb string `json:"verb"`
	// Text is what's asked, in a sentence or two.
	Text string `json:"text"`
	// Step is the step of the plan (or stage of the brief) it's for, from
	// 1; 0 for none.
	Step int `json:"step,omitempty"`
	// Options are the answers it suggests: you can always give your own.
	Options []string  `json:"options,omitempty"`
	By      string    `json:"by,omitempty"` // who opened it: ws, the orchestrator, crew
	At      time.Time `json:"at"`
	// Reply is the reply a reply task is about, by its number.
	Reply int `json:"reply,omitempty"`
}

// Question reports whether the task is a question that isn't a step's:
// what the orchestrator (or crew) asked you.
func (t Task) Question() bool { return t.Verb == TaskAnswer && t.Step == 0 }

// Asking is the task of yours at the head of the queue (the oldest open),
// or nil when the mission isn't waiting on you for any.
func (m *Mission) Asking() *Task {
	if len(m.Tasks) == 0 {
		return nil
	}
	t := m.Tasks[0]
	return &t
}

// Task is the open task numbered n.
func (m *Mission) Task(n int) (Task, bool) {
	i := slices.IndexFunc(m.Tasks, func(t Task) bool { return t.N == n })
	if i < 0 {
		return Task{}, false
	}
	return m.Tasks[i], true
}

// StepTask is the open task for step n, if there is one.
func (m *Mission) StepTask(n int) (Task, bool) {
	i := slices.IndexFunc(m.Tasks, func(t Task) bool { return t.Step == n && n > 0 })
	if i < 0 {
		return Task{}, false
	}
	return m.Tasks[i], true
}

// Question is the oldest open question that isn't a step's: what crew you
// ask to answer for you take on.
func (m *Mission) Question() (Task, bool) {
	i := slices.IndexFunc(m.Tasks, Task.Question)
	if i < 0 {
		return Task{}, false
	}
	return m.Tasks[i], true
}

// openTask adds t to the queue, numbered: a step's task takes the place of
// that step's open one, and a question takes the place of the one its
// asker has open (asking again), so neither piles up.
func (m *Mission) openTask(t Task) Task {
	m.Tasks = slices.DeleteFunc(m.Tasks, func(o Task) bool {
		return t.Step > 0 && o.Step == t.Step || t.Question() && o.Question() && o.By == t.By || t.Reply > 0 && o.Reply == t.Reply
	})
	m.TaskSeq++
	t.N = m.TaskSeq
	if t.Kind == "" {
		t.Kind = TaskYours
	}
	if t.At.IsZero() {
		t.At = now()
	}
	m.Tasks = append(m.Tasks, t)
	return t
}

// closeTask takes task n out of the queue.
func (m *Mission) closeTask(n int) (Task, bool) {
	t, ok := m.Task(n)
	if ok {
		m.Tasks = slices.DeleteFunc(m.Tasks, func(o Task) bool { return o.N == n })
	}
	return t, ok
}

// migrateFlag makes the one question a mission kept before tasks (its
// flag: a question, or ws's check-in at a step) a task.
func (m *Mission) migrateFlag() {
	f := m.OldFlag
	if f == nil {
		return
	}
	m.OldFlag = nil
	t := Task{Verb: TaskAnswer, Text: f.Text, Options: f.Options, By: f.By, At: f.At}
	if f.CheckIn > 0 {
		t.Verb, t.Step = TaskApprove, f.CheckIn
	}
	m.openTask(t)
}

// ErrNoTask is answering or closing a task the mission hasn't open.
var ErrNoTask = errors.New("nothing to answer: the mission isn't asking you anything")

// Ask asks you a question on a mission (an answer task, with the answers
// it suggests), for whoever asks (by): it takes the place of one they have
// open. "" takes theirs back.
func (s Store) Ask(slug, text, by string, options ...string) (Task, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		_, err := s.Update(slug, func(m *Mission) error {
			m.Tasks = slices.DeleteFunc(m.Tasks, func(t Task) bool { return t.Question() && t.By == by })
			return nil
		})
		return Task{}, err
	}
	return s.OpenTask(slug, Task{Verb: TaskAnswer, Text: text, Options: cleanOptions(options), By: by})
}

// AskAtStep asks you at a step of the route, for ws: to review it and go
// on, answer it, land it, or approve going on. It takes the place of the
// step's open task. Its answers are go on and not yet, unless options.
func (s Store) AskAtStep(slug, verb, text string, step int, options ...string) (Task, error) {
	if len(options) == 0 {
		options = []string{CheckInGoOn, CheckInNotYet}
	}
	return s.OpenTask(slug, Task{Verb: verb, Text: strings.TrimSpace(text), Step: step, Options: options, By: "ws"})
}

// OpenTask opens a task of yours on a mission, and logs it.
func (s Store) OpenTask(slug string, t Task) (Task, error) {
	var opened Task
	if _, err := s.Update(slug, func(m *Mission) error {
		opened = m.openTask(t)
		return nil
	}); err != nil {
		return Task{}, err
	}
	return opened, s.AppendLog(slug, Event{Kind: EventFlag, Crew: firstNonEmpty(t.By, "ws"), Text: t.Text})
}

// CloseTask takes task n out of the queue without its being done: moot,
// dismissed, or done some other way (why, for the log; "" logs nothing).
func (s Store) CloseTask(slug string, n int, by, why string) error {
	var closed bool
	if _, err := s.Update(slug, func(m *Mission) error {
		_, closed = m.closeTask(n)
		return nil
	}); err != nil {
		return err
	}
	if !closed {
		return fmt.Errorf("%w (no task %d)", ErrNoTask, n)
	}
	if why == "" {
		return nil
	}
	return s.AppendLog(slug, Event{Kind: EventNote, Crew: by, Text: fmt.Sprintf("task %d closed: %s", n, why)})
}

// CloseStepTasks takes a step's task out of the queue: the step's done, or
// ws is asking about it afresh.
func (s Store) CloseStepTasks(slug string, step int) error {
	_, err := s.Update(slug, func(m *Mission) error {
		m.Tasks = slices.DeleteFunc(m.Tasks, func(t Task) bool { return step > 0 && t.Step == step })
		return nil
	})
	return err
}

// AnswerTask records your answer to task n (or crew's you asked to answer
// it): the question and answer are kept for the orchestrator's next run,
// what the mission waited on outside it is cleared, and the count of crew
// started without you starts again. The task closes, unless keep: a step
// of yours answered with something other than going on stays yours.
func (s Store) AnswerTask(slug string, n int, answer, by string, keep bool) (Task, Answer, error) {
	answer = strings.TrimSpace(answer)
	if answer == "" {
		return Task{}, Answer{}, errors.New("the answer is empty")
	}
	var t Task
	var a Answer
	if _, err := s.Update(slug, func(m *Mission) error {
		var ok bool
		if t, ok = m.Task(n); !ok {
			return fmt.Errorf("%w (no task %d)", ErrNoTask, n)
		}
		if !keep {
			m.closeTask(n)
		}
		a = Answer{Question: t.Text, Text: answer, At: now(), By: by, Step: t.Step}
		if t.Step > 0 {
			a.Verb = t.Verb
		}
		m.Answer, m.Dispatches, m.Waiting = &a, 0, nil
		return nil
	}); err != nil {
		return Task{}, Answer{}, err
	}
	return t, a, s.AppendLog(slug, Event{Kind: EventFlagAnswered, Crew: by, Text: answer})
}

// cleanOptions are options without the empty ones.
func cleanOptions(options []string) []string {
	var out []string
	for _, o := range options {
		if o = strings.TrimSpace(o); o != "" {
			out = append(out, o)
		}
	}
	return out
}
