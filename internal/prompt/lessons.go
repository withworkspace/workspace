package prompt

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// LessonsAsk is a crew's handoff, for the agent reading it for lessons
// (internal/lessons): what it says of where crew work, and what's known.
type LessonsAsk struct {
	Mission, Title, Crew, Repo string
	// Said is the handoff's Environment section; for a handoff kept from
	// before that was asked for, the whole handoff (Whole).
	Said  string
	Whole bool
	// AfterCreate is the repo's capsule setup, as ws.toml has it ("" for
	// none): what a setup lesson changes.
	AfterCreate string
	Known       []KnownLesson
}

// KnownLesson is a lesson known so far, as the agent's told it.
type KnownLesson struct {
	ID                  int64
	Repo, Title, Change string
	Crew                int
}

// FoundLesson is one lesson the agent found: a known one (Same), or a new
// one, with what the crew said of it.
type FoundLesson struct {
	Same   int64  `json:"same"`
	Said   string `json:"said"`
	Title  string `json:"title"`
	Repo   string `json:"repo"`
	Fix    string `json:"fix"`
	Change string `json:"change"`
	Why    string `json:"why"`
}

// LessonsPrompt is what the agent reading a handoff for lessons is asked.
func LessonsPrompt(a LessonsAsk) string {
	var b strings.Builder
	b.WriteString(`You keep the lessons that coding agents in this workspace keep having to learn about where they work: how a repo is set up, built and tested, how ws (the tool that runs them) and their surroundings behave, and how this machine is set up. ws runs agents ("crew") on missions, each in a fresh git worktree ("capsule") of a repo, and runs checks on their work. When crew finish they leave a handoff for whoever comes next.

`)
	if a.Whole {
		fmt.Fprintf(&b, "Below is the handoff crew %s left on the mission %q, working in the repo %s. Most of a handoff is about the work: take only what's about where the work is done.\n\n", a.Crew, a.Title, orUnknown(a.Repo))
	} else {
		fmt.Fprintf(&b, "Below is what crew %s said under \"Environment\" in their handoff on the mission %q, working in the repo %s: what they found anyone working there would need, whatever the work.\n\n", a.Crew, a.Title, orUnknown(a.Repo))
	}
	b.WriteString(`For each thing it says that anyone working in that repo or on this machine would need, whatever the work, give one entry:
- same: the id of the known lesson below that it is (the same cause, or what one change would fix), or 0 for a new one. Only the same thing: two different tests that are slow are two lessons.
- said: what these crew found, in one plain sentence. Name the exact command, file, version or error.

For a new one (same is 0), also:
- title: a short plain name for the problem, as the user would say it ("Web dependencies missing in a fresh capsule").
- repo: the repo it's about, or "" when it's about this machine or ws itself.
- fix: who or what should change so the next crew doesn't run into it at all:
  - "ws": ws itself (how it makes capsules, sets crew's environment, runs checks or tries, what it tells crew).
  - "setup": the repo's capsule setup, the command ws runs when it makes a capsule (installing dependencies, generating code).
  - "repo": the repo's own code, scripts or tests (a recipe, a test that fails for crew, a slow suite).
  - "docs": a line in the repo's agent instructions (CLAUDE.md or AGENTS.md), for something that's right as it is but has to be known.
  - "you": something only the user can do on their machine or accounts.
  Prefer what takes the problem away (ws, setup, repo) over telling agents of it (docs): say docs only when nothing should change.
- change: what to change, for the user to act on, in one or two sentences. Name the file, command, test, setting or version. For setup, give the whole after_create line to put under [repos.<name>] in ws.toml.
- why: one sentence on why crew run into it.

For a known one, give change only when what these crew found shows a better change than the one known; else "".

Leave out what's only about this work (what was built or decided, where its code went, what's left), how the repo's code works, and anything with nothing to change. An empty list is fine.

`)
	if a.Repo != "" {
		if a.AfterCreate != "" {
			fmt.Fprintf(&b, "The repo %s's capsule setup now: after_create = %q.\n\n", a.Repo, a.AfterCreate)
		} else {
			fmt.Fprintf(&b, "The repo %s has no capsule setup (no after_create).\n\n", a.Repo)
		}
	}
	if len(a.Known) == 0 {
		b.WriteString("Known lessons: none yet.\n\n")
	} else {
		b.WriteString("Known lessons (id, repo, how many crew ran into it, title: change):\n")
		for _, k := range a.Known {
			fmt.Fprintf(&b, "- %d [%s, %d crew] %s: %s\n", k.ID, orAny(k.Repo), k.Crew, k.Title, oneLineOf(k.Change))
		}
		b.WriteString("\n")
	}
	b.WriteString("----\n")
	b.WriteString(strings.TrimSpace(a.Said))
	b.WriteString("\n----\n\n")
	b.WriteString("Answer with only this JSON, in a ```json block:\n\n```json\n{\"lessons\": [{\"same\": 0, \"said\": \"\", \"title\": \"\", \"repo\": \"\", \"fix\": \"\", \"change\": \"\", \"why\": \"\"}]}\n```\n")
	return b.String()
}

func orUnknown(repo string) string {
	if repo == "" {
		return "(not known)"
	}
	return repo
}

func orAny(repo string) string {
	if repo == "" {
		return "this machine or ws"
	}
	return repo
}

func oneLineOf(s string) string { return strings.Join(strings.Fields(s), " ") }

// ReadLessons reads the agent's answer: the lessons it found, in order.
// It takes the last JSON object in what was said, in a block or not.
func ReadLessons(said string) ([]FoundLesson, error) {
	text := said
	if i := strings.LastIndex(text, "```json"); i >= 0 {
		text = text[i+len("```json"):]
		if j := strings.Index(text, "```"); j >= 0 {
			text = text[:j]
		}
	}
	start, end := strings.Index(text, "{"), strings.LastIndex(text, "}")
	if start < 0 || end < start {
		return nil, errors.New("its answer had no JSON in it")
	}
	var got struct {
		Lessons []FoundLesson `json:"lessons"`
	}
	if err := json.Unmarshal([]byte(text[start:end+1]), &got); err != nil {
		return nil, fmt.Errorf("its answer wasn't the JSON asked for: %w", err)
	}
	out := got.Lessons[:0]
	for _, l := range got.Lessons {
		l.Said, l.Title, l.Change, l.Why = strings.TrimSpace(l.Said), strings.TrimSpace(l.Title), strings.TrimSpace(l.Change), strings.TrimSpace(l.Why)
		l.Fix, l.Repo = strings.ToLower(strings.TrimSpace(l.Fix)), strings.TrimSpace(l.Repo)
		if l.Said == "" || l.Same == 0 && l.Title == "" {
			continue
		}
		out = append(out, l)
	}
	return out, nil
}
