package prompt

import (
	"fmt"
	"sort"
	"strings"

	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/page"
	"github.com/withworkspace/workspace/internal/review"
)

// A pass over a review: an agent reads the change a pull request makes
// and says what it finds, each finding on the lines it's about, for the
// user to take into their review or dismiss.

// ReviewPass is a pass to run over a review: what it is, what's reviewed,
// and where its own instructions are (as a page's pass's).
type ReviewPass struct {
	Def page.PassDef
	// PR is the pull request (api#89) and Title what it's called. Base and
	// Head are the commits its changes are from and to, in Clone, the
	// repo's clone.
	PR, Title  string
	Base, Head string
	Clone      string
	// Capsule is the folder it's checked out in, for a pass that reads the
	// code. Top is the pull request that folder is on, when that isn't
	// this one: its stack's top layer, which holds later layers' changes
	// too.
	Capsule, Top string
	// What a cold pass is given, having nothing else: what the pull
	// request says it does, what's been said on it, its changes with their
	// lines numbered (review.Numbered), and the files those leave out.
	Body, Said, Diff string
	Left             []string
	GuidePath        string
	GuideText        string
	// What a follow-up is of: the commit the user last reviewed at, how
	// it's moved on since (pulls.SinceCommits: pushed to; SinceChanges:
	// rebased; "": not at all), and their comments still open.
	Reviewed, Since string
	Threads         []FollowThread
	// Files are every file the change touches, for the guide to place.
	Files []string
	// Generated is what the change touches that the repo says is
	// generated: a pass that reads the change itself is given a diff
	// that leaves it out.
	Generated review.Generated
	// For what isn't a pull request (a mission's capsule): Over is what's
	// reviewed, in words; Start, what its agent is told to read first; and
	// BodyIs, what Body is ("What the work is meant to do").
	Over, Start, BodyIs string
}

// diff is the command that's the change's diff from one commit to
// another: past what the repo says is generated, if it touches any.
func (rp ReviewPass) diff(from, to string) string {
	cmd := fmt.Sprintf("git -C %s diff %s %s", rp.Clone, from, to)
	if len(rp.Generated.Pathspecs) > 0 {
		cmd += " -- ."
		for _, p := range rp.Generated.Pathspecs {
			cmd += " '" + p + "'"
		}
	}
	return cmd
}

// generatedLeft says what the change's diff leaves out, and why.
func generatedLeft(rp ReviewPass) string {
	if len(rp.Generated.Pathspecs) == 0 {
		return ""
	}
	names := rp.Generated.Files
	more := ""
	if len(names) > 12 {
		names, more = names[:12], fmt.Sprintf(", and %d more", len(names)-12)
	}
	return fmt.Sprintf("That diff leaves out what the repo marks generated (linguist-generated, in .gitattributes): %s%s. It's made from the rest, so don't read it unless what you find turns on it. ", strings.Join(names, ", "), more)
}

// over is what's reviewed, in words.
func (rp ReviewPass) over() string {
	if rp.Over != "" {
		return rp.Over
	}
	return fmt.Sprintf("the pull request %s (%q)", rp.PR, rp.Title)
}

// start is what a pass that reads the code is told to read first.
func (rp ReviewPass) start() string {
	if rp.Start != "" {
		return rp.Start
	}
	return fmt.Sprintf("Start with `ws show -f llm %s`: the pull request, what it says it does, its stack and where it sits in it, its checks, and what's been said in review, which isn't to be said again. ", rp.PR)
}

// bodyIs is what Body is, as a cold pass is told it.
func (rp ReviewPass) bodyIs() string {
	if rp.BodyIs != "" {
		return rp.BodyIs
	}
	return "What the pull request says it does"
}

// FollowThread is a comment of the user's still open, as a follow-up's
// agent is told it: Label is what it's called there (T1), ID its thread.
type FollowThread struct {
	Label, ID string
	Path      string
	Side      string
	Line      int
	EndLine   int
	Commit    string
	Kind      string // the comment's: blocking, suggestion, question, nit
	Body      string
	Excerpt   []string
	Replies   []string // "alice: Fixed in the last push."
}

