package page

import (
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/withworkspace/workspace/internal/conversation"
)

// PassDef says what a pass over a page is: its name (a word: the pass is
// run and kept by it), what it's called, what it's for in a line, what its
// agent is to look for, the kinds its findings can be, and what it says of
// the page as a whole before them.
//
// Cold is a pass whose agent gets the page and nothing else: it can't read
// the workspace, so what it says is what the page alone carries.
//
// File is a file that says how to do the pass (a checklist), by its full
// path: its agent reads it first.
//
// Skill is one of the workspace's skills, by name, and Args what the pass
// asks of it: its agent follows the skill (File is then its SKILL.md) and
// reports in ws's form, not the skill's. Unavailable says why a pass can't
// be run (its skill isn't the workspace's), for a tab that's there but
// can't be.
//
// A review's pass (over a pull request, or a capsule) is defined the same
// way, with two things more: Blocks, the kinds that make it no go while a
// finding of one is open, and Repos, the repos whose reviews have it (all,
// with none). Cold, its agent gets the diff and the description.
type PassDef struct {
	Name        string   `json:"name"`
	Title       string   `json:"title"`
	About       string   `json:"about"`
	Looks       string   `json:"-"`
	File        string   `json:"-"`
	Skill       string   `json:"skill,omitempty"`
	Args        string   `json:"args,omitempty"`
	Kinds       []string `json:"kinds"`
	Summary     string   `json:"-"`
	Cold        bool     `json:"cold,omitempty"`
	Unavailable string   `json:"unavailable,omitempty"`
	Blocks      []string `json:"blocks,omitempty"`
	Repos       []string `json:"-"`
	// Auto: it runs by itself over a pull request that needs you.
	Auto bool `json:"auto,omitempty"`
}

// Built reports whether a pass is one of the built-in ones.
func (p PassDef) Built() bool {
	for _, b := range Builtin {
		if b.Name == p.Name {
			return true
		}
	}
	return false
}

// Builtin are the passes every workspace has.
var Builtin = []PassDef{
	{
		Name:    "check",
		Title:   "Check",
		About:   "What's open, ambiguous or missing, for someone who'd build from this page.",
		Looks:   "Read it as someone about to build from it. Find what's open (a choice not made), ambiguous (words that could mean two things) or missing (something they'd need that isn't there). Where the page says something about the code, check it against the code.",
		Kinds:   []string{"question", "ambiguity", "missing"},
		Summary: "a line or two on how ready the page is to build from",
	},
	{
		Name:    "formal",
		Title:   "Formalise",
		About:   "What the page requires, as statements that can be checked, and what's too vague to check.",
		Looks:   "Find what the page requires. A requirement that could be checked as it's written is a `requirement`: say it as a statement someone could test. One that couldn't (\"fast\", \"simple\", \"as it does now\") is `vague`: say what would make it checkable, and where you can, propose words that are.",
		Kinds:   []string{"requirement", "vague"},
		Summary: "a line on how much of the page could be checked as it stands",
	},
	{
		Name:    "cold",
		Title:   "Cold read",
		About:   "What an agent would build from this page alone, and everything it had to assume.",
		Looks:   "You have the page and nothing else, as someone handed it cold. Each thing you'd have to assume to build it, that the page doesn't settle, is an `assumption`: say what you'd assume, and what else it could be.",
		Kinds:   []string{"assumption"},
		Summary: "two or three sentences on what you'd build",
		Cold:    true,
	},
}

var passName = regexp.MustCompile(`^[a-z]{1,20}$`)

