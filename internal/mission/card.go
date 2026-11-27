package mission

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Cards: a mission's headlines as its page draws them.
//
// A headline says where a mission stands and what moves it on. How that's
// drawn used to be worked out by the page, from the headline's verb, the
// task it answers, the step the plan is at and the punctuation of its
// sentence: four places, and each kind of headline came out its own way.
// A card is that worked out once, here, for every front end: its tone, a
// title, a sentence, the one action that's meant to move it, and what
// comes after that. The page draws what it's given.
//
// The rule a card keeps:
//
//   - One action is meant to move it (Primary): it's first, and the only
//     one accented. Where there are answers to choose between, none is
//     ahead of another, and there's no Primary.
//   - What overrides that, or puts it aside, comes after it, quiet.
//   - A field for words is open only where words are all there is to
//     give, or they go with the Primary (a note with marking it done).
//     Otherwise it's behind the action that takes them.
//   - A mission is one card while things are as they should be. The step
//     the plan waits at (a Wait, a Review) isn't a second card under a
//     question: the question's card carries the step's own control, first.

// Tone is whose a card is, as its colour says: yours, failing (and yours),
// an agent's, outside's, done, or nobody's in particular.
type Tone string

const (
	ToneYou     Tone = "you"
	ToneFailing Tone = "failing"
	ToneAgent   Tone = "agent"
	ToneOutside Tone = "outside"
	ToneDone    Tone = "done"
	ToneQuiet   Tone = "quiet"
)

// What an Act does, for a page to do it.
const (
	DoAnswer     = "answer"        // answer task Task with Text (and the field's words after Prefix)
	DoAccept     = "accept"        // pass the current step, the field's words as why
	DoLook       = "look"          // have ws run a watched step's check now
	DoDismiss    = "dismiss"       // put the question (or queued crew) aside
	DoAskAgent   = "ask-agent"     // have crew look into the question and answer it
	DoResume     = "resume"        // resume crew Target
	DoLaunch     = "launch"        // launch new crew
	DoLaunchNext = "launch-queued" // launch the queued crew
	DoBrief      = "brief"         // start briefing it
	DoConfirm    = "confirm"       // confirm its brief and plan
	DoEnd        = "end"           // end it, as suggested
	DoNotYet     = "not-yet"       // put the suggested ending aside
	DoReopen     = "unend"         // take the ending back
	DoArchive    = "archive"       // archive it
	DoUnpark     = "unpark"        // take it off parked
	DoReply      = "reply"         // write to crew Target, or answer what it's stopped on
	DoPermission = "permission"    // answer crew Target's permission prompt
	DoURL        = "url"           // open URL, outside ws
	DoCrew       = "crew"          // open crew Target's conversation
	DoReview     = "review"        // open capsule Target's review
	DoVerify     = "verify"        // open bundle Target on the Verify tab
	DoRead       = "read"          // open document Target
	DoTalk       = "talk"          // open conversation Target
	DoBriefPage  = "brief-page"    // open the brief and plan
	DoLog        = "log"           // open the log
)

// Act is one thing a card offers.
type Act struct {
	Do    string `json:"do"`
	Label string `json:"label"`
	Icon  string `json:"icon,omitempty"`
	// Target is what it's done to: crew, a capsule, a bundle, a document,
	// a conversation.
	Target string `json:"target,omitempty"`
	URL    string `json:"url,omitempty"`
	// Task is the task an answer is to; Text what it answers with.
	Task int    `json:"task,omitempty"`
	Text string `json:"text,omitempty"`
	// Field is the placeholder of a field for words that go with it: the
	// card's Primary has it beside it; any other opens it when pressed.
	// What's written goes after Prefix, in place of Text.
	Field  string `json:"field,omitempty"`
	Prefix string `json:"prefix,omitempty"`
	// Quiet is one that overrides the Primary, or puts the card aside.
	Quiet bool `json:"quiet,omitempty"`
	// Key is the key that does it on the mission's page.
	Key string `json:"key,omitempty"`
}