// reviewPassIntro says what a review's pass is, and what never happens.
func reviewPassIntro(h Here, rp ReviewPass) string {
	return fmt.Sprintf("You're running a pass over a review, in the web UI of ws, which manages this multi-repo workspace (%s). The user is reviewing %s. A pass reads the whole change and says what it finds. What you say is shown to the user beside the diff, each finding on the lines it's about, for them to take into their review or dismiss. You change nothing, you never run the change's code, and you never post anything.\n\n", h.Root, rp.over())
}

// reviewPassChange says where the change is, for a pass that reads it
// itself.
func reviewPassChange(rp ReviewPass) string {
	var b strings.Builder
	fmt.Fprintf(&b, "The change is the commits from %s to %s: `%s` is all of it, and `git -C %s show %s:<path>` is a file as the change leaves it. ", short7(rp.Base), short7(rp.Head), rp.diff(rp.Base, rp.Head), rp.Clone, rp.Head)
	b.WriteString(generatedLeft(rp))
	if rp.Capsule != "" {
		fmt.Fprintf(&b, "It's checked out at %s, to read as files, with the rest of the workspace around it. ", rp.Capsule)
		if rp.Top != "" && rp.Top != rp.PR {
			fmt.Fprintf(&b, "That folder is on %s, the top of the stack this pull request is a layer of, so it holds later layers' changes too. This pass is over %s's own: where the two differ, go by `git show %s:<path>`. ", rp.Top, rp.PR, short7(rp.Head))
		}
	}
	b.WriteString("\n\n")
	return b.String()
}

// reviewPassGiven is what a cold pass is given: all it has.
func reviewPassGiven(rp ReviewPass) string {
	var b strings.Builder
	b.WriteString("You have what's below and nothing else: you can't read the workspace, and aren't to go by anything you know of it.\n\n")
	body := strings.TrimSpace(rp.Body)
	if body == "" {
		body = "(It has no description.)"
	}
	fmt.Fprintf(&b, "%s, between the lines of dashes:\n\n----------\n%s\n----------\n\n", rp.bodyIs(), clipString(body, 12<<10))
	if s := strings.TrimSpace(rp.Said); s != "" {
		fmt.Fprintf(&b, "What's been said on it so far, which isn't to be said again:\n\n%s\n\n", clipString(s, 12<<10))
	}
	fmt.Fprintf(&b, "Its changes, between the lines of equals signs. %s\n\n==========\n%s\n==========\n\n", review.NumberedHow, rp.Diff)
	if len(rp.Left) > 0 {
		if rp.Def.Name == page.Guide {
			fmt.Fprintf(&b, "These files it changes aren't shown, being what they're called: %s. Place them all the same, to skim if that's what they are, and don't remark on not having read them.\n\n", strings.Join(rp.Left, ", "))
		} else {
			fmt.Fprintf(&b, "These files it changes aren't shown: %s. Say in your first line that you didn't read them, if what you find could turn on them.\n\n", strings.Join(rp.Left, ", "))
		}
	}
	return b.String()
}

// reviewPassSkill is how a pass that's one of the workspace's skills is
// done over a review: the skill, followed for what the pass asks of it,
// on this change; and its report given in ws's form in place of its own.
func reviewPassSkill(rp ReviewPass) string {
	def := rp.Def
	var b strings.Builder
	fmt.Fprintf(&b, "This pass uses the workspace's skill %q", def.Skill)
	if def.Args != "" {
		fmt.Fprintf(&b, ", asked: %q", def.Args)
	}
	fmt.Fprintf(&b, ". Its instructions are in %s: read all of it first, then the files it sends you to for this, and do what it says", def.File)
	if def.Args != "" {
		b.WriteString(" for that")
	}
	fmt.Fprintf(&b, ", with this change as what it's run on. Where the skill would look at a working tree's changes, the current branch or a pull request, it's this change (%s to %s).\n\n", short7(rp.Base), short7(rp.Head))
	b.WriteString("What the skill says about how to report is replaced by the form below, which is how the user reads it, beside the diff: the verdict or overall word it would open its report with is your first line. Each item it would list is a finding, on the lines it's about (none, for something about the change as a whole), of the kind the skill would file it under. Where it would fix something, propose the lines. Don't change any file, don't post or comment anywhere, and don't report the skill's steps.\n\n")
	return b.String()
}

