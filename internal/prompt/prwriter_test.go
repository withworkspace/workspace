package prompt

import (
	"strings"
	"testing"
)

// The PR writer's answer: its last title and description, the title on
// one line and no longer than GitHub takes; no title is an error.
func TestReadPR(t *testing.T) {
	title, body, err := ReadPR("Looking at it.\n<title>Draft</title>\n\nBetter:\n<title> Cache the\n  token lookup </title>\n<body>\n## What\n\nCaches it.\n</body>\n")
	if err != nil || title != "Cache the token lookup" || body != "## What\n\nCaches it." {
		t.Errorf("got %q, %q, %v", title, body, err)
	}
	if _, _, err := ReadPR("I couldn't."); err == nil {
		t.Error("no title read as one")
	}
	if _, _, err := ReadPR("<title> </title>"); err == nil {
		t.Error("an empty title read as one")
	}
	long, _, _ := ReadPR("<title>" + strings.Repeat("a", 300) + "</title>")
	if n := len([]rune(long)); n != MaxPRTitle {
		t.Errorf("title of %d runes", n)
	}
	if title, body, _ := ReadPR("<TITLE>`Fix it`</TITLE>"); title != "Fix it" || body != "" {
		t.Errorf("got %q, %q", title, body)
	}
}

// The prompt is for the task, as the workspace's guide says, quoted so
// its headings stay its own; without one, ws's own few words; the repo's
// template where it has one.
func TestPRWriterPrompt(t *testing.T) {
	p := PRWriting{Mission: "cache", Title: "Cache", Goal: "Cache the lookup.", Repo: "api", Capsule: "api/cache", Branch: "cache", Base: "main",
		Task:  []string{"Step 3 (Work): Cache the token.\n  It makes: a pull request.\n"},
		Guide: "# PRs\nTitles start with the area.", GuidePath: "PR.md", Template: "## Testing", Commits: []string{"abc123 Cache it"}, Diff: "x | 1 +"}
	got := PRWriter(p)
	for _, want := range []string{
		"from `api/cache` (the branch `cache`) onto `main` in the repo `api`",
		"Write it for the task it's for, below: what this pull request does, not what the mission does.",
		"The workspace says so in `PR.md`", "> # PRs\n> Titles start with the area.",
		"> ## Testing", "Step 3 (Work): Cache the token.", "The mission's goal: Cache the lookup.",
		"- abc123 Cache it", "```diff\nx | 1 +\n```", "<title>The title</title>",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the prompt lacks %q:\n%s", want, got)
		}
	}
	p.Guide, p.Template, p.Task = "", "", nil
	got = PRWriter(p)
	for _, want := range []string{"The workspace doesn't say (no PR.md).", "The mission has no plan to say which part of it this is"} {
		if !strings.Contains(got, want) {
			t.Errorf("the prompt lacks %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "pull request template") {
		t.Error("speaks of a template there isn't")
	}
}
