package mission

import (
	"strings"
	"testing"
	"time"
)

// bugPlan is the design's bug plan, in the plan's syntax.
const bugPlan = `## Z
- Merged, with its regression test.
- The user has said whether it goes to pre and prd.

## Stops early
- It can't be reproduced, or isn't a bug: that's an answer for product.

## Steps
1. Reproduce it as a failing test. "Bugs are reproduced first"
   - who: crew, in ` + "`api/invoice-total`" + `
   - makes: the failing test,
     and the cause
2. Is this the cause?
   - who: you
3. Plan the fix.
   - who: the planner
4. Fix it, and open a pull request.
   - who: crew, in ` + "`api/invoice-total`" + `, on haiku
   - makes: a pull request
5. Merged.
   - who: outside: a teammate's review
6. Hotfix to pre and prd, or done? "we might need to hotfix them"
   - Who: You
`

// says is a process that has the bug's words.
func says(quote string) string {
	for _, words := range []string{"Bugs are reproduced first", "we might need to hotfix them"} {
		if strings.EqualFold(quote, words) {
			return ""
		}
	}
	return "isn't in the process"
}

func TestParsePlanReadsEachPart(t *testing.T) {
	p, err := ParsePlan(bugPlan, says)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Z) != 2 || p.Z[1] != "The user has said whether it goes to pre and prd." {
		t.Errorf("Z = %q", p.Z)
	}
	if len(p.Early) != 1 {
		t.Errorf("stops early = %q", p.Early)
	}
	if len(p.Steps) != 6 {
		t.Fatalf("steps = %+v", p.Steps)
	}
	first := p.Steps[0]
	if first.N != 1 || first.Text != "Reproduce it as a failing test." || first.Quote != "Bugs are reproduced first" || first.Who != WhoCrew || first.Capsule != "api/invoice-total" || first.Makes != "the failing test, and the cause" || first.Line != 9 {
		t.Errorf("step 1 = %+v", first)
	}
	if s := p.Steps[1]; s.Who != WhoYou || !s.Checkpoint() || s.Quote != "" {
		t.Errorf("step 2, a checkpoint = %+v", s)
	}
	if s := p.Steps[2]; s.Who != WhoPlanner {
		t.Errorf("step 3, the planner's = %+v", s)
	}
	if s := p.Steps[3]; s.Model != "haiku" || s.Capsule != "api/invoice-total" || s.WhoSays() != "crew, in `api/invoice-total`, on haiku" {
		t.Errorf("step 4, on a model = %+v (%s)", s, s.WhoSays())
	}
	if s := p.Steps[4]; s.Who != WhoOutside || s.Whom != "a teammate's review" || s.Whose() != "outside ws's" {
		t.Errorf("step 5, outside = %+v", s)
	}
	if s := p.Steps[5]; s.Who != WhoYou || s.Text != "Hotfix to pre and prd, or done?" {
		t.Errorf("step 6, who: in capitals = %+v", s)
	}
	if !strings.HasPrefix(p.Text, "## Z\n") || !strings.HasSuffix(p.Text, "- Who: You\n") {
		t.Errorf("text = %q", p.Text)
	}
}

func TestParsePlanTakesNoneAndNoQuotesChecked(t *testing.T) {
	p, err := ParsePlan("## Steps\n1. Bump it. \"words of no process\"\n   - who: crew\n## Z\n- Merged.\n## Stops early:\n- None.\n", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Early) != 0 || p.Steps[0].Quote != "words of no process" {
		t.Errorf("got %+v", p)
	}
}

