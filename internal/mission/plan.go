package mission

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// The plan: a mission's route from A to Z, plan.md beside the brief. The
// planner (briefing's agent) writes it from the brief and the workspace's
// process, and you confirm it with the brief. It says what Z is (what has
// to be true for the brief's purpose to be met), where the mission may
// stop early, and its steps in order, each a kind of step from the
// catalogue (steps.go) with what it's for and its parameters. ws reads it
// strictly (ParsePlan) and refuses one that isn't right, saying each thing
// wrong and its line. A plan written before step types, each step saying
// who's on it (who:), still reads: each step is the type its who: is. A
// mission with a plan works from its steps where one without works from
// its brief's stages: see StageProgress.

const planFile = "plan.md"

// Plan is a mission's plan, as written and as read.
type Plan struct {
	Text  string   // plan.md, as written
	Z     []string // what has to be true for the brief's purpose to be met
	Early []string // where it may stop before Z, and what that end is
	Steps []Step
}

// Step is one leg of a plan's route.
type Step struct {
	N int
	// Type is its kind, from the catalogue (StepWork, StepOpenPR…). A step
	// of a plan written before step types is the type its who: is.
	Type string
	Text string // what's done, in a sentence or so
	// Quote is the words of the process the step carries out, if it says.
	Quote string
	// Who is on it, as its type says (or, before step types, its who:):
	// WhoCrew, WhoYou, WhoWS, WhoCI, WhoOutside or WhoPlanner.
	Who string
	// Capsule is where it works: a repo, or one of the mission's capsules
	// (repo/name). Model is what its crew run on, if it says. Whom is
	// who's on it, in words, for CI or outside, in a plan before types.
	Capsule, Model, Whom string
	// Makes is what it produces, if the step says. From is where a Decide
	// step's options are; Draft, that an Open Pull Request step opens a
	// draft.
	Makes, From string
	Draft       bool
	// Checks are what has to be true for a Work step to be done, as it
	// says them (checks.go).
	Checks []Check
	// CarryOn is the Work step before it whose crew carry on with this
	// one (0: fresh crew, the default; Fresh says so).
	CarryOn int
	Fresh   bool
	// Limit is how long a Work step's crew have (0 for as long as it
	// takes), and LimitText the limit as written, with any words after.
	Limit     time.Duration
	LimitText string
	// OnlyIf is when the step applies, in words, for one that doesn't
	// always: the orchestrator starts it or skips it. InEach is the repos
	// it runs in, in words, for one that runs in several: named when it
	// starts.
	OnlyIf, InEach string
	Line           int // its line in plan.md
	// Bare is a step written as its kind alone (4. Open Pull Request):
	// its Text is made from its kind and where it is, and it's listed by
	// its kind.
	Bare bool
}

// Who's on a step.
const (
	WhoCrew    = "crew"
	WhoYou     = "you"
	WhoWS      = "ws" // ws itself: an Open Pull Request step
	WhoCI      = "CI"
	WhoOutside = "outside"
	WhoPlanner = "the planner"
)

// Checkpoint reports whether the step is yours: you look at what the steps
// before made and say go on, and nothing else passes it.
func (s Step) Checkpoint() bool { return s.Who == WhoYou }

// WaitOnYou reports whether the step is a Wait on the user: something they
// do themselves, which ws asks them for and they mark done.
func (s Step) WaitOnYou() bool { return s.Type == StepWait && s.Who == WhoYou }

// Watched reports whether ws watches for what a Wait step waits for: it
// has checks, commands ws runs until they're met. A Wait without one is
// passed only by someone saying it's happened.
func (s Step) Watched() bool { return s.Type == StepWait && len(s.Checks) > 0 }

// WaitUnsaid reports whether the step is a Wait that doesn't say who it's
// on: one written before a Wait said, which waits on someone outside.
func (s Step) WaitUnsaid() bool { return s.Type == StepWait && s.Who == WhoOutside && s.Whom == "" }

// makesPR is a makes: that names a pull request.
var makesPR = regexp.MustCompile(`(?i)\bpull[- ]requests?\b|\bPRs?\b`)

