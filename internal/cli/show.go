package cli

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/withworkspace/workspace/internal/artifact"
	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/doc"
	"github.com/withworkspace/workspace/internal/git"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/orchestrator"
	"github.com/withworkspace/workspace/internal/page"
	wsref "github.com/withworkspace/workspace/internal/ref"
	"github.com/withworkspace/workspace/internal/review"
	"github.com/withworkspace/workspace/internal/store"
	"github.com/withworkspace/workspace/internal/ui"
	"github.com/withworkspace/workspace/internal/workspace"
)

// ws show: one of a mission's things, by its ref, with what it links to.
// It's how an agent finds its way: it starts from a ref, reads what the
// thing is and why, and follows the links it needs. Every view has the
// same shape: what it is in a line, the essentials, links (each a ref to
// show next), what happened lately, and how to read more. Views are kept
// short; --full has everything.

// showRefs is what a ref can be, for help and for errors.
const showRefs = `  (none)            the mission: where it stands, and everything in it
  s2                a step of the mission's plan, or a stage of its brief's
  c1                crew
  repo/name         one of the mission's capsules
  an artifact's ref repo/capsule:path, repo:path, mission:path, repo#12, a URL
                    (or the path of a file you've recorded, from where you are)
  r1, r1.c2         a review, or one of its comments
  t3                a thread on a document
  d1                a decision
  b1                a verification bundle
  20261109-150405   an orchestrator run
  an address        ws:<a name> (ws:fix-token-refresh, ws:w4), or one of the
                    workspace's on GitHub (a file's, a pull request's), as a
                    link in a document has it

Outside a mission:
  skills            the workspace's guide for agents, and its skills: what
                    each is for, and the file that says how it's done
  kinds             the kinds of document it has: where each goes, what
                    it's named, its commit messages, and its guide
  passes            each pass over pages: what it found, and how much of
                    it was right and acted on
  process           the workspace's process (how work goes here, in prose)
                    and what an agent understood of it: its kinds of work,
                    its rules, and what ws can't see or do
  w3                a workspace conversation, or a thread on a document
  p3                a page: its file, what it is, its cells and passes, and
                    what it says
  a3, a3.v2         an artifact (a versioned HTML prototype), or a version
                    of it: its versions, what each changed, and where each
                    came from
  repo:path         a file in a repo (repo/path, repo/capsule:path in a
                    capsule, :12-20 for lines), and what's been said of it
                    in any mission: comments on it, decisions and threads
                    about it, talk that named it
  repo#12           an open pull request: what it says it does, its stack,
                    its checks, what's been said in review, where it's docked`

func newShowCmd() *cobra.Command {
	var slug, format string
	var full bool
	cmd := &cobra.Command{
		Use:   "show [ref]",
		Short: "Show one of a mission's things by its ref, with what it links to",
		Long: `Show one of a mission's things: what it is, the essentials, and what it
links to, each as a ref to show next. Start anywhere and follow the links
you need. The mission is the --mission flag, $WS_MISSION, or the one that
owns the capsule you're in.

Refs:
` + showRefs + `

Views are kept short and say what they left out; --full shows it all.`,
		Args:              cobra.MaximumNArgs(1),
		PersistentPreRun:  func(cmd *cobra.Command, args []string) { cmd.SilenceUsage = true },
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, args []string) error {
			if format != "" && format != "llm" && format != "text" {
				return fmt.Errorf("unknown format %q (want text or llm)", format)
			}
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			ref := fromAddress(ctx, strings.TrimSpace(firstArg(args)), &slug)
			if ref == "skills" || ref == "kinds" || ref == "passes" || ref == "process" { // the workspace's own way of working
				v := skillsView(ctx.WS())
				switch ref {
				case "kinds":
					v = kindsView(ctx.WS())
				case "passes":
					v = passesView(ctx)
				case "process":
					if v, err = processView(ctx); err != nil {
						return err
					}
				}
				if format == "llm" {
					_, err = os.Stdout.WriteString(v.render(false, full))
				} else {
					_, err = io.WriteString(ui.Stdout, v.render(true, full))
				}
				return err
			}
			if page.IsRef(ref) || conversation.IsWorkspaceRef(ref) || artifact.IsRef(ref) { // the workspace's, not a mission's
				var v *view
				switch {
				case page.IsRef(ref):
					v, err = pageView(ctx, ref, full)
				case artifact.IsRef(ref):
					v, err = artifactShowView(ctx, ref, full)
				default:
					v, err = workspaceTalkView(ctx, ref, full)
				}
				if err != nil {
					return err
				}
				if format == "llm" {
					_, err = os.Stdout.WriteString(v.render(false, full))
				} else {
					_, err = io.WriteString(ui.Stdout, v.render(true, full))
				}
				return err
			}
			m, err := resolveMission(ctx, slug)
			// repo#12 is a mission's artifact when one records it; else, and
			// outside a mission, it's the pull request itself.
			_, _, isPR := wsref.ParsePR(ref)
			// repo:path is a mission's document when one records it; else,
			// and outside a mission, it's the file itself (as refs name it
			// too: repo/path, with lines).
			isFile := isFileName(ref)
			if err != nil {
				if isPR && slug == "" {
					return showPR(ctx, ref, format, full)
				}
				if isFile && slug == "" {
					return showDoc(ctx, ref, format, full)
				}
				return err
			}
			if ref == "" {
				return showMission(ctx, m, format)
			}
			g := loadGraph(ctx, m, full)
			v, err := g.view(ref)
			if err != nil && isPR {
				return showPR(ctx, ref, format, full)
			}
			if err != nil && isFile {
				if derr := showDoc(ctx, ref, format, full); derr == nil {
					return nil
				}
			}
			if err != nil {
				return err
			}
			if format == "llm" {
				_, err = os.Stdout.WriteString(v.render(false, full))
			} else {
				_, err = io.WriteString(ui.Stdout, v.render(true, full))
			}
			return err
		},
	}
	addMissionFlag(cmd, &slug)
	cmd.Flags().StringVarP(&format, "format", "f", "text", "output format: text or llm")
	cmd.Flags().BoolVar(&full, "full", false, "show everything a view would leave out")
	return cmd
}

// showMission is the mission's own view: ws mission show.
func showMission(ctx *ops.Workspace, m *mission.Mission, format string) error {
	store := missionStore(ctx)
	v := ops.BuildMissionViews(ctx.WS(), ctx.GitHub(), []*mission.Mission{m})[0]
	brief, _ := store.ReadBrief(m.Slug)
	events, _ := store.ReadLog(m.Slug, missionShowLogLimit)
	if format == "llm" {
		_, err := os.Stdout.WriteString(formatMissionLLM(ctx, v, store.BriefPath(m.Slug), brief, events))
		return err
	}
	printMissionText(ctx, v, store.BriefPath(m.Slug), brief, events)
	return nil
}

