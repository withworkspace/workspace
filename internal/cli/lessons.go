package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/withworkspace/workspace/internal/lessons"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/ui"
)

// ws lessons: what crew keep running into where they work (internal/lessons).
func newLessonsCmd() *cobra.Command {
	var all, asJSON bool
	cmd := &cobra.Command{
		Use:   "lessons",
		Short: "What crew keep running into where they work, and what would stop it",
		Long: `Lessons are what crew keep having to learn about where they work, read
from their handoffs' Environment section: a test that has to be run a
certain way, a step a fresh capsule needs, a tool on this machine that gets
in the way. Each is one thing, with every crew that ran into it, and what
would stop the next crew running into it at all: a change to ws, to the
repo's capsule setup (after_create), to the repo, a line in its agent
instructions, or something only you can do.

The daemon reads each crew's handoff once, a few every five minutes, with
the [lessons] agent (one turn, no tools, on a small model). Handoffs from
before Environment was asked for are read whole. One that two crew have
run into is a recommendation, and you're told of it; one only one crew has
is a note.

  ws lessons                 the open ones: recommendations, then notes
  ws lessons --all           fixed and dismissed ones too
  ws lessons show <n>        one, with what each crew said
  ws lessons mission <n>     make a mission of it, waiting to be briefed
  ws lessons fixed <n>       you fixed it: crew running into it again open it again
  ws lessons dismiss <n>     nothing to do: kept, never raised again
  ws lessons reopen <n>      open it again
  ws lessons read            read the handoffs waiting now, rather than in the daemon's time`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error { return listLessons(all, asJSON) },
	}
	cmd.Flags().BoolVar(&all, "all", false, "fixed and dismissed ones too")
	cmd.Flags().BoolVar(&asJSON, "json", false, "one JSON object a line")
	cmd.AddCommand(newLessonsShowCmd(), newLessonsMissionCmd(), newLessonsReadCmd(), newLessonsPutCmd(),
		newLessonsStateCmd("fixed", lessons.Fixed, "You fixed it: crew running into it again open it again"),
		newLessonsStateCmd("dismiss", lessons.Dismissed, "Nothing to do: kept so it's known, never raised again"),
		newLessonsStateCmd("reopen", lessons.Open, "Open it again"))
	return cmd
}

// fixSays is what a lesson's fix is, in words.
var fixSays = map[string]string{
	lessons.FixWS:    "change ws",
	lessons.FixSetup: "capsule setup",
	lessons.FixRepo:  "change the repo",
	lessons.FixDocs:  "agent instructions",
	lessons.FixYou:   "yours to do",
}

func lessonArg(s string) (int64, error) {
	id, ok := ops.LessonOf(s)
	if !ok {
		return 0, fmt.Errorf("%q isn't a lesson: say its number (ws lessons lists them)", s)
	}
	return id, nil
}

func listLessons(all, asJSON bool) error {
	w, err := loadOps()
	if err != nil {
		return err
	}
	info, err := w.Lessons()
	if err != nil {
		return err
	}
	var shown []lessons.Lesson
	for _, l := range info.Lessons {
		if all || l.State == lessons.Open {
			shown = append(shown, l)
		}
	}
	if asJSON {
		for _, l := range shown {
			line, _ := json.Marshal(l)
			fmt.Fprintln(ui.Stdout, string(line))
		}
		return nil
	}
	if len(shown) == 0 {
		fmt.Fprintln(ui.Stderr, ui.Dim.Render("  none yet: crew's handoffs are read for them as they're handed off"))
	}
	notes := false
	for _, l := range shown {
		if !l.IsRecommended() && !notes {
			notes = true
			fmt.Fprintln(ui.Stdout, ui.Dim.Render("Seen once"))
		}
		printLessonLine(l)
	}
	fmt.Fprintf(ui.Stderr, "  %s\n", ui.Dim.Render(readingSays(info)))
	return nil
}

func printLessonLine(l lessons.Lesson) {
	state := ""
	switch {
	case l.State != lessons.Open:
		state = "  " + ui.Dim.Render(l.State)
	case l.Again != nil:
		state = "  " + ui.Orange.Render("seen again after it was fixed")
	}
	fmt.Fprintf(ui.Stdout, "%3d  %s%s\n", l.ID, l.Title, state)
	fmt.Fprintf(ui.Stdout, "     %s\n", ui.Dim.Render(lessonFacts(l)))
	if l.IsRecommended() {
		fmt.Fprintf(ui.Stdout, "     %s\n", firstLineOf(l.Change))
	}
}

// lessonFacts is who ran into it and what's to change: "capsule setup ·
// frontend · 3 crew in 2 missions · last 2026-11-27".
func lessonFacts(l lessons.Lesson) string {
	parts := []string{fixSays[l.Fix]}
	if l.Repo != "" {
		parts = append(parts, l.Repo)
	}
	crew := fmt.Sprintf("%d crew", l.Crew)
	if l.Missions > 1 {
		crew += fmt.Sprintf(" in %d missions", l.Missions)
	}
	parts = append(parts, crew, "last "+l.LastSeen.Local().Format(time.DateOnly))
	if l.Mission != "" {
		parts = append(parts, "mission "+l.Mission)
	}
	return strings.Join(parts, " · ")
}

