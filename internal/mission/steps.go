package mission

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// The catalogue of plan steps: the kinds of step a plan is made of, which
// the planner assembles a plan from and ws expands into what each takes
// (crew, the user's part, waiting on what happens). The planner sees each
// type's name, what it's for and its parameters, as PlanFormat says them;
// what a step does once it's reached is ws's, the same for every mission
// (ops' reachStep). A type is added here, with its expansion, when a real
// mission can't be planned without it.

// The step types.
const (
	StepWork   = "Work"
	StepVerify = "Verify"
	StepReview = "Review"
	StepDecide = "Decide"
	StepOpenPR = "Open Pull Request"
	StepLand   = "Land"
	StepWait   = "Wait"
	StepPlan   = "Plan"
)

// StepType is a kind of plan step: its name as a plan writes it, what it's
// for, who's on it once it's reached, and the parameters a step of it
// takes, each an indented "- key: value" under it.
type StepType struct {
	Name  string
	About string
	Who   string
	// Words says whether a step of the type says what's done in words
	// after its name ("Work: fix it"): required, or (WordsMay) welcome.
	Words  int
	Params []StepParam
}

// Whether a step type takes words after its name.
const (
	WordsNeeded = iota
	WordsMay
)

// StepParam is a parameter a step type takes: its key, what it says, and
// how it's read into the step ("" when it reads, else what's wrong).
type StepParam struct {
	Key   string
	About string
	// Many: a step may say it more than once (- check:). Flag: it's a
	// word alone (- fresh).
	Many, Flag bool
	read       func(st *Step, value string) string
}

// Catalogue is every step type, in the order the planner's told them.
var Catalogue = []StepType{
	{Name: StepWork, Who: WhoCrew, Words: WordsNeeded,
		About:  "crew (agents) do a piece of work in a repo: investigate, write a failing test, fix it, write a document. One step is one crew's session, so make each end in something to look at. They report done, with a handoff; then ws runs its checks, and has an agent who didn't do the work judge those in words. All met, it's done; unmet, it goes back to the crew or to the user, as the process says. In a repo the mission has no capsule in, ws lifts one for it.",
		Params: []StepParam{paramIn, paramInEach, paramCarryOn, paramFresh, paramOn, paramLimit, paramMakes, paramCheck, paramOnlyIf}},
	{Name: StepVerify, Who: WhoCrew, Words: WordsNeeded,
		About:  "a verifier (an agent who didn't do the work) shows that the work before it is as its checks say, by using it: it runs it, looks, and hands over a verification bundle, each check with what it came to and what shows it (screenshots, a recording, what a command printed). Say what's to be shown, and where the process says how it's shown (a script, a skill, what to capture), say that too: the verifier isn't given the process. It changes nothing. All met, or left for the user, it's done; unmet, the work goes back to the crew who made it, and is verified again once they report done. Put a Review after it where the user approves what it shows.",
		Params: []StepParam{paramIn, paramOn, paramLimit, paramCheck, paramYours, paramOnlyIf}},
	{Name: StepReview, Who: WhoYou, Words: WordsNeeded,
		About:  "the user looks at what the steps before made (a diff, a document, the findings) and goes on, or not: a checkpoint, which nothing else passes. Say what they look at.",
		Params: []StepParam{paramOnlyIf}},
	{Name: StepDecide, Who: WhoYou, Words: WordsNeeded,
		About:  "the user decides something only they can: say the question. Their answer is recorded as a decision, and passes the step.",
		Params: []StepParam{paramFrom, paramOnlyIf}},
	{Name: StepOpenPR, Who: WhoWS, Words: WordsMay,
		About:  "ws opens a pull request from the mission's capsule in a repo, once the work before it is committed, and the step is done when it merges. Crew at other steps commit and don't open one.",
		Params: []StepParam{paramIn, paramInEach, paramDraft, paramOnlyIf}},
	{Name: StepLand, Who: WhoYou, Words: WordsMay,
		About:  "the user lands the mission's capsule in a repo that has no pull requests (its commits go straight to its default branch). The step is done once it's landed.",
		Params: []StepParam{paramIn, paramOnlyIf}},
	{Name: StepWait, Who: WhoOutside, Words: WordsNeeded,
		About:  "the mission waits on something outside ws: a deploy, a teammate's review, another team. Say what it waits for. It's passed once it's happened.",
		Params: []StepParam{paramOnlyIf}},
	{Name: StepPlan, Who: WhoPlanner, Words: WordsMay,
		About:  "the planner writes the steps after this one, from what the steps before it made: for what can't be planned until they've run (the fix, once the cause is found).",
		Params: []StepParam{paramOnlyIf}},
}

// StepTypeOf is the step type named, as written or in any case.
func StepTypeOf(name string) (StepType, bool) {
	name = strings.Join(strings.Fields(name), " ")
	for _, t := range Catalogue {
		if strings.EqualFold(t.Name, name) {
			return t, true
		}
	}
	return StepType{}, false
}

// stepTypeNames is the catalogue's names, for saying what a step can be.
func stepTypeNames() string {
	names := make([]string, len(Catalogue))
	for i, t := range Catalogue {
		names[i] = t.Name
	}
	return strings.Join(names, ", ")
}

// param is the type's parameter with key, if it takes it.
func (t StepType) param(key string) (StepParam, bool) {
	for _, p := range t.Params {
		if p.Key == key {
			return p, true
		}
	}
	return StepParam{}, false
}

func (t StepType) paramKeys() string {
	if len(t.Params) == 0 {
		return "nothing under it"
	}
	keys := make([]string, len(t.Params))
	for i, p := range t.Params {
		keys[i] = "- " + p.Key + ":"
		if p.Flag {
			keys[i] = "- " + p.Key
		}
	}
	return strings.Join(keys, ", ")
}

