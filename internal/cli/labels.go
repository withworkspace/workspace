package cli

import (
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/withworkspace/workspace/internal/web"
)

// labels are what each of ws's commands is doing, as a conversation shows
// an agent's step that ran it, in place of the command as it was typed:
// ws show sketch-5 -f llm is "Looking up sketch-5". {1} is the command's
// first argument, {2} its second; a step missing one shows the command.
// Words for someone watching the agent work, so they say what it's doing,
// not what the command is. Every command has one (TestEveryCommandHasALabel);
// "" is ws on its own, which is ws status. A plugin's commands say their
// own (plugin.Command.Label).
var labels = map[string]string{
	"": "Seeing workspace state",

	"artifact":          "Looking at artifacts",
	"artifact add":      "Adding an artifact",
	"artifact checkout": "Checking out {1}",
	"artifact clear":    "Clearing expired artifacts",
	"artifact discard":  "Discarding {1}",
	"artifact list":     "Listing artifacts",
	"artifact promote":  "Keeping {1}",
	"artifact restore":  "Restoring {1}",
	"artifact show":     "Opening {1}",
	"artifact tweaks":   "Reading how tweaks work",
	"artifact url":      "Getting {1}'s address",

	"board":   "Adding {2} to the editor",
	"unboard": "Taking {2} out of the editor",
	"burn":    "Removing a capsule",
	"dock":    "Checking out {2}",
	"lift":    "Lifting {2}",
	"land":    "Landing a capsule",
	"jump":    "Finding a capsule",
	"debrief": "Clearing landed capsules",
	"open":    "Opening the editor",

	"conversation":          "Looking at conversations",
	"conversation accept":   "Accepting a proposal",
	"conversation decline":  "Declining a proposal",
	"conversation keep":     "Keeping {1}",
	"conversation list":     "Listing conversations",
	"conversation reopen":   "Reopening {1}",
	"conversation resolve":  "Putting {1} away",
	"conversation withdraw": "Taking back {1}",
	"talk":                  "Starting a conversation",

	"events": "Reading what ws has done",

	"notifications":         "Reading notifications",
	"notifications dismiss": "Dismissing notifications",
	"notifications read":    "Marking notifications read",

	"daemon":         "Looking at the daemon",
	"daemon restart": "Restarting the daemon",
	"daemon status":  "Checking the daemon",
	"daemon stop":    "Stopping the daemon",

	"doc":                      "Looking at documents",
	"doc add":                  "Adding a {1}",
	"doc amend":                "Changing {1}",
	"documents":                "Looking through documents",
	"documents search":         "Searching documents for {1}",
	"find":                     "Searching for {1}",
	"rg":                       "Searching the code",
	"show":                     "Looking up {1}",
	"export":                   "Writing {1} out",
	"process":                  "Reading how work goes here",
	"prompt":                   "Reading the prompt",
	"root":                     "Finding the workspace",
	"team":                     "Seeing what the team's doing",
	"web":                      "Opening the web UI",
	"pin":                      "Pinning to focus",
	"unpin":                    "Taking {1} out of focus",
	"page":                     "Writing a page",
	"page list":                "Listing pages",
	"propose":                  "Proposing {1}",
	"pr":                       "Looking at pull requests",
	"pr comment":               "Proposing a review comment",
	"status":                   "Seeing workspace state",
	"doctor":                   "Checking workspace health",
	"sync":                     "Syncing the workspace",
	"setup":                    "Cloning repos",
	"init":                     "Setting up a workspace",
	"upgrade":                  "Upgrading ws",
	"shell-init":               "Setting up the shell",
	"group":                    "Looking at repo groups",
	"group hide":               "Hiding {1}",
	"group reset":              "Resetting {1}",
	"group show":               "Showing {1}",
	"repo":                     "Looking at the repos",
	"repo share":               "Giving {1} to the team",
	"repo add":                 "Adding {1}",
	"repo group":               "Grouping {1}",
	"router":                   "Looking at the router",
	"silo":                     "Looking at silos",
	"silo point":               "Pointing {1} at {2}",
	"silo status":              "Checking silos",
	"silo stop":                "Removing {1}'s silo",
	"silo watch":               "Watching silos",
	"run":                      "Starting a run",
	"run keep":                 "Keeping {1} running",
	"run logs":                 "Reading {1}'s output",
	"run ls":                   "Listing runs",
	"run point":                "Pointing {1}",
	"run show":                 "Looking at what {1} came to",
	"run stop":                 "Stopping runs",
	"plugin":                   "Looking at plugins",
	"plugin add":               "Adding a plugin",
	"plugin check":             "Checking a plugin",
	"plugin config":            "Reading {1}'s config",
	"plugin list":              "Listing plugins",
	"plugin logs":              "Reading {1}'s log",
	"plugin remove":            "Removing {1}",
	"plugin restart":           "Restarting {1}",
	"plugin update":            "Updating plugins",
	"review":                   "Looking at reviews",
	"review addressed":         "Marking {1} addressed",
	"review list":              "Listing reviews",
	"review reply":             "Replying to {1}",
	"review show":              "Reading the review",
	"review wontfix":           "Declining {1}",
	"thread":                   "Looking at threads",
	"thread accept":            "Accepting {1}",
	"thread changed":           "Saying {1} is done",
	"thread decline":           "Declining {1}",
	"thread list":              "Listing threads",
	"thread reopen":            "Reopening {1}",
	"thread reply":             "Replying in {1}",
	"thread resolve":           "Resolving {1}",
	"thread send":              "Sending threads to crew",
	"thread suggest":           "Suggesting an edit",
	"mission":                  "Looking at missions",
	"mission about":            "Saying what the mission is about",
	"mission accept":           "Accepting the step",
	"mission answer":           "Answering the mission",
	"mission archive":          "Archiving the mission",
	"mission artifact":         "Recording what the mission made",
	"mission artifact add":     "Recording {1}",
	"mission artifact rm":      "Forgetting {1}",
	"mission ask":              "Asking you",
	"mission attach":           "Adding {2} to the mission",
	"mission brief":            "Reading the brief",
	"mission bundle":           "Reading verification bundles",
	"mission bundle approve":   "Approving {1}",
	"mission bundle send-back": "Sending {1} back",
	"mission conclude":         "Suggesting how it ends",
	"mission decide":           "Recording a decision",
	"mission detach":           "Taking {2} off the mission",
	"mission dispatch":         "Starting crew",
	"mission done":             "Handing off",
	"mission drop":             "Dropping {1}",
	"mission end":              "Ending the mission",
	"mission flag":             "Flagging for you",
	"mission hand-on":          "Handing {1} on",
	"mission limit":            "Setting the step's time",
	"mission list":             "Listing missions",
	"mission look":             "Looking at what it waits for",
	"mission new":              "Starting a mission",
	"mission note":             "Noting it in the log",
	"mission open-pr":          "Opening a pull request",
	"mission orchestrate":      "Deciding what's next",
	"mission park":             "Parking the mission",
	"mission plan":             "Reading the plan",
	"mission pr-ready":         "Marking the pull request ready",
	"mission ready":            "Finishing the briefing",
	"mission reopen":           "Reopening the mission",
	"mission repeat":           "Starting the step again",
	"mission replan":           "Asking for a new plan",
	"mission reply":            "Drafting a reply on {1}",
	"mission resume-with":      "Carrying {1} on",
	"mission show":             "Reading the mission",
	"mission skip":             "Skipping the step",
	"mission start":            "Starting the step",
	"mission title":            "Naming the mission",
	"mission wait":             "Saying what it waits on",
}

