package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"github.com/withworkspace/workspace/internal/crew"
	"github.com/withworkspace/workspace/internal/execlog"
	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/mission"
)

// hookPayloadMax bounds what a hook may hand over on stdin.
const hookPayloadMax = 1 << 20

func newMissionHookCmd() *cobra.Command {
	var root string
	cmd := &cobra.Command{
		Use:   "hook <event>",
		Short: "For agents' hooks: record what crew is doing (ws installs these when it launches crew)",
		Long: `Run by an agent's own hooks, which ws installs in each crew session it
launches (Claude Code through --settings, Cursor through a plugin, pi through
an extension): it records
what the crew reported, from WS_MISSION and WS_CREW and the event's JSON on
stdin. Events: start, prompt (it was sent a message: working), reply (its
turn's final message), stop (it finished its turn: waiting, for you or on
work it left running), notify.
It never fails, so a hook can't get in the agent's way.`,
		Hidden: true,
		Args:   cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, _ := io.ReadAll(io.LimitReader(cmd.InOrStdin(), hookPayloadMax))
			slug, crewID := os.Getenv(mission.EnvMission), os.Getenv(mission.EnvCrew)
			if root == "" || slug == "" || crewID == "" {
				return nil // not crew ws launched
			}
			var payload map[string]any
			_ = json.Unmarshal(data, &payload) // a payload that isn't JSON is as none: a hook never stops crew
			store := mission.Store{Root: root}
			c := crewRecord(store, slug, crewID)
			h, ok := hookFromEvent(args[0], payload, c.Harness, func() *harness.Found { return processCheck(slug, c) })
			if !ok {
				return nil
			}
			if err := store.CrewHook(slug, crewID, h); err != nil {
				execlog.Note(root, "crew "+slug+"/"+crewID, "hook %s: %v", args[0], err)
			}
			if args[0] == harness.HookStop {
				orchestrateQuietTurn(root, slug, crewID)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&root, "root", "", "the workspace root")
	return cmd
}

// orchestrateQuietTurn hands crew's turn to the orchestrator when it ended
// without the crew saying where it is (no ask, no done) after a message ws
// sent it rather than one of yours: someone has to decide what's next, and
// it's the orchestrator's call before it's yours. A step that was held
// for the turn to end is looked at first (ops.TurnEnded).
func orchestrateQuietTurn(root, slug, crewID string) {
	if ctx, err := LoadOps(root); err == nil {
		ctx.TurnEnded(slug, crewID)
	}
}

// crewRecord is crew's record, as the mission has it: with no harness if it
// can't be read.
func crewRecord(store mission.Store, slug, crewID string) mission.Crew {
	m, err := store.Load(slug)
	if err != nil {
		return mission.Crew{ID: crewID}
	}
	c, _ := m.CrewByID(crewID)
	return c
}

// readProcs reads the user's processes, for the process check: swapped in
// tests.
var readProcs = harness.ReadProcs

// processCheck is what the process check finds that crew left running, run
// from their hook command, under their harness (their record's pid): nil
// when it can't be made.
func processCheck(slug string, c mission.Crew) *harness.Found {
	procs, err := readProcs()
	if err != nil {
		return nil
	}
	return procs.CrewWork(harness.ProcCrew{Mission: slug, Crew: c.ID, Harness: c.PID, Hook: os.Getpid()})
}

// hookFromEvent reads one of ws's hook events, with its agent's payload:
// Claude Code's, Cursor's and pi's use different field names for the same
// things. (pi's are what ws's own extension sends: see harness.Pi.) What
// only the agent's harness can read in it (harness.TurnReader: what it left
// running, a prompt that wakes it) is its harness's to say, with what the
// process check finds (check, which a stop makes for a harness that reads
// processes: harness.ProcessReader).
func hookFromEvent(event string, p map[string]any, harnessName string, check func() *harness.Found) (mission.Hook, bool) {
	str := func(keys ...string) string {
		for _, k := range keys {
			if s, ok := p[k].(string); ok && s != "" {
				return s
			}
		}
		return ""
	}
	switch event {
	case harness.HookStart:
		return mission.Hook{Conversation: str("conversation_id", "session_id")}, true
	case harness.HookPrompt:
		return mission.Hook{Prompted: true, WakeUp: harness.WakeUp(harnessName, p)}, true
	case harness.HookReply:
		return mission.Hook{Reply: str("text")}, true
	case harness.HookStop:
		reply := str("last_assistant_message") // Claude Code says it
		if reply == "" {
			reply = lastAssistantText(str("transcript_path")) // Cursor's is in its transcript
		}
		return mission.Hook{Activity: mission.ActivityWaiting, Reply: reply, Work: crew.LeftRunning(harness.AtStop(harnessName, p, check))}, true
	case harness.HookQuestion:
		q, options := askedQuestion(p)
		if q == "" {
			return mission.Hook{}, false
		}
		return mission.Hook{Activity: mission.ActivityQuestion, Notice: q, Options: options}, true
	case harness.HookAnswered:
		return mission.Hook{Activity: mission.ActivityWorking}, true
	case harness.HookNotify:
		h := mission.Hook{Notice: str("message")}
		// pi's ui_prompt is an extension's dialog waiting for an answer.
		if t := str("notification_type"); strings.Contains(t, "permission") || strings.Contains(t, "elicitation") || t == "ui_prompt" {
			h.Activity = mission.ActivityPermission
		}
		return h, true
	}
	return mission.Hook{}, false
}

// askedQuestion is the question an agent put to you with its question tool,
// and its options' labels: tool_input.questions, each a question (Claude
// Code's AskUserQuestion) or a prompt under a title (Cursor's
// AskQuestion), with options that have a label. Several questions read as
// the first, with how many more.
func askedQuestion(p map[string]any) (string, []string) {
	input, _ := p["tool_input"].(map[string]any)
	if raw, ok := p["tool_input"].(string); ok { // sent as JSON text
		_ = json.Unmarshal([]byte(raw), &input) // text that isn't JSON is as no questions
	}
	qs, _ := input["questions"].([]any)
	if len(qs) == 0 {
		return "", nil
	}
	first, _ := qs[0].(map[string]any)
	q := ""
	for _, k := range []string{"question", "prompt", "promptMarkdown", "prompt_markdown"} {
		if v, _ := first[k].(string); strings.TrimSpace(v) != "" {
			q = strings.TrimSpace(v)
			break
		}
	}
	if q == "" {
		q, _ = input["title"].(string)
		q = strings.TrimSpace(q)
	}
	if q == "" {
		return "", nil
	}
	if len(qs) > 1 {
		q += fmt.Sprintf(" (and %d more)", len(qs)-1)
	}
	var options []string
	opts, _ := first["options"].([]any)
	for _, o := range opts {
		if om, ok := o.(map[string]any); ok {
			if label, _ := om["label"].(string); strings.TrimSpace(label) != "" {
				options = append(options, strings.TrimSpace(label))
			}
		}
	}
	return q, options
}

// transcriptTail is how much of a transcript's end is read for its last
// message.
const transcriptTail = 512 << 10

// lastAssistantText is the last message the agent wrote in a transcript of
// JSON lines (Cursor's: {"role":"assistant","message":{"content":[{"type":
// "text","text":…}]}}), or "" if there's none to read.
func lastAssistantText(path string) string {
	if !strings.HasSuffix(path, ".jsonl") {
		return ""
	}
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	if info, err := f.Stat(); err == nil && info.Size() > transcriptTail {
		_, _ = f.Seek(info.Size()-transcriptTail, io.SeekStart) // one it can't seek in is read from the start
	}
	data, _ := io.ReadAll(f)
	lines := strings.Split(string(data), "\n")
	for _, line := range slices.Backward(lines) {
		var entry struct {
			Role    string `json:"role"`
			Message struct {
				Content []struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"content"`
			} `json:"message"`
		}
		if json.Unmarshal([]byte(line), &entry) != nil || entry.Role != "assistant" {
			continue
		}
		var parts []string
		for _, c := range entry.Message.Content {
			if c.Type == "text" && strings.TrimSpace(c.Text) != "" {
				parts = append(parts, c.Text)
			}
		}
		if len(parts) > 0 {
			return strings.Join(parts, "\n")
		}
	}
	return ""
}
