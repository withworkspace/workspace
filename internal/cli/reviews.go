package cli

import (
	"errors"
	"fmt"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/store"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/git"
	"github.com/withworkspace/workspace/internal/page"
	"github.com/withworkspace/workspace/internal/pulls"
	"github.com/withworkspace/workspace/internal/review"
)

// The Reviews page's side of the CLI: the service the web UI serves it
// from, asking an agent about a pull request (docking it first), Open
// here, ws show <repo>#<number>, and ws pr comment --propose, which is
// how a conversation's agent proposes a review comment.

// prView is ws show <repo>#<number>: a pull request outside any mission.
// It's where an agent asked about one starts: what it is and says it
// does, its stack and where it sits in it, its checks, what's been said
// in review, and where it's docked.
func prView(ctx *ops.Workspace, repo string, number int, full bool) (*view, error) {
	svc := ctx.PRService()
	d, err := svc.Detail(repo, number)
	if err != nil {
		return nil, err
	}
	pr := d.PR
	v := &view{head: fmt.Sprintf("Pull request %s: %s", pr.Ref(), pr.Title)}
	who := "by " + pr.Author
	if pr.Mine {
		who = "yours (" + pr.Author + ")"
	}
	line := fmt.Sprintf("%s, open; %s. +%d −%d in %d %s.", who, pr.Words, pr.Additions, pr.Deletions, pr.Files, plural(pr.Files, "file"))
	switch pr.Checks {
	case "success":
		line += " Its checks pass."
	case "failure":
		line += " Its checks are failing."
	case "pending":
		line += " Its checks are running."
	}
	v.body = append(v.body, line, fmt.Sprintf("Branch %s, for %s. %s", pr.Branch, pr.Base, pr.URL))
	if pr.Mission != nil {
		v.body = append(v.body, fmt.Sprintf("It's from mission %s (%s): ws show -m %s.", pr.Mission.Slug, pr.Mission.Title, pr.Mission.Slug))
	}
	// (A PR template's instructions, left in as HTML comments, aren't what
	// it says.)
	if body := strings.TrimSpace(htmlComment.ReplaceAllString(d.Body, "")); body != "" {
		if !full {
			body = clipString(body, showText)
		}
		v.body = append(v.body, "", "What it says it does:", indent(body, "  "))
	}

	// Its changes, and where to read them.
	bare := svc.BareDir(repo)
	v.body = append(v.body, "", fmt.Sprintf("Its changes are %s..%s (where it left %s, to its head), in the repo's clone:", short7(d.Base), short7(d.Head), d.BaseRef),
		fmt.Sprintf("  git -C %s diff %s %s", bare, d.Base, d.Head))
	if d.NeedsRebase {
		v.body = append(v.body, "  The layer below has moved on and this one hasn't followed (it needs a rebase): these are its changes against the layer below as it was.")
	}
	if files := git.Lines(bare, "diff", "--stat=100", "--stat-count=40", d.Base, d.Head); len(files) > 0 {
		v.body = append(v.body, "", "Files changed:")
		for _, f := range files {
			v.body = append(v.body, "  "+strings.TrimSpace(f))
		}
	}

	if st := d.Stack; st != nil {
		name := "a stack ws worked out from what each PR is based on"
		if !st.WorkedOut {
			name = fmt.Sprintf("GitHub's stack %d", st.Number)
		}
		v.body = append(v.body, "", fmt.Sprintf("It's layer %d of %d of %s, for %s. Each layer's PR shows only its own changes, on the layer below; they merge from the bottom up. Bottom first:", d.Layer, len(st.Layers), name, st.Base))
		if st.Merged > 0 {
			v.body = append(v.body, fmt.Sprintf("  (%d merged below)", st.Merged))
		}
		for i, l := range st.Layers {
			mark := "  "
			if l.Number == number {
				mark = "> "
			}
			v.body = append(v.body, fmt.Sprintf("%s%d. %s  %s (%s)", mark, i+1, l.Ref(), l.Title, l.Words))
			if l.Number != number {
				v.add(l.Ref(), fmt.Sprintf("layer %d: %s", i+1, l.Title))
			}
		}
	} else if pr.OnTopOf != 0 {
		v.body = append(v.body, "", fmt.Sprintf("It's on top of %s#%d: its base is that PR's branch.", repo, pr.OnTopOf))
		v.add(fmt.Sprintf("%s#%d", repo, pr.OnTopOf), "the PR it's on top of")
	}

	// Where it's checked out.
	top := ops.DockedTop(&d.PR, d.Stack)
	switch capsule := ctx.CapsuleOnBranch(repo, top.Branch); {
	case capsule != "" && top.Number == number:
		v.body = append(v.body, "", fmt.Sprintf("It's docked at %s: read the code there as files.", filepath.Join(ctx.WS().RepoDir(repo), capsule)))
	case capsule != "":
		v.body = append(v.body, "", fmt.Sprintf("Its stack is docked at its top layer (%s), at %s: read the code there as files. Layers above this one may have changed a file since: `git -C %s show %s:<path>` is a file as this layer has it.",
			top.Ref(), filepath.Join(ctx.WS().RepoDir(repo), capsule), bare, short7(d.Head)))
	case pr.Fork:
		v.body = append(v.body, "", "It's from a fork, so it isn't docked: read it from the repo's clone (git show <commit>:<path>).")
	default:
		v.body = append(v.body, "", fmt.Sprintf("It isn't docked here: read its files from the repo's clone (`git -C %s show %s:<path>`), or ws dock %s %d.", bare, short7(d.Head), repo, number))
	}

	// What's been said in review.
	if threads, err := svc.Threads(repo, number); err == nil && len(threads.Comments) > 0 {
		open := 0
		for _, n := range threads.Comments {
			if n.Status != review.Resolved {
				open++
			}
		}
		v.body = append(v.body, "", fmt.Sprintf("Said in review on GitHub (%d %s, %d open):", len(threads.Comments), plural(len(threads.Comments), "thread"), open))
		shown := threads.Comments
		if !full && len(shown) > 15 {
			v.body = append(v.body, fmt.Sprintf("  … %d earlier: --full shows them", len(shown)-15))
			shown = shown[len(shown)-15:]
		}
		for _, n := range shown {
			by := firstNonEmpty(n.By, "the user")
			kind := ""
			if n.Kind != "" {
				kind = n.Kind + ": "
			}
			state := ""
			if n.Status == review.Resolved {
				state = " [resolved]"
			}
			text := n.Body
			if !full {
				text = clipLine(text, 240)
			}
			v.body = append(v.body, fmt.Sprintf("  %s:%d  %s: %s%s%s", n.Path, n.Line, by, kind, text, state))
			for _, r := range n.Thread {
				reply := r.Body
				if !full {
					reply = clipLine(reply, 200)
				}
				v.body = append(v.body, fmt.Sprintf("      %s: %s", firstNonEmpty(strings.Replace(r.By, "you", "the user", 1), "?"), reply))
			}
		}
	}

	// The user's own reviews of it, made here.
	store := svc.Store(repo, number)
	if all, _ := store.List(); len(all) > 0 {
		v.body = append(v.body, "", "The user's reviews of it, made in ws:")
		for _, r := range all {
			switch r.State {
			case review.Draft:
				v.body = append(v.body, fmt.Sprintf("  a draft, not posted yet, with %d %s:", len(r.Comments), plural(len(r.Comments), "comment")))
				for _, n := range r.Comments {
					by := ""
					if n.By != "" {
						by = " (proposed by " + n.By + ")"
					}
					v.body = append(v.body, fmt.Sprintf("    %s:%d  %s: %s%s", n.Path, n.Line, n.Kind, clipLine(n.Body, 200), by))
				}
			default:
				at := ""
				if r.Submitted != nil {
					at = ", " + when(*r.Submitted)
				}
				v.body = append(v.body, fmt.Sprintf("  %s: %s at %s%s%s", r.ID, verdictText(r.Verdict), short7(r.Head), at, reviewCounts(r)))
			}
		}
	}
	// What passes over its review found, that the user hasn't settled.
	passes := conversation.ForWorkspace(ctx.WS().Root)
	said := false
	for _, def := range ctx.ReviewPasses(repo) {
		c := passes.ReviewPassOn(pr.Ref(), def.Name)
		st := page.ReviewStanding(def, c)
		if c == nil || c.Written() == nil {
			continue
		}
		if !said {
			v.body = append(v.body, "", "What passes over its review found (an agent's findings, for the user to take into their review or dismiss; nobody's said them to the author):")
			said = true
		}
		moved := ""
		if w := c.Written(); w.Around != "" && w.Around != d.Head {
			moved = fmt.Sprintf(", at %s: it's been pushed to since", short7(w.Around))
		}
		v.body = append(v.body, fmt.Sprintf("  %s (%s): %s%s", def.Title, def.Name, st.Words(), moved))
		for _, f := range c.Written().Findings {
			if c.FindingState(f.Key) != conversation.FindingOpen {
				continue
			}
			where := "the whole change"
			if f.Placed() {
				where = fmt.Sprintf("%s:%d", f.Path, f.Line)
			}
			text := f.Text
			if !full {
				text = clipLine(text, 240)
			}
			v.body = append(v.body, fmt.Sprintf("    %s  %s: %s", where, f.Kind, text))
		}
	}
	if d.Since != "" {
		v.body = append(v.body, "", "It's changed since the user last reviewed it.")
	}
	v.more = append(v.more,
		fmt.Sprintf("gh pr view %d --repo %s/%s --comments   (the conversation on GitHub)", number, ctx.WS().Org, repo),
		fmt.Sprintf("gh pr checks %d --repo %s/%s", number, ctx.WS().Org, repo))
	return v, nil
}

