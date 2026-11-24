package orchestrator

import (
	"context"
	"github.com/withworkspace/workspace/internal/turn"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/crew"
	"github.com/withworkspace/workspace/internal/execlog"
	"github.com/withworkspace/workspace/internal/git"
	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/mission"
)

const (
	// LogEvents is how many recent log events the orchestrator is shown.
	LogEvents       = 40
	transcriptLines = 200
	outputKeep      = 1500 // bytes of the orchestrator's final message kept in the log
)

// Timeout bounds one run; tests shorten it.
var Timeout = 10 * time.Minute

// Run runs the orchestrator on a mission with its role, returning what it
// printed. It runs in its own process group so a timeout kills everything it
// started (a shell, a `gh pr checks --watch`), not just the agent: anything
// left holding its output would keep Run waiting forever.
func Run(life context.Context, l harness.Launch, root, slug, run string) (Report, error) {
	// The report is what the agent prints; what it says on stderr (Claude
	// Code's sign-in warnings, say) goes to the process's stderr too
	// (orchestrator.log, run by hand the terminal), and into the error if
	// the run fails.
	rep, err := turn.Run(turn.Spec{
		Life:    life,
		Launch:  l,
		Env:     append(crew.Env(root, slug, ""), mission.EnvRole+"="+mission.RoleOrchestrator, mission.EnvRun+"="+run),
		Timeout: Timeout,
		Stderr:  os.Stderr,
		Root:    root,
		Who:     "orchestrator " + slug,
	})
	return Report{Output: rep.Output, Calls: rep.Calls}, err
}

// PromptFile is where a run's prompt is written for the agent to read.
func PromptFile(store mission.Store, slug, run string) string {
	return filepath.Join(runsDir(store, slug), run+".prompt.md")
}

// WritePrompt writes the prompt where the agent will read it, and returns
// the one line that goes on the agent's command line instead. A long,
// multi-line prompt passed as an argument gets cursor-agent killed (as it
// did crew before their prompts were typed in), and could pass the
// argument size limit; a file has neither problem.
func WritePrompt(store mission.Store, slug, run, prompt string) (string, error) {
	path := PromptFile(store, slug, run)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(prompt), 0o644); err != nil {
		return "", err
	}
	execlog.Note(store.Root, "orchestrator "+slug, "wrote its prompt (%d bytes) to %s", len(prompt), path)
	return "You are the orchestrator for the ws mission " + slug + ". Read " + path + " first: it has your instructions and the mission's context. Then follow it.", nil
}

// Summary is what's logged of a run: the end of its final message, or why
// it failed.
func Summary(out string, err error) string {
	s := clipTail(strings.TrimSpace(out), outputKeep, "…")
	if err != nil {
		if s != "" {
			return "orchestrator failed: " + err.Error() + " · " + s
		}
		return "orchestrator failed: " + err.Error()
	}
	return s
}

// RetryNote is added to the prompt for a second run when the first ran no
// ws mission command: what it said, the commands that were refused, and
// that its shell works.
func RetryNote(last Report) string {
	said := clipTail(strings.TrimSpace(last.Output), outputKeep, "…")
	if said == "" {
		said = "(nothing)"
	}
	var b strings.Builder
	b.WriteString("\n## Your last run\n\nYour last run on this ended without running a single ws mission command, so nothing happened, not even your note. It said:\n\n" + said + "\n")
	var refused []string
	for _, c := range last.Calls {
		if c.Refused {
			refused = append(refused, "`"+c.Input+"`")
		}
	}
	if len(refused) > 0 {
		b.WriteString("\nOnly these were refused, each on its own (they aren't on your allowed list): " + strings.Join(refused, ", ") + ".\n")
	}
	b.WriteString("\nYour shell works. Every ws mission command above is allowed. Run them now, one per call: your note, and whatever leaves the mission in someone's court.\n")
	return b.String()
}

// Gist is the start of what the orchestrator said, for a line or two: its
// first paragraph with something in it, without markdown's emphasis.
func Gist(out string) string {
	for _, para := range strings.Split(strings.TrimSpace(out), "\n\n") {
		p := strings.Join(strings.Fields(strings.NewReplacer("**", "", "__", "", "`", "").Replace(para)), " ")
		if p == "" || p == "…" || p == "..." {
			continue
		}
		p = strings.TrimPrefix(p, "… ")
		if r := []rune(p); len(r) > 300 {
			p = string(r[:299]) + "…"
		}
		return p
	}
	return ""
}

// CrewOutput is the last lines crew showed: live, from live (the server
// running it; nil for none), if it's still running, else from its saved
// transcript.
func CrewOutput(store mission.Store, slug, crewID string, live func(key string) string) string {
	text := ""
	if live != nil {
		text = live(slug + "/" + crewID)
	}
	if text == "" {
		if data, err := os.ReadFile(store.TranscriptPath(slug, crewID)); err == nil {
			text = string(data)
		}
	}
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	if len(lines) > transcriptLines {
		lines = lines[len(lines)-transcriptLines:]
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

// DiffStat summarises a capsule's changes against its base.
func DiffStat(dir, base string) string {
	out, err := git.Output(dir, "diff", "--stat", base+"...HEAD")
	if err != nil {
		return ""
	}
	return strings.TrimRight(out, "\n")
}