// commandLabels are ws's own commands' labels, each with its words after
// ws and the flags it takes that have a value: how the web UI reads a
// command an agent ran. A command's aliases are listed too.
func commandLabels(root *cobra.Command) []web.CommandLabel {
	var out []web.CommandLabel
	var walk func(c *cobra.Command, path [][]string)
	walk = func(c *cobra.Command, path [][]string) {
		key := strings.Join(firsts(path), " ")
		if label, ok := labels[key]; ok {
			options := valueFlags(c)
			for _, words := range spellings(path) {
				out = append(out, web.CommandLabel{Words: words, Label: label, Options: options})
			}
		}
		for _, k := range c.Commands() {
			walk(k, append(path[:len(path):len(path)], append([]string{k.Name()}, k.Aliases...)))
		}
	}
	walk(root, nil)
	return out
}

// firsts is each word of a path as it's named, without its aliases.
func firsts(path [][]string) []string {
	out := make([]string, len(path))
	for i, names := range path {
		out[i] = names[0]
	}
	return out
}

// spellings are every way a path's words are written, aliases included:
// mission list, mission ls, missions list, missions ls.
func spellings(path [][]string) [][]string {
	out := [][]string{{}}
	for _, names := range path {
		var next [][]string
		for _, was := range out {
			for _, n := range names {
				next = append(next, append(was[:len(was):len(was)], n))
			}
		}
		out = next
	}
	return out
}

// valueFlags are the flags a command takes that have a value, as they're
// typed: --format and -f.
func valueFlags(c *cobra.Command) []string {
	var out []string
	add := func(f *pflag.Flag) {
		if f.Value.Type() == "bool" || f.Value.Type() == "count" || f.NoOptDefVal != "" {
			return
		}
		out = append(out, "--"+f.Name)
		if f.Shorthand != "" {
			out = append(out, "-"+f.Shorthand)
		}
	}
	c.LocalFlags().VisitAll(add)
	c.InheritedFlags().VisitAll(add)
	sort.Strings(out)
	return out
}
