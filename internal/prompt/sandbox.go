package prompt

import (
	"fmt"
	"strings"

	"github.com/withworkspace/workspace/internal/conversation"
)

// What a conversation's agent is told where its shell is sandboxed: that
// it can run things, where it writes, which commands leave the sandbox,
// and the capsules it may lift, which are its conversation's.

// Sandbox is a conversation's sandboxed shell, as its agent's told of it.
type Sandbox struct {
	// Own is the folder it writes what it likes in.
	Own string
	// Capsules are those it lifted, there now; Away, those put away with
	// the conversation, their branches kept.
	Capsules, Away []SandboxCapsule
	// NoLift is why it can't lift capsules in this conversation, or "".
	NoLift string
}

// SandboxCapsule is a capsule a conversation's agent lifted.
type SandboxCapsule struct {
	Repo, Name, Branch string
	Path               string // where it is; "" for one put away
}

// Sandboxed is whether a conversation is one whose agent runs things,
// where its harness sandboxes its shell: one that talks (about the
// workspace, a mission, a pull request, a document, a mission's brief and
// plan). Not one that fills a page or passes over one, nor a thread on a
// document, which read.
func Sandboxed(c *conversation.Conversation) bool {
	switch c.What {
	case conversation.WhatTalk, conversation.WhatPR, conversation.WhatDoc, conversation.WhatBriefing:
		return true
	}
	return false
}

// sandboxReading says what an agent with a sandboxed shell may run, read
// and write, and how.
func sandboxReading(h Here) string {
	s := h.Sandbox
	var b strings.Builder
	b.WriteString("You can read anything here, and run what you like: your shell is in a sandbox. So where running something would answer the question (the tests and how long they take, a build, a script that measures), run it, and say what you found rather than what you'd expect.\n\n")
	b.WriteString("- The shell runs any command, piped and chained as you like, reads everything, and reaches the network.\n")
	fmt.Fprintf(&b, "- It writes in a folder of your own, %s (scripts, notes, what a command prints), in the system's temp folder, and in the capsules you lift (below). Nowhere else in the workspace: writing into a repo's .ground, a mission's capsule or the workspace's own files fails with \"operation not permitted\", which is the sandbox and not a fault. Your tools for writing and editing files write in the same places.\n", s.Own)
	b.WriteString("- The repos' default branches are at repos/<repo>/.ground; missions' capsules (worktrees on their branches) are at repos/<repo>/<capsule>. Run things in them as they stand where that writes nothing into them; where it would, lift a capsule of your own.\n")
	b.WriteString("- `ws` and `gh` run outside the sandbox, so each runs on its own, as one command (not piped, chained or redirected to a file), and only these. `ws status -f llm` lists the repos and capsules; `ws mission list` the missions. `ws show -m <slug> -f llm` shows a mission, and `ws show -m <slug> -f llm <ref>` anything in it (stages s1, crew c1, capsules repo/name, documents by their ref, threads t3, decisions d1), each with what it links to. `ws find -f llm <words>` finds what's been said by what it says, in any mission and in the workspace's conversations, reviews and documents, each with the command that shows it. Of gh, reading: `gh pr view|list|checks|diff`, `gh issue view|list`, `gh run list|view`, `gh workflow list|view`, `gh release list|view`, `gh repo view`, `gh search`.\n\n")
	b.WriteString("A refusal, whatever its words, is of that one command: the shell still works, and so do your tools for reading and searching files. Where ws refuses one, it says what to do in its place.\n\n")
	b.WriteString(sandboxLifting(h))
	return b.String()
}

// sandboxLifting says how an agent lifts a capsule of its conversation's,
// what becomes of it, and how what it tries there goes on.
func sandboxLifting(h Here) string {
	s := h.Sandbox
	var b strings.Builder
	if s.NoLift != "" {
		fmt.Fprintf(&b, "You can't lift a capsule here: %s.\n\n", s.NoLift)
		return b.String()
	}
	b.WriteString("To change a repo's files, or run what writes into its checkout, lift a capsule of your own: `ws lift <repo> <name>` makes a worktree at repos/<repo>/<name>, on a new branch of that name from the repo's default branch (`ws lift <repo> <name> <branch>` from another: a mission's branch, to try its change without touching its capsule). Name it for what you're trying. It's this conversation's, not a mission's")
	if h.Mission != nil {
		b.WriteString(" (not this mission's either: its capsules are its plan's, and its crew's to work in)")
	}
	b.WriteString(": yours to change, build and commit in. It's put away when the user puts this conversation away, its branch kept with what's not committed committed to it, and `ws lift <repo> <name>` brings it back as it was.\n\n")
	b.WriteString("What you try there stays there: don't push it or open a pull request. If it's worth keeping, commit it, and propose a mission (`ws mission new --propose`, below) whose brief says what you did and names the branch under `## Context`, for its crew to start from (every capsule of a repo has its branches: `git merge <branch>` in theirs). If it was to find something out, say what you found.\n\n")
	b.WriteString(sandboxCapsules(s))
	return b.String()
}

// sandboxCapsules says which capsules the agent has lifted, and which
// were put away: nothing where there are none.
func sandboxCapsules(s *Sandbox) string {
	var b strings.Builder
	if len(s.Capsules) > 0 {
		b.WriteString("Capsules you've lifted in this conversation:\n")
		for _, c := range s.Capsules {
			fmt.Fprintf(&b, "- %s/%s, at %s (branch %s)\n", c.Repo, c.Name, c.Path, c.Branch)
		}
		b.WriteString("\n")
	}
	if len(s.Away) > 0 {
		b.WriteString("Capsules you lifted, put away when the conversation was (their branches kept):\n")
		for _, c := range s.Away {
			fmt.Fprintf(&b, "- %s/%s: `ws lift %s %s` brings it back\n", c.Repo, c.Name, c.Repo, c.Name)
		}
		b.WriteString("\n")
	}
	return b.String()
}

// shellNext is what a later turn says of the shell, in the agent's
// session, up to where what's said next starts: that it's as its first
// message said. A sandboxed one says how it is in full, shortly, as a
// session begun before conversations ran things was told it may only
// read; and which capsules it has now.
func shellNext(h Here) string {
	s := h.Sandbox
	if s == nil {
		return "(The shell works for the commands your first message listed: if one was refused, that was it alone.) "
	}
	var b strings.Builder
	fmt.Fprintf(&b, "(Your shell is in a sandbox: it runs any command, piped and chained as you like, reads everything and reaches the network, and writes in %s, the system's temp folder and the capsules you've lifted, nowhere else in the workspace. `ws` and `gh` run outside it, each on its own as one command, and only those your first message listed", s.Own)
	if s.NoLift == "" {
		b.WriteString(", with `ws lift <repo> <name> [<branch>]` for a capsule of this conversation's own to change files in (put away with it, its branch kept; commit what's worth keeping and propose a mission that names the branch, rather than pushing it)")
	}
	b.WriteString(". If your first message said you may only read, one command at a time, this is how it is now. A refusal is of that one command alone.)\n\n")
	b.WriteString(sandboxCapsules(s))
	return b.String()
}