// Passes are a workspace's passes: the built-in ones, then its own (from
// ws.toml), by name. One of its own named as a built-in one replaces it.
func Passes(own []PassDef) []PassDef {
	out := append([]PassDef{}, Builtin...)
	sort.SliceStable(own, func(i, j int) bool { return own[i].Name < own[j].Name })
	for _, p := range own {
		if !passName.MatchString(p.Name) || (strings.TrimSpace(p.Looks) == "" && p.File == "" && p.Skill == "") {
			continue
		}
		if p.Title == "" {
			p.Title = strings.ToUpper(p.Name[:1]) + p.Name[1:]
		}
		// A skill's findings are of the kinds it names them by.
		if len(p.Kinds) == 0 && p.Skill == "" {
			p.Kinds = []string{"finding"}
		}
		if p.Summary == "" {
			p.Summary = "a line or two on how the page stands"
			if p.Skill != "" {
				p.Summary = "the verdict the skill would open its report with, or, if it gives none, a line on how the page stands"
			}
		}
		replaced := false
		for i := range out {
			if out[i].Name == p.Name {
				out[i], replaced = p, true
			}
		}
		if !replaced {
			out = append(out, p)
		}
	}
	return out
}

// ForPage are the passes a page of a kind has, of a workspace's passes
// (defs) and the passes each of its kinds names (kinds, by kind): the
// built-in ones, the workspace's own that no kind names, and those the
// page's own kind names, after the rest, in the kind's order.
func ForPage(defs []PassDef, kinds map[string][]string, kind string) []PassDef {
	named := map[string]bool{}
	for _, passes := range kinds {
		for _, n := range passes {
			named[n] = true
		}
	}
	var out []PassDef
	for _, d := range defs {
		if d.Built() || !named[d.Name] {
			out = append(out, d)
		}
	}
	for _, n := range kinds[kind] {
		if d, ok := PassNamed(defs, n); ok && !d.Built() {
			out = append(out, d)
		}
	}
	return out
}

// PassNamed is the pass named, among passes.
func PassNamed(passes []PassDef, name string) (PassDef, bool) {
	for _, p := range passes {
		if p.Name == name {
			return p, true
		}
	}
	return PassDef{}, false
}

// A review's passes.

// ReviewKinds are the kinds a review's pass finds, when it names none: a
// review's comments' own.
var ReviewKinds = []string{"blocking", "suggestion", "question", "nit"}

// ReviewBlocks are the kinds that block, when a pass names none.
var ReviewBlocks = []string{"blocking"}

// FollowUp is the built-in pass that says, of each comment of yours still
// open, whether what's been pushed since deals with it.
const FollowUp = "followup"

// What a follow-up says of a comment.
const (
	FollowedAddressed = "addressed"
	FollowedPartly    = "partly"
	FollowedNot       = "not-addressed"
)

// Guide is the pass that says what a change does and what order to read
// it in. It finds nothing to settle, so it isn't among a review's passes:
// a workspace's own pass can't be named for it.
const Guide = "guide"

// GuideDef is the guide, as a pass: its agent is given the change and
// the description, and nothing else.
func GuideDef(looks, file string, auto bool) PassDef {
	return PassDef{Name: Guide, Title: "Guide", About: "What the change does, where that differs from what it says it does, and what order to read it in.",
		Looks: strings.TrimSpace(looks), File: file, Cold: true, Auto: auto}
}

// ReviewBuiltin are the passes every review has.
var ReviewBuiltin = []PassDef{
	{
		Name:    FollowUp,
		Title:   "Follow-up",
		About:   "Whether what's been pushed since your review deals with each comment of yours that's still open.",
		Kinds:   []string{FollowedAddressed, FollowedPartly, FollowedNot},
		Summary: "a line on where it stands: how many of the comments are dealt with",
	},
}