// MakesPR reports whether the step opens a pull request: an Open Pull
// Request step, which ws opens; or, in a plan written before step types, a
// crew step whose makes: says it makes one, which its crew open through ws
// (ws mission open-pr). Either way it's opened through ws, which holds its
// guarantees.
func (s Step) MakesPR() bool { return s.Type == StepOpenPR || makesPR.MatchString(s.Makes) }

// Whose is whose the step is, in words: crew's, the user's, CI's, outside
// ws's, or the planner's.
func (s Step) Whose() string {
	switch s.Who {
	case WhoYou:
		return "the user's"
	case WhoWS:
		return "ws's"
	case WhoOutside:
		return "outside ws's"
	}
	return s.Who + "'s"
}

// WhoSays is who's on the step, as the plan says it.
func (s Step) WhoSays() string {
	var parts []string
	switch s.Who {
	case WhoCrew:
		parts = append(parts, "crew")
		if s.Capsule != "" {
			parts = append(parts, "in `"+s.Capsule+"`")
		}
		if s.Model != "" {
			parts = append(parts, "on "+s.Model)
		}
		return strings.Join(parts, ", ")
	case WhoCI, WhoOutside:
		if s.Whom != "" {
			return s.Who + ": " + s.Whom
		}
	}
	return s.Who
}

// The parts of a plan, by their headings.
const (
	PlanZ     = "Z"
	PlanEarly = "Stops early"
	PlanSteps = "Steps"
)

// PlanFormat says how a plan is written: what the planner is told, and
// what ParsePlan holds it to. Its kinds of step are the catalogue's.
var PlanFormat = "The plan is markdown in three parts, each a `##` heading:\n\n" +
	"```markdown\n" +
	"## Z\n" +
	"- Merged, with its regression test.\n" +
	"- The user has said whether it goes to pre and prd.\n\n" +
	"## Stops early\n" +
	"- It can't be reproduced, or isn't a bug: that's an answer for product.\n\n" +
	"## Steps\n" +
	"1. Work: **Reproduce.** Reproduce it as a failing test, and find the cause. \"Bugs are reproduced first\"\n" +
	"   - in: `api`\n" +
	"   - limit: 1h\n" +
	"   - makes: the failing test, and the cause\n" +
	"   - check: `go test ./invoice -run TestTotalRounding` fails\n" +
	"   - check: the test fails because of the cause the handoff names\n" +
	"2. Review: **Check the cause.** The failing test, and is this the cause?\n" +
	"3. Work: **Fix.** Fix it.\n" +
	"   - carrying on from: 1\n" +
	"   - check: `go test ./...` passes\n" +
	"   - check: it fixes the cause, not the symptom\n" +
	"4. Open Pull Request\n" +
	"   - in: `api`\n" +
	"5. Decide: **Pre and prd?** Does it go to pre and prd?\n" +
	"   - only if: it changes what pre and prd run\n" +
	"```\n\n" +
	"- `## Z` lists what has to be true for the brief's purpose to be met: where the mission ends. It has at least one item.\n" +
	"- `## Stops early` lists where the mission may end before Z, and what that end is (an answer, work handed on as a mission of its own). `None.` if it can't.\n" +
	"- `## Steps` is the route: a numbered list, 1, 2, 3… in order, each step done after the one before. Each step starts with its kind (below), then a colon, its name in bold, then what it's for, in words (`3. Work: **Fix.** Fix it.`). The name is how the step is listed in the web UI beside the mission: one to four words, a heading, not a sentence (`**Rework CSS.**`, `**Settings picker.**`, `**Browser check.**`), with the detail in the words after it. A kind whose words are optional can stand alone (`4. Open Pull Request`), and is listed by its kind. Under it, indented, go its parameters, each `- key: value`, and only those its kind takes.\n" +
	"- A Work step's checks are how ws knows it's done, so give it one for each thing that has to be true: a command where one shows it, words where it takes judgement. Unmet, it goes back to its crew (or the user), so a check is what you'd send it back for.\n" +
	"- A check that's a command is run by ws as it's written, from the root of the step's capsule, with nothing set up for it first. So write it as it runs from there (`cd api && go test ./...` where it has to be run from a folder inside), and name in it only files, folders and recipes you've looked at and seen are there: not ones you'd expect from the names around them. Ask of each what it does today, before the step's work. A `passes` check that can't pass for a reason the step won't change (a setting the root hasn't got, a tool that isn't there), or a `fails` check that would fail whatever the step did (a file it names that doesn't exist), shows nothing and stops the step at its end. Where you can't tell whether a command runs, say which in your reply, for the user to try before they confirm the plan.\n" +
	"- A Wait step says who it's on (`- on: you` for what the user does themselves, else who or what), and nothing passes it by itself unless it has an `- until:` or a `- check:`. An until is an event ws hears of (`- until: pr.merged pr=api#212`, `- until: thing.changed thing=RAM-881 status=Done`), and ws passes the step as it happens: give one wherever ws hears of what it waits for. A check is a command that shows it's happened, which ws runs every few minutes. With neither it waits until someone says it's happened.\n" +
	"- No branches or loops: the plan is a list. A step that doesn't always apply says when, in words, with `- only if:`, and the orchestrator starts it or skips it; a step that runs in several repos says which, in words, with `- in each:` (repo where step 1 made changes), and the orchestrator names them when it starts.\n" +
	"- A step the process asks for ends with the words of the process it carries out, in double quotes, as they're written there (a few words are enough; `…` leaves words out).\n" +
	"- Nothing else: no title, no text outside the lists. A long item can go on over indented lines.\n\n" +
	"The kinds of step, and what each takes:\n\n" + catalogueFormat()