var (
	paramIn = StepParam{Key: "in", About: "the repo it's in, in backticks (`api`), or one of the mission's capsules (`api/fix-rounding`)",
		read: func(st *Step, v string) string {
			c := planIn.FindStringSubmatch(v)
			if c == nil {
				return fmt.Sprintf("in: %q isn't a repo or a capsule: name the repo (`api`), or the capsule as `repo/name`", v)
			}
			st.Capsule = c[1]
			return ""
		}}
	paramOn = StepParam{Key: "on", About: "what its crew run on, as harness:model (claude:opus), or a model alone, when it's not crew's own: one of what ws holds crew may run on ([crew] allowed), as the process prefers",
		read: func(st *Step, v string) string {
			if !planModel.MatchString(v) {
				return fmt.Sprintf("on: %q isn't a harness and model: write it harness:model (claude:opus), or a model alone (opus)", v)
			}
			st.Model = v
			return ""
		}}
	paramMakes = StepParam{Key: "makes", About: "what it produces: a document of a kind, findings, a failing test. Not a pull request: an Open Pull Request step opens that",
		read: func(st *Step, v string) string {
			if makesPR.MatchString(v) {
				return fmt.Sprintf("makes: %q: a pull request is opened by an Open Pull Request step, after this one, not by crew: say what the work makes, and put that step after it", v)
			}
			st.Makes = v
			return ""
		}}
	paramOnlyIf = StepParam{Key: "only if", About: "when the step applies, in words (the fix isn't obvious), for a step that doesn't always: the orchestrator starts it or skips it, saying why. A step without it always runs",
		read: func(st *Step, v string) string {
			st.OnlyIf = v
			return ""
		}}
	paramInEach = StepParam{Key: "in each", About: "the repos it runs in, in words, for a step that runs in several (repo of the workspace; repo where step 1 made changes): the orchestrator names them when the step starts, and it runs once in each, the step done when every one is. In place of in:",
		read: func(st *Step, v string) string {
			st.InEach = v
			return ""
		}}
	paramCarryOn = StepParam{Key: "carrying on from", About: "a Work step before it whose crew carry on with this one, knowing what they found (1): resumed where they can be, else fresh crew with their handoff. Without it, its crew start fresh",
		read: func(st *Step, v string) string {
			m := carryOnStep.FindStringSubmatch(strings.TrimSpace(v))
			if m == nil {
				return fmt.Sprintf("carrying on from: %q: name the step by its number (1)", v)
			}
			st.CarryOn, _ = strconv.Atoi(m[1])
			return ""
		}}
	paramFresh = StepParam{Key: "fresh", Flag: true, About: "fresh crew, given the handoffs before them: the default, worth saying where carrying on might be expected",
		read: func(st *Step, v string) string {
			st.Fresh = true
			return ""
		}}
	paramLimit = StepParam{Key: "limit", About: "how long its crew have, as 45m or 1h, with words after it if they help (1h, set lower by crew if they like): ws holds the clock from when they start, and says once when it runs out",
		read: func(st *Step, v string) string {
			d, _, ok := ParseLimit(v)
			if !ok {
				return fmt.Sprintf("limit: %q: start with how long, as 45m, 1h or 1h30m (words after it are welcome)", v)
			}
			st.Limit, st.LimitText = d, v
			return ""
		}}
	paramCheck = StepParam{Key: "check", Many: true, About: "what has to be true for it to be done, a line each: a command in backticks, then passes or fails (`go test ./...` passes), which ws runs in the step's capsule; or anything else, in words (the cause is named, with where and why), which an agent who didn't do the work settles: the judge, reading it, for a Work step; its verifier, using it, for a Verify step",
		read: func(st *Step, v string) string {
			c, why := parseCheck(v)
			if why != "" {
				return why
			}
			st.Checks = append(st.Checks, c)
			return ""
		}}
	paramYours = StepParam{Key: "yours", Many: true, About: "a check the verifier doesn't settle, in words: a matter of taste or judgement the process keeps for the user (it reads well at phone width). The verifier leaves what helps them settle it, and it's theirs when they look at the bundle",
		read: func(st *Step, v string) string {
			c, why := parseCheck(v)
			if why != "" {
				return why
			}
			if c.Run() {
				return fmt.Sprintf("yours: %q is a command, which ws runs itself: a check that's the user's is in words", v)
			}
			c.Yours = true
			st.Checks = append(st.Checks, c)
			return ""
		}}
	paramFrom = StepParam{Key: "from", About: "where the options are, if they're written somewhere (the options in `mission:findings.md`)",
		read: func(st *Step, v string) string {
			st.From = v
			return ""
		}}
	paramDraft = StepParam{Key: "draft", About: "yes, to open it as a draft (no is the default)",
		read: func(st *Step, v string) string {
			switch strings.ToLower(strings.TrimRight(v, ".")) {
			case "yes", "true":
				st.Draft = true
			case "no", "false":
				st.Draft = false
			default:
				return fmt.Sprintf("draft: %q: say yes or no", v)
			}
			return ""
		}}
)

// carryOnStep is the step a step carries on from: 1, step 1.
var carryOnStep = regexp.MustCompile(`(?i)^(?:step\s+)?(\d+)\.?$`)

// catalogueFormat is the catalogue as the plan's format says it.
func catalogueFormat() string {
	var b strings.Builder
	for _, t := range Catalogue {
		fmt.Fprintf(&b, "- **%s**: %s\n", t.Name, t.About)
		for _, p := range t.Params {
			key := p.Key + ":"
			if p.Flag {
				key = p.Key
			}
			fmt.Fprintf(&b, "  - `- %s` %s\n", key, p.About)
		}
	}
	return b.String()
}