// ReviewPasses are a workspace's passes over a review: the built-in ones,
// then its own (from ws.toml and ws.local.toml), by name. One of its own
// named as a built-in one replaces it.
func ReviewPasses(own []PassDef) []PassDef {
	out := append([]PassDef{}, ReviewBuiltin...)
	sort.SliceStable(own, func(i, j int) bool { return own[i].Name < own[j].Name })
	for _, p := range own {
		if !passName.MatchString(p.Name) || p.Name == Guide || (strings.TrimSpace(p.Looks) == "" && p.File == "" && p.Skill == "") {
			continue
		}
		if p.Title == "" {
			p.Title = strings.ToUpper(p.Name[:1]) + p.Name[1:]
		}
		if len(p.Kinds) == 0 && p.Skill == "" {
			p.Kinds = ReviewKinds
		}
		if len(p.Blocks) == 0 {
			p.Blocks = ReviewBlocks
		}
		if p.Summary == "" {
			p.Summary = "a line or two on how the change stands"
			if p.Skill != "" {
				p.Summary = "the verdict the skill would open its report with, or, if it gives none, a line on how the change stands"
			}
		}
		replaced := false
		for i := range out {
			if out[i].Name == p.Name {
				out[i], replaced = p, true
			}
		}
		if !replaced {
			out = append(out, p)
		}
	}
	return out
}

// ForRepo are the passes a review in a repo has, of the workspace's.
func ForRepo(defs []PassDef, repo string) []PassDef {
	var out []PassDef
	for _, d := range defs {
		if len(d.Repos) == 0 || slices.Contains(d.Repos, repo) {
			out = append(out, d)
		}
	}
	return out
}

// Blocking is whether a finding of a kind makes the pass no go while it's
// open.
func (p PassDef) Blocking(kind string) bool { return slices.Contains(p.Blocks, kind) }

// How a review's pass stands. Go or no go is worked out from what it
// found, never stated: no go while a finding of a kind that blocks is
// open, so it moves as you settle them.
const (
	StandingNone    = "none"    // never run
	StandingRunning = "running" // on its first run
	StandingFailed  = "failed"  // it couldn't say anything, and never has
	StandingNoGo    = "nogo"    // a finding that blocks is open
	StandingNotes   = "notes"   // findings are open, and none blocks
	StandingGo      = "go"      // it ran, and nothing's open
)

// Standing is how a review's pass stands over a review: its status, how
// many findings are open and how many of those block, and how many that
// block you dismissed (what you overruled).
type Standing struct {
	Status    string `json:"status"`
	Open      int    `json:"open"`
	Blocking  int    `json:"blocking"`
	Overruled int    `json:"overruled,omitempty"`
}

// ReviewStanding is how a pass stands, from its conversation (nil, if
// it's never been run).
func ReviewStanding(def PassDef, c *conversation.Conversation) Standing {
	if c == nil {
		return Standing{Status: StandingNone}
	}
	w := c.Written()
	if w == nil {
		if c.Answering || c.NeedsAnswer() {
			return Standing{Status: StandingRunning}
		}
		return Standing{Status: StandingFailed}
	}
	st := Standing{Status: StandingGo}
	for _, f := range w.Findings {
		blocks := def.Blocking(f.Kind) || f.Blocks
		switch c.FindingState(f.Key) {
		case conversation.FindingOpen:
			st.Open++
			if blocks {
				st.Blocking++
			}
		case conversation.FindingDismissed:
			if blocks {
				st.Overruled++
			}
		}
	}
	switch {
	case st.Blocking > 0:
		st.Status = StandingNoGo
	case st.Open > 0:
		st.Status = StandingNotes
	}
	return st
}

// Words are how a pass stands, as it's said: "no go, 2 blocking", "go,
// with notes (3 open)", "go".
func (s Standing) Words() string {
	over := ""
	if s.Overruled > 0 {
		over = fmt.Sprintf("; %d that blocked dismissed", s.Overruled)
	}
	switch s.Status {
	case StandingNone:
		return "not run"
	case StandingRunning:
		return "running"
	case StandingFailed:
		return "couldn't run"
	case StandingNoGo:
		return fmt.Sprintf("no go, %d blocking of %d open%s", s.Blocking, s.Open, over)
	case StandingNotes:
		return fmt.Sprintf("go, with notes (%d open%s)", s.Open, over)
	}
	if over != "" {
		return "go (" + strings.TrimPrefix(over, "; ") + ")"
	}
	return "go"
}