var htmlComment = regexp.MustCompile(`(?s)<!--.*?-->\s*`)

func short7(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

func verdictText(v string) string {
	switch v {
	case review.Approve:
		return "approved"
	case review.Changes:
		return "changes requested"
	}
	return "comments"
}

// reviewCounts is " (2 blocking, 1 nit)", or "".
func reviewCounts(r *review.Review) string {
	c := r.Counts()
	var parts []string
	for _, k := range review.Kinds {
		if c[k] > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", c[k], k))
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return " (" + strings.Join(parts, ", ") + ")"
}

// prShowView is ws show <repo>#<number>: the pull request, and what
// missions have said of it.
func prShowView(ctx *ops.Workspace, ref string, full bool) (*view, error) {
	repo, number, err := ctx.ParsePRRef(ref)
	if err != nil {
		return nil, err
	}
	v, err := prView(ctx, repo, number, full)
	if err != nil {
		return nil, err
	}
	// What missions have said of it: the workspace's talk and its own
	// reviews are in the view already.
	key := pulls.Key(repo, number)
	if said, err := ctx.SaidOf(store.Key{Name: key}); err == nil {
		said = slices.DeleteFunc(said, func(x ops.Said) bool { return x.Scope == "" || x.Scope == key })
		v.addSaid(saidSection{title: "Said of it in missions", said: said})
	}
	return v, nil
}

func newPRCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pr",
		Short: "Pull requests outside a mission: what the web UI's Reviews page shows",
		Long: `Pull requests across the workspace's repos are listed and reviewed in the
web UI (ws web /reviews). ws show <repo>#<number> shows one here.`,
		PersistentPreRun: func(cmd *cobra.Command, args []string) { cmd.SilenceUsage = true },
	}
	cmd.AddCommand(newPRCommentCmd())
	return cmd
}