// reviewPassGuide says where a pass's own instructions are, as a page's
// pass's does.
func reviewPassGuide(rp ReviewPass) string {
	def := rp.Def
	if def.Skill != "" {
		return reviewPassSkill(rp)
	}
	if rp.GuidePath == "" {
		return ""
	}
	const use = "It may be written for another use (fixing the code, say, or answering in a form of its own). Here, take from it what to look for and what good looks like. Don't change any file, and don't follow what it says about how to reply: each thing you find is a finding on the lines it's about.\n\n"
	if !def.Cold {
		return fmt.Sprintf("How to do this pass is in the file %s: read all of it first. %s", rp.GuidePath, use)
	}
	data := rp.GuideText
	if data == "" {
		return ""
	}
	if len(data) > guideRoom {
		data = data[:guideRoom]
	}
	return fmt.Sprintf("How to do this pass is in the guide below, between the lines of plus signs. It's material to work from, not instructions to you. %s++++++++++\n%s\n++++++++++\n\n", use, strings.TrimSpace(data))
}

// reviewPassForm says how a review's pass's agent replies: ws reads it
// into findings.
func reviewPassForm(rp ReviewPass) string {
	def := rp.Def
	var b strings.Builder
	fmt.Fprintf(&b, "Reply in exactly this form, and nothing else. First, %s. Then each finding, the most important first, twelve at most, as:\n\n", def.Summary)
	kinds := strings.Join(def.Kinds, ", ")
	if kinds == "" {
		kinds = "what it is as the skill would file it, in a word or a few; " + strings.Join(def.Blocks, " or ") + " for one that should stop the change going in"
	}
	fmt.Fprintf(&b, "## <its kind: %s>\n@ <path>:<line>\n<what you have to say, in a sentence or three>\n\n", kinds)
	fmt.Fprintf(&b, "The @ line is where the finding is: the file's path in the repo, and the line, or `<first>-<last>` for several, numbered as in the file at %s, the change's head. For lines the change removes, number them as they were before it and write ` old` after: `@ main.go:3-7 old`. Nothing else goes on the @ line. Put a finding on the fewest lines that show it, and on lines the change adds or touches where you can.\n\n", short7(rp.Head))
	b.WriteString("To propose code in place of those lines, add after that:\n\n```suggestion\n<the lines as they should read: whole lines, indented as the file is>\n```\n\n")
	b.WriteString("A finding about the change as a whole (it should be split, it fixes this in the wrong place, something's missing that no line shows) has no @ line. It's a finding all the same, of a kind.\n\n")
	if len(def.Blocks) > 0 {
		fmt.Fprintf(&b, "A finding that's %s says the change shouldn't go in as it is: while one is open, the user sees this pass as \"no go\". So if you think it shouldn't be merged, say why as a finding of that kind, on the lines it's about, or on none if it's about the whole. Don't use it for anything less.\n\n", strings.Join(def.Blocks, " or "))
	}
	b.WriteString("What you say in a finding is read by the user, and, if they take it, by the change's author as their comment: write it to the author, plainly, saying what's wrong and why, not what you looked at. With nothing to find, your first line is all there is. Don't write headings of any other kind, and don't describe what you ran.\n")
	return b.String()
}