// graph is what one ws show reads of a mission, once.
type graph struct {
	ctx     *ops.Workspace
	m       *mission.Mission
	v       ops.MissionView
	store   mission.Store
	brief   string
	stages  []mission.StageState
	events  []mission.Event // the whole log, oldest first
	arts    []ops.ArtifactView
	reviews []*review.Review
	threads []*conversation.Conversation
	full    bool
}

func loadGraph(ctx *ops.Workspace, m *mission.Mission, full bool) *graph {
	g := &graph{ctx: ctx, m: m, store: missionStore(ctx), full: full}
	g.v = ops.BuildMissionViewsFrom(ctx.WS(), nil, []*mission.Mission{m})[0]
	g.brief, _ = g.store.ReadBrief(m.Slug)
	g.stages, _ = m.StageProgress(g.brief)
	g.events, _ = g.store.ReadLog(m.Slug, 0)
	g.arts = ops.ResolveArtifacts(ctx.WS(), g.store, g.v)
	g.reviews, _ = review.For(ctx.WS().Root, m.Slug).List()
	g.threads, _ = conversation.For(ctx.WS().Root, m.Slug).List()
	return g
}

var (
	stageRefShape = regexp.MustCompile(`^s([1-9][0-9]*)$`)
	crewRefShape  = regexp.MustCompile(`^c[1-9][0-9]*$`)
	reviewShape   = regexp.MustCompile(`^r[1-9][0-9]*$`)
	commentShape  = regexp.MustCompile(`^r[1-9][0-9]*\.c[1-9][0-9]*$`)
	runShape      = regexp.MustCompile(`^[0-9]{8}-[0-9]{6}(-[0-9]+)?$`)
)

// view finds what ref names, and shows it, with the decisions about it.
func (g *graph) view(ref string) (*view, error) {
	v, canon, err := g.find(ref)
	if err != nil {
		return nil, err
	}
	if canon != "" {
		g.decisionLinks(v, canon)
	}
	// What's said of it beyond this mission: the mission's own is above.
	keys, _, _ := ops.KeysOf(g.m.Slug, cmp.Or(canon, ref))
	if said, err := g.ctx.SaidOf(keys...); err == nil {
		said = slices.DeleteFunc(said, func(x ops.Said) bool { return x.Scope == g.m.Slug })
		v.addSaid(saidSection{title: "Said of it elsewhere", said: said})
	}
	return v, nil
}

// find shows what ref names, and says its ref as decisions name it (an
// artifact found by its path is known by its ref).
func (g *graph) find(ref string) (*view, string, error) {
	var v *view
	var err error
	switch {
	case stageRefShape.MatchString(ref):
		n, _ := strconv.Atoi(ref[1:])
		v, err = g.stageView(n)
	case crewRefShape.MatchString(ref):
		v, err = g.crewView(ref)
	case reviewShape.MatchString(ref):
		v, err = g.reviewView(ref)
	case commentShape.MatchString(ref):
		v, err = g.commentView(ref)
	case runShape.MatchString(ref):
		v, err = g.runView(ref)
	case conversation.IsRef(ref):
		v, err = g.threadView(ref)
	case mission.IsDecisionRef(ref):
		v, err = g.decisionView(ref)
		return v, "", err
	case mission.IsBundleRef(ref):
		v, err = g.bundleView(ref)
	default:
		if repo, name, ok := strings.Cut(ref, "/"); ok && !strings.ContainsAny(ref, ":#") && g.m.HasCapsule(repo, name) {
			v, err = g.capsuleView(repo, name)
			break
		}
		if av, ok := g.artifact(ref); ok {
			return g.artifactView(av), av.Ref, nil
		}
		return nil, "", fmt.Errorf("mission %s has nothing called %q. A ref is one of:\n%s", g.m.Slug, ref, showRefs)
	}
	return v, ref, err
}

// artifact finds an artifact by its ref, or by a path to its file.
func (g *graph) artifact(ref string) (ops.ArtifactView, bool) {
	for _, av := range g.arts {
		if av.Ref == ref {
			return av, true
		}
	}
	if r, err := g.ctx.NormalizeArtifact(g.m, ref, userDirOr()); err == nil {
		for _, av := range g.arts {
			if av.Ref == r.String() {
				return av, true
			}
		}
	}
	return ops.ArtifactView{}, false
}

// view is what ws show prints of one thing.
type view struct {
	head   string   // what it is, in a line
	body   []string // the essentials: lines and paragraphs
	links  []link
	lately []mission.Event
	more   []string // how to read more
	cut    int      // links left out
	saids  []saidSection
}

type link struct{ ref, what string }

const (
	showLinks  = 12   // links a view lists, without --full
	showLately = 8    // events a view lists
	showText   = 1500 // characters of a long text (a handoff, a dispatch)
)

func (v *view) add(ref, what string) { v.links = append(v.links, link{ref, what}) }

func (v *view) render(styled, full bool) string {
	var b strings.Builder
	head := v.head
	if styled {
		head = ui.Bold.Render(head)
	}
	b.WriteString(head + "\n")
	if len(v.body) > 0 {
		b.WriteString("\n")
		for _, l := range v.body {
			b.WriteString(strings.TrimRight(l, "\n") + "\n")
		}
	}
	if len(v.links) > 0 {
		links := v.links
		if !full && len(links) > showLinks {
			links, v.cut = links[:showLinks], len(links)-showLinks
		}
		width := 0
		for _, l := range links {
			width = max(width, len(l.ref))
		}
		b.WriteString("\nLinks (ws show <ref> for any):\n")
		for _, l := range links {
			fmt.Fprintf(&b, "  %-*s  %s\n", width, l.ref, l.what)
		}
		if v.cut > 0 {
			fmt.Fprintf(&b, "  … %d more: --full shows them\n", v.cut)
		}
	}
	for _, s := range v.saids {
		s.render(&b, full)
	}
	if len(v.lately) > 0 {
		b.WriteString("\nLately:\n")
		events := v.lately
		if !full && len(events) > showLately {
			fmt.Fprintf(&b, "  … %d earlier: --full shows them\n", len(events)-showLately)
			events = events[len(events)-showLately:]
		}
		for _, e := range events {
			b.WriteString("  " + formatEventLine(e, true) + "\n")
		}
	}
	if len(v.more) > 0 {
		b.WriteString("\nMore:\n")
		for _, l := range v.more {
			b.WriteString("  " + l + "\n")
		}
	}
	return b.String()
}

// clip cuts long text, saying how to see the rest.
func (g *graph) clip(text string) string { return clipText(text, g.full) }

// clipText cuts long text, unless full, saying how to see the rest.
func clipText(text string, full bool) string {
	text = strings.TrimSpace(text)
	if full || len(text) <= showText {
		return text
	}
	cut := strings.LastIndex(text[:showText], "\n")
	if cut < showText/2 {
		cut = showText
	}
	return text[:cut] + "\n… (cut: --full shows it all)"
}