// TestParsePlanRefusesWithWhy: each malformed plan is refused, saying
// what's wrong and where.
func TestParsePlanRefusesWithWhy(t *testing.T) {
	steps := "## Steps\n1. Do it.\n   - who: crew\n"
	head := "## Z\n- Done.\n## Stops early\n- None.\n"
	for _, c := range []struct {
		name, plan string
		want       []string
	}{
		{"empty", "", []string{"missing ## Z", "missing ## Stops early", "missing ## Steps"}},
		{"a title", "# The plan\n" + head + steps, []string{"line 1: # The plan: no titles"}},
		{"text before any part", "Here's the plan.\n" + head + steps, []string{`line 1: "Here's the plan." is not a list item`}},
		{"an unknown part", head + steps + "## Risks\n- Some.\n", []string{"## Risks is not a plan part"}},
		{"a part twice", head + steps + "## Z\n- Again.\n", []string{"## Z appears twice"}},
		{"no Z", "## Stops early\n- None.\n" + steps, []string{"missing ## Z"}},
		{"Z of nothing", "## Z\n## Stops early\n- None.\n" + steps, []string{"## Z is empty"}},
		{"Z as None", "## Z\n- None.\n## Stops early\n- None.\n" + steps, []string{"## Z cannot be None."}},
		{"no stops early", "## Z\n- Done.\n" + steps, []string{"missing ## Stops early: add one, with None. under it"}},
		{"something after None", "## Z\n- Done.\n## Stops early\n- None.\n- Or not.\n" + steps, []string{`line 5: "Or not." comes after None.`}},
		{"no steps", head + "## Steps\n", []string{"## Steps is empty"}},
		{"a step not numbered", head + "## Steps\n- Do it.\n   - who: crew\n", []string{`line 6: "- Do it." is not numbered`}},
		{"steps out of order", head + "## Steps\n1. One.\n   - who: crew\n3. Three.\n   - who: crew\n", []string{"line 8: step 3 where step 2 should be"}},
		{"no who", head + "## Steps\n1. Do it.\n   - makes: a thing\n", []string{"line 6: step 1 has no kind: start with one of Work, Verify, Review"}},
		{"who twice", head + "## Steps\n1. Do it.\n   - who: crew\n   - who: you\n", []string{"line 8: step 1 has who: twice"}},
		{"an unknown who", head + "## Steps\n1. Do it.\n   - who: the team\n", []string{"who: the team is not valid"}},
		{"an unknown field", head + "## Steps\n1. Do it.\n   - who: crew\n   - when: Monday\n", []string{"line 8: step 1 has no kind, so when: does not apply"}},
		{"not a field", head + "## Steps\n1. Do it.\n   - who: crew\n   - and then this\n", []string{`line 8: "- and then this" is not a step parameter`}},
		{"crew in no capsule", head + "## Steps\n1. Do it.\n   - who: crew, in the api\n", []string{`step 1's crew are in "the api": name the capsule as` + " `repo/name`"}},
		{"crew with more", head + "## Steps\n1. Do it.\n   - who: crew, quickly\n", []string{`step 1's who: crew, then "quickly"`}},
		{"you with more", head + "## Steps\n1. Look.\n   - who: you, and a teammate\n", []string{`step 1's who: you, then "and a teammate"`}},
		{"only a quote", head + "## Steps\n1. \"Bugs are reproduced first\"\n   - who: crew\n", []string{"line 6: step 1 has only a quote"}},
		{"a quote not the process's", head + "## Steps\n1. Do it. \"words of no process\"\n   - who: crew\n", []string{`line 6: "words of no process": isn't in the process`}},
		{"makes twice", head + "## Steps\n1. Do it.\n   - who: crew\n   - makes: a\n   - makes: b\n", []string{"line 9: step 1 has makes: twice"}},
		{"a list in Z", "## Z\n- Done.\n  - and dusted\n## Stops early\n- None.\n" + steps, []string{`line 3: "- and dusted" is a nested list`}},
		{"several at once", "# Plan\n## Z\n- Done.\n## Steps\n2. Do it.\n", []string{"line 1: # Plan", "line 5: step 2 where step 1 should be", "line 5: step 1 has no kind", "missing ## Stops early"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, err := ParsePlan(c.plan, says)
			if err == nil {
				t.Fatalf("not refused:\n%s", c.plan)
			}
			if !IsMalformedPlan(err) {
				t.Errorf("refused, but not as malformed: %v", err)
			}
			for _, w := range c.want {
				if !strings.Contains(err.Error(), w) {
					t.Errorf("why %q isn't in:\n%v", w, err)
				}
			}
		})
	}
}