// Card is one headline as it's drawn.
type Card struct {
	Kind  Kind  `json:"kind"`
	Court Court `json:"court"`
	Tone  Tone  `json:"tone"`
	// Title says what's happening; Detail is a sentence more, or "".
	Title  string `json:"title"`
	Detail string `json:"detail,omitempty"`
	// Nothing is true where there's nothing for you to do.
	Nothing bool `json:"nothing,omitempty"`
	// Primary is the one action meant to move it, or nil.
	Primary *Act `json:"primary,omitempty"`
	// More is what comes after it, in order: choices, then the quiet ones.
	More []Act `json:"more,omitempty"`
	// Folded is the kind of the step's own headline this card carries the
	// control of, where the plan waits at a step and something else leads.
	Folded Kind       `json:"folded,omitempty"`
	By     string     `json:"by,omitempty"`
	Since  *time.Time `json:"since,omitempty"`
	// Checking is what's running, while ws checks a step.
	Checking *Checking `json:"checking,omitempty"`
}

// Cards is the mission's headlines as cards, the lead first.
func Cards(s Snapshot) []Card {
	return CardsOf(s, Resolve(s))
}

// CardsOf is Cards, of headlines already resolved.
func CardsOf(s Snapshot, hs []Headline) []Card {
	if len(hs) == 0 {
		return nil
	}
	b := cardBuilder{s: s}
	if s.M != nil && s.M.Plan != nil {
		b.states, _ = s.M.StageProgress(s.Brief)
		if cur, ok := CurrentStage(b.states); ok && cur.Step != nil {
			b.cur = &cur
		}
	}
	// The step the plan waits at, under something else.
	var folded *Headline
	if i := slices.IndexFunc(hs[1:], func(h Headline) bool { return h.Action.Verb == VerbWait || h.Action.Verb == VerbReviewStep }); i >= 0 {
		step := hs[i+1]
		switch {
		case b.question(hs[0]):
			folded = &step
			hs = slices.Delete(slices.Clone(hs), i+1, i+2)
		case step.Action.Verb == VerbWait && b.watched():
			// It passes by itself: the lead's step line says where it stands.
			hs = slices.Delete(slices.Clone(hs), i+1, i+2)
		}
	}
	out := make([]Card, 0, len(hs))
	for i, h := range hs {
		c := b.card(h)
		if i == 0 && folded != nil {
			b.fold(&c, h, *folded)
		}
		out = append(out, c)
	}
	return out
}

type cardBuilder struct {
	s      Snapshot
	states []StageState
	cur    *StageState
}

func (b cardBuilder) watched() bool { return b.cur != nil && b.cur.Step.Watched() }

// task is the task an answer's headline is of.
func (b cardBuilder) task(h Headline) (Task, bool) {
	n, err := strconv.Atoi(h.Action.Target)
	if err != nil || b.s.M == nil {
		return Task{}, false
	}
	return b.s.M.Task(n)
}

// question reports whether a headline is a question put to you: the
// orchestrator's, or crew's through it, not a step's task.
func (b cardBuilder) question(h Headline) bool {
	if h.Action.Verb != VerbAnswer {
		return false
	}
	t, ok := b.task(h)
	return !ok || t.Question()
}

const nothingToDo = " · nothing for you to do"

func toneOf(h Headline) Tone {
	switch h.Kind {
	case KindChecksFailing, KindChangesRequested, KindConflict:
		return ToneFailing
	case KindWrapUp, KindLanded, KindEnded:
		return ToneDone
	case KindToBrief, KindReady, KindParked, KindArchived:
		return ToneQuiet
	}
	switch h.Court {
	case CourtAgent:
		return ToneAgent
	case CourtOutside:
		return ToneOutside
	}
	return ToneYou
}

