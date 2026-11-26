package ops

import (
	"strings"

	"github.com/withworkspace/workspace/internal/automation"
	"github.com/withworkspace/workspace/internal/events"
	"github.com/withworkspace/workspace/internal/notification"
)

// What your automations do is ws's own operations (internal/automation's
// hooks): a notification in your feed, from the automations source; a
// conversation of the workspace's, started with what it's to look at; the
// refs a text names, as ws finds them. ws show's views are the commands',
// and the daemon gives them (show is left to it).

// AutomationHooks are the operations an automation's builtins are, but
// show.
func (w *Workspace) AutomationHooks() automation.Hooks {
	return automation.Hooks{
		Notify: func(a *automation.Automation, text string, refs []string) error {
			_, err := notification.Add(w.Root(), notification.Notification{
				Source: notification.SourceAutomations, Kind: "automation:" + a.Name, Text: text, Refs: refs, By: events.ByWorkflow(a.Name),
			})
			return err
		},
		Failed: func(a *automation.Automation, why string) error {
			_, err := notification.Add(w.Root(), notification.Notification{
				Source: notification.SourceAutomations, Kind: "automation:" + a.Name, Attention: true,
				Text: "Automation " + a.Name + " couldn't run: " + why,
			})
			return err
		},
		Converse: func(a *automation.Automation, text string, about []string) (string, error) {
			if len(about) > 0 {
				text += "\n\n(About " + strings.Join(about, ", ") + ".)"
			}
			c, err := w.StartTalkVia("automation:"+a.Name, text)
			if err != nil {
				return "", err
			}
			return c.ID, nil
		},
		RefsIn: func(text string) [][2]string {
			known := w.namesKnown()
			if known == nil {
				return nil
			}
			var out [][2]string
			seen := map[string]bool{}
			for _, f := range known.Find(text) {
				name := f.Ref.Name()
				if seen[name] {
					continue
				}
				seen[name] = true
				kind := string(f.Ref.Kind)
				if f.Ref.Plugin != "" {
					kind = f.Ref.Plugin + ":" + f.Ref.What
				}
				out = append(out, [2]string{name, kind})
			}
			return out
		},
	}
}