func TestStepMakesAPullRequest(t *testing.T) {
	for makes, want := range map[string]bool{
		"a pull request":                     true,
		"the fix, and its pull request":      true,
		"Pull Requests, one a repo":          true,
		"a PR":                               true,
		"the failing test, and the cause":    false,
		"a Build Proposal (prd, proposals/)": false,
		"":                                   false,
	} {
		if got := (Step{Makes: makes}).MakesPR(); got != want {
			t.Errorf("makes: %q makes a pull request = %v, want %v", makes, got, want)
		}
	}
}

func TestPlanIsTheRoute(t *testing.T) {
	plan, err := ParsePlan(bugPlan, nil)
	if err != nil {
		t.Fatal(err)
	}
	m := &Mission{Plan: plan, Stages: []StageRecord{{Text: "Reproduce it as a failing test.", N: 1, Of: 6, Crew: []string{"c1"}}}}
	// A brief's stages give way to the plan's steps.
	states, orphans := m.StageProgress("## Stages\n1. Something else.\n")
	if len(states) != 6 || len(orphans) != 0 {
		t.Fatalf("states = %d, orphans = %d", len(states), len(orphans))
	}
	first := states[0]
	if first.Step == nil || first.Who() != WhoCrew || first.Noun() != "step" || first.Record == nil || first.Record.Crew[0] != "c1" || !first.Current {
		t.Errorf("step 1 = %+v", first)
	}
	if got := first.NamedCapsules(); len(got) != 1 || got[0] != "api/invoice-total" {
		t.Errorf("step 1's capsules = %q", got)
	}
	if got := first.Whole(); got != "Reproduce it as a failing test.\n\nIt makes: the failing test, and the cause." {
		t.Errorf("step 1 as crew are given it = %q", got)
	}
	if states[1].Who() != WhoYou {
		t.Errorf("step 2's who = %s", states[1].Who())
	}
	// Without a plan, the brief's stages are the route.
	m.Plan = nil
	if states, _ := m.StageProgress("## Stages\n1. Something else.\n"); len(states) != 1 || states[0].Step != nil || states[0].Who() != WhoCrew || states[0].Noun() != "stage" {
		t.Errorf("without a plan = %+v", states)
	}
}

// typedPlan is the bug's plan as the planner writes it now: each step a
// kind from the catalogue, with its parameters.
const typedPlan = `## Z
- Merged, with its regression test.

## Stops early
- It can't be reproduced: that's an answer for product.

## Steps
1. Work: reproduce it as a failing test. "Bugs are reproduced first"
   - in: ` + "`api`" + `
   - on: claude:opus
   - makes: the failing test, and the cause
2. Review: the failing test, and is this the cause?
3. Decide: which fix?
   - from: the options in ` + "`mission:findings.md`" + `
4. Work: fix it.
   - in: ` + "`api/invoice-total`" + `
5. Open Pull Request
   - in: ` + "`api`" + `
   - draft: yes
6. Land
7. Wait: for the deploy to prd.
8. plan
`