func (g *graph) about(match func(e mission.Event) bool) []mission.Event {
	var out []mission.Event
	for _, e := range g.events {
		if match(e) {
			out = append(out, e)
		}
	}
	return out
}

func when(t time.Time) string { return t.Local().Format("2006-01-02 15:04") }

// crewStanding is where crew stands, in words.
func crewStanding(c mission.Crew) string {
	var parts []string
	if c.Ended != nil {
		ended := "ended"
		if c.Exit != "" && c.Exit != "0" {
			ended += " (" + c.Exit + ")"
		}
		parts = append(parts, ended)
	} else {
		running := "running"
		if c.Activity != "" {
			running += ", " + c.Activity
		}
		if work, ok := c.OwnWork(time.Now()); ok {
			running += " on its own work (" + mission.WorkWhat(work) + ")"
		}
		parts = append(parts, running)
	}
	switch c.Status {
	case mission.CrewDone:
		parts = append(parts, "reported done"+quoteIf(c.Note))
	case mission.CrewAsking:
		parts = append(parts, "asking the user"+quoteIf(c.Note))
	}
	return strings.Join(parts, ", ")
}

func quoteIf(s string) string {
	if s = oneLine(s); s == "" {
		return ""
	}
	if len(s) > 120 {
		s = s[:117] + "…"
	}
	return ": " + s
}

// stageOf is the stage crew worked on, as a ref, or "".
func (g *graph) stageOf(crewID string) string {
	if st, ok := mission.StageFor(g.stages, crewID); ok && !st.Implicit {
		return fmt.Sprintf("s%d", st.N)
	}
	return ""
}

func (g *graph) crew(id string) (mission.Crew, bool) {
	for _, c := range g.m.Crew {
		if c.ID == id {
			return c, true
		}
	}
	return mission.Crew{}, false
}

func (g *graph) crewLink(v *view, id, why string) {
	if c, ok := g.crew(id); ok {
		v.add(id, why+" ("+crewStanding(c)+")")
	}
}

var stageInEvent = regexp.MustCompile(`\b(?:stage|step) ([0-9]+)\b`)

func (g *graph) stageView(n int) (*view, error) {
	i := slices.IndexFunc(g.stages, func(st mission.StageState) bool { return st.N == n })
	if i < 0 || g.stages[i].Implicit {
		if len(g.stages) == 1 && g.stages[0].Implicit {
			return nil, fmt.Errorf("mission %s's brief lists no stages: the whole brief is one (ws show shows it)", g.m.Slug)
		}
		if g.m.Plan != nil {
			return nil, fmt.Errorf("mission %s's plan has steps s1 to s%d, not s%d", g.m.Slug, len(g.stages), n)
		}
		return nil, fmt.Errorf("mission %s's brief has stages s1 to s%d, not s%d", g.m.Slug, len(g.stages), n)
	}
	st := g.stages[i]
	noun := st.Noun()
	Noun := strings.ToUpper(noun[:1]) + noun[1:]
	state := "to do"
	switch {
	case st.Skipped():
		state = "skipped"
	case st.Accepted():
		state = "accepted"
	case st.Current && st.Held():
		state = "current, waiting for the orchestrator's call, or the user's"
	case st.Current:
		state = "current"
	}
	v := &view{head: fmt.Sprintf("%s s%d of %d (%s): %s", Noun, st.N, len(g.stages), state, st.Short())}
	v.body = append(v.body, st.Text)
	if s := st.Step; s != nil {
		article := "A "
		if s.Type != "" && strings.ContainsAny(s.Type[:1], "AEIOU") {
			article = "An "
		}
		who := article + s.Type + " step. Who: " + s.WhoSays() + "."
		if s.Type == mission.StepVerify {
			who = "A Verify step. Who: a verifier, crew who didn't do the work, " + strings.TrimPrefix(s.WhoSays(), "crew, ") + ". It shows the work by using it, changes nothing, and hands over a bundle; ws passes the step once nothing in it is unmet."
		}
		switch s.Who {
		case mission.WhoYou:
			who += " It's a checkpoint: only the user passes it."
		case mission.WhoPlanner:
			who += " It writes the steps after it."
		}
		v.body = append(v.body, who)
		if s.Makes != "" {
			v.body = append(v.body, "It makes: "+s.Makes)
		}
		if s.From != "" {
			v.body = append(v.body, "The options: "+s.From)
		}
		switch {
		case s.Type == mission.StepOpenPR:
			draft := ""
			if s.Draft {
				draft = ", as a draft"
			}
			v.body = append(v.body, "ws opens its pull request itself"+draft+", holding the guarantees about pull requests, and passes the step when it merges. Crew don't open one.")
		case s.MakesPR():
			v.body = append(v.body, "It opens a pull request, through ws: its crew run ws mission open-pr, which holds the guarantees about pull requests. Crew at other steps don't open one, and the orchestrator doesn't accept it until one's open.")
		}
		if s.OnlyIf != "" {
			v.body = append(v.body, "Only if "+s.OnlyIf+": the orchestrator starts it or skips it, saying why.")
		}
		if s.InEach != "" {
			v.body = append(v.body, "In each "+s.InEach+": the orchestrator names the repos when it starts, and it runs once in each, done when every one is.")
		}
		if s.CarryOn > 0 {
			v.body = append(v.body, fmt.Sprintf("Carrying on from step %d: the crew who did it carry on, resumed where they can be.", s.CarryOn))
		}
		if s.LimitText != "" {
			v.body = append(v.body, "Its limit: "+s.LimitText+". ws holds the clock from when crew start on it.")
		}
		if len(s.Checks) > 0 {
			lines := []string{"It's done when these are true (ws runs the commands in its capsule once crew report done, and the judge judges the rest):"}
			if s.Type == mission.StepVerify {
				lines[0] = "It shows whether these are true (ws runs the commands in its capsule, its verifier settles the rest, and one marked yours is the user's):"
			}
			for i, c := range s.Checks {
				line := fmt.Sprintf("%d. %s", i+1, c.Text)
				if c.Yours {
					line += " (yours)"
				}
				lines = append(lines, line)
			}
			v.body = append(v.body, strings.Join(lines, "\n"))
		}
		if s.Quote != "" {
			v.body = append(v.body, "From the process: “"+s.Quote+"”")
		}
	}
	if st.CheckIn {
		v.body = append(v.body, "Check in with the user after it, before the next stage starts.")
	}
	if r := st.Record; r != nil {
		if r.Started != nil {
			v.body = append(v.body, "Started "+when(*r.Started)+".")
		}
		if r.Go != nil {
			v.body = append(v.body, fmt.Sprintf("Started %s by %s, at their call.", when(*r.Go), firstNonEmpty(r.GoBy, "someone")))
		}
		for _, in := range r.Instances {
			line := "Runs in " + in.Repo
			if in.Capsule != "" {
				line += " (" + in.Capsule + ")"
			}
			if in.Done != nil {
				line += ": done " + when(*in.Done)
			}
			v.body = append(v.body, line+".")
		}
		if r.Attempts > 0 {
			v.body = append(v.body, fmt.Sprintf("Started again %d %s: attempt %d.", r.Attempts, plural(r.Attempts, "time"), r.Attempts+1))
		}
		switch {
		case r.OutOfTime != nil && r.Accepted == nil:
			v.body = append(v.body, "Out of time "+when(*r.OutOfTime)+".")
		case r.Deadline != nil && r.Accepted == nil:
			v.body = append(v.body, "Its time runs out at "+r.Deadline.Local().Format("15:04")+" ("+when(*r.Deadline)+").")
		}
		for _, round := range r.Checks {
			lines := []string{fmt.Sprintf("Checks, round %d, of %s's done (%s, %s): %s.", round.N, round.Crew, firstNonEmpty(shortSHA(round.Commit), "no commit"), when(round.At), round.Summary())}
			for _, c := range round.Results {
				line := fmt.Sprintf("- %s: %s, by %s", c.Check, c.Result, c.By)
				if ev := strings.TrimSpace(c.Evidence); ev != "" {
					line += ": " + clipLine(ev, 300)
				}
				lines = append(lines, line)
			}
			v.body = append(v.body, strings.Join(lines, "\n"))
		}
		if r.Accepted != nil {
			a := fmt.Sprintf("Accepted %s by %s", when(*r.Accepted), firstNonEmpty(r.AcceptedBy, "someone"))
			if r.Skipped {
				a = fmt.Sprintf("Skipped %s by %s", when(*r.Accepted), firstNonEmpty(r.AcceptedBy, "someone"))
			}
			if r.Why != "" {
				a += ": " + r.Why
			}
			v.body = append(v.body, a+".")
		}
	}
	var crew []string
	if st.Record != nil {
		crew = st.Record.Crew
	}
	for _, id := range crew {
		g.crewLink(v, id, "worked on it")
	}
	for _, av := range g.arts {
		if at, ok := mission.ArtifactStage(g.stages, av.Artifact); ok && at.N == st.N && av.Kind != "handoff" && av.Kind != "review" {
			v.add(av.Ref, fmt.Sprintf("%s%s, made for it by %s", av.Kind, labelled(av.Label), firstNonEmpty(av.Crew, "the user")))
		}
	}
	for _, b := range g.m.Bundles {
		if b.Stage == st.Text {
			v.add(b.ID, "a bundle of it, by "+b.Crew+": "+b.Tally())
		}
	}
	for _, c := range st.NamedCapsules() {
		v.add(c, "the capsule it works in")
	}
	if i > 0 {
		v.add(fmt.Sprintf("s%d", st.N-1), "the "+noun+" before: "+g.stages[i-1].Short())
	}
	if i+1 < len(g.stages) {
		v.add(fmt.Sprintf("s%d", st.N+1), "the "+noun+" after: "+g.stages[i+1].Short())
	}
	v.lately = g.about(func(e mission.Event) bool {
		m := stageInEvent.FindStringSubmatch(e.Text)
		return e.Kind == mission.EventStage && m != nil && m[1] == strconv.Itoa(n)
	})
	return v, nil
}