func readingSays(info ops.LessonsInfo) string {
	if !info.On {
		return fmt.Sprintf("%d handoffs read; reading is off ([lessons] enabled = false)", info.Read)
	}
	s := fmt.Sprintf("%d handoffs read", info.Read)
	if info.Failed > 0 {
		s += fmt.Sprintf(", %d couldn't be", info.Failed)
	}
	if info.Waiting > 0 {
		s += fmt.Sprintf("; %d to read (ws lessons read reads them now)", info.Waiting)
	}
	return s
}

func newLessonsShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <n>",
		Short: "A lesson, with what each crew said of it",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := lessonArg(args[0])
			if err != nil {
				return err
			}
			w, err := loadOps()
			if err != nil {
				return err
			}
			l, err := lessons.Get(w.Root(), id)
			if err != nil {
				return err
			}
			printLessonLine(l)
			if !l.IsRecommended() {
				fmt.Fprintf(ui.Stdout, "     %s\n", l.Change)
			} else if strings.Contains(l.Change, "\n") {
				fmt.Fprintf(ui.Stdout, "     %s\n", l.Change)
			}
			if l.Why != "" {
				fmt.Fprintf(ui.Stdout, "     %s\n", ui.Dim.Render("Why: "+l.Why))
			}
			fmt.Fprintln(ui.Stdout)
			for _, s := range l.Seen {
				fmt.Fprintf(ui.Stdout, "  %s  %s %s\n", ui.Dim.Render(s.At.Local().Format(time.DateOnly)), s.Mission, ui.Dim.Render(s.Crew))
				fmt.Fprintf(ui.Stdout, "    %s\n", s.Said)
			}
			return nil
		},
	}
}

func newLessonsStateCmd(use, state, short string) *cobra.Command {
	return &cobra.Command{
		Use:   use + " <n>",
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := lessonArg(args[0])
			if err != nil {
				return err
			}
			w, err := loadOps()
			if err != nil {
				return err
			}
			if err := w.SetLessonState(id, state, ops.AsYou); err != nil {
				return err
			}
			fmt.Fprintf(ui.Stderr, "  %s lesson %d is %s\n", ui.Green.Render("✓"), id, state)
			return nil
		},
	}
}

func newLessonsMissionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "mission <n>",
		Short: "Make a mission of a lesson, waiting to be briefed (one it has already, it says)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := lessonArg(args[0])
			if err != nil {
				return err
			}
			w, err := loadOps()
			if err != nil {
				return err
			}
			slug, err := w.LessonMission(id, ops.AsYou)
			if err != nil {
				return err
			}
			fmt.Fprintf(ui.Stderr, "  %s mission %s, waiting to be briefed (ws web)\n", ui.Green.Render("✓"), slug)
			return nil
		},
	}
}

func newLessonsReadCmd() *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "read",
		Short: "Read the handoffs waiting now, rather than in the daemon's time",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			w, err := loadOps()
			if err != nil {
				return err
			}
			if !w.Config().LessonsEnabled() {
				return errors.New("reading handoffs is off ([lessons] enabled = false)")
			}
			got, err := w.ReadLessons(limit)
			if err != nil {
				return err
			}
			for _, c := range got.Changed {
				mark := "·"
				switch {
				case c.New:
					mark = "new"
				case c.Recommended:
					mark = ui.Green.Render("recommended")
				case c.Again:
					mark = ui.Orange.Render("again")
				}
				fmt.Fprintf(ui.Stdout, "%3d  %s  %s\n", c.Lesson.ID, mark, c.Lesson.Title)
			}
			for _, f := range got.Failed {
				fmt.Fprintf(ui.Stderr, "  %s couldn't read %s\n", ui.Orange.Render("!"), f)
			}
			left := ""
			if got.Left > 0 {
				left = fmt.Sprintf(", %d to go", got.Left)
			}
			fmt.Fprintf(ui.Stderr, "  %s read %d handoffs%s\n", ui.Green.Render("✓"), got.Read, left)
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 0, "read at most this many (0 for all)")
	return cmd
}

// ws lessons put: lessons as given, for a seed, which has no agent to read
// its handoffs.
func newLessonsPutCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "put",
		Short:  "Keep lessons given as JSON on stdin, as they are (for seeds)",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			w, err := loadOps()
			if err != nil {
				return err
			}
			data, err := io.ReadAll(os.Stdin)
			if err != nil {
				return err
			}
			var list []lessons.Lesson
			if err := json.Unmarshal(data, &list); err != nil {
				return fmt.Errorf("lessons, as a JSON list: %w", err)
			}
			return lessons.Put(w.Root(), list)
		},
	}
}
