package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/withworkspace/workspace/internal/notification"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/ui"
)

// ws notifications: the feed of what happened (internal/notification),
// and what you've read of it.

func newNotificationsCmd() *cobra.Command {
	var all bool
	var format string
	cmd := &cobra.Command{
		Use:     "notifications",
		Aliases: []string{"notification"},
		Short:   "What happened that you'd want to know: the unread, or with --all the feed",
		Long: `What happened that you'd want to know, newest first: a mission ended by
itself or was started for you, a pull request this ws opened merged, a
plugin's update is out, ws was upgraded. Each is kept until you dismiss
it, read or not. What needs you isn't here: that's worked out from where
things stand (ws status, and the web UI's Needs you).

  ws notifications               the unread
  ws notifications --all         the whole feed, the read marked
  ws notifications read n3 n4    say you've read them (or --all)
  ws notifications dismiss n3    put one away

Where each source is sent (the feed, told as it happens as well, or nowhere) is
[notifications] in ws.local.toml, and Settings › Notifications.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			feed, err := ctx.Notifications(!all, 0)
			if err != nil {
				return err
			}
			switch format {
			case "", "text":
			case "json":
				for _, n := range feed.Notifications {
					line, _ := json.Marshal(n)
					fmt.Fprintln(ui.Stdout, string(line))
				}
				return nil
			default:
				return fmt.Errorf("-f takes text or json, not %q", format)
			}
			if len(feed.Notifications) == 0 {
				said := "nothing unread"
				if all {
					said = "nothing yet"
				}
				fmt.Fprintln(ui.Stderr, ui.Dim.Render("  "+said))
				return nil
			}
			for _, n := range feed.Notifications {
				mark := ui.Orange.Render("●")
				if n.Read != nil {
					mark = ui.Dim.Render("·")
				}
				fmt.Fprintf(ui.Stdout, "%s %s %s %s\n", mark,
					ui.Dim.Render(fmt.Sprintf("%-4s", n.Ref())),
					ui.Dim.Render(n.At.Local().Format("Jan 02 15:04")), n.Text)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "the whole feed, not only the unread")
	cmd.Flags().StringVarP(&format, "format", "f", "", "text (the default) or json (one notification per line)")
	cmd.AddCommand(newNotificationsReadCmd(), newNotificationsDismissCmd(), newNotificationsAddCmd())
	return cmd
}

// notificationNumbers reads notifications' numbers as they're named: n3,
// or 3.
func notificationNumbers(args []string) ([]int64, error) {
	var out []int64
	for _, a := range args {
		n, err := strconv.ParseInt(strings.TrimPrefix(a, "n"), 10, 64)
		if err != nil || n <= 0 {
			return nil, fmt.Errorf("%q isn't a notification: they're named n1, n2, … (ws notifications)", a)
		}
		out = append(out, n)
	}
	return out, nil
}

func newNotificationsReadCmd() *cobra.Command {
	var all bool
	cmd := &cobra.Command{
		Use:   "read [n3...]",
		Short: "Say you've read notifications: those named, or --all",
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			ns, err := notificationNumbers(args)
			if err != nil {
				return err
			}
			done, err := ctx.ReadNotifications(ns, all, ops.ActorFromEnv())
			if err != nil {
				return err
			}
			fmt.Fprintf(ui.Stderr, "  %s %s\n", ui.Green.Render("✓"), done)
			return nil
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "every one in the feed")
	return cmd
}

// ws notifications add is for seeds: what a try can't make happen (a
// pull request merged, a release out) is put in its feed by hand.
func newNotificationsAddCmd() *cobra.Command {
	var source, kind string
	var refs []string
	var attention bool
	cmd := &cobra.Command{
		Use:    "add <what happened>",
		Short:  "Add a notification by hand (for a seed)",
		Hidden: true,
		Args:   cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			n, err := ctx.AddNotification(notification.Notification{
				Source: source, Kind: kind, Text: strings.Join(args, " "), Refs: refs, Attention: attention,
			}, ops.ActorFromEnv())
			if err != nil {
				return err
			}
			fmt.Fprintf(ui.Stderr, "  %s %s added\n", ui.Green.Render("✓"), n.Ref())
			return nil
		},
	}
	cmd.Flags().StringVar(&source, "source", notification.SourceMissions, "where it's from: "+strings.Join(notification.Sources, ", "))
	cmd.Flags().StringVar(&kind, "kind", "", "what happened (pr.merged, plugin.update…)")
	cmd.Flags().StringArrayVar(&refs, "ref", nil, "what it's about, the first what it opens (repeatable)")
	cmd.Flags().BoolVar(&attention, "attention", false, "mark it as something that went wrong")
	return cmd
}

func newNotificationsDismissCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "dismiss <n3...>",
		Short: "Put notifications away: read, and out of the feed",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			ctx, err := loadOps()
			if err != nil {
				return err
			}
			ns, err := notificationNumbers(args)
			if err != nil {
				return err
			}
			done, err := ctx.DismissNotifications(ns, ops.ActorFromEnv())
			if err != nil {
				return err
			}
			fmt.Fprintf(ui.Stderr, "  %s %s\n", ui.Green.Render("✓"), done)
			return nil
		},
	}
}