func (g *graph) crewView(id string) (*view, error) {
	c, ok := g.crew(id)
	if !ok {
		return nil, fmt.Errorf("mission %s has no crew %s", g.m.Slug, id)
	}
	head := fmt.Sprintf("Crew %s (%s; %s)", c.ID, c.Agent(), crewStanding(c))
	switch c.Role {
	case mission.RoleBriefing:
		head = fmt.Sprintf("Crew %s, briefing the mission (%s; %s)", c.ID, c.Agent(), crewStanding(c))
	case mission.RoleAnswer:
		head = fmt.Sprintf("Crew %s, answering the orchestrator's question for the user (%s; %s)", c.ID, c.Agent(), crewStanding(c))
	}
	v := &view{head: head}
	span := "Started " + when(c.Started)
	if c.Ended != nil {
		span += ", ended " + when(*c.Ended)
	}
	v.body = append(v.body, span+".")
	if c.Capsule != "" {
		v.body = append(v.body, "Worked in "+c.Capsule+".")
	}
	if d := strings.TrimSpace(c.Dispatch); d != "" {
		v.body = append(v.body, "", "Dispatched with:", g.clip(d))
	}
	if c.Activity == mission.ActivityWaiting && c.Ended == nil && strings.TrimSpace(c.LastReply) != "" {
		waiting := "waiting for a reply"
		if _, ok := c.OwnWork(time.Now()); ok {
			waiting = "waiting on its own work"
		}
		v.body = append(v.body, "", "Its last message ("+waiting+"):", g.clip(c.LastReply))
	}
	if c.ResumeWith != "" {
		v.body = append(v.body, "", "The orchestrator suggests resuming it with: "+c.ResumeWith)
	}
	if c.HandedOff != nil {
		if data, err := os.ReadFile(g.store.HandoffPath(g.m.Slug, c.ID)); err == nil {
			v.body = append(v.body, "", "Its handoff ("+mission.HandoffRef(c.ID)+"):", "", g.clip(string(data)))
		}
	}
	if c.Capsule != "" {
		v.add(c.Capsule, "the capsule it worked in")
	}
	if s := g.stageOf(c.ID); s != "" {
		v.add(s, "the stage it worked on")
	}
	if c.HandedOff != nil {
		v.add(mission.HandoffRef(c.ID), "its handoff")
	}
	for _, av := range g.arts {
		if av.Crew == c.ID && av.Kind != "handoff" {
			v.add(av.Ref, fmt.Sprintf("it recorded this %s%s", av.Kind, labelled(av.Label)))
		}
	}
	for _, r := range g.reviews {
		for _, n := range r.Comments {
			if slices.ContainsFunc(n.Thread, func(t review.Reply) bool { return t.By == c.ID }) {
				v.add(n.ID, "a review comment it answered ("+n.Status+")")
			}
		}
	}
	for _, th := range g.threads {
		if slices.ContainsFunc(th.Turns, func(t conversation.Turn) bool { return t.By == c.ID }) {
			v.add(th.ID, "a thread it answered: "+threadLine(th))
		}
	}
	v.lately = g.about(func(e mission.Event) bool { return e.Crew == c.ID })
	if c.Conversation != "" && c.Ended != nil {
		v.more = append(v.more, "It recorded its conversation, so it can be resumed (ws mission resume-with "+c.ID+" \"…\").")
	}
	return v, nil
}

func labelled(label string) string {
	if label == "" {
		return ""
	}
	return ": " + label
}

