package prompt

import (
	"strings"

	"github.com/withworkspace/workspace/internal/mission"
)

// Here is what a conversation's agent is told of where it is, gathered by
// whoever asks for its prompt, so a prompt reads nothing itself.
type Here struct {
	Root   string // the workspace's folder
	Agents string // what every agent's told of its guide, skills and kinds: "" for none
	// Kind is what the document or page is, as one of the workspace's
	// kinds, as a paragraph of a prompt: "" for none.
	Kind  string
	Repos []string // each repo's ground, a line each
	Git   []string // the git commands it may run
	// Kinds are the workspace's kinds of document, by name (doc, plan): a
	// talk may propose one of them. None: it isn't told it can.
	Kinds []string
	// Proposes are what the workspace's plugins propose: a talk may
	// propose them, with ws propose.
	Proposes []PluginProposes
	// Mission is the mission it's in; nil for the workspace's.
	Mission *mission.Mission
	Page    string // the page's file, for a cell, a question, an outline or a pass
	Doc     string // the document's file, for an ask about one
	// Plugin is what the plugin a thing's asked about from is called
	// (Sketch), for an ask about one.
	Plugin string
	Style   string // the file saying how pages should read, if there's one
	// Planning is the process a mission's planner works from, for a
	// briefing conversation; Brief the brief it's written already, if
	// it is ("" for none): one it was proposed with, from the
	// conversation it came out of.
	Planning Planning
	Brief    string
	// Sandbox is its shell's sandbox, where its harness has one and the
	// conversation is one that runs things (Sandboxed): nil for an agent
	// that only reads.
	Sandbox *Sandbox
}

// PluginProposes is something a plugin proposes, as an agent's told it:
// its name (jira:issues), what it is, and when to propose it.
type PluginProposes struct {
	Name, Title, When string
}

// para is a note as a paragraph of a prompt, or nothing.
func para(note string) string {
	if note == "" {
		return ""
	}
	return note + "\n\n"
}

func firstNonEmpty(s, fallback string) string {
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	return s
}

func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }

// clipLine is s on one line, to n bytes.
func clipLine(s string, n int) string {
	s = oneLine(s)
	if len(s) > n {
		return s[:n-1] + "…"
	}
	return s
}

// clipString is s, trimmed, to n bytes.
func clipString(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}

func short7(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}
