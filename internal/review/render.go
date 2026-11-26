package review

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Render is a review as crew and the orchestrator read it: everything to
// act on without the diff. Comments are grouped by file, each with its
// lines as they were when commented on, and what to do comes last.
func Render(r *Review) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", Title(r))
	when := ""
	if r.Submitted != nil {
		when = ", " + r.Submitted.Local().Format("2006-01-02 15:04")
	}
	fmt.Fprintf(&b, "From the user%s, of commit %s (the capsule's changes since %s).\n\n", when, short(r.Head), r.BaseRef)
	if s := strings.TrimSpace(r.Summary); s != "" {
		quote(&b, s)
	}
	if len(r.Comments) == 0 {
		b.WriteString("No comments on the code.\n\n")
	} else {
		fmt.Fprintf(&b, "%d %s%s.\n\n", len(r.Comments), plural(len(r.Comments), "comment"), countWords(r))
	}

	var paths []string
	byPath := map[string][]Note{}
	for _, n := range r.Comments {
		if _, ok := byPath[n.Path]; !ok {
			paths = append(paths, n.Path)
		}
		byPath[n.Path] = append(byPath[n.Path], n)
	}
	for _, p := range paths {
		heading := p
		if heading == "" {
			heading = "The change as a whole"
		}
		fmt.Fprintf(&b, "## %s\n\n", heading)
		for _, n := range byPath[p] {
			renderNote(&b, n)
		}
	}

	b.WriteString("## What to do\n\n")
	switch r.Verdict {
	case Approve:
		b.WriteString("Approved: the work is good to go. Act on any comments above only if they're quick and clearly right.\n")
	default:
		b.WriteString("Address every blocking comment. Take each suggestion and nit too unless you have a reason not to, and answer each question. ")
		b.WriteString("Work in the capsule and commit your changes. Then answer each comment, so the user sees what you did when they look again:\n\n")
		ex := r.ID + ".c1"
		fmt.Fprintf(&b, "    ws review addressed %s \"what you did\"   (after committing: it records the commit)\n", ex)
		fmt.Fprintf(&b, "    ws review wontfix %s \"why not\"\n", ex)
		fmt.Fprintf(&b, "    ws review reply %s \"your answer\"   (a question, or anything to say)\n\n", ex)
		b.WriteString("Then report done, with a handoff that says what you did for each comment. The user re-reviews what changed: until they approve, the stage isn't accepted.\n")
	}
	return b.String()
}

func renderNote(b *strings.Builder, n Note) {
	// New-side lines are where they are at the reviewed commit; old-side
	// ones are lines the change removed, numbered as they were before it.
	where := fmt.Sprintf("%s:%d", n.Path, n.Line)
	if n.EndLine > n.Line {
		where += fmt.Sprintf("-%d", n.EndLine)
	}
	where += " at " + short(n.Commit)
	if n.Side == "old" {
		lines := fmt.Sprintf("line %d", n.Line)
		if n.EndLine > n.Line {
			lines = fmt.Sprintf("lines %d–%d", n.Line, n.EndLine)
		}
		where = fmt.Sprintf("%s, removed %s (as numbered at %s, before the change)", n.Path, lines, short(n.Commit))
	}
	if n.General() {
		where = "the whole change, at " + short(n.Commit)
	}
	state := ""
	if n.Status != "" && n.Status != Open {
		state = " · " + stateWords[n.Status]
	}
	fmt.Fprintf(b, "### %s · %s · %s%s\n\n", n.ID, n.Kind, where, state)
	lang := strings.TrimPrefix(filepath.Ext(n.Path), ".")
	if len(n.Excerpt) > 0 {
		var ex strings.Builder
		width := len(fmt.Sprint(n.EndLine))
		for i, l := range n.Excerpt {
			fmt.Fprintf(&ex, "%*d  %s\n", width, n.Line+i, l)
		}
		fence(b, lang, ex.String())
	}
	if body := strings.TrimSpace(n.Body); body != "" {
		b.WriteString(body + "\n\n")
	}
	if n.Suggestion != nil {
		if *n.Suggestion == "" {
			b.WriteString("Suggested change: delete these lines.\n\n")
		} else {
			b.WriteString("Suggested change, replacing these lines:\n\n")
			fence(b, lang, *n.Suggestion+"\n")
		}
	}
	renderThread(b, n.Thread)
}

// renderThread is what's been said on a comment since, a line each.
func renderThread(b *strings.Builder, thread []Reply) {
	for _, t := range thread {
		what := ""
		switch t.Status {
		case Addressed:
			what = " (addressed"
			if t.Commit != "" {
				what += " in " + short(t.Commit)
			}
			what += ")"
		case WontFix, Resolved, Open:
			what = " (" + stateWords[t.Status] + ")"
		}
		line := fmt.Sprintf("- %s, %s%s", t.By, t.At.Local().Format("2006-01-02 15:04"), what)
		if t.Body != "" {
			line += ": " + t.Body
		}
		b.WriteString(line + "\n")
	}
	if len(thread) > 0 {
		b.WriteString("\n")
	}
}

// RenderComment is one comment of a review as crew read it: what it's on,
// what was said, and its thread. ok is false if the review has no such
// comment.
func RenderComment(r *Review, id string) (text string, ok bool) {
	for _, n := range r.Comments {
		if n.ID != id {
			continue
		}
		var b strings.Builder
		renderNote(&b, n)
		return b.String(), true
	}
	return "", false
}

// quote writes text as a blockquote.
func quote(b *strings.Builder, text string) {
	for l := range strings.SplitSeq(text, "\n") {
		b.WriteString(strings.TrimRight("> "+l, " ") + "\n")
	}
	b.WriteString("\n")
}

// fence writes code in a fence longer than any run of backticks in it.
func fence(b *strings.Builder, lang, code string) {
	n, run := 3, 0
	for _, c := range code {
		if c == '`' {
			run++
			n = max(n, run+1)
		} else {
			run = 0
		}
	}
	f := strings.Repeat("`", n)
	fmt.Fprintf(b, "%s%s\n%s%s\n\n", f, lang, code, f)
}

var stateWords = map[string]string{Open: "reopened", Addressed: "addressed", WontFix: "won't fix", Resolved: "resolved"}

func plural(n int, word string) string {
	if n == 1 {
		return word
	}
	return word + "s"
}