func (g *graph) capsuleView(repo, name string) (*view, error) {
	key := repo + "/" + name
	var c ops.CapsuleView
	for _, cv := range g.v.Capsules {
		if cv.Repo == repo && cv.Name == name {
			c = cv
		}
	}
	facts := capsuleFacts(c)
	head := "Capsule " + key
	if facts != "" {
		head += " (" + facts + ")"
	}
	v := &view{head: head}
	if c.Exists {
		v.body = append(v.body, "At "+c.Path+".")
		st := workspace.QueryWorktreeStatus(c.Path)
		base := "origin/" + firstNonEmpty(g.ctx.WS().DefaultBranch, "main")
		state := "clean"
		if st.Dirty {
			state = "uncommitted changes"
		}
		commits := git.Lines(c.Path, "log", "--format=%h %s", base+"..HEAD")
		v.body = append(v.body, fmt.Sprintf("%d %s ahead of %s, %s.", len(commits), plural(len(commits), "commit"), base, state))
		limit := 10
		if g.full {
			limit = len(commits)
		}
		for i, l := range commits {
			if i == limit {
				v.body = append(v.body, fmt.Sprintf("  … %d more: --full shows them", len(commits)-limit))
				break
			}
			v.body = append(v.body, "  "+l)
		}
	} else {
		v.body = append(v.body, "It's been burned: its work is on the default branch, or gone.")
	}
	if c.PR != nil {
		v.body = append(v.body, fmt.Sprintf("PR #%d: %s (%s).", c.PR.Number, c.PR.Title, c.PR.URL))
	}
	for _, cr := range g.m.Crew {
		if cr.Capsule == key {
			g.crewLink(v, cr.ID, "worked in it")
		}
	}
	for _, st := range g.stages {
		if slices.Contains(st.NamedCapsules(), key) {
			v.add(fmt.Sprintf("s%d", st.N), "a "+st.Noun()+" that works in it: "+st.Short())
		}
	}
	for _, av := range g.arts {
		if av.Parsed.Loc == ops.LocCapsule && av.Parsed.Repo == repo && av.Parsed.Capsule == name {
			v.add(av.Ref, av.Kind+labelled(av.Label))
		}
	}
	for _, r := range g.reviews {
		if r.State == review.Submitted && r.Capsule == key {
			v.add(r.ID, reviewLine(r))
		}
	}
	v.lately = g.about(func(e mission.Event) bool { return e.Repo == repo && e.Capsule == name })
	return v, nil
}

func plural(n int, word string) string {
	if n == 1 {
		return word
	}
	return word + "s"
}

func (g *graph) artifactView(av ops.ArtifactView) *view {
	what := "Artifact"
	switch {
	case av.Parsed.Loc == ops.LocPR:
		what = "PR"
	case av.Parsed.Loc == ops.LocLink:
		what = "Link"
	case strings.HasSuffix(strings.ToLower(av.Parsed.Path), ".md"):
		what = "Document"
	case av.Parsed.IsFile():
		what = "File"
	}
	where := av.State
	switch {
	case av.Parsed.Loc == ops.LocCapsule && av.State == "draft":
		where = "draft, in capsule " + av.Parsed.Repo + "/" + av.Parsed.Capsule + ", no PR yet"
	case av.Parsed.Loc == ops.LocCapsule && av.State == "in review":
		where = "in review: in capsule " + av.Parsed.Repo + "/" + av.Parsed.Capsule + ", its PR open"
	case av.State == "landed":
		where = "landed, on " + av.Parsed.Repo + "'s default branch"
	case av.State == "private":
		where = "the mission's own file"
	}
	head := fmt.Sprintf("%s %s", what, av.Ref)
	if av.Label != "" {
		head += fmt.Sprintf(" — %q", av.Label)
	}
	v := &view{head: head + fmt.Sprintf(" (%s, %s)", av.Kind, where)}
	by := "Recorded by the user"
	if av.Crew != "" {
		by = "Recorded by " + av.Crew
	}
	v.body = append(v.body, by+", "+when(av.Added)+".")
	if av.Path != "" && av.State != "missing" {
		if data, err := os.ReadFile(av.Path); err == nil {
			lines := strings.Count(strings.TrimSuffix(string(data), "\n"), "\n") + 1
			v.body = append(v.body, fmt.Sprintf("Read it at: %s (%d %s).", av.Path, lines, plural(lines, "line")))
		}
	}
	if av.URL != "" {
		v.body = append(v.body, "Open it at: "+av.URL)
	}
	if av.Parsed.IsFile() && av.Path != "" && av.State != "missing" && av.Parsed.Loc != ops.LocMission {
		dir := filepath.Dir(av.Path)
		versions := git.Lines(dir, "log", "-n", "6", "--format=%h %ad %s", "--date=short", "--", filepath.Base(av.Path))
		if len(versions) > 0 {
			v.body = append(v.body, "Its history, newest first:")
			for _, l := range versions {
				v.body = append(v.body, "  "+l)
			}
		}
	}
	if av.Crew != "" {
		g.crewLink(v, av.Crew, "recorded it")
	}
	if st, ok := mission.ArtifactStage(g.stages, av.Artifact); ok {
		v.add(fmt.Sprintf("s%d", st.N), "the "+st.Noun()+" it was made for: "+st.Short())
	}
	if av.Parsed.Loc == ops.LocCapsule && g.m.HasCapsule(av.Parsed.Repo, av.Parsed.Capsule) {
		v.add(av.Parsed.Repo+"/"+av.Parsed.Capsule, "the capsule it's in")
	}
	for _, c := range g.threads {
		if c.Anchor.Doc == av.Ref {
			v.add(c.ID, threadLine(c))
		}
	}
	if ap, _ := conversation.For(g.ctx.WS().Root, g.m.Slug).Approved(av.Ref); ap != nil {
		as := "as it is now"
		if data, err := os.ReadFile(av.Path); err != nil || doc.Version(data) != ap.Version {
			as = "an earlier version (" + ap.Version + "): it's changed since"
		}
		v.body = append(v.body, fmt.Sprintf("Approved by the user %s, %s.", when(ap.At), as))
	}
	// Handoffs that mention it: what crew said about it.
	for _, c := range g.m.Crew {
		if c.HandedOff == nil || mission.HandoffRef(c.ID) == av.Ref {
			continue
		}
		if data, err := os.ReadFile(g.store.HandoffPath(g.m.Slug, c.ID)); err == nil && mentions(string(data), av) {
			v.add(mission.HandoffRef(c.ID), c.ID+"'s handoff mentions it")
		}
	}
	v.lately = g.about(func(e mission.Event) bool { return mentions(e.Text, av) })
	return v
}

// mentions is whether text names an artifact, by its ref or (for a file)
// its path.
func mentions(text string, av ops.ArtifactView) bool {
	return strings.Contains(text, av.Ref) || (av.Parsed.Path != "" && strings.Contains(text, av.Parsed.Path))
}