// kindWords is a kind's title where its sentence has none of its own.
var kindWords = map[Kind]string{
	KindCrewAsks: "Crew question", KindPermission: "Permission request", KindFlag: "Question", KindReReview: "Re-review",
	KindStepReview: "To review", KindToDo: "To do", KindReplies: "Reply to post", KindBriefTalk: "Plan", KindProposed: "To confirm",
	KindQueued: "Crew queued", KindResume: "Crew to resume", KindReplied: "Waiting on you", KindChecksFailing: "Checks failing",
	KindChangesRequested: "Changes requested", KindConflict: "Conflicts with base", KindApprove: "To approve", KindAccept: "To accept", KindMerge: "Ready to merge", KindLand: "To land",
	KindWrapUp: "Ready to end", KindStalled: "Idle", KindWorking: "Crew at work", KindLaunching: "Crew starting", KindOwnWork: "Background work",
	KindAnswering: "Agent answering", KindBriefing: "Briefing", KindChecking: "Checking", KindWritingPR: "Writing pull request", KindMovingOn: "Moving on", KindDeciding: "Orchestrator deciding",
	KindChecksRunning: "Checks running", KindAwaitingReview: "In review", KindWaitingOn: "Waiting", KindLanded: "Landed", KindEnded: "Ended",
	KindToBrief: "To brief", KindReady: "Ready", KindParked: "Parked", KindArchived: "Archived",
}

// titleEnd is where a sentence's title ends: at its first colon, or -1
// when it has none before its first sentence ends. A colon or a full stop
// inside brackets is neither.
func titleEnd(text string) int {
	depth := 0
	for i, c := range text {
		switch {
		case c == '\n':
			return -1
		case c == '(':
			depth++
		case c == ')':
			if depth > 0 {
				depth--
			}
		case depth > 0:
		case c == ':':
			return i
		case c == '.' && strings.HasPrefix(text[i+1:], " "):
			return -1
		}
	}
	return -1
}

// titled is a sentence as a title and what follows: "Looks finished:
// everything in the brief is met". One with no title of its own is its
// own title when it's short, else it's under the kind's words.
func titled(kind Kind, text string) (title, detail string) {
	text = strings.TrimSpace(text)
	if at := titleEnd(text); at >= 3 && at <= 90 && strings.HasPrefix(text[at+1:], " ") && len(text) > at+2 {
		return text[:at], upperFirstWord(strings.TrimSpace(text[at+2:]))
	}
	if len(text) <= 100 {
		return strings.TrimSuffix(text, "."), ""
	}
	return kindWords[kind], text
}

