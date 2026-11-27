package prompt

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/withworkspace/workspace/internal/mission"
)

var update = flag.Bool("update", false, "rewrite the golden files from what the prompts say now")

// golden compares a prompt with its golden file, so a change to what an
// agent's told shows in a diff: -update rewrites them.
func golden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name+".golden")
	if *update {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (go test ./internal/prompt -update writes it)", err)
	}
	if string(want) != got {
		t.Errorf("%s isn't what %s says: go test ./internal/prompt -update, and look at the diff\n%s", name, path, got)
	}
}

func TestCrew_Golden(t *testing.T) {
	golden(t, "crew-work", Crew(CrewBrief{Slug: "cache", Title: "Cache lookups", Brief: "## Goal\nCache the lookup.\n",
		Dispatch: "Add eviction", Stage: "Build it in `api/cache`.", StageN: 2, StageOf: 3,
		LeftOff: "## Where earlier crew left off\n\nc1's handoff", Guide: "Read GUIDE.md first.", RunChecks: []string{"go test ./...", "cd web && pnpm test"}}))
	golden(t, "crew-briefing", Crew(CrewBrief{Slug: "fe-issue-45", Role: "briefing", Request: "fe issue 45", Org: "acme", Conversation: "t3"}))
	golden(t, "crew-answer", Crew(CrewBrief{Slug: "cache", Title: "Cache lookups", Role: "answer",
		Question: "Redis or in memory?", Options: []string{"Redis", "In memory"}}))
	// Crew in a repo with recipes are told them, by name and command.
	golden(t, "crew-recipes", Crew(CrewBrief{Slug: "cache", Title: "Cache lookups", Recipes: []Recipe{
		{Name: "test", Type: "task", Command: "go test ./..."},
		{Name: "web", Type: "service", Command: "pnpm dev --port {port}", Description: "The storefront, on its dev server."},
	}}))
}

func TestBriefingPrompt(t *testing.T) {
	p := Crew(CrewBrief{Slug: "fe-issue-45", Role: "briefing", Request: "fe issue 45", Org: "acme"})
	for _, want := range []string{
		"NOT doing the work",
		"fe issue 45",
		"acme/<repo>",
		"gh issue view",
		"ws mission brief --set -",
		"ws mission title",
		"ws dock <repo> <branch|PR>",
		"ws mission ready",
		"Don't change any code",
	} {
		if !strings.Contains(p, want) {
			t.Errorf("briefing prompt missing %q", want)
		}
	}
	if work := Crew(CrewBrief{Slug: "x", Title: "X"}); strings.Contains(work, "ws mission ready") {
		t.Error("work crew prompt mentions ws mission ready")
	}
}

func TestWorkPromptTeachesSignals(t *testing.T) {
	p := Crew(CrewBrief{Slug: "m", Title: "M"})
	for _, want := range []string{"ws mission ask", "ws mission done", "ws mission artifact add", "ws review show -f llm", "ws review addressed <id>", "ws review wontfix <id>", "commit it"} {
		if !strings.Contains(p, want) {
			t.Errorf("work prompt missing %q", want)
		}
	}
}

// Crew asked to answer the orchestrator's question get the question, not
// the whole brief, and are told to answer with ws mission answer and stop.
func TestAnswerPrompt(t *testing.T) {
	p := Crew(CrewBrief{
		Slug: "hello", Title: "Hello", Role: "answer",
		Brief:    "## Goal\nA long brief nobody needs here.",
		Question: "Commit and open PRs, or leave them?", Options: []string{"Commit both and open PRs", "Leave them uncommitted"},
	})
	for _, want := range []string{
		"Commit and open PRs, or leave them?", "1. Commit both and open PRs", "2. Leave them uncommitted",
		`ws mission answer "<the answer>"`, "ws show -f llm", "Don't do the work the answer leads to",
	} {
		if !strings.Contains(p, want) {
			t.Errorf("prompt missing %q:\n%s", want, p)
		}
	}
	if strings.Contains(p, "A long brief nobody needs here") || strings.Contains(p, "ws mission done") {
		t.Errorf("answer prompt pastes the brief or asks for done:\n%s", p)
	}
}

// Work crew are taught to leave a handoff, and new crew get where earlier
// crew left off after what they're dispatched for and before the brief.
func TestWorkPrompt_HandoffsInAndOut(t *testing.T) {
	p := Crew(CrewBrief{Slug: "m", Title: "M", Dispatch: "Add eviction", LeftOff: "## Where earlier crew left off\n\nc1's handoff", Brief: "## Goal\nCache it."})
	for _, want := range []string{"--handoff - <<'EOF'", "## Learned", "## Read first", "Learned matters most"} {
		if !strings.Contains(p, want) {
			t.Errorf("work prompt missing %q", want)
		}
	}
	dispatch, left, brief := strings.Index(p, "Add eviction"), strings.Index(p, "Where earlier crew left off"), strings.Index(p, "Cache it.")
	if dispatch < 0 || left < dispatch || brief < left {
		t.Errorf("want the dispatch, then where earlier crew left off, then the brief:\n%s", p)
	}
	if strings.Contains(Crew(CrewBrief{Slug: "m", Title: "M"}), "left off") {
		t.Error("left off shown with no handoffs")
	}
}