// reviewLine is a submitted review in a line: its verdict, and how its
// comments stand.
func reviewLine(r *review.Review) string {
	line := fmt.Sprintf("review of %s: %s", r.Capsule, map[string]string{review.Approve: "approved", review.Changes: "changes requested", review.Comment: "comments"}[r.Verdict])
	if t := r.Tally(); t.Total() > 0 {
		line += fmt.Sprintf(", %d of %d comments unresolved", t.Total()-t.Resolved, t.Total())
	}
	return line
}

func (g *graph) findReview(id string) (*review.Review, error) {
	for _, r := range g.reviews {
		if r.ID == id {
			if r.State != review.Submitted {
				return nil, fmt.Errorf("%s is the user's draft: it isn't anyone else's to read until they submit it", id)
			}
			return r, nil
		}
	}
	return nil, fmt.Errorf("mission %s has no review %s", g.m.Slug, id)
}

func (g *graph) reviewView(id string) (*view, error) {
	r, err := g.findReview(id)
	if err != nil {
		return nil, err
	}
	v := &view{head: fmt.Sprintf("Review %s %s (submitted %s)", r.ID, strings.TrimPrefix(reviewLine(r), "review "), when(*r.Submitted))}
	if s := strings.TrimSpace(r.Summary); s != "" {
		v.body = append(v.body, "The user's summary: "+s)
	}
	v.body = append(v.body, "Of capsule "+r.Capsule+" at "+shortSHA(r.Head)+".")
	v.add(r.Capsule, "what it reviewed")
	answered := map[string]bool{}
	for _, n := range r.Comments {
		where := fmt.Sprintf("%s:%d", n.Path, n.Line)
		v.add(n.ID, fmt.Sprintf("%s, %s, %s: %s", n.Kind, n.Status, where, clipLine(n.Body, 90)))
		for _, t := range n.Thread {
			if t.By != "you" && !answered[t.By] {
				answered[t.By] = true
			}
		}
	}
	for _, c := range g.m.Crew {
		if answered[c.ID] {
			g.crewLink(v, c.ID, "answered its comments")
		}
	}
	v.lately = g.about(func(e mission.Event) bool {
		return (e.Kind == mission.EventReview || e.Kind == mission.EventReviewReply) && (strings.HasPrefix(e.Text, r.ID+" ") || strings.HasPrefix(e.Text, r.ID+":") || strings.HasPrefix(e.Text, r.ID+"."))
	})
	v.more = append(v.more, "ws review show "+r.ID+" prints it as crew read it: each comment with what it's on, and what to do.")
	return v, nil
}

func clipLine(s string, n int) string {
	s = oneLine(s)
	if len(s) > n {
		return s[:n-1] + "…"
	}
	return s
}

func (g *graph) commentView(id string) (*view, error) {
	rid, _, _ := strings.Cut(id, ".")
	r, err := g.findReview(rid)
	if err != nil {
		return nil, err
	}
	text, ok := review.RenderComment(r, id)
	if !ok {
		return nil, fmt.Errorf("review %s has no comment %s", rid, id)
	}
	v := &view{head: fmt.Sprintf("Comment %s, in review %s %s", id, rid, strings.TrimPrefix(reviewLine(r), "review "))}
	v.body = append(v.body, strings.TrimSpace(text))
	v.add(rid, "the review it's in")
	v.add(r.Capsule, "the capsule it's on")
	v.lately = g.about(func(e mission.Event) bool {
		return e.Kind == mission.EventReviewReply && strings.HasPrefix(e.Text, id+" ")
	})
	v.more = append(v.more, "Answer it with ws review addressed|wontfix|reply "+id+" \"…\".")
	return v, nil
}

func (g *graph) runView(id string) (*view, error) {
	run, err := orchestrator.LoadRun(g.store, g.m.Slug, id)
	if errors.Is(err, os.ErrNotExist) || run == nil {
		return nil, fmt.Errorf("mission %s has no orchestrator run %s (only the latest are kept)", g.m.Slug, id)
	}
	if err != nil {
		return nil, err
	}
	outcome := "ran ws mission commands"
	switch {
	case run.Error != "":
		outcome = "failed: " + run.Error
	case !run.Acted:
		outcome = "ran no ws mission command"
	}
	model := run.Harness
	if run.Model != "" {
		model += ", " + run.Model
	}
	head := fmt.Sprintf("Orchestrator run %s, for %s (%s; %s): %s", run.ID, firstNonEmpty(run.For, run.Trigger), model, when(run.Started), outcome)
	if run.Retry {
		head += " (a retry, after a run that did nothing)"
	}
	v := &view{head: head}
	if out := strings.TrimSpace(run.Output); out != "" {
		v.body = append(v.body, "It said:", g.clip(out))
	}
	if len(run.Calls) > 0 {
		v.body = append(v.body, "", "What it ran:")
		for i, c := range run.Calls {
			if i == 20 && !g.full {
				v.body = append(v.body, fmt.Sprintf("  … %d more: --full shows them", len(run.Calls)-20))
				break
			}
			mark := ""
			switch {
			case c.Refused:
				mark = " (refused)"
			case c.Error:
				mark = " (failed)"
			}
			v.body = append(v.body, "  "+clipLine(firstNonEmpty(c.Input, c.Tool), 160)+mark)
		}
	}
	if run.Crew != "" {
		g.crewLink(v, run.Crew, "what it looked at")
	}
	if g.full {
		v.body = append(v.body, "", "Its prompt:", run.Prompt)
	} else {
		v.more = append(v.more, "--full adds the prompt it was given.")
	}
	return v, nil
}

// threadLine is a thread in a line: its kind, where it stands, what it's
// on, and its latest word.
func threadLine(c *conversation.Conversation) string {
	state := c.Status
	switch {
	case c.Status == conversation.Open && c.Answering:
		state = "open, an agent answering"
	case c.Status == conversation.Open && c.WithCrew():
		state = "open, with " + c.Sent.Crew
	case c.Status == conversation.Open && c.Pending() != nil:
		state = "open, an edit suggested"
	case c.WaitingOnYou():
		state = "open, waiting on the user"
	}
	on := fmt.Sprintf("lines %d–%d", c.Anchor.Line, c.Anchor.EndLine)
	if c.Anchor.Quote != nil && strings.TrimSpace(c.Anchor.Quote.Exact) != "" {
		on = "“" + clipLine(c.Anchor.Quote.Exact, 50) + "”"
	}
	last := c.Last()
	return fmt.Sprintf("%s, %s, on %s: %s: %s", c.Kind, state, on, last.By, clipLine(last.Body, 80))
}