func upperFirstWord(s string) string {
	if s == "" || s[0] < 'a' || s[0] > 'z' {
		return s
	}
	// A name that starts small (api#12, c1) stays as it's written.
	if i := strings.IndexAny(s, " :,."); i > 0 && strings.ContainsAny(s[:i], "0123456789#/_-") {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

var stepDone = regexp.MustCompile(`^Step \d+ is done(?: \((.*?)\))?\. `)

func (b cardBuilder) card(h Headline) Card {
	c := Card{Kind: h.Kind, Court: h.Court, Tone: toneOf(h), By: h.By, Checking: h.Checking}
	if !h.Since.IsZero() {
		t := h.Since
		c.Since = &t
	}
	text := h.Text
	if before, ok := strings.CutSuffix(text, nothingToDo); ok {
		text, c.Nothing = before, true
	}
	c.Title, c.Detail = titled(h.Kind, text)
	if h.Action.Verb == VerbNone && (h.Court == CourtAgent || h.Court == CourtOutside) {
		c.Nothing = true
	}
	b.words(&c, h, text)
	b.acts(&c, h)
	return c
}

// words is the title and sentence of the kinds whose headline's sentence
// isn't one to split: a task's, written when it was asked.
func (b cardBuilder) words(c *Card, h Headline, text string) {
	switch h.Kind {
	case KindFlag:
		t, ok := b.task(h)
		c.Title, c.Detail = flagWho(h.By), strings.TrimSpace(strings.TrimPrefix(text, flagWho(h.By)+":"))
		if ok && t.Verb == TaskChecks && t.Step > 0 && b.s.M != nil {
			if st, round, unmet := b.s.M.ChecksUnmet(b.s.Brief); unmet && st.N == t.Step {
				c.Title = fmt.Sprintf("Step %d's checks: %d of %d met", st.N, len(round.With(CheckMet)), len(round.Results))
			}
		}
	case KindToDo:
		t, ok := b.task(h)
		if !ok || b.cur == nil || b.cur.N != t.Step {
			return
		}
		c.Title = fmt.Sprintf("Step %d is yours to do: %s", b.cur.N, b.cur.Short())
		c.Detail = sentenceOf(strings.TrimPrefix(b.cur.Plain(), b.cur.Short()+". "))
		if b.cur.Step.Watched() {
			c.Detail += fmt.Sprintf(" Watched (%s): passes once it happens.", checkCommands(b.cur.Step.Checks))
		} else {
			c.Detail += " Nothing moves until marked done."
		}
	case KindStepReview:
		t, ok := b.task(h)
		n := t.Step
		if !ok && b.cur != nil {
			n = b.cur.N
		}
		if n < 1 || n > len(b.states) {
			return
		}
		c.Title = fmt.Sprintf("Step %d is yours to review: %s", n, b.states[n-1].Short())
		c.Detail = text
		if m := stepDone.FindStringSubmatch(text); m != nil {
			// What the crew before it said they did is the sentence; the
			// rest says the step is yours, which the title does.
			c.Detail = m[1]
			if c.Detail == "" {
				c.Detail = sentenceOf(b.states[n-1].Plain())
			}
		}
	case KindResume:
		if who := h.Action.Target; who != "" {
			c.Title = "The orchestrator suggests resuming " + who
			if _, said, ok := strings.Cut(text, ": "); ok {
				c.Detail = said
			}
		}
	}
}

// lookAct is opening one thing a step looks at.
func lookAct(l Look) Act {
	switch {
	case l.Bundle != "" && l.Approved:
		return Act{Do: DoVerify, Label: l.Name + ", approved", Icon: "check-circle", Target: l.Bundle}
	case l.Bundle != "":
		return Act{Do: DoVerify, Label: "Verify " + l.Name, Icon: "list-checks", Target: l.Bundle}
	case l.Capsule != "":
		return Act{Do: DoReview, Label: "Review " + l.Name, Icon: "eye", Target: l.Capsule}
	}
	return Act{Do: DoRead, Label: "Read " + l.Name, Icon: "file", Target: l.Doc}
}

// lookWord is what having ws look is called: again, once it has.
func (b cardBuilder) lookWord() string {
	if b.cur != nil {
		if w := b.cur.Watch(); w != nil && w.Looked != nil {
			return "Look again"
		}
	}
	return "Look now"
}

func upperFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// acts is what a card offers, by what its headline's action does.
func (b cardBuilder) acts(c *Card, h Headline) {
	a := h.Action
	label := upperFirst(a.Label)
	switch a.Verb {
	case VerbReply:
		c.Primary = &Act{Do: DoReply, Label: "Reply", Icon: "message", Target: a.Target, Field: "Reply to " + a.Target}
		c.More = []Act{{Do: DoCrew, Label: "Open " + a.Target, Icon: "message", Target: a.Target, Quiet: true}}
	case VerbPermission:
		c.Primary = &Act{Do: DoPermission, Label: "Answer " + a.Target, Icon: "message", Target: a.Target}
	case VerbWatch:
		c.Primary = &Act{Do: DoCrew, Label: "Watch " + a.Target, Icon: "message", Target: a.Target}
	case VerbAnswer:
		b.answer(c, h)
	case VerbReview:
		c.Primary = &Act{Do: DoReview, Label: "Re-review " + a.Target, Icon: "rotate", Target: a.Target, Key: "v"}
	case VerbLand:
		c.Primary = &Act{Do: DoReview, Label: label, Icon: "eye", Target: a.Target, Key: "v"}
	case VerbReviewDoc:
		c.Primary = &Act{Do: DoRead, Label: label, Icon: "reply", Target: a.Target, Key: "v"}
	case VerbReviewStep:
		task, _ := strconv.Atoi(a.Target)
		c.Primary, c.More = reviewActs(a.Look, task)
	case VerbLaunch:
		launch := Act{Do: DoLaunch, Label: "Launch crew", Icon: "play"}
		if a.Target != "" {
			c.Primary = &Act{Do: DoResume, Label: "Resume " + a.Target, Icon: "rotate", Target: a.Target}
			launch.Label = "Launch new crew"
			c.More = []Act{launch}
		} else {
			c.Primary = &launch
		}
	case VerbLaunchNext:
		c.Primary = &Act{Do: DoLaunchNext, Label: "Launch queued crew", Icon: "play"}
		c.More = []Act{{Do: DoDismiss, Label: "Dismiss", Quiet: true}}
	case VerbResume:
		c.Primary = &Act{Do: DoResume, Label: "Resume " + a.Target, Icon: "rotate", Target: a.Target}
	case VerbAccept:
		c.Primary = &Act{Do: DoAccept, Label: label, Icon: "check"}
	case VerbWait:
		c.Primary, c.More = b.waitActs(false, 0, false)
	case VerbOpen:
		c.Primary = &Act{Do: DoURL, Label: "Open " + a.Target, Icon: "pr", URL: a.URL}
	case VerbMerge:
		c.Primary = &Act{Do: DoURL, Label: "Open " + a.Target + " to merge", Icon: "pr", URL: a.URL}
	case VerbArchive:
		c.Primary = &Act{Do: DoArchive, Label: label, Icon: "archive"}
		if h.Kind == KindEnded {
			c.More = []Act{{Do: DoReopen, Label: "Reopen", Quiet: true}}
		}
	case VerbEnd:
		// A press ends it: as suggested, or (nothing suggested, every
		// step done) as its work stands. Putting a suggestion aside is
		// dismissing, which takes a question first where one's open:
		// offered only where it's this.
		c.Primary = &Act{Do: DoEnd, Label: label, Icon: "check"}
		if m := b.s.M; m != nil && m.Conclude != nil && m.Conclude.Ending != "" && len(m.Tasks) == 0 && m.Dispatch == nil {
			c.More = []Act{{Do: DoNotYet, Label: "Not yet", Quiet: true}}
		}
	case VerbBrief:
		c.Primary = &Act{Do: DoBrief, Label: "Brief", Icon: "pencil"}
	case VerbConfirm:
		c.Primary = &Act{Do: DoConfirm, Label: "Confirm and start", Icon: "play"}
		c.More = []Act{{Do: DoBriefPage, Label: "Read brief and plan"}}
	case VerbTalk:
		c.Primary = &Act{Do: DoTalk, Label: label, Icon: "reply", Target: a.Target}
	case VerbUnpark:
		c.Primary = &Act{Do: DoUnpark, Label: label, Icon: "play"}
	case VerbLog:
		c.More = []Act{{Do: DoLog, Label: "Open log", Quiet: true}}
	}
}

// waitActs is a Wait step's own control. One ws watches moves when its
// check is met, so having ws look is what's meant; marking it done
// overrides that. One nobody's watching moves when it's marked done, with
// a note for the crew after. yours is a Wait on you, answered at its task.
func (b cardBuilder) waitActs(yours bool, task int, passIt bool) (*Act, []Act) {
	mark := Act{Do: DoAccept, Label: "Mark done", Icon: "check", Field: "What happened, for later crew"}
	if yours {
		mark = Act{Do: DoAnswer, Label: "Mark done", Icon: "check", Task: task, Text: WaitDone, Prefix: WaitDone + ": ", Field: "What you did, for later crew"}
	}
	if !b.watched() {
		return &mark, nil
	}
	mark.Quiet = true
	if !yours {
		mark.Label = "Mark done anyway"
	}
	mark.Label += "…"
	more := []Act{mark}
	if passIt {
		more = append(more, Act{Do: DoAnswer, Label: "Pass as is", Task: task, Text: ChecksPass, Quiet: true})
	}
	return &Act{Do: DoLook, Label: b.lookWord(), Icon: "rotate"}, more
}

// reviewActs is a Review step's control: what there is to look at, the
// first not yet approved leading, then going on, and sending it back.
func reviewActs(looks []Look, task int) (*Act, []Act) {
	goOn := Act{Do: DoAccept, Label: "Go on"}
	if task > 0 {
		goOn = Act{Do: DoAnswer, Label: "Go on", Task: task, Text: CheckInGoOn}
	}
	var primary *Act
	var more []Act
	for _, l := range looks {
		act := lookAct(l)
		if primary == nil && !l.Approved && !l.Read {
			act.Key = "v"
			primary = &act
			continue
		}
		more = append(more, act)
	}
	if primary == nil {
		// Nothing's left to approve: going on is what moves it.
		primary, goOn.Icon = &goOn, "check"
	} else {
		more = append(more, goOn)
	}
	if task > 0 {
		more = append(more, Act{Do: DoAnswer, Label: "Send back…", Task: task, Field: "What to change", Quiet: true})
	}
	return primary, more
}

// answer is the card of a task: a Wait on you, a step's checks that are
// yours to say, or a question.
func (b cardBuilder) answer(c *Card, h Headline) {
	a := h.Action
	t, ok := b.task(h)
	if ok && t.Verb == TaskDo {
		c.Primary, c.More = b.waitActs(true, t.N, slices.Contains(a.Options, ChecksPass))
		return
	}
	question := !ok || t.Question()
	var bundles []Look
	for _, l := range a.Look {
		if l.Bundle != "" {
			bundles = append(bundles, l)
		}
	}
	// The bundle that shows a step's checks is what's meant: verifying it
	// passes the step. With every one approved, the first answer is.
	passFirst := len(bundles) > 0
	for _, l := range bundles {
		if l.Approved {
			continue
		}
		passFirst = false
		act := lookAct(l)
		if c.Primary == nil {
			act.Key = "v"
			c.Primary = &act
		} else {
			c.More = append(c.More, act)
		}
	}
	for i, o := range a.Options {
		act := Act{Do: DoAnswer, Label: o, Task: t.N, Text: o}
		if passFirst && i == 0 {
			act.Icon = "check"
			c.Primary = &act
			continue
		}
		c.More = append(c.More, act)
	}
	for _, l := range bundles {
		if l.Approved {
			c.More = append(c.More, lookAct(l))
		}
	}
	own := Act{Do: DoAnswer, Label: "Answer", Task: t.N, Field: "Your answer"}
	if c.Primary == nil && len(c.More) == 0 {
		// Words are all there is to give: the field's open.
		c.Primary = &own
	} else {
		own.Label, own.Field, own.Quiet = "Answer…", "Your answer", true
		c.More = append(c.More, own)
	}
	if question {
		c.More = append(c.More, Act{Do: DoAskAgent, Label: "Ask an agent", Quiet: true}, Act{Do: DoDismiss, Label: "Dismiss", Quiet: true})
	}
}

// fold gives the lead, a question, the control of the step the plan waits
// at, in front of its answers: the step's is what's meant to move the
// mission, and the question is about it as often as not.
func (b cardBuilder) fold(c *Card, lead, step Headline) {
	c.Folded = step.Kind
	answers := c.More
	if c.Primary != nil {
		answers = append([]Act{*c.Primary}, answers...)
	}
	switch step.Action.Verb {
	case VerbWait:
		primary, _ := b.waitActs(false, 0, false)
		if primary.Do == DoAccept && b.cur != nil {
			// Beside a question's answers, marking it done says which step,
			// and takes no note: the field is the answer's.
			primary = &Act{Do: DoAccept, Label: fmt.Sprintf("Mark step %d done", b.cur.N), Icon: "check"}
		}
		c.Primary, c.More = primary, answers
	case VerbReviewStep:
		// The step is yours to review whatever was asked of it: its own
		// card, with the question beside it to put aside.
		task, _ := strconv.Atoi(step.Action.Target)
		c.Kind = step.Kind
		reviewed := b.card(step)
		c.Title = reviewed.Title
		c.Primary, c.More = reviewActs(step.Action.Look, task)
		asked := c.Detail
		c.Detail = flagWho(lead.By) + ": " + asked
		c.More = append(c.More, Act{Do: DoDismiss, Label: "Dismiss question", Quiet: true})
	}
}