// PlanQuotes says whether a step's quote is the process's words: "" if it
// is, else why not. ParsePlan without one doesn't check quotes: a plan
// is held to the process it's written against when it's written, not
// again each time it's read.
type PlanQuotes func(quote string) string

// ParsePlan reads a plan as it's written (PlanFormat), and refuses one that
// isn't right, saying each thing wrong and the line it's on.
func ParsePlan(written string, quotes PlanQuotes) (*Plan, error) {
	p := planParser{quotes: quotes, seen: map[string]bool{}, none: map[string]bool{}}
	p.read(written)
	p.carryingOn()
	p.inOrInEach()
	if len(p.problems) > 0 {
		return nil, &MalformedPlan{Problems: p.problems}
	}
	return &Plan{Text: strings.TrimSpace(written) + "\n", Z: p.z, Early: p.early, Steps: p.steps}, nil
}

// MalformedPlan is a plan ParsePlan refused: what's wrong with it.
type MalformedPlan struct{ Problems []string }

func (m *MalformedPlan) Error() string {
	if len(m.Problems) == 1 {
		return "plan is invalid: " + m.Problems[0]
	}
	return "plan is invalid:\n- " + strings.Join(m.Problems, "\n- ")
}

// IsMalformedPlan reports whether err is a plan refused for its form.
func IsMalformedPlan(err error) bool {
	var m *MalformedPlan
	return errors.As(err, &m)
}

