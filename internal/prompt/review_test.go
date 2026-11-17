package prompt

import (
	"strings"
	"testing"

	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/page"
	"github.com/withworkspace/workspace/internal/review"
)

func has(t *testing.T, what, p string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(p, want) {
			t.Errorf("%s lacks %q:\n%s", what, want, p)
		}
	}
}

func hasnt(t *testing.T, what, p string, nots ...string) {
	t.Helper()
	for _, not := range nots {
		if strings.Contains(p, not) {
			t.Errorf("%s has %q:\n%s", what, not, p)
		}
	}
}

// A review's pass's agent is told what a pass is, where the change is (or,
// cold, the change itself), what blocks, and the form: a place where a
// page's pass has a quote.
func TestReviewPassPrompts(t *testing.T) {
	h := Here{Root: "/ws", Git: []string{"log", "diff", "show"}, Repos: []string{"api: repos/api/.ground"}}
	defs := page.ReviewPasses([]page.PassDef{
		{Name: "security", About: "What an attacker could do.", Looks: "Find what could leak a secret.", Kinds: []string{"vulnerability", "hardening"}, Blocks: []string{"vulnerability"}},
		{Name: "quick", Looks: "Anything odd.", Cold: true},
		{Name: "skilled", Skill: "code-review", File: "/skills/code-review/SKILL.md", Args: "this change"},
	})
	def := func(name string) page.PassDef { d, _ := page.PassNamed(defs, name); return d }
	c := &conversation.Conversation{ID: "w5", Kind: conversation.Pass, Goal: "security", Anchor: conversation.Anchor{PR: "api#89"}, Turns: []conversation.Turn{{N: 1, By: conversation.You, Body: "Run it."}}}
	rp := ReviewPass{Def: def("security"), PR: "api#89", Title: "Key by tenant", Base: "aaaaaaaaaaaa", Head: "bbbbbbbbbbbb", Clone: "/ws/repos/api/.bare", Capsule: "/ws/repos/api/revoke", Top: "api#90"}

	p := ReviewPassFirst(h, c, rp)
	has(t, "a pass that reads the code", p,
		"api#89", "Key by tenant", "you never post anything", "ws show -f llm api#89",
		"git -C /ws/repos/api/.bare diff aaaaaaaaaaaa bbbbbbbbbbbb", "It's checked out at /ws/repos/api/revoke",
		"That folder is on api#90", "You can read anything here",
		"Find what could leak a secret.", "## <its kind: vulnerability, hardening>", "@ <path>:<line>", "```suggestion",
		"has no @ line", "A finding that's vulnerability says the change shouldn't go in", "numbered as in the file at bbbbbbb")
	hasnt(t, "a pass that reads the code", p, "You have what's below and nothing else")

	// At its own layer, there's nothing to say of a stack.
	rp.Top = "api#89"
	hasnt(t, "docked at itself", ReviewPassFirst(h, c, rp), "That folder is on")

	// What the repo says is generated is left out of the diff it's given,
	// and named.
	gen := rp
	gen.Generated = review.Generated{Files: []string{"client/api.ts", "client/types.ts", "web/api.gen.ts"}, Pathspecs: []string{":(exclude)client/", ":(exclude)web/api.gen.ts"}}
	has(t, "a change with generated files", ReviewPassFirst(h, c, gen),
		"`git -C /ws/repos/api/.bare diff aaaaaaaaaaaa bbbbbbbbbbbb -- . ':(exclude)client/' ':(exclude)web/api.gen.ts'` is all of it",
		"leaves out what the repo marks generated (linguist-generated, in .gitattributes): client/api.ts, client/types.ts, web/api.gen.ts.")
	hasnt(t, "a change with none", p, "linguist-generated", "-- .")

	cold := ReviewPass{Def: def("quick"), PR: "api#89", Title: "Key by tenant", Base: "aaaaaaaaaaaa", Head: "bbbbbbbbbbbb", Clone: "/ws/repos/api/.bare",
		Body: "Keys the cache by tenant.", Said: "- carol (changes): The key collides.", Diff: "=== key.go (modified)\n4      +\treturn tenant + id", Left: []string{"go.sum (lockfile)"}}
	p = ReviewPassFirst(h, c, cold)
	has(t, "a cold pass", p, "You have what's below and nothing else", "Keys the cache by tenant.", "The key collides.", "=== key.go (modified)", "go.sum (lockfile)",
		"## <its kind: blocking, suggestion, question, nit>", "A finding that's blocking says")
	hasnt(t, "a cold pass", p, "ws show -f llm", "You can read anything here", "git -C")

	sk := rp
	sk.Def = def("skilled")
	p = ReviewPassFirst(h, c, sk)
	has(t, "a skill's pass", p, `skill "code-review", asked: "this change"`, "/skills/code-review/SKILL.md", "Don't change any file, don't post or comment anywhere", "as the skill would file it", "blocking for one that should stop the change going in")

	// Run again: what was taken and dismissed, and that it's moved on.
	fs := []conversation.Finding{
		{Kind: "vulnerability", Path: "key.go", Side: "new", Line: 4, EndLine: 4, Excerpt: []string{"x"}, Text: "No tenant."},
		{Kind: "hardening", Text: "Split it."},
	}
	conversation.KeyFindings(fs)
	c.Turns = append(c.Turns, conversation.Turn{N: 2, By: conversation.Agent, Body: "No.", Findings: fs, Around: "aaaa111122223333"})
	c.Settled = map[string]string{fs[0].Key: conversation.FindingDone, fs[1].Key: conversation.FindingDismissed}
	c.Dismissed = map[string]string{fs[1].Key: conversation.DismissMinor}
	p = ReviewPassNext(h, c, rp)
	has(t, "run again", p, "run the pass again", "pushed to since you last read it, at aaaa111", "took these into their review", "- vulnerability, key.go:4: No tenant.",
		"They dismissed these", "- hardening, the whole change: Split it. (so, but not worth raising)", "@ <path>:<line>")
	p = ReviewPassNext(h, c, cold)
	has(t, "a cold pass run again", p, "The change as it now is", "=== key.go (modified)")
}