func newPRCommentCmd() *cobra.Command {
	var proposing bool
	var path, kind, side, suggest string
	var line, endLine int
	cmd := &cobra.Command{
		Use:   "comment --propose <repo>#<number> --path <file> --line <n> --kind <kind> [what to say]",
		Short: "Propose a review comment on a pull request's lines (a conversation's agent)",
		Long: `Propose a comment on lines of a pull request, for the user to add to their
draft review of it (or not): it shows under your reply. Nothing is posted
to GitHub until they submit the review.

Lines are numbered as in the file at the pull request's head (--side old:
as in the file before its changes, for lines it removes). --kind is
blocking, suggestion, question or nit. --suggest - reads lines to replace
them with from stdin, which the author can apply.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			if !proposing {
				return errors.New("a review comment is written in the web UI's review; --propose is how a conversation's agent proposes one for the user to add")
			}
			repo, number, err := ctx.ParsePRRef(args[0])
			if err != nil {
				return err
			}
			p := conversation.Proposal{Kind: conversation.ProposeComment, PR: pulls.Key(repo, number), Path: path, Line: line, EndLine: endLine, Side: side,
				NoteKind: kind, Text: strings.TrimSpace(strings.Join(args[1:], " "))}
			if suggest != "" {
				if suggest != "-" {
					return errors.New("--suggest - reads the replacement lines from stdin")
				}
				data, err := io.ReadAll(os.Stdin)
				if err != nil {
					return err
				}
				s := strings.TrimRight(string(data), "\n")
				p.Suggestion = &s
			}
			if err := checkProposedComment(ctx.PRService(), repo, number, &p); err != nil {
				return err
			}
			return propose(ctx, p)
		},
	}
	cmd.Flags().BoolVar(&proposing, "propose", false, "propose it in the conversation you're answering, for the user to add to their review")
	revisesFlag(cmd)
	cmd.Flags().StringVar(&path, "path", "", "the file, as the pull request names it")
	cmd.Flags().IntVar(&line, "line", 0, "the line it's on (the first, of several)")
	cmd.Flags().IntVar(&endLine, "end-line", 0, "the last line, when it's on several")
	cmd.Flags().StringVar(&side, "side", "new", "new: lines as the pull request has them; old: lines it removes")
	cmd.Flags().StringVar(&kind, "kind", "", "blocking, suggestion, question or nit")
	cmd.Flags().StringVar(&suggest, "suggest", "", "- reads lines to replace them with from stdin")
	return cmd
}

// checkProposedComment checks a comment an agent proposes against the
// pull request as it is, so what's wrong comes back to the agent rather
// than to the user: the file and lines are there, and its kind is one.
// It fills in the lines' text.
func checkProposedComment(svc *pulls.Service, repo string, number int, p *conversation.Proposal) error {
	if p.EndLine == 0 {
		p.EndLine = p.Line
	}
	valid := false
	for _, k := range review.Kinds {
		valid = valid || k == p.NoteKind
	}
	switch {
	case !valid:
		return fmt.Errorf("--kind is %s, not %q", strings.Join(review.Kinds, ", "), p.NoteKind)
	case p.Path == "" || p.Line < 1 || p.EndLine < p.Line:
		return errors.New("say which lines: --path <file> --line <n> (and --end-line <m> for several)")
	case p.Side != "new" && p.Side != "old":
		return fmt.Errorf("--side is new or old, not %q", p.Side)
	case p.Text == "" && p.Suggestion == nil:
		return errors.New("say something, or suggest a change (--suggest -)")
	}
	d, err := svc.Detail(repo, number)
	if err != nil {
		return err
	}
	commit := d.Head
	if p.Side == "old" {
		commit = d.Base
	}
	lines, err := review.Blob(svc.BareDir(repo), commit, p.Path)
	if err != nil {
		return fmt.Errorf("%s isn't a file of %s#%d at %s", p.Path, repo, number, short7(commit))
	}
	if p.EndLine > len(lines) {
		return fmt.Errorf("%s has %d lines at %s, not %d", p.Path, len(lines), short7(commit), p.EndLine)
	}
	p.Excerpt = lines[p.Line-1 : p.EndLine]
	return nil
}