var (
	planHeading  = regexp.MustCompile(`^(#{1,6})\s+(.*?)\s*#*\s*$`)
	planItem     = regexp.MustCompile(`^(?:(\d+)[.)]|[-*+])\s+(.*)$`)
	planField    = regexp.MustCompile(`^([A-Za-z]+(?: [A-Za-z]+)*)\s*:\s*(.*)$`)
	planQuoted   = regexp.MustCompile(`^(.*)["“]([^"“”]+)["”]\)?[.]?\s*$`)
	planCapsule  = regexp.MustCompile("^`?([A-Za-z0-9][A-Za-z0-9._-]*/[A-Za-z0-9._/-]+)`?$")
	planIn       = regexp.MustCompile("^`?([A-Za-z0-9][A-Za-z0-9._-]*(?:/[A-Za-z0-9._/-]+)?)`?$")
	planModel    = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/-]*$`)
	planWhoSplit = regexp.MustCompile(`^([A-Za-z]+(?:\s+[A-Za-z]+)?)\s*(?:[:,]\s*(.*))?$`)
)

// planOpen is the item being read, continued over indented lines: a
// part's item, a step, or one of a step's fields.
type planOpen struct {
	line  int
	text  string
	field string // a step's field: its parameter's key, or who: or makes:
}

type planParser struct {
	quotes PlanQuotes

	part   string
	seen   map[string]bool
	none   map[string]bool
	step   *Step           // the step being read
	fields map[string]bool // its fields read
	open   *planOpen

	z, early []string
	steps    []Step
	problems []string
}

func (p *planParser) problem(line int, format string, args ...any) {
	p.problems = append(p.problems, fmt.Sprintf("line %d: ", line)+fmt.Sprintf(format, args...))
}

func (p *planParser) read(written string) {
	lines := strings.Split(strings.ReplaceAll(written, "\r\n", "\n"), "\n")
	for i, raw := range lines {
		n := i + 1
		line := strings.TrimRight(raw, " \t")
		if strings.TrimSpace(line) == "" {
			continue
		}
		trimmed := strings.TrimSpace(line)
		indented := strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t")
		if indented && p.open != nil && !planItem.MatchString(trimmed) {
			p.open.text += " " + trimmed
			continue
		}
		if m := planHeading.FindStringSubmatch(trimmed); m != nil && !indented {
			p.close()
			p.endStep()
			p.heading(n, len(m[1]), m[2])
			continue
		}
		if m := planItem.FindStringSubmatch(trimmed); m != nil {
			p.close()
			if indented {
				p.field(n, trimmed, m[2])
				continue
			}
			p.endStep()
			p.item(n, trimmed, m[1], m[2])
			continue
		}
		p.close()
		p.problem(n, "%q is not a list item: each part is a list under its heading (## %s, ## %s, ## %s)", clipPlan(trimmed), PlanZ, PlanEarly, PlanSteps)
	}
	p.close()
	p.endStep()
	for _, part := range []string{PlanZ, PlanEarly, PlanSteps} {
		if !p.seen[part] {
			what := "add one"
			if part == PlanEarly {
				what = "add one, with None. under it if the mission cannot end before Z"
			}
			p.problems = append(p.problems, fmt.Sprintf("missing ## %s: %s", part, what))
		}
	}
	if p.seen[PlanZ] && len(p.z) == 0 && !p.none[PlanZ] {
		p.problems = append(p.problems, "## Z is empty: list what has to be true for the brief's purpose to be met")
	}
	if p.seen[PlanSteps] && len(p.steps) == 0 && !hasProblemFor(p.problems) {
		p.problems = append(p.problems, "## Steps is empty: add the steps as a numbered list")
	}
}

// hasProblemFor reports whether a step was refused already, so an empty
// ## Steps isn't said twice.
func hasProblemFor(problems []string) bool {
	for _, pr := range problems {
		if strings.Contains(pr, "step") {
			return true
		}
	}
	return false
}

func (p *planParser) heading(n, level int, title string) {
	if level != 2 {
		p.problem(n, "%s %s: no titles, only the ## parts (## %s, ## %s, ## %s)", strings.Repeat("#", level), title, PlanZ, PlanEarly, PlanSteps)
		return
	}
	part := planPartOf(title)
	switch {
	case part == "":
		p.problem(n, "## %s is not a plan part (## %s, ## %s, ## %s)", title, PlanZ, PlanEarly, PlanSteps)
		p.part = "?"
		return
	case p.seen[part]:
		p.problem(n, "## %s appears twice", part)
	}
	p.seen[part] = true
	p.part = part
}

// planPartOf is which part a heading is, as written or near enough.
func planPartOf(title string) string {
	t := strings.ToLower(strings.TrimSpace(strings.TrimRight(strings.TrimSpace(title), ":")))
	switch t {
	case "z":
		return PlanZ
	case "stops early", "stopping early", "stops early if", "ends early":
		return PlanEarly
	case "steps", "the steps":
		return PlanSteps
	}
	return ""
}

// item is a part's item, not indented: a step under ## Steps, else a line
// of Z or of where it stops early.
func (p *planParser) item(n int, trimmed, number, text string) {
	switch p.part {
	case "":
		p.problem(n, "%q comes before any part: start with ## %s", clipPlan(trimmed), PlanZ)
		return
	case "?":
		return // under a heading already refused
	case PlanSteps:
		if number == "" {
			p.problem(n, "%q is not numbered: number steps 1, 2, 3… in order", clipPlan(trimmed))
			return
		}
		if want := len(p.steps) + 1; number != fmt.Sprint(want) {
			p.problem(n, "step %s where step %d should be: number steps 1, 2, 3… in order", number, want)
		}
		p.step = &Step{N: len(p.steps) + 1, Line: n}
		p.fields = map[string]bool{}
	}
	p.open = &planOpen{line: n, text: strings.TrimSpace(text)}
}

// field is an indented item: one of a step's parameters (or, in a plan
// written before step types, its who: or makes:).
func (p *planParser) field(n int, trimmed, text string) {
	if p.part != PlanSteps || p.step == nil {
		p.problem(n, "%q is a nested list: only steps have items under them (parameters)", clipPlan(trimmed))
		return
	}
	m := planField.FindStringSubmatch(strings.TrimSpace(text))
	if m == nil {
		// A parameter that's a word alone: - fresh.
		if word := strings.ToLower(strings.TrimRight(strings.TrimSpace(text), ".")); p.flagParam(word) {
			p.open = &planOpen{line: n, text: "yes", field: word}
			return
		}
		p.problem(n, "%q is not a step parameter: each is - key: value", clipPlan(trimmed))
		return
	}
	p.open = &planOpen{line: n, text: strings.TrimSpace(m[2]), field: strings.ToLower(strings.Join(strings.Fields(m[1]), " "))}
}

// close reads the item being read into its part, or its step.
func (p *planParser) close() {
	it := p.open
	p.open = nil
	if it == nil {
		return
	}
	switch {
	case it.field != "":
		p.closeField(it)
	case p.part == PlanSteps:
		p.closeStep(it)
	case p.part == PlanZ || p.part == PlanEarly:
		p.closeLine(it)
	}
}

func (p *planParser) closeLine(it *planOpen) {
	if isNone(it.text) {
		if p.part == PlanZ {
			p.problem(it.line, "## Z cannot be None.: say what has to be true for the brief's purpose to be met")
			return
		}
		if len(p.early) > 0 {
			p.problem(it.line, "None. is for an empty part, and ## %s is not empty", p.part)
		}
		p.none[p.part] = true
		return
	}
	if p.none[p.part] {
		p.problem(it.line, "%q comes after None.", clipPlan(it.text))
	}
	if p.part == PlanZ {
		p.z = append(p.z, it.text)
	} else {
		p.early = append(p.early, it.text)
	}
}

func (p *planParser) closeStep(it *planOpen) {
	st := p.step
	if st == nil {
		return
	}
	text := it.text
	if m := planQuoted.FindStringSubmatch(text); m != nil {
		text = strings.TrimRight(strings.TrimSpace(m[1]), " -—–:(")
		st.Quote = strings.TrimSpace(m[2])
		if p.quotes != nil {
			if why := p.quotes(st.Quote); why != "" {
				p.problem(it.line, "%q: %s", clipPlan(st.Quote), why)
			}
		}
	}
	if t, words, ok := typePrefix(text); ok {
		st.Type, st.Who, text = t.Name, t.Who, words
		if text == "" && t.Words == WordsNeeded {
			p.problem(it.line, "step %d is a bare %s step: add what it is for (%d. %s: …)", st.N, t.Name, st.N, t.Name)
		}
	} else if text == "" {
		p.problem(it.line, "step %d has only a quote: say what is done in your words, then the process's in double quotes", st.N)
	}
	st.Text = text
}

// typePrefix is the kind of step a step's line starts with, and its words
// after it: "Work: fix it." is Work, "fix it."; "Open Pull Request" alone
// is that, and no words.
func typePrefix(text string) (StepType, string, bool) {
	for _, t := range Catalogue {
		if len(text) < len(t.Name) || !strings.EqualFold(text[:len(t.Name)], t.Name) {
			continue
		}
		rest := text[len(t.Name):]
		switch {
		case strings.TrimSpace(rest) == "":
			return t, "", true
		case rest[0] == ':' || rest[0] == '.':
			return t, strings.TrimSpace(rest[1:]), true
		}
	}
	return StepType{}, "", false
}

// flagParam reports whether the step being read takes word as a
// parameter alone (- fresh).
func (p *planParser) flagParam(word string) bool {
	if p.step == nil || p.step.Type == "" {
		return false
	}
	t, _ := StepTypeOf(p.step.Type)
	param, ok := t.param(word)
	return ok && param.Flag
}

func (p *planParser) closeField(it *planOpen) {
	st := p.step
	if st == nil {
		return
	}
	t, typed := StepTypeOf(st.Type)
	param, known := t.param(it.field)
	if p.fields[it.field] && !(typed && known && param.Many) {
		p.problem(it.line, "step %d has %s: twice", st.N, it.field)
		return
	}
	p.fields[it.field] = true
	if st.Type == "" {
		p.closeOldField(it)
		return
	}
	if it.field == "who" {
		p.problem(it.line, "step %d is a %s step, which sets who is on it: remove who:", st.N, t.Name)
		return
	}
	if !known {
		p.problem(it.line, "%s: does not apply to a %s step, which takes %s", it.field, t.Name, t.paramKeys())
		return
	}
	if it.text == "" {
		p.problem(it.line, "step %d's %s: is empty: fill it in or remove the line", st.N, it.field)
		return
	}
	if why := param.read(st, it.text); why != "" {
		p.problem(it.line, "step %d's %s", st.N, why)
	}
}

// closeOldField is a field of a step that doesn't start with its kind: a
// plan written before step types, whose steps say who's on them and what
// they make.
func (p *planParser) closeOldField(it *planOpen) {
	st := p.step
	switch it.field {
	case "makes":
		if it.text == "" {
			p.problem(it.line, "step %d's makes: is empty: say what the step produces, or remove the line", st.N)
			return
		}
		st.Makes = it.text
	case "who":
		p.who(it, st)
	default:
		p.problem(it.line, "step %d has no kind, so %s: does not apply: start with one of %s (%d. Work: …)", st.N, it.field, stepTypeNames(), st.N)
	}
}

// who reads a step's who: crew (in a capsule, on a model), you, CI,
// outside (and who, in words), or the planner.
func (p *planParser) who(it *planOpen, st *Step) {
	m := planWhoSplit.FindStringSubmatch(it.text)
	if m == nil {
		p.problem(it.line, "who: %s is not valid: use crew, you, CI, outside or the planner", it.text)
		st.Who = "?"
		return
	}
	name, rest := strings.ToLower(m[1]), strings.TrimSpace(m[2])
	st.Who = "?"
	switch name {
	case "crew":
		st.Who = WhoCrew
		for _, part := range splitWho(rest) {
			word, value, _ := strings.Cut(part, " ")
			value = strings.TrimSpace(value)
			switch strings.ToLower(word) {
			case "in":
				c := planCapsule.FindStringSubmatch(value)
				if c == nil {
					p.problem(it.line, "step %d's crew are in %q: name the capsule as `repo/name`", st.N, value)
					continue
				}
				st.Capsule = c[1]
			case "on":
				if !planModel.MatchString(value) {
					p.problem(it.line, "step %d's crew are on %q: name the model in a word (on haiku)", st.N, value)
					continue
				}
				st.Model = value
			default:
				p.problem(it.line, "step %d's who: crew, then %q: crew takes only in `repo/name` (the capsule) and on <model>", st.N, part)
			}
		}
	case "you":
		st.Who = WhoYou
		if rest != "" {
			p.problem(it.line, "step %d's who: you, then %q: put what to look at in the step, not here", st.N, rest)
		}
	case "ci":
		st.Who, st.Whom = WhoCI, rest
	case "outside":
		st.Who, st.Whom = WhoOutside, rest
	case "the planner", "planner":
		st.Who = WhoPlanner
		if rest != "" {
			p.problem(it.line, "step %d's who: the planner, then %q: put what to plan in the step, not here", st.N, rest)
		}
	default:
		p.problem(it.line, "who: %s is not valid: use crew, you, CI, outside or the planner", it.text)
	}
}

// splitWho is what follows crew in a who:, by its commas: "in `x/y`",
// "on haiku".
func splitWho(rest string) []string {
	var out []string
	for part := range strings.SplitSeq(rest, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

// oldType is the kind of step a step of a plan written before step types
// is, by who's on it.
func oldType(who string) string {
	switch who {
	case WhoCrew:
		return StepWork
	case WhoYou:
		return StepReview
	case WhoCI, WhoOutside:
		return StepWait
	case WhoPlanner:
		return StepPlan
	}
	return ""
}

// carryingOn holds each step that carries on from another to one before
// it whose crew it can be: a Work step's.
func (p *planParser) carryingOn() {
	for _, st := range p.steps {
		switch {
		case st.CarryOn == 0:
		case st.Fresh:
			p.problem(st.Line, "step %d carries on from step %d and is fresh: choose one", st.N, st.CarryOn)
		case st.CarryOn >= st.N:
			p.problem(st.Line, "step %d carries on from step %d, which is not before it: carry on only from an earlier step", st.N, st.CarryOn)
		case p.steps[st.CarryOn-1].Type != StepWork:
			p.problem(st.Line, "step %d carries on from step %d, a %s step: carry on only from a Work step", st.N, st.CarryOn, p.steps[st.CarryOn-1].Type)
		}
	}
}

// inOrInEach holds a step to one repo (in:) or several (in each:), not
// both.
func (p *planParser) inOrInEach() {
	for _, st := range p.steps {
		if st.InEach != "" && st.Capsule != "" {
			p.problem(st.Line, "step %d has both in: and in each: use in: for one repo, in each: for several", st.N)
		}
	}
}

// endStep keeps the step being read, once it's all been read.
func (p *planParser) endStep() {
	st := p.step
	p.step = nil
	if st == nil {
		return
	}
	if st.Type == StepVerify && len(st.Checks) == 0 {
		p.problem(st.Line, "step %d verifies nothing: add a - check: for each thing to show (or - yours: for one the user settles)", st.N)
	}
	switch {
	case st.Type != "":
		if st.Text == "" {
			st.Text, st.Bare = st.Type, true
			if st.Capsule != "" {
				st.Text += " in `" + st.Capsule + "`"
			}
			if st.InEach != "" {
				st.Text += " in each " + strings.TrimSuffix(st.InEach, ".")
			}
		}
	case st.Who == "":
		p.problem(st.Line, "step %d has no kind: start with one of %s, then what it is for (%d. Work: …)", st.N, stepTypeNames(), st.N)
	case st.Who != "?":
		st.Type = oldType(st.Who)
	}
	p.steps = append(p.steps, *st)
}

func isNone(s string) bool {
	s = strings.ToLower(strings.TrimRight(strings.TrimSpace(s), "."))
	return s == "none" || s == "nothing"
}

func clipPlan(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 80 {
		return s[:79] + "…"
	}
	return s
}

// PlanPath is the path of a mission's plan.
func (s Store) PlanPath(slug string) string { return filepath.Join(s.missionDir(slug), planFile) }

// ReadPlan returns a mission's plan as written, or "" if it has none.
func (s Store) ReadPlan(slug string) (string, error) {
	data, err := os.ReadFile(s.PlanPath(slug))
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	return string(data), err
}

// WritePlan replaces a mission's plan: written as it reads, which ParsePlan
// has read. "" removes it, and the mission works from its brief's stages.
func (s Store) WritePlan(slug, plan string) error {
	if _, err := s.Load(slug); err != nil {
		return err
	}
	return s.withFiles(slug, func() error { return s.writePlan(slug, plan) })
}

// withPlan reads a mission's plan into it, as it's loaded: its steps are
// its route (StageProgress). A plan that doesn't read (changed by hand,
// outside ws) is kept as PlanErr, and the mission works from its brief.
func (s Store) withPlan(m *Mission) *Mission {
	if m == nil {
		return m
	}
	text, err := s.ReadPlan(m.Slug)
	switch {
	case err != nil:
		m.PlanErr = err
	case strings.TrimSpace(text) != "":
		m.Plan, m.PlanErr = ParsePlan(text, nil)
	}
	return m
}