// threadView is ws show t3: what it's on, the conversation, and where it
// stands.
// threadSaid is a thread on a document as a view: where it stands, the
// text it's on, what's been said in it and the edits suggested, without
// what links to it (a mission's has crew and decisions; one on a document
// outside a mission has only its document).
func threadSaid(c *conversation.Conversation, clip func(string) string, full bool) *view {
	state := c.Status
	switch {
	case c.Status == conversation.Resolved:
		state = "resolved by the user"
	case c.Answering:
		state = "open, an agent answering it now"
	case c.WithCrew():
		state = "open, sent to " + c.Sent.Crew + " to change the document for"
	case c.WaitingOnYou():
		state = "open, waiting on the user"
	case c.Kind == conversation.Note:
		state = "open, a note"
	default:
		state = "open, waiting for an answer"
	}
	v := &view{head: fmt.Sprintf("Thread %s (%s, %s) on %s", c.ID, c.Kind, state, c.Anchor.Doc)}
	lines := fmt.Sprintf("line %d", c.Anchor.Line)
	if c.Anchor.EndLine > c.Anchor.Line {
		lines = fmt.Sprintf("lines %d–%d", c.Anchor.Line, c.Anchor.EndLine)
	}
	if q := c.Anchor.Quote; q != nil && strings.TrimSpace(q.Exact) != "" {
		v.body = append(v.body, "On the text (as it reads rendered):", "", "> "+strings.ReplaceAll(clip(q.Exact), "\n", "\n> "), "")
	} else {
		v.body = append(v.body, "On the whole of "+lines+".", "")
	}
	if len(c.Anchor.Excerpt) > 0 {
		v.body = append(v.body, fmt.Sprintf("In the markdown, %s as they were (version %s):", lines, c.Anchor.Version))
		width := len(fmt.Sprint(c.Anchor.EndLine))
		for i, l := range c.Anchor.Excerpt {
			v.body = append(v.body, fmt.Sprintf("  %*d  %s", width, c.Anchor.Line+i, l))
		}
		v.body = append(v.body, "")
	}
	v.body = append(v.body, "The thread:")
	for _, t := range c.Turns {
		who := turnBy(t)
		if t.Commit != "" {
			who += " (" + shortSHA(t.Commit) + ")"
		}
		v.body = append(v.body, fmt.Sprintf("- %s, %s: %s", who, when(t.At), clip(t.Body)))
	}
	for _, sg := range c.Suggestions {
		state := sg.State
		switch sg.State {
		case conversation.SuggestOpen:
			state = "waiting for the user to accept or decline"
		case conversation.SuggestAccepted:
			state = "accepted"
			if sg.Commit != "" {
				state += " and committed (" + shortSHA(sg.Commit) + ")"
			}
		case conversation.SuggestSuperseded:
			state = "replaced by a later one"
		}
		v.body = append(v.body, "", fmt.Sprintf("Suggested edit by %s, %s: %s", sg.By, when(sg.At), state))
		if sg.State == conversation.SuggestOpen || full {
			v.body = append(v.body, "  replace:", indent(clip(sg.From), "    "), "  with:", indent(firstNonEmpty(clip(sg.To), "(nothing: delete it)"), "    "))
		}
	}
	if c.Status == conversation.Resolved && c.StatusAt != nil {
		v.body = append(v.body, fmt.Sprintf("Resolved %s.", when(*c.StatusAt)))
	}
	return v
}

func (g *graph) threadView(id string) (*view, error) {
	c, err := conversation.For(g.ctx.WS().Root, g.m.Slug).Load(id)
	if err != nil {
		return nil, err
	}
	if c.Kind == conversation.Talk {
		return talkView(c, g.full, "about mission "+g.m.Slug), nil
	}
	v := threadSaid(c, g.clip, g.full)
	v.add(c.Anchor.Doc, "the document it's on")
	seen := map[string]bool{}
	for _, t := range c.Turns {
		if !seen[t.By] {
			seen[t.By] = true
			if cr, ok := g.crew(t.By); ok {
				v.add(cr.ID, "spoke in it ("+crewStanding(cr)+")")
			}
		}
	}
	v.lately = g.about(func(e mission.Event) bool {
		return e.Kind == mission.EventThread && (strings.HasPrefix(e.Text, c.ID+" ") || e.Text == c.ID)
	})
	if d := c.Draft; d != nil && d.State == conversation.DraftOpen {
		draft := "What was decided, as " + d.By + " drafted it for the user to record: " + d.What
		if d.Why != "" {
			draft += " Why: " + d.Why
		}
		v.body = append(v.body, "", draft)
	}
	if c.Sent != nil {
		sent := fmt.Sprintf("The user sent it to %s, %s", c.Sent.Crew, when(c.Sent.At))
		if c.Sent.Finished {
			sent += "; " + c.Sent.Crew + " has finished since without saying it changed the document"
		}
		v.body = append(v.body, sent+".")
		if cr, ok := g.crew(c.Sent.Crew); ok {
			v.add(cr.ID, "has it ("+crewStanding(cr)+")")
		}
	}
	if c.Status != conversation.Resolved {
		v.more = append(v.more, "Answer it with ws thread reply "+c.ID+" \"…\" (after committing any change to the document, if it's in a capsule).")
		if c.WithCrew() {
			v.more = append(v.more, "Once the document's changed for it, say what changed with ws thread changed "+c.ID+" \"…\": that hands it back to the user.")
		}
	}
	return v, nil
}

func indent(text, by string) string {
	return by + strings.ReplaceAll(text, "\n", "\n"+by)
}

// talkTurnsShown is how many of a conversation's latest turns ws show
// gives without --full: the summary has the rest.
const talkTurnsShown = 6

// workspaceTalkView is ws show w3: a workspace conversation's summary and
// latest turns.
func workspaceTalkView(ctx *ops.Workspace, id string, full bool) (*view, error) {
	c, err := conversation.ForWorkspace(ctx.WS().Root).Load(id)
	if err != nil {
		return nil, err
	}
	v := talkView(c, full, "the workspace's")
	if isDocThread(c) { // a thread on a document that's in no mission
		v = workspaceThreadView(ctx, c, full)
	}
	if said, err := ctx.SaidOf(store.Key{Name: id}); err == nil {
		v.addSaid(saidSection{title: "Said of it", said: said})
	}
	return v, nil
}

