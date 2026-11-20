package prompt

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// The PR writer: an agent with fresh context, headless and read-only, who
// writes the title and description of a pull request ws is about to open
// from a mission's capsule. It writes for the task the pull request is
// for (the step, or steps, of the plan whose work it carries), not for
// the whole mission, and as the workspace says its pull requests are
// written (PR.md).

// PRWriting is what the PR writer is told.
type PRWriting struct {
	Mission, Title string // the mission's slug and title
	// Goal is the brief's goal: what the mission is for, which the pull
	// request may be only part of.
	Goal string
	// Task is what the pull request is for, a block each: the plan's steps
	// whose work it carries, as the plan says them, with what each makes
	// and its checks. With no plan, nothing: the goal is the task.
	Task []string
	// Repo, Capsule (repo/name), Branch and Base are where it's from and
	// onto; Dir is the capsule's folder, where the writer runs.
	Repo, Capsule, Branch, Base, Dir string
	// Handoffs are what crew handed off for the task's steps in this
	// capsule, each headed by who.
	Handoffs []string
	// Decided is what the user has decided on the mission, a line each:
	// what they chose stands over the plan's words.
	Decided []string
	// Guide is how the workspace's pull requests are written (PR.md), as
	// it's written, and GuidePath where it is; "" with none.
	Guide, GuidePath string
	// Template is the repo's own pull request template, if it has one.
	Template string
	// Commits are the capsule's commits onto Base, a line each, oldest
	// first.
	Commits []string
	// Diff is what the capsule's commits change: a stat, then the change,
	// cut to fit.
	Diff string
}

// PRWriter is the PR writer's prompt.
func PRWriter(p PRWriting) string {
	var b strings.Builder
	fmt.Fprintf(&b, "You are writing a pull request's title and description. ws (the tool managing this workspace) is about to open it from `%s` (the branch `%s`) onto `%s` in the repo `%s`. It's work done on the mission %q (%s), and ws opens it once you've written it.\n\n", p.Capsule, p.Branch, p.Base, p.Repo, p.Title, p.Mission)
	b.WriteString("Write it for the task it's for, below: what this pull request does, not what the mission does. A mission can take several pull requests, in this repo and others, and this one is the work of the task's steps alone. The mission's goal is there so you know why the task matters; don't describe the rest of it as if it were here.\n\n")
	fmt.Fprintf(&b, "You're in the capsule's folder, at what's to be opened. Read whatever helps you say what changed and why (Read, Grep, Glob; `git log`, `git diff %s...HEAD`, `git show`), but change nothing, and don't push, open or comment on anything: ws opens it. `ws show -f llm <ref>` shows any of the mission's documents and handoffs.\n\n", "origin/"+p.Base)
	b.WriteString("Write for whoever reviews it: someone who knows the repo, not the mission. Don't name the mission, its plan or steps, crew, agents or ws unless the workspace's guide below asks for them. Say what's true of the change as it is: what crew say they did is where to look, not what shows it.\n\n")

	b.WriteString("## How pull requests are written here\n\n")
	switch {
	case strings.TrimSpace(p.Guide) != "":
		fmt.Fprintf(&b, "The workspace says so in `%s`, which stands over everything else here about how it's written (its title's form, the description's sections, what it links):\n\n%s\n\n", p.GuidePath, quoteBlock(p.Guide))
	default:
		b.WriteString("The workspace doesn't say (no PR.md). A title that says what the change does, in the imperative, under 72 characters. A description that says what changed and why, then how to check it, in plain words and short paragraphs or a short list; nothing about what it doesn't do.\n\n")
	}
	if strings.TrimSpace(p.Template) != "" {
		b.WriteString("The repo has its own pull request template, below. Fill it in, as far as the workspace's guide doesn't say otherwise:\n\n")
		b.WriteString(quoteBlock(p.Template) + "\n\n")
	}

	b.WriteString("## Your answer\n\nAnswer with the title and the description, in markdown, as below, and nothing else:\n\n")
	b.WriteString("```\n<title>The title</title>\n<body>\nThe description.\n</body>\n```\n\n")

	b.WriteString("## The task\n\n")
	if len(p.Task) > 0 {
		b.WriteString("The steps of the mission's plan whose work this pull request carries:\n\n")
		for _, t := range p.Task {
			b.WriteString(strings.TrimRight(t, "\n") + "\n\n")
		}
	} else {
		b.WriteString("The mission has no plan to say which part of it this is: the goal below is the task.\n\n")
	}
	if g := strings.TrimSpace(p.Goal); g != "" {
		fmt.Fprintf(&b, "The mission's goal: %s\n\n", sentenceEnd(oneLine(g)))
	}
	if len(p.Decided) > 0 {
		b.WriteString("## Decided by the user\n\nWhat the user chose stands over the plan's words, where the two differ:\n\n")
		for _, d := range p.Decided {
			b.WriteString("- " + d + "\n")
		}
		b.WriteString("\n")
	}
	for _, h := range p.Handoffs {
		b.WriteString(strings.TrimRight(h, "\n") + "\n\n")
	}
	if len(p.Commits) > 0 {
		fmt.Fprintf(&b, "## Its commits onto %s\n\n", p.Base)
		for _, c := range p.Commits {
			b.WriteString("- " + c + "\n")
		}
		b.WriteString("\n")
	}
	if strings.TrimSpace(p.Diff) != "" {
		fmt.Fprintf(&b, "## What it changes\n\n```diff\n%s\n```\n", strings.TrimRight(p.Diff, "\n"))
	}
	return b.String()
}

// quoteBlock is text quoted as a block, each line behind "> ", so what it
// says can't be taken for the prompt's own headings.
func quoteBlock(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight("> "+l, " ")
	}
	return strings.Join(lines, "\n")
}

// MaxPRTitle is the longest title GitHub takes.
const MaxPRTitle = 256

var (
	prTitleTag = regexp.MustCompile(`(?is)<title>(.*?)</title>`)
	prBodyTag  = regexp.MustCompile(`(?is)<body>\n?(.*?)</body>`)
)

// ReadPR reads the PR writer's answer: the title (one line, as long as
// GitHub takes) and the description. An answer with no title is an
// error: there's nothing to open.
func ReadPR(said string) (title, body string, err error) {
	m := prTitleTag.FindAllStringSubmatch(said, -1)
	if len(m) == 0 {
		return "", "", errors.New("its answer had no <title>")
	}
	// What it said last: a writer that drafts and corrects gives its
	// answer last.
	title = strings.Join(strings.Fields(m[len(m)-1][1]), " ")
	title = strings.Trim(title, "`*")
	if title == "" {
		return "", "", errors.New("its answer's <title> was empty")
	}
	if r := []rune(title); len(r) > MaxPRTitle {
		title = string(r[:MaxPRTitle-1]) + "…"
	}
	if b := prBodyTag.FindAllStringSubmatch(said, -1); len(b) > 0 {
		body = strings.TrimSpace(b[len(b)-1][1])
	}
	return title, body, nil
}