func TestParsePlanReadsKindsOfStep(t *testing.T) {
	p, err := ParsePlan(typedPlan, says)
	if err != nil {
		t.Fatal(err)
	}
	want := []struct{ typ, who, text string }{
		{StepWork, WhoCrew, "reproduce it as a failing test."},
		{StepReview, WhoYou, "the failing test, and is this the cause?"},
		{StepDecide, WhoYou, "which fix?"},
		{StepWork, WhoCrew, "fix it."},
		{StepOpenPR, WhoWS, "Open Pull Request in `api`"},
		{StepLand, WhoYou, "Land"},
		{StepWait, WhoOutside, "for the deploy to prd."},
		{StepPlan, WhoPlanner, "Plan"},
	}
	if len(p.Steps) != len(want) {
		t.Fatalf("steps = %+v", p.Steps)
	}
	for i, w := range want {
		if st := p.Steps[i]; st.Type != w.typ || st.Who != w.who || st.Text != w.text {
			t.Errorf("step %d = %s, %s, %q; want %s, %s, %q", i+1, st.Type, st.Who, st.Text, w.typ, w.who, w.text)
		}
	}
	if st := p.Steps[0]; st.Capsule != "api" || st.Model != "claude:opus" || st.Makes != "the failing test, and the cause" || st.Quote != "Bugs are reproduced first" || st.MakesPR() {
		t.Errorf("step 1 = %+v", st)
	}
	if st := p.Steps[2]; st.From != "the options in `mission:findings.md`" || !st.Checkpoint() {
		t.Errorf("step 3 = %+v", st)
	}
	if st := p.Steps[3]; st.Capsule != "api/invoice-total" {
		t.Errorf("step 4 = %+v", st)
	}
	if st := p.Steps[4]; !st.MakesPR() || !st.Draft || st.Capsule != "api" || st.Whose() != "ws's" || st.Checkpoint() {
		t.Errorf("step 5 = %+v", st)
	}
}

// A plan written before kinds of step reads as it did, each step the kind
// its who: is.
func TestParsePlanReadsOneFromBeforeKinds(t *testing.T) {
	p, err := ParsePlan(bugPlan, says)
	if err != nil {
		t.Fatal(err)
	}
	for i, typ := range []string{StepWork, StepReview, StepPlan, StepWork, StepWait, StepReview} {
		if p.Steps[i].Type != typ {
			t.Errorf("step %d is %q, want %q", i+1, p.Steps[i].Type, typ)
		}
	}
	if st := p.Steps[3]; !st.MakesPR() || st.Who != WhoCrew {
		t.Errorf("step 4, crew who make a pull request = %+v", st)
	}
}

func TestParsePlanRefusesKindsWithWhy(t *testing.T) {
	head := "## Z\n- Done.\n## Stops early\n- None.\n## Steps\n"
	for _, c := range []struct{ name, steps, want string }{
		{"no words", "1. Work\n", "step 1 is a bare Work step"},
		{"no words after a colon", "1. Review:\n", "step 1 is a bare Review step"},
		{"a parameter it doesn't take", "1. Review: the diff.\n   - in: `api`\n", "in: does not apply to a Review step, which takes - only if:"},
		{"who: on a kind", "1. Work: fix it.\n   - who: crew\n", "step 1 is a Work step, which sets who is on it: remove who:"},
		{"a pull request made by work", "1. Work: fix it.\n   - makes: a pull request\n", "pull requests come from an Open Pull Request step"},
		{"in no repo", "1. Work: fix it.\n   - in: the api\n", `in: "the api" is not a repo or capsule`},
		{"a draft of maybe", "1. Open Pull Request\n   - draft: maybe\n", `draft: "maybe": say yes or no`},
		{"a parameter twice", "1. Work: fix it.\n   - in: `api`\n   - in: `web`\n", "step 1 has in: twice"},
		{"an empty parameter", "1. Work: fix it.\n   - on:\n", "step 1's on: is empty"},
		{"a kind it isn't", "1. Deploy: to prd.\n", "step 1 has no kind"},
		{"a limit of no length", "1. Work: fix it.\n   - limit: an hour\n", `limit: "an hour": start with a duration`},
		{"a command with more", "1. Work: fix it.\n   - check: `go test ./...` passes on main\n", `put "on main" in a check of its own`},
		{"carrying on from nothing", "1. Work: fix it.\n   - carrying on from: the start\n", `carrying on from: "the start": name the step by its number`},
		{"carrying on from ahead", "1. Work: fix it.\n   - carrying on from: 2\n2. Work: more.\n", "step 1 carries on from step 2, which is not before it"},
		{"carrying on from a review", "1. Review: it.\n2. Work: fix it.\n   - carrying on from: 1\n", "step 2 carries on from step 1, a Review step"},
		{"carrying on, fresh", "1. Work: find it.\n2. Work: fix it.\n   - carrying on from: 1\n   - fresh\n", "step 2 carries on from step 1 and is fresh: choose one"},
		{"a word alone it doesn't take", "1. Review: it.\n   - fresh\n", `"- fresh" is not a step parameter`},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, err := ParsePlan(head+c.steps, nil)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("want %q, got %v", c.want, err)
			}
		})
	}
}