// talkView shows a conversation that isn't on a document: its title,
// summary and latest turns (all of them, full).
func talkView(c *conversation.Conversation, full bool, whose string) *view {
	state := c.Status
	switch {
	case c.Status == conversation.Open && c.Answering:
		state = "open, the agent answering now"
	case c.WaitingOnYou():
		state = "open, the agent replied"
	}
	head := fmt.Sprintf("Conversation %s (%s, %s)", c.ID, whose, state)
	if c.Title != "" {
		head = fmt.Sprintf("Conversation %s — %q (%s, %s)", c.ID, c.Title, whose, state)
	}
	if c.Kind == conversation.Cell {
		head = fmt.Sprintf("Cell %s of page %s (%s)", c.ID, c.Anchor.Page, state)
	}
	if c.Kind == conversation.Pass {
		head = fmt.Sprintf("Pass %s over page %s, %s (%s)", c.Goal, c.Anchor.Page, c.ID, state)
		if of := c.Anchor.Reviewed(); of != "" {
			head = fmt.Sprintf("Pass %s over the review of %s, %s (%s)", c.Goal, of, c.ID, state)
		}
	}
	v := &view{head: head}
	v.body = append(v.body, fmt.Sprintf("Started by %s, %s.", c.By, when(c.At)))
	if c.Kind == conversation.Cell {
		v.body = append(v.body, "Its first message is what the user asked for there; each reply of the agent's is the cell as it then read, and what the user says after is what they wanted changed. The page is .ws/pages/"+c.Anchor.Page+".md, the cell between its `<!-- ws:cell "+c.ID+" … -->` and `<!-- /ws:cell -->` comments.")
	}
	if a := c.Anchor; a.PR != "" {
		about := "It's about pull request " + a.PR
		if a.Path != "" {
			about += fmt.Sprintf(", %s:%d", a.Path, a.Line)
			if a.EndLine > a.Line {
				about += fmt.Sprintf("-%d", a.EndLine)
			}
		}
		v.body = append(v.body, about+" (ws show "+a.PR+").")
		v.add(a.PR, "the pull request it's about")
	}
	if a := c.Anchor; c.What == conversation.WhatDoc {
		about := "It's asked about " + a.Path
		if a.Quote != nil && a.Quote.Exact != "" {
			about += ", the passage “" + clipLine(a.Quote.Exact, 120) + "”"
			if a.Section != "" {
				about += " in the section “" + a.Section + "”"
			}
			if a.Line > 0 {
				about += fmt.Sprintf(" (line %d", a.Line)
				if a.EndLine > a.Line {
					about += fmt.Sprintf("-%d", a.EndLine)
				}
				about += ")"
			}
		}
		v.body = append(v.body, about+" (ws show "+a.Doc+").")
		v.add(a.Doc, "the document it's about")
	}
	if c.Ephemeral {
		v.body = append(v.body, "It's an ask nobody's kept: it isn't listed, and it goes an hour after its last turn (conversation.AskFor) unless the user keeps it (ws conversation keep "+c.ID+").")
	} else if c.Kept != nil {
		v.body = append(v.body, "It was an ask, kept "+when(*c.Kept)+".")
	}
	if c.Summary != "" {
		v.body = append(v.body, "", "Summary: "+c.Summary)
	}
	if m := c.Became(); m != "" {
		v.body = append(v.body, "", "It became mission "+m+" (ws show -m "+m+").")
	}
	if len(c.Proposals) > 0 {
		v.body = append(v.body, "", "Proposed in it:")
		for _, p := range c.Proposals {
			what := "decision: " + p.What
			switch p.Kind {
			case conversation.ProposeMission:
				what = "mission: " + p.Title
				if p.Brief == "" {
					what += " (an idea, to brief later)"
				}
			case conversation.ProposeComment:
				what = fmt.Sprintf("review comment on %s %s:%d (%s): %s", p.PR, p.Path, p.Line, p.NoteKind, p.Text)
			case conversation.ProposeBrief:
				what = "the brief"
				if p.Plan != "" {
					what = "the brief, and its plan"
				}
			case conversation.ProposeDispatch:
				what = "crew: " + p.Text
			case conversation.ProposeReplan:
				what = "replan: " + p.Text
			case conversation.ProposePlan:
				what = "a revision of the plan"
				if p.Why != "" {
					what += ": " + p.Why
				}
				if p.Step > 0 {
					what += fmt.Sprintf(" Accepting it passes step %d.", p.Step)
				}
			case conversation.ProposeDocument:
				what = fmt.Sprintf("document (%s): %s, to add as %s", p.DocKind, p.Title, p.Path)
				if p.To != "" {
					what = fmt.Sprintf("an edit to the document %s (%s): %s", p.To, p.DocKind, p.Title)
				}
			case conversation.ProposeArtifact:
				what = fmt.Sprintf("artifact: %s — %s", p.Title, p.Note)
				if p.To != "" {
					what = fmt.Sprintf("a version of %s, on v%d — %s", p.To, p.On, p.Note)
				}
			}
			state := p.State
			switch {
			case p.State == conversation.ProposalOpen:
				state = "waiting for the user"
			case p.State == conversation.ProposalAccepted && p.Kind == conversation.ProposeComment:
				state = "added to the user's draft review"
			case p.State == conversation.ProposalAccepted && p.Result != "" && p.Kind != conversation.ProposePlan && p.Kind != conversation.ProposeBrief:
				state = "accepted: " + p.Result
			case p.State == conversation.ProposalAccepted && p.Kind != conversation.ProposeDecision:
				state = "accepted"
			case p.State == conversation.ProposalAccepted:
				state = "accepted, kept with " + c.ID + " until it's a mission"
			}
			v.body = append(v.body, fmt.Sprintf("  %d. %s (%s)", p.N, clipLine(what, 140), state))
		}
	}
	turns := c.Turns
	if !full && len(turns) > talkTurnsShown {
		v.body = append(v.body, "", fmt.Sprintf("The latest %d of %d turns (--full for all):", talkTurnsShown, len(turns)))
		turns = turns[len(turns)-talkTurnsShown:]
	} else {
		v.body = append(v.body, "", "The conversation:")
	}
	for _, t := range turns {
		body := t.Body
		if !full {
			body = clipString(body, 1200)
		}
		v.body = append(v.body, fmt.Sprintf("- %s, %s: %s", turnBy(t), when(t.At), body))
	}
	return v
}

// turnBy is who said a turn, as a transcript has it: one ws wrote for you
// says what for (a card you settled), as you didn't type it.
func turnBy(t conversation.Turn) string {
	if n := t.OriginProposal(); n > 0 {
		return fmt.Sprintf("%s (settled proposal %d)", t.By, n)
	}
	return t.By
}

// fromAddress is what ws show is given as an address (ws:fix-token-refresh,
// ws:w4, or one of the workspace's on GitHub, as a link in a document has
// it), as the ref it shows that by. A mission's sets slug and shows the
// mission. Anything else is as it was given.
func fromAddress(ctx *ops.Workspace, s string, slug *string) string {
	if !strings.HasPrefix(s, "ws:") && !strings.Contains(s, "://") {
		return s
	}
	if rest, ok := strings.CutPrefix(s, "ws:"); ok && artifact.IsRef(rest) {
		return rest // the grammar reads a3 as a mission's slug, as yet
	}
	r, ok := wsref.ParseAddress(s, ctx.RefsKnown(context.Background(), false))
	if !ok {
		return s
	}
	switch r.Kind {
	case wsref.Mission:
		*slug = r.Slug
		return ""
	case wsref.File:
		// A document's ref: repo:path, or a capsule's repo/capsule:path.
		if r.InCapsule() {
			return r.Repo + "/" + r.Worktree + ":" + r.Path
		}
		return r.Repo + ":" + r.Path
	}
	return r.Name()
}