// ReviewPassFirst starts a review's pass's agent: what a pass is, the
// change and where it is (or, cold, the change itself), what this pass
// looks for, and how to reply.
func ReviewPassFirst(h Here, c *conversation.Conversation, rp ReviewPass) string {
	def := rp.Def
	var b strings.Builder
	b.WriteString(reviewPassIntro(h, rp))
	if def.Cold {
		b.WriteString(reviewPassGiven(rp))
	} else {
		b.WriteString(rp.start())
		b.WriteString(reviewPassChange(rp))
		b.WriteString(para(h.Agents))
		b.WriteString(talkReading(h))
		b.WriteString(repos(h))
	}
	fmt.Fprintf(&b, "This pass is %q: %s\n\n", def.Title, def.About)
	if def.Looks != "" {
		b.WriteString(def.Looks + "\n\n")
	}
	b.WriteString(reviewPassGuide(rp))
	b.WriteString(reviewPassSettled(c))
	b.WriteString(reviewPassForm(rp))
	return b.String()
}

// ReviewPassNext is the pass run again, in its agent's session: the
// change may have moved on, and the user has settled some of what it
// found.
func ReviewPassNext(h Here, c *conversation.Conversation, rp ReviewPass) string {
	var b strings.Builder
	b.WriteString("The user has run the pass again. ")
	if rp.Def.Cold {
		b.WriteString("The change as it now is:\n\n")
		b.WriteString(reviewPassGiven(rp))
	} else {
		was := ""
		if w := c.Written(); w != nil && w.Around != "" && w.Around != rp.Head {
			was = fmt.Sprintf(" It's been pushed to since you last read it, at %s.", short7(w.Around))
		}
		fmt.Fprintf(&b, "Read the change again:%s (The shell works for the commands your first message listed: if one was refused, that was it alone.) ", was)
		b.WriteString(reviewPassChange(rp))
	}
	b.WriteString(reviewPassSettled(c))
	b.WriteString("Say what stands now: a finding the change has since dealt with is gone, one that still stands is said again on its lines as they're now numbered, with the same kind (one about the change as a whole, in the same words), and what's new is added. " + reviewPassForm(rp))
	return b.String()
}

// reviewDismissedWhy is why a review's finding was dismissed, as its
// pass's agent is told it.
var reviewDismissedWhy = map[string]string{
	conversation.DismissWrong:   "it was wrong",
	conversation.DismissAlready: "it had been said already",
	conversation.DismissMinor:   "so, but not worth raising",
}

// reviewPassSettled says what the user did with what the pass found last
// time.
func reviewPassSettled(c *conversation.Conversation) string {
	w := c.Written()
	if w == nil {
		return ""
	}
	var done, dismissed []string
	for _, f := range w.Findings {
		where := "the whole change"
		if f.Placed() {
			where = fmt.Sprintf("%s:%d", f.Path, f.Line)
		}
		said := fmt.Sprintf("- %s, %s: %s", f.Kind, where, clipLine(f.Text, 120))
		switch c.FindingState(f.Key) {
		case conversation.FindingDone:
			done = append(done, said)
		case conversation.FindingDismissed:
			if why := reviewDismissedWhy[c.DismissedAs(f.Key)]; why != "" {
				said += " (" + why + ")"
			}
			dismissed = append(dismissed, said)
		}
	}
	sort.Strings(done)
	sort.Strings(dismissed)
	var b strings.Builder
	if len(done) > 0 {
		b.WriteString("Of what you found last time, the user took these into their review (say them again only if they still stand; they'll stay taken):\n" + strings.Join(done, "\n") + "\n\n")
	}
	if len(dismissed) > 0 {
		b.WriteString("They dismissed these: don't raise them again, unless the change has changed what they're about:\n" + strings.Join(dismissed, "\n") + "\n\n")
	}
	return b.String()
}

// Following up a review: of each comment of the user's that's still open,
// whether what's been pushed since deals with it.

