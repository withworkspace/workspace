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
	// Confined is a shell in its harness's own sandbox, which stops a
	// write outside where it may; one that isn't (a trusted harness's)
	// is only told where to keep to. Asks is a sandbox with no network,
	// that a command leaves, whole, when the agent asks (Codex's).
	Confined, Asks bool
}

// SandboxCapsule is a capsule a conversation's agent lifted.
type SandboxCapsule struct {
	Repo, Name, Branch string
	Path               string // where it is; "" for one put away
	// Joined is one that was there already, which it took on.
	Joined bool
}

// Sandboxed is whether a conversation is one whose agent runs things,
// where its harness sandboxes its shell: one that talks (about the
// workspace, a mission, a pull request, a document, a mission's brief and
// plan). Not one that fills a page or passes over one, nor a thread on a
// document, which read.
func Sandboxed(c *conversation.Conversation) bool {
	switch c.What {
	case conversation.WhatTalk, conversation.WhatPR, conversation.WhatDoc, conversation.WhatThing, conversation.WhatBriefing:
		return true
	}
	return false
}

// sandboxReading says what an agent with a shell that runs things may
// run, read and write, and how: in its harness's sandbox (Confined), or
// trusted to keep to where it's told.
func sandboxReading(h Here) string {
	s := h.Sandbox
	var b strings.Builder
	switch {
	case s.Confined && s.Asks:
		b.WriteString("You can read anything here, and run what you like: your shell is in a sandbox, and what needs more you ask for. So where running something would answer the question (the tests and how long they take, a build, a script that measures), run it, and say what you found rather than what you'd expect.\n\n")
		fmt.Fprintf(&b, "- The shell runs any command, piped and chained as you like, and reads everything. In its sandbox it writes in a folder of your own, %s (scripts, notes, what a command prints), in the system's temp folder, and in the capsules you'd lifted when this turn began, and it has no network.\n", s.Own)
		b.WriteString("- A command that needs more (the network: installing packages, fetching, `git push`; or writing in a capsule you've lifted since) runs outside the sandbox: request escalated permissions for it as you run it, not after it's failed. ws grants them, but for a `ws` or `gh` command you may not run. Outside, keep to the same places: never write in a repo's .ground, a mission's capsule or the workspace's own files. Your tools for editing files write where the shell does, and in the capsules you lift as you go.\n")
	case s.Confined:
		b.WriteString("You can read anything here, and run what you like: your shell is in a sandbox. So where running something would answer the question (the tests and how long they take, a build, a script that measures), run it, and say what you found rather than what you'd expect.\n\n")
		b.WriteString("- The shell runs any command, piped and chained as you like, reads everything, and reaches the network.\n")
		fmt.Fprintf(&b, "- It writes in a folder of your own, %s (scripts, notes, what a command prints), in the system's temp folder, and in the capsules you lift (below). Nowhere else in the workspace: writing into a repo's .ground, a mission's capsule or the workspace's own files fails with \"operation not permitted\", which is the sandbox and not a fault. Your tools for writing and editing files write in the same places.\n", s.Own)
	default:
		b.WriteString("You can read anything here, and run what you like. So where running something would answer the question (the tests and how long they take, a build, a script that measures), run it, and say what you found rather than what you'd expect.\n\n")
		b.WriteString("- The shell runs any command, piped and chained as you like, reads everything, and reaches the network.\n")
		fmt.Fprintf(&b, "- Write in a folder of your own, %s (scripts, notes, what a command prints), in the system's temp folder, and in the capsules you lift (below), and nowhere else in the workspace: not in a repo's .ground, a mission's capsule or the workspace's own files. Your tools for writing and editing files are refused anywhere else; your shell isn't stopped, so keep to these places yourself.\n", s.Own)
	}
	b.WriteString("- The repos' default branches are at repos/<repo>/.ground; missions' capsules (worktrees on their branches) are at repos/<repo>/<capsule>. Run things in them as they stand where that writes nothing into them; where it would, lift a capsule of your own.\n")
	switch {
	case s.Confined && s.Asks:
		b.WriteString("- `ws` and `gh` run outside the sandbox, always with escalated permissions, each on its own, as one command (not piped, chained or redirected to a file), and only these.")
	case s.Confined:
		b.WriteString("- `ws` and `gh` run outside the sandbox, so each runs on its own, as one command (not piped, chained or redirected to a file), and only these.")
	default:
		b.WriteString("- `ws` and `gh` each run on their own, as one command (not piped, chained or redirected to a file), and only these.")
	}
	b.WriteString(" `ws status -f llm` lists the repos and capsules; `ws mission list` the missions. `ws show -m <slug> -f llm` shows a mission, and `ws show -m <slug> -f llm <ref>` anything in it (stages s1, crew c1, capsules repo/name, documents by their ref, threads t3, decisions d1), each with what it links to. `ws find -f llm <words>` finds what's been said by what it says, in any mission and in the workspace's conversations, reviews and documents, each with the command that shows it. Of gh, reading: `gh pr view|list|checks|diff`, `gh issue view|list`, `gh run list|view`, `gh workflow list|view`, `gh release list|view`, `gh repo view`, `gh search`.\n\n")
	b.WriteString("A refusal, whatever its words, is of that one command: the shell still works, and so do your tools for reading and searching files. Where ws refuses one, it says what to do in its place.\n\n")
	b.WriteString(sandboxLifting(h))
	return b.String()
}