// Crew on a staged brief are told their stage and to do only that.
func TestWorkPrompt_Stage(t *testing.T) {
	p := Crew(CrewBrief{Slug: "m", Title: "M", Stage: "Implement it in `api/cache`.", StageN: 2, StageOf: 3})
	for _, want := range []string{"plan has 3 stages", "You're on stage 2:", "Implement it in `api/cache`.", "Do that stage only", "starting with `ws show -f llm s2`, your stage"} {
		if !strings.Contains(p, want) {
			t.Errorf("prompt missing %q:\n%s", want, p)
		}
	}
	if strings.Contains(p, "first stage not yet done") {
		t.Error("told to find its own stage")
	}
}

// A step of the mission's plan is told as one: whoever's on each, in turn.
func TestWorkPrompt_Step(t *testing.T) {
	p := Crew(CrewBrief{Slug: "m", Title: "M", Stage: "Reproduce it.\n\nIt makes: the failing test.", StageN: 1, StageOf: 4, Steps: true})
	for _, want := range []string{"The mission's plan", "has 4 steps", "You're on step 1:", "It makes: the failing test.", "Do that step only", "`ws show -f llm s1`, your step"} {
		if !strings.Contains(p, want) {
			t.Errorf("prompt missing %q:\n%s", want, p)
		}
	}
	if strings.Contains(p, "stages") {
		t.Errorf("a step told as a stage:\n%s", p)
	}
	// At a step that doesn't make a pull request, they don't open one.
	if !strings.Contains(p, "Don't push or open a pull request: the plan's step that makes one opens it") || strings.Contains(p, "open PRs if") {
		t.Errorf("a step that doesn't make a pull request:\n%s", p)
	}
	// At one that does, they open it through ws.
	pr := Crew(CrewBrief{Slug: "m", Title: "M", Stage: "Open it.\n\nIt makes: a pull request.", StageN: 3, StageOf: 4, Steps: true, OpensPR: true})
	for _, want := range []string{"ws mission open-pr", "for want of the user's review, ask them", "no `gh pr create`"} {
		if !strings.Contains(pr, want) {
			t.Errorf("the step that makes a pull request: prompt missing %q", want)
		}
	}
	// Without a plan, as before.
	if p := Crew(CrewBrief{Slug: "m", Title: "M", Stage: "Do it.", StageN: 1, StageOf: 2}); !strings.Contains(p, "open PRs if that's how this repo works") {
		t.Error("crew without a plan aren't told to open their PRs")
	}
}

// The planner (briefing crew) writes a plan from the brief and the
// process, in the plan's format, and no stages in the brief.
func TestBriefingPrompt_WritesAPlan(t *testing.T) {
	process := "Bugs are reproduced first, then fixed.\n"
	b := Crew(CrewBrief{Slug: "b", Role: "briefing", Request: "x", Planning: Planning{Text: process, Guarantees: []string{"ws opens a pull request only once the user's latest review approves it"}}})
	for _, want := range []string{"ws mission plan --set -", "no `## Stages`", mission.PlanFormat, process, "What ws holds for every mission", "- ws opens a pull request only once the user's latest review approves it.", "an Open Pull Request step comes after the Work that makes the change", "`repo/name`, one you dock in step 6"} {
		if !strings.Contains(b, want) {
			t.Errorf("briefing prompt missing %q:\n%s", want, b)
		}
	}
	none := Crew(CrewBrief{Slug: "b", Role: "briefing", Request: "x"})
	if !strings.Contains(none, "has no process written down") {
		t.Errorf("a workspace without a process:\n%s", none)
	}
}

// Crew of every kind are told where the workspace's guide and skills are,
// when it has them.
func TestCrew_SaysWhereTheWorkspacesGuideIs(t *testing.T) {
	note := "This workspace has a guide for agents: read /ws/AGENTS.md before you start, and follow it."
	for _, role := range []string{"", "briefing", "answer"} {
		b := CrewBrief{Slug: "cache", Title: "Cache", Role: role, Request: "cache it", Question: "which key?", Guide: note}
		if got := Crew(b); !strings.Contains(got, note) {
			t.Errorf("role %q: the prompt doesn't say where the guide is", role)
		}
		b.Guide = ""
		if got := Crew(b); strings.Contains(got, "guide for agents") {
			t.Errorf("role %q: a workspace with no guide is told of one", role)
		}
	}
}