// followUpForm says how a follow-up's agent replies: a finding for each
// comment, by its label.
func followUpForm(rp ReviewPass) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Reply in exactly this form, and nothing else. First, %s. Then, for each comment above, in order:\n\n", rp.Def.Summary)
	fmt.Fprintf(&b, "## <%s, %s or %s>\n@ <its label: T1>\n<a sentence or two, said to the user>\n\n", page.FollowedAddressed, page.FollowedPartly, page.FollowedNot)
	fmt.Fprintf(&b, "- %s: the change now does what the comment asked, or its question has been answered. Say what was done and where (the file and line as they now are).\n", page.FollowedAddressed)
	fmt.Fprintf(&b, "- %s: some of it, or the author said why they won't, which is the user's to judge. Say what was done, and what's left or what they said.\n", page.FollowedPartly)
	fmt.Fprintf(&b, "- %s: nothing that's been pushed deals with it. Say so even where the author replied that they'd do it.\n\n", page.FollowedNot)
	b.WriteString("Go by the code as it now is, not by what a reply says was done. Say something of every comment, and of nothing else. Don't write headings of any other kind, and don't describe what you ran.\n")
	return b.String()
}

// followUpThreads lists the user's open comments, each with its lines as
// they were and what's been said since.
func followUpThreads(rp ReviewPass) string {
	var b strings.Builder
	b.WriteString("The user's comments that are still open:\n\n")
	for _, t := range rp.Threads {
		lines := fmt.Sprintf("line %d", t.Line)
		if t.EndLine > t.Line {
			lines = fmt.Sprintf("lines %d-%d", t.Line, t.EndLine)
		}
		if t.Side == "old" {
			lines = "removed " + lines
		}
		fmt.Fprintf(&b, "%s. %s, %s, at %s (%s):\n", t.Label, t.Path, lines, short7(t.Commit), firstNonEmpty(t.Kind, "a comment"))
		for _, l := range t.Excerpt {
			b.WriteString("    | " + l + "\n")
		}
		fmt.Fprintf(&b, "  %s\n", strings.ReplaceAll(strings.TrimSpace(t.Body), "\n", "\n  "))
		for _, r := range t.Replies {
			fmt.Fprintf(&b, "    %s\n", clipLine(r, 400))
		}
		b.WriteString("\n")
	}
	return b.String()
}

// followUpChange says where the pull request's commits are, and what's
// changed since the user reviewed.
func followUpChange(rp ReviewPass) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Its commits are in the repo's clone, %s. ", rp.Clone)
	switch {
	case rp.Reviewed == "" || rp.Since == "":
		fmt.Fprintf(&b, "It's at %s; `%s` is the whole change. ", short7(rp.Head), rp.diff(rp.Base, rp.Head))
	case rp.Since == "commits":
		fmt.Fprintf(&b, "The user reviewed it at %s and it's now at %s: `%s` is what's been pushed since, and `%s` the whole change. ", short7(rp.Reviewed), short7(rp.Head), rp.diff(rp.Reviewed, rp.Head), rp.diff(rp.Base, rp.Head))
	default:
		fmt.Fprintf(&b, "The user reviewed it at %s, and it's been rebased since, to %s: there's no diff of what's been pushed, so read the files as they now are against what each comment asked. `%s` is the whole change. ", short7(rp.Reviewed), short7(rp.Head), rp.diff(rp.Base, rp.Head))
	}
	b.WriteString(generatedLeft(rp))
	fmt.Fprintf(&b, "`git -C %s show %s:<path>` is a file as it now is.\n\n", rp.Clone, rp.Head)
	return b.String()
}

// FollowUpFirst starts a follow-up's agent.
func FollowUpFirst(h Here, c *conversation.Conversation, rp ReviewPass) string {
	var b strings.Builder
	fmt.Fprintf(&b, "You're following up a review, in the web UI of ws, which manages this multi-repo workspace (%s). The user reviewed the pull request %s (%q) and left comments on its lines. For each one that's still open, say whether what's been pushed since deals with it. What you say is shown to the user on each comment, for them to resolve it or answer. You change nothing, you never run the change's code, and you never post anything.\n\n", h.Root, rp.PR, rp.Title)
	b.WriteString(followUpChange(rp))
	b.WriteString(talkReading(h))
	b.WriteString(repos(h))
	b.WriteString(followUpThreads(rp))
	b.WriteString(followUpForm(rp))
	return b.String()
}