// The format the planner's told names every kind of step, and what each
// takes, from the catalogue.
func TestPlanFormatHasTheCatalogue(t *testing.T) {
	for _, typ := range Catalogue {
		if !strings.Contains(PlanFormat, "**"+typ.Name+"**") {
			t.Errorf("the format doesn't name %s", typ.Name)
		}
		for _, p := range typ.Params {
			key := p.Key + ":"
			if p.Flag {
				key = p.Key
			}
			if !strings.Contains(PlanFormat, "`- "+key+"`") {
				t.Errorf("the format doesn't say %s takes %s:", typ.Name, p.Key)
			}
		}
	}
	if _, err := ParsePlan(PlanFormat[strings.Index(PlanFormat, "## Z"):strings.Index(PlanFormat, "```\n\n")], nil); err != nil {
		t.Errorf("the format's own example doesn't read: %v", err)
	}
}

// A Work step says what has to be true for it to be done (checks, a line
// each: commands ws runs, words the judge judges), whose crew it carries
// on from, and how long it has.
func TestParsePlanReadsWorksExpansion(t *testing.T) {
	p, err := ParsePlan(`## Z
- Fixed.
## Stops early
- None.
## Steps
1. Work: find what causes it.
   - in: `+"`api`"+`
   - limit: 1h, set lower by crew if they like
   - check: the cause is named, with where in the code and why
2. Work: reproduce it as a failing test.
   - carrying on from: step 1
   - check: `+"`go test ./invoice -run TestTotalRounding` fails"+`
   - check: the test exercises the cause in the findings
3. Work: fix it.
   - in: `+"`api`"+`
   - fresh
   - limit: 1h30m
   - check: `+"`go test ./...` passes."+`
`, nil)
	if err != nil {
		t.Fatal(err)
	}
	one, two, three := p.Steps[0], p.Steps[1], p.Steps[2]
	if one.Limit != time.Hour || one.LimitText != "1h, set lower by crew if they like" || len(one.Checks) != 1 || one.Checks[0].Run() || one.CarryOn != 0 {
		t.Errorf("step 1 = %+v", one)
	}
	if two.CarryOn != 1 || len(two.Checks) != 2 || two.Checks[0].Command != "go test ./invoice -run TestTotalRounding" || two.Checks[0].Expect != CheckFails || two.Checks[1].Run() {
		t.Errorf("step 2 = %+v", two)
	}
	if !three.Fresh || three.Limit != 90*time.Minute || len(three.Checks) != 1 || three.Checks[0].Expect != CheckPasses || three.Checks[0].Command != "go test ./..." {
		t.Errorf("step 3 = %+v", three)
	}
}

func TestParseLimit(t *testing.T) {
	for _, c := range []struct {
		in    string
		d     time.Duration
		words string
		ok    bool
	}{
		{"45m", 45 * time.Minute, "", true},
		{"1h, set lower by crew if they like", time.Hour, "set lower by crew if they like", true},
		{"1h30m", 90 * time.Minute, "", true},
		{"2 hours", 2 * time.Hour, "", true},
		{"an hour", 0, "", false},
		{"0m", 0, "", false},
	} {
		d, words, ok := ParseLimit(c.in)
		if d != c.d || words != c.words || ok != c.ok {
			t.Errorf("ParseLimit(%q) = %v, %q, %v", c.in, d, words, ok)
		}
	}
	if FormatLimit(90*time.Minute) != "1h30m" || FormatLimit(time.Hour) != "1h" || FormatLimit(45*time.Minute) != "45m" {
		t.Error("FormatLimit")
	}
}