// sandboxLifting says how an agent lifts a capsule of its conversation's,
// joins one that's there or docks one on a pull request's branch, what
// becomes of it, and how what it does there goes on: pushed when the user
// asks (a small job asked for in a conversation needs no mission), else
// carried by a mission.
func sandboxLifting(h Here) string {
	s := h.Sandbox
	var b strings.Builder
	if s.NoLift != "" {
		fmt.Fprintf(&b, "You can't lift a capsule here: %s.\n\n", s.NoLift)
		return b.String()
	}
	b.WriteString("To change a repo's files, or run what writes into its checkout, work in a capsule of this conversation's:\n\n")
	b.WriteString("- `ws lift <repo> <name>` makes a worktree at repos/<repo>/<name>, on a new branch of that name from the repo's default branch (`ws lift <repo> <name> <branch>` from another: a mission's branch, to try its change without touching its capsule). Name it for what you're trying.\n")
	b.WriteString("- `ws dock <repo> <PR number or branch>` makes one on a branch that's on the remote already, a pull request's: what you commit and push there updates the pull request.\n")
	b.WriteString("- Naming a capsule that's there already (`ws lift <repo> <its name>`), one the user lifted by hand, joins it: you work in it as it is, on its branch. Not a mission's, which is its crew's: ws says so, and you lift one of your own from its branch.\n\n")
	b.WriteString("Either way, it's this conversation's, not a mission's")
	if h.Mission != nil {
		b.WriteString(" (not this mission's either: its capsules are its plan's, and its crew's to work in)")
	}
	b.WriteString(": yours to change, build and commit in. One you lifted or docked is put away when the user puts this conversation away, its branch kept with what's not committed committed to it, and `ws lift <repo> <name>` brings it back as it was; one you joined is left where it is.\n\n")
	b.WriteString("Push when the user asks you to, and only then: `git push` from the capsule, to its own branch, once what they asked for is done and what checks it (the tests, the build) passes, or you've said why it doesn't. Don't force a push unless they've said to (`--force-with-lease`, after a rebase they asked for); bring the default branch in by merging it, so nothing's forced. Say in your reply what you pushed, and where. Opening a pull request isn't a conversation's: if what you did is worth keeping and nobody asked you to push it, commit it, and propose a mission (`ws mission new --propose`, below) whose brief says what you did and names the branch under `## Context`, for its crew to start from (every capsule of a repo has its branches: `git merge <branch>` in theirs). If it was to find something out, say what you found.\n\n")
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
			joined := ""
			if c.Joined {
				joined = ", joined: it was there already"
			}
			fmt.Fprintf(&b, "- %s/%s, at %s (branch %s%s)\n", c.Repo, c.Name, c.Path, c.Branch, joined)
		}
		b.WriteString("\n")
	}
	if len(s.Away) > 0 {
		b.WriteString("Capsules you lifted, put away when the conversation was (their branches kept):\n")
		for _, c := range s.Away {
			if c.Joined {
				fmt.Fprintf(&b, "- %s/%s, joined and left where it was: `ws lift %s %s` joins it again, if it's there\n", c.Repo, c.Name, c.Repo, c.Name)
				continue
			}
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
	switch {
	case s.Confined && s.Asks:
		fmt.Fprintf(&b, "(Your shell is in a sandbox: it runs any command, piped and chained as you like, and reads everything; it writes in %s, the system's temp folder and the capsules you'd lifted when this turn began, and has no network. What needs more (the network, a capsule lifted since) you run with escalated permissions, which ws grants but for a `ws` or `gh` command you may not run; `ws` and `gh` always need them, each on its own as one command, and only those your first message listed", s.Own)
	case s.Confined:
		fmt.Fprintf(&b, "(Your shell is in a sandbox: it runs any command, piped and chained as you like, reads everything and reaches the network, and writes in %s, the system's temp folder and the capsules you've lifted, nowhere else in the workspace. `ws` and `gh` run outside it, each on its own as one command, and only those your first message listed", s.Own)
	default:
		fmt.Fprintf(&b, "(Your shell runs any command, piped and chained as you like, reads everything and reaches the network. Write in %s, the system's temp folder and the capsules you've lifted, nowhere else in the workspace. `ws` and `gh` each run on their own as one command, and only those your first message listed", s.Own)
	}
	if s.NoLift == "" {
		b.WriteString(", with `ws lift <repo> <name> [<branch>]` for a capsule of this conversation's own to change files in (naming one that's there already joins it), and `ws dock <repo> <PR number or branch>` for one on a pull request's branch. Push its branch when the user asks you to, never forced unless they say so; otherwise commit what's worth keeping and propose a mission that names the branch")
	}
	b.WriteString(". If your first message said you may only read, one command at a time, or that you mayn't push, this is how it is now. A refusal is of that one command alone.)\n\n")
	b.WriteString(sandboxCapsules(s))
	return b.String()
}