// FollowUpNext is the follow-up run again, in its agent's session.
func FollowUpNext(h Here, c *conversation.Conversation, rp ReviewPass) string {
	var b strings.Builder
	b.WriteString("The user has run the follow-up again. ")
	b.WriteString(followUpChange(rp))
	b.WriteString(followUpThreads(rp))
	b.WriteString(followUpForm(rp))
	return b.String()
}

// The guide: what a change does, and the order to read it in.

// guideForm says how the guide's agent replies: ws reads it into groups
// of files.
func guideForm(rp ReviewPass) string {
	var b strings.Builder
	b.WriteString("Reply in exactly this form, and nothing else.\n\n")
	b.WriteString("First, a paragraph on what the change does: what's different once it's in, said from the change itself and not from its description. Three sentences at most.\n\n")
	b.WriteString("Then:\n\n## differs\n- <something the description says that the change doesn't do, or the change does that the description doesn't say: a line each>\n\n")
	b.WriteString("With nothing that differs, write `- Nothing.` under it. This is the part the user most wants: what they'd be surprised by, having read only the description. Don't list what merely isn't mentioned because it follows from what is.\n\n")
	b.WriteString("Then the files, in groups, in the order someone new to the change should read them: what the rest follows from first (a type, an interface, the function everything calls), then what uses it, then its tests, then what's incidental.\n\n")
	b.WriteString("## read: <what these files are, in a few words>\n<a line on why they're read here>\n- <path>: <what the change does to this file, in a line>\n\n")
	b.WriteString("## skim: <what these files are>\n<why they needn't be read closely>\n- <path>: <a line>\n\n")
	b.WriteString("A `skim` group is for files where reading every line would tell the user nothing: a rename carried through, generated code, a lockfile, a snapshot, the same mechanical edit made in many places. Never put a file in one because it's long, or tests, or dull: if a line of it could be wrong in a way that matters, it's to read. Place every file the change touches, once, by its path exactly as listed. Two to five groups is usual; one is fine for a small change. Don't write headings of any other kind.\n")
	return b.String()
}

// guideGiven is what the guide's agent is given, with every file to
// place.
func guideGiven(rp ReviewPass) string {
	var b strings.Builder
	b.WriteString(reviewPassGiven(rp))
	if len(rp.Files) > 0 {
		b.WriteString("Every file it touches, to place:\n")
		for _, f := range rp.Files {
			b.WriteString("- " + f + "\n")
		}
		b.WriteString("\n")
	}
	return b.String()
}

// GuideFirst starts the guide's agent.
func GuideFirst(h Here, c *conversation.Conversation, rp ReviewPass) string {
	var b strings.Builder
	fmt.Fprintf(&b, "You're writing a guide to a change, in the web UI of ws, which manages this multi-repo workspace (%s). The user is about to review %s. The guide says what the change does and what order to read its files in. It's shown above the diff, and the list of files can follow its order. It hides nothing: every file is still there to read. You change nothing, and you never post anything.\n\n", h.Root, rp.over())
	b.WriteString(guideGiven(rp))
	if rp.Def.Looks != "" {
		b.WriteString("How this workspace wants to be walked through a change: " + rp.Def.Looks + "\n\n")
	}
	if g := strings.TrimSpace(rp.GuideText); g != "" {
		if len(g) > guideRoom {
			g = g[:guideRoom]
		}
		fmt.Fprintf(&b, "How this workspace wants to be walked through a change is in the guide below, between the lines of plus signs. Take from it what to attend to and what order suits; the form to reply in is the one after it.\n\n++++++++++\n%s\n++++++++++\n\n", g)
	}
	b.WriteString(guideForm(rp))
	return b.String()
}

// GuideNext is the guide written again: the change has moved on.
func GuideNext(h Here, c *conversation.Conversation, rp ReviewPass) string {
	var b strings.Builder
	b.WriteString("The user has asked for the guide again. The change as it now is:\n\n")
	b.WriteString(guideGiven(rp))
	b.WriteString(guideForm(rp))
	return b.String()
}