// A follow-up's agent is told each comment of the user's still open, what
// was said since, what's been pushed since they reviewed, and to say of
// each whether it's dealt with.
func TestFollowUpPrompts(t *testing.T) {
	h := Here{Root: "/ws", Git: []string{"log", "diff", "show"}}
	def, _ := page.PassNamed(page.ReviewPasses(nil), page.FollowUp)
	c := &conversation.Conversation{ID: "w6", Kind: conversation.Pass, Goal: page.FollowUp, Anchor: conversation.Anchor{PR: "api#89"}}
	rp := ReviewPass{Def: def, PR: "api#89", Title: "Key by tenant", Base: "aaaaaaaaaaaa", Head: "cccccccccccc", Clone: "/ws/repos/api/.bare",
		Reviewed: "bbbbbbbbbbbb", Since: "commits",
		Threads: []FollowThread{
			{Label: "T1", ID: "PRRT_1", Path: "key.go", Side: "new", Line: 41, EndLine: 42, Commit: "bbbbbbbbbbbb", Kind: "blocking", Body: "The key leaves the tenant out.", Excerpt: []string{"\tkey := token"},
				Replies: []string{"alice: Fixed in the last push."}},
			{Label: "T2", ID: "PRRT_2", Path: "old.go", Side: "old", Line: 3, EndLine: 3, Commit: "aaaaaaaaaaaa", Kind: "nit", Body: "Its comment is stale."},
		}}
	p := FollowUpFirst(h, c, rp)
	has(t, "a follow-up", p, "following up a review", "api#89", "you never post anything",
		"git -C /ws/repos/api/.bare diff bbbbbbbbbbbb cccccccccccc` is what's been pushed since",
		"T1. key.go, lines 41-42, at bbbbbbb (blocking):", "    | \tkey := token", "  The key leaves the tenant out.", "    alice: Fixed in the last push.",
		"T2. old.go, removed line 3, at aaaaaaa (nit):",
		"## <addressed, partly or not-addressed>", "@ <its label: T1>", "Go by the code as it now is")
	hasnt(t, "a follow-up", p, "```suggestion", "It's checked out at")

	rp.Since = "changes"
	has(t, "after a rebase", FollowUpNext(h, c, rp), "run the follow-up again", "it's been rebased since", "T1. key.go")
	rp.Reviewed, rp.Since = "", ""
	has(t, "with no review kept", FollowUpFirst(h, c, rp), "It's at ccccccc")

	rp.Reviewed, rp.Since = "bbbbbbbbbbbb", "commits"
	rp.Generated = review.Generated{Files: []string{"client/api.ts"}, Pathspecs: []string{":(exclude)client/"}}
	has(t, "a follow-up past what's generated", FollowUpFirst(h, c, rp),
		"`git -C /ws/repos/api/.bare diff bbbbbbbbbbbb cccccccccccc -- . ':(exclude)client/'` is what's been pushed since",
		"`git -C /ws/repos/api/.bare diff aaaaaaaaaaaa cccccccccccc -- . ':(exclude)client/'` the whole change",
		"marks generated (linguist-generated, in .gitattributes): client/api.ts.")
}
