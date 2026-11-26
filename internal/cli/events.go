package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/withworkspace/workspace/internal/events"
	"github.com/withworkspace/workspace/internal/team"
	"github.com/withworkspace/workspace/internal/ui"
)

// ws events: what this ws did, and heard of, as it's kept (internal/events).

func newEventsCmd() *cobra.Command {
	var since, format, slug string
	var kinds []string
	var follow, heard, did bool
	var limit int
	cmd := &cobra.Command{
		Use:   "events",
		Short: "What this ws did, and heard of: capsules landed, pull requests opened, commits landed…",
		Long: `What this ws did, for you or for crew and the orchestrator on your missions,
oldest first: a capsule landed or burned, a pull request opened, merged or
failing its checks, a mission started or ended, an artifact added or
promoted, a document added or amended, a decision made. Each says who did
it, and what caused it where something did.

And what it heard of, marked heard: what teammates did, as the daemon
found it moving what it keeps. Commits landing on a repo's default branch
(with the documents and things they changed), a pull request asking for
your review, approved or merged, a run starting or ending. Each says who
did it, as a person where ws knows who (person:brendan). Only what this
ws did makes a notification. Events are kept for 90 days.

  ws events                          the latest 50
  ws events --since 1d               since this time yesterday (today, monday, 7d, 2h, 2026-11-03…)
  ws events --kind capsule.landed    one kind (repeatable)
  ws events --heard                  only what it heard of (--did: only what it did)
  ws events --follow                 and each new one as it's kept
  ws events -f json                  one event per line, as JSON

The kinds: ` + strings.Join(events.Kinds, ", ") + ".",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			root := ctx.Root()
			f := events.Filter{Kinds: kinds, Mission: slug, Limit: limit}
			switch {
			case heard && did:
				return fmt.Errorf("--heard and --did each leave out the other: give neither, for both")
			case heard:
				f.Origin, f.Only = events.Heard, true
			case did:
				f.Origin, f.Only = events.Did, true
			}
			for _, k := range kinds {
				if !events.Known(k) {
					return fmt.Errorf("there's no event %q: they're %s", k, strings.Join(events.Kinds, ", "))
				}
			}
			if since != "" {
				if f.Since, err = sinceOf(since, time.Now()); err != nil {
					return err
				}
				f.Limit = 0
			}
			switch format {
			case "", "text", "json":
			default:
				return fmt.Errorf("-f takes text or json, not %q", format)
			}
			list, err := events.List(root, f)
			if err != nil {
				return err
			}
			w := ui.Stdout
			for _, e := range list {
				printEvent(w, e, format)
			}
			if !follow {
				if len(list) == 0 && format != "json" {
					fmt.Fprintln(ui.Stderr, ui.Dim.Render("  nothing yet"))
				}
				return nil
			}
			last := int64(0)
			if len(list) > 0 {
				last = list[len(list)-1].N
			} else if latest, _ := events.List(root, events.Filter{Limit: 1}); len(latest) > 0 {
				last = latest[0].N
			}
			stop := make(chan os.Signal, 1)
			signal.Notify(stop, os.Interrupt)
			tick := time.NewTicker(time.Second)
			defer tick.Stop()
			for {
				select {
				case <-stop:
					return nil
				case <-tick.C:
				}
				more, err := events.List(root, events.Filter{After: last, Kinds: kinds, Mission: slug, Origin: f.Origin, Only: f.Only})
				if err != nil {
					return err
				}
				for _, e := range more {
					printEvent(w, e, format)
					last = e.N
				}
			}
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "those since then: today, yesterday, monday, week, 7d, 2w, 2h, 2026-11-03")
	cmd.Flags().StringArrayVar(&kinds, "kind", nil, "those of this kind (repeatable): capsule.landed, pr.opened…")
	cmd.Flags().StringVarP(&slug, "mission", "m", "", "those of this mission")
	cmd.Flags().IntVarP(&limit, "limit", "n", 50, "the latest this many, without --since (0 for every one)")
	cmd.Flags().BoolVar(&follow, "follow", false, "and each new one as it's kept, until interrupted")
	cmd.Flags().BoolVar(&heard, "heard", false, "only what it heard of: what teammates did")
	cmd.Flags().BoolVar(&did, "did", false, "only what this ws did")
	cmd.Flags().StringVarP(&format, "format", "f", "", "text (the default) or json (one event per line)")
	return cmd
}

// sinceOf is when --since says: a length of time back (2h, 90m), or what
// ws team --since takes.
func sinceOf(s string, now time.Time) (time.Time, error) {
	if d, err := time.ParseDuration(s); err == nil && d > 0 {
		return now.Add(-d), nil
	}
	return team.Since(s, now)
}

// printEvent writes e as one line: when, its ref, its kind, who did it,
// what it's about, and what it says.
func printEvent(w io.Writer, e events.Event, format string) {
	if format == "json" {
		line, _ := json.Marshal(e)
		fmt.Fprintln(w, string(line))
		return
	}
	about := strings.Join(e.Refs(), " ")
	text := e.Text
	if text != "" {
		text = " · " + text
	}
	cause := ""
	if e.Cause != "" {
		cause = ui.Dim.Render(" (" + e.Cause + ")")
	}
	if text == "" && e.Title != "" {
		text = " · " + e.Title
	}
	if n := len(e.Commits); n > 1 {
		text += ui.Dim.Render(fmt.Sprintf(" (%d commits)", n))
	}
	kind := fmt.Sprintf("%-20s", e.Kind)
	if e.IsHeard() {
		kind = ui.Dim.Render("heard ") + fmt.Sprintf("%-14s", e.Kind)
	}
	fmt.Fprintf(w, "%s %s %s %s%s %s%s\n",
		ui.Dim.Render(e.At.Local().Format("Jan 02 15:04")),
		ui.Dim.Render(fmt.Sprintf("%-4s", e.Ref())),
		kind, e.By, cause, about, text)
}
