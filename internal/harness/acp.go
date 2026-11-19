package harness

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/withworkspace/workspace/internal/acp"
)

// Agents spoken to over ACP (internal/acp). Headless, a run is `ws acp run
// <spec>`: a ws process that starts the agent, sends it one prompt and
// prints the session's record (StreamACP) as it goes. The runners start it
// as they would any agent, keep what it prints in a run file, and read it
// back with ReadStream. ws answers what the agent asks itself, by a policy
// (CommandPolicy): a command the role may run, run alone, is let out of
// the agent's sandbox, and anything else is refused.

// StreamACP is an ACP session's record (acp.Event JSON lines), as
// `ws acp run` prints it.
const StreamACP = "acp"

// ACPRun is one headless run of an ACP agent: what `ws acp run` is given.
type ACPRun struct {
	// Agent is the agent's command; Env is added to the environment it
	// runs in (the run's, from its runner).
	Agent []string `json:"agent"`
	Env   []string `json:"env,omitempty"`
	// Base is the environment the agent starts from (Env added to it): the
	// turn's, when it's held in a process that isn't the agent's own (the
	// server). nil for this process's.
	Base []string `json:"-"`
	// Dir is the session's folder, and Dirs others it may work in.
	Dir  string   `json:"dir"`
	Dirs []string `json:"dirs,omitempty"`
	// Load is a session to go on with ("" starts one).
	Load string `json:"load,omitempty"`
	// Mode, Model and Settings are chosen by name (acp.Options).
	Mode     string        `json:"mode,omitempty"`
	Model    string        `json:"model,omitempty"`
	Settings []acp.Setting `json:"settings,omitempty"`
	// Allow are the shell commands it may run outside its sandbox, each
	// the start of a command run alone (CommandPolicy).
	Allow  []string `json:"allow,omitempty"`
	Prompt string   `json:"prompt"`
	// Auth is the auth method to sign in by first ("" for none).
	Auth string `json:"auth,omitempty"`
	// Answers are what the agent's own requests (an extension's, by
	// method) are answered, as nobody's there to: Cursor's question,
	// skipped. Others aren't offered.
	Answers map[string]json.RawMessage `json:"answers,omitempty"`
	// Ask are the agent's own requests a person answers, for crew someone
	// watches (Cursor's question): they wait, as a permission does.
	Ask []string `json:"ask,omitempty"`
}

// acpModeler is a harness whose crew run over ACP, putting a model, as
// ws's config names it for that harness, on a run.
type acpModeler interface {
	acpModel(m string) (string, []acp.Setting)
}

func (c *Codex) acpModel(m string) (string, []acp.Setting)     { return codexModel(m) }
func (c *CursorACP) acpModel(m string) (string, []acp.Setting) { return cursorACPModel(m), nil }

// WithModel puts a role's model on crew's ACP run where it has none:
// Resume isn't told one, and a session resumed with none goes on with the
// agent's own default (Codex warns that it was recorded with another).
func WithModel(h Harness, l Launch, model string) Launch {
	if l.ACP == nil || l.ACP.Model != "" || model == "" {
		return l
	}
	if m, ok := h.(acpModeler); ok {
		run := *l.ACP
		run.Model, run.Settings = m.acpModel(model)
		l.ACP = &run
	}
	return l
}

// crewCommands are what crew may run outside the agent's sandbox without
// asking you: ws's commands (the mission's, lift, show; ws enforces what
// crew may do), committing in their capsule, pushing their branch and
// fetching, and GitHub's pull requests. Anything else they ask to run outside it, or any other call
// they ask leave for, waits for you on the crew's page.
var crewCommands = []string{
	"ws", "workspace",
	"git add", "git commit", "git push", "git fetch", "git pull",
	"gh pr create", "gh pr view", "gh pr checks", "gh pr list", "gh pr edit", "gh pr comment", "gh issue view",
}

// gitDirsOf are the git folders a capsule's commits write, outside it: a
// worktree's .git is a file naming its folder in the repo's bare clone
// (repos/<repo>/.bare/worktrees/<name>), and commits write to the clone
// itself (its objects and refs). None for a folder that isn't one.
func gitDirsOf(dir string) []string {
	data, err := os.ReadFile(filepath.Join(dir, ".git"))
	if err != nil {
		return nil // a folder (.git inside it), or not a repo
	}
	gitdir, ok := strings.CutPrefix(strings.TrimSpace(string(data)), "gitdir:")
	if !ok {
		return nil
	}
	gitdir = strings.TrimSpace(gitdir)
	if !filepath.IsAbs(gitdir) {
		gitdir = filepath.Join(dir, gitdir)
	}
	if i := strings.LastIndex(gitdir, string(filepath.Separator)+"worktrees"+string(filepath.Separator)); i > 0 {
		return []string{gitdir[:i]}
	}
	return []string{gitdir}
}

// acpLaunch is a headless run of an ACP agent: no command to start, but
// the run, which turn.Run holds in process for its one turn (RunACP),
// reading the session's record as it goes.
func acpLaunch(r ACPRun, scratch string) (Launch, error) {
	return Launch{ACP: &r, Dir: r.Dir, Stream: StreamACP, Conversation: r.Load}, nil
}

// base is the environment the agent starts from: Base, else this process's.
func (r ACPRun) base() []string {
	if r.Base != nil {
		return r.Base
	}
	return os.Environ()
}

// dialACP connects RunACP to its agent instead of starting it (tests').
var dialACP func(ctx context.Context, cb acp.Callbacks) (*acp.Client, error)

// acpCloseGrace is how long an agent has to exit once its run is over.
const acpCloseGrace = 5 * time.Second

// acpCarryOn is how many times a run is told to carry on after a refusal
// ended its turn.
const acpCarryOn = 3

// RunACP runs one turn of an ACP agent headless, writing the session's
// record to out as it goes; the agent's own logging goes to errOut. The
// agent shares this process's group, so whatever ends the run by killing
// its group ends the agent too. It fails if the turn does: the agent
// gone, or its turn ending other than end_turn.
//
// An agent may take a refusal as the end of its turn: Codex's only way to
// say no to some calls is "No, and tell Codex what to do differently",
// which cancels the turn (VERIFIED, codex-acp 2.6.0). A turn that ends
// cancelled just after ws refused something is followed by another,
// telling it what was refused and to carry on without, up to acpCarryOn
// times: one refused command mustn't cost the whole run.
func RunACP(ctx context.Context, r ACPRun, out, errOut io.Writer) error {
	rec := acp.NewRecorder(out)
	var mu sync.Mutex
	var refused []string
	policy := CommandPolicy(r.Allow)
	ag, err := acp.Run(ctx, acp.Options{
		Command:  acp.Command{Args: r.Agent, Dir: r.Dir, Env: append(r.base(), r.Env...), Stderr: errOut},
		Session:  acp.SessionParams{Dir: r.Dir, Dirs: r.Dirs},
		Load:     r.Load,
		Mode:     r.Mode,
		Model:    r.Model,
		Settings: r.Settings,
		Policy: func(a acp.Ask) (acp.Answer, bool) {
			ans, ok := policy(a)
			if ok && a.Permission != nil && !permissionAllowed(*a.Permission, ans) {
				mu.Lock()
				refused = append(refused, askedFor(*a.Permission))
				mu.Unlock()
			}
			return ans, ok
		},
		Record: rec,
		Dial:   dialACP,
		Auth:   r.Auth,
		Extension: func(ctx context.Context, asks *acp.Asks, method string, params json.RawMessage) (any, error) {
			if ans, ok := r.Answers[method]; ok {
				return ans, nil
			}
			return nil, acp.ErrNotOffered
		},
	})
	if err != nil {
		rec.Write(acp.Event{Error: err.Error()}) //nolint:errcheck // failing anyway
		return err
	}
	defer ag.Close(acpCloseGrace) //nolint:errcheck // its turn is what counts
	prompt := r.Prompt
	for carried := 0; ; carried++ {
		mu.Lock()
		refused = nil
		mu.Unlock()
		turn, err := ag.Prompt(ctx, acp.Text(prompt))
		if ctx.Err() != nil {
			ag.Interrupt() //nolint:errcheck // closing it next
			return ctx.Err()
		}
		if err != nil {
			return err
		}
		mu.Lock()
		was := refused
		mu.Unlock()
		if turn.Stop == "cancelled" && len(was) > 0 && carried < acpCarryOn {
			prompt = carryOnPrompt(was)
			continue
		}
		if turn.Stop != "end_turn" {
			return errors.New(stopFailure(turn.Stop))
		}
		return rec.Err()
	}
}

// askedFor is what a permission was asked for, in a few words: its
// command, or its title.
func askedFor(r acp.PermissionRequest) string {
	if cmd, ok := permissionCommand(r.ToolCall); ok {
		return "`" + cmd + "`"
	}
	if r.ToolCall.Title != nil {
		return *r.ToolCall.Title
	}
	return "a " + firstNonEmptyString(r.ToolCall.Kind, "tool") + " call"
}

// carryOnPrompt tells an agent whose turn a refusal ended to go on.
func carryOnPrompt(refused []string) string {
	return "ws refused " + strings.Join(refused, ", ") + ": that isn't something you may do here (your instructions say what you may run). " +
		"Carry on without it, as your instructions say, and finish your reply."
}

// stopFailure says why a turn that didn't end end_turn failed.
func stopFailure(stop string) string {
	switch stop {
	case "max_tokens":
		return "it ran out of tokens before answering"
	case "max_turn_requests":
		return "it ran out of turns before answering"
	case "refusal":
		return "it refused"
	case "cancelled":
		return "its turn was cancelled"
	}
	return "its turn ended: " + stop
}

// CommandPolicy answers a headless run's asks, with nobody there to: a
// shell command that's one of allow (or starts with one), run alone, may
// run, and a read may be made; anything else asked for (a command not
// allowed, one chained to another, an edit, a fetch) is refused, and a form or anything else the
// agent asks is cancelled. Only what leaves the agent's sandbox is asked
// for: the sandbox is the limit for the rest.
func CommandPolicy(allow []string) acp.Policy {
	return func(a acp.Ask) (acp.Answer, bool) {
		if a.Permission == nil {
			return acp.Answer{Cancelled: true}, true
		}
		r := *a.Permission
		if cmd, ok := permissionCommand(r.ToolCall); ok && commandAllowed(cmd, allow) {
			return acp.Allow(r), true
		}
		if r.ToolCall.Kind == "read" {
			// The headless roles read what they like. Cursor asks to read
			// outside its session's folder where a read boundary is set,
			// and its headless runs work on a folder that isn't that one.
			return acp.Allow(r), true
		}
		return acp.Reject(r), true
	}
}

// CrewPolicy answers what crew's agent asks that ws may answer for you: a
// command crew may run outside the sandbox (allow: crewCommands), or a
// chain of them. Everything else waits for you, on the crew's page.
func CrewPolicy(allow []string) acp.Policy {
	return func(a acp.Ask) (acp.Answer, bool) {
		if a.Permission == nil {
			return acp.Answer{}, false
		}
		if cmd, ok := permissionCommand(a.Permission.ToolCall); ok && commandsAllowed(cmd, allow) {
			return acp.Allow(*a.Permission), true
		}
		return acp.Answer{}, false
	}
}

// AskedFor is what a permission's for, in a few words, for the mission's
// notice: "run `go test ./...`", "edit /w/a.go".
func AskedFor(r acp.PermissionRequest) string {
	if cmd, ok := permissionCommand(r.ToolCall); ok {
		return "run `" + cmd + "`"
	}
	verb := strings.ToLower(firstNonEmptyString(acpTools[r.ToolCall.Kind], "use"))
	for _, l := range r.ToolCall.Locations {
		return verb + " " + l.Path
	}
	// OpenCode names an edit's file in its input, not its locations.
	var in struct {
		Path string `json:"filepath"`
	}
	if acpTools[r.ToolCall.Kind] != "" && json.Unmarshal(r.ToolCall.RawInput, &in) == nil && in.Path != "" {
		return verb + " " + in.Path
	}
	if r.ToolCall.Title != nil {
		return *r.ToolCall.Title
	}
	return "use a tool"
}

// AskQuestion is an ask that's a question for a person (a form, Cursor's
// question tool): what it asks, and the answers it offers.
func AskQuestion(a acp.Ask) (string, []string, bool) {
	q, ok := askQuestion(a)
	return q.Question, q.Options, ok
}

// permissionCommand is the shell command a permission is asked for, if
// it's for one: its input's command (Codex's), else its title (Cursor's,
// in backticks).
func permissionCommand(tc acp.SessionUpdate) (string, bool) {
	var in struct {
		Command json.RawMessage `json:"command"`
	}
	if json.Unmarshal(tc.RawInput, &in) == nil && len(in.Command) > 0 {
		var s string
		if json.Unmarshal(in.Command, &s) == nil {
			return unwrapScript(s), true
		}
		var argv []string
		if json.Unmarshal(in.Command, &argv) == nil && len(argv) > 0 {
			// A shell given the command (bash -lc "…") stands for it.
			if len(argv) == 3 && isShell(argv[0]) && (argv[1] == "-c" || argv[1] == "-lc") {
				return argv[2], true
			}
			return strings.Join(argv, " "), true
		}
	}
	if tc.Kind == "execute" && tc.Title != nil {
		t := *tc.Title
		if len(t) >= 2 && t[0] == '`' && t[len(t)-1] == '`' {
			// Cursor's: the command in backticks, each backtick in it
			// escaped (\`). Unescaped, so the policy judges the command
			// Cursor runs: `ws mission note "\`id\`"` is a substitution.
			return strings.ReplaceAll(t[1:len(t)-1], "\\`", "`"), true
		}
		return t, true
	}
	return "", false
}

func isShell(path string) bool {
	switch filepath.Base(path) {
	case "sh", "bash", "zsh":
		return true
	}
	return false
}

// commandAllowed is whether cmd is one command, its words starting with
// one of allow's (so "git -C /r log" allows "git -C /r log -3" but not
// "git -C /r logs"). A heredoc giving it its input counts as part of it.
func commandAllowed(cmd string, allow []string) bool {
	cmds, ok := parseShell(unwrapScript(cmd))
	return ok && len(cmds) == 1 && wordsAllowed(cmds[0], allow)
}

// commandsAllowed is commandAllowed for a chain (a && b, a; b, one a line):
// each command in it allowed. Crew chain what they'd type together (git
// add && git commit); the headless roles are told to run one at a time.
func commandsAllowed(cmd string, allow []string) bool {
	cmds, ok := parseShell(unwrapScript(cmd))
	if !ok || len(cmds) == 0 {
		return false
	}
	for _, words := range cmds {
		if !wordsAllowed(words, allow) {
			return false
		}
	}
	return true
}

func wordsAllowed(words []string, allow []string) bool {
	for _, a := range allow {
		want := strings.Fields(a)
		if len(want) == 0 || len(want) > len(words) {
			continue
		}
		match := true
		for i, w := range want {
			if words[i] != w {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// unwrapScript is a command that came quoted whole, as one word, as the
// script it is: Codex sends a script over lines (a heredoc) as `bash -lc`
// is given it, quoted ("ws mission done \"…\" --handoff - <<'EOF'…").
// Anything else is as it came.
func unwrapScript(cmd string) string {
	cmds, ok := parseShell(cmd)
	if ok && len(cmds) == 1 && len(cmds[0]) == 1 && strings.ContainsAny(cmds[0][0], " \t\n") {
		return cmds[0][0]
	}
	return cmd
}

// shellWords splits a command as a shell would, if it's one simple
// command (a heredoc may give it its input): ok false for anything that
// would run more, or other than it reads.
func shellWords(s string) ([]string, bool) {
	cmds, ok := parseShell(s)
	if !ok || len(cmds) != 1 {
		return nil, false
	}
	return cmds[0], true
}

// heredoc is a heredoc waiting for its body: its delimiter, whether that's
// quoted (its body read as it is) and whether it's <<- (tabs before lines
// dropped).
type heredoc struct {
	delim          string
	quoted, tabbed bool
}

// parseShell splits a script into its simple commands, each as its words,
// as a shell would: commands are parted by &&, ||, ; and line ends, and a
// heredoc gives its command input. ok false for anything else that would
// run more, or other than what's read: a pipe, a redirection, a
// substitution or expansion ($, backticks), a subshell, a background job,
// an unclosed quote, a heredoc with no end, or one whose body would expand.
func parseShell(s string) (cmds [][]string, ok bool) {
	cmds, _, ok = parseScript(s)
	return cmds, ok
}

// parseScript is parseShell, with each heredoc's body as it was given, in
// the order they were.
func parseScript(s string) (cmds [][]string, bodies []string, ok bool) {
	var words []string
	var cur strings.Builder
	var pending []heredoc
	inWord := false
	endWord := func() {
		if inWord {
			words = append(words, cur.String())
			cur.Reset()
			inWord = false
		}
	}
	endCmd := func() bool {
		endWord()
		if len(words) == 0 {
			return false // an operator with nothing before it
		}
		cmds, words = append(cmds, words), nil
		return true
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == ' ' || c == '\t':
			endWord()
		case c == '\n':
			endWord()
			if len(pending) > 0 {
				next, got, ok := heredocBodies(s, i+1, pending)
				if !ok {
					return nil, nil, false
				}
				bodies = append(bodies, got...)
				pending, i = nil, next-1
			}
			if len(words) > 0 {
				cmds, words = append(cmds, words), nil
			}
		case c == '\'':
			j := strings.IndexByte(s[i+1:], '\'')
			if j < 0 {
				return nil, nil, false
			}
			cur.WriteString(s[i+1 : i+1+j])
			inWord, i = true, i+1+j
		case c == '"':
			inWord = true
			i++
			for ; i < len(s) && s[i] != '"'; i++ {
				switch s[i] {
				case '$', '`':
					return nil, nil, false
				case '\\':
					if i+1 < len(s) && strings.IndexByte("\"\\$`\n", s[i+1]) >= 0 {
						i++
					}
				}
				cur.WriteByte(s[i])
			}
			if i >= len(s) {
				return nil, nil, false
			}
		case c == '\\':
			if i+1 >= len(s) || s[i+1] == '\n' {
				return nil, nil, false
			}
			i++
			cur.WriteByte(s[i])
			inWord = true
		case strings.HasPrefix(s[i:], "&&"), strings.HasPrefix(s[i:], "||"):
			if !endCmd() {
				return nil, nil, false
			}
			i++
		case c == ';':
			if !endCmd() {
				return nil, nil, false
			}
		case strings.HasPrefix(s[i:], "<<") && !strings.HasPrefix(s[i:], "<<<"):
			endWord()
			i += 2
			h := heredoc{}
			if i < len(s) && s[i] == '-' {
				h.tabbed = true
				i++
			}
			for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
				i++
			}
			if i < len(s) && (s[i] == '\'' || s[i] == '"') {
				j := strings.IndexByte(s[i+1:], s[i])
				if j < 0 {
					return nil, nil, false
				}
				h.delim, h.quoted, i = s[i+1:i+1+j], true, i+1+j
			} else {
				j := i
				for j < len(s) && (s[j] == '_' || s[j] == '-' || s[j] >= 'a' && s[j] <= 'z' || s[j] >= 'A' && s[j] <= 'Z' || s[j] >= '0' && s[j] <= '9') {
					j++
				}
				h.delim, i = s[i:j], j-1
			}
			if h.delim == "" {
				return nil, nil, false
			}
			pending = append(pending, h)
		case strings.IndexByte(";&|<>()`$\r", c) >= 0:
			return nil, nil, false
		default:
			cur.WriteByte(c)
			inWord = true
		}
	}
	if len(pending) > 0 {
		return nil, nil, false // a heredoc never given its body
	}
	endWord()
	if len(words) > 0 {
		cmds = append(cmds, words)
	}
	return cmds, bodies, true
}

// heredocBodies reads the bodies of heredocs, one after another, from the
// line starting at from: each ends at a line that's its delimiter. It's
// where the script goes on after them, and each body, as its command
// would read it. ok false for one with no end, or an unquoted one whose
// body would expand ($, backticks, a backslash).
func heredocBodies(s string, from int, hs []heredoc) (int, []string, bool) {
	i := from
	var bodies []string
	for _, h := range hs {
		var body strings.Builder
		for {
			if i >= len(s) {
				return 0, nil, false
			}
			end := strings.IndexByte(s[i:], '\n')
			line, next := s[i:], len(s)
			if end >= 0 {
				line, next = s[i:i+end], i+end+1
			}
			if h.tabbed {
				line = strings.TrimLeft(line, "\t")
			}
			i = next
			if line == h.delim {
				break
			}
			if !h.quoted && strings.ContainsAny(line, "$`\\") {
				return 0, nil, false
			}
			body.WriteString(line + "\n")
		}
		bodies = append(bodies, body.String())
	}
	return i, bodies, true
}

// readACPStream reads an ACP session's record: its session, each tool call
// as it ended (refused ones marked, a command's output where the agent
// gave it), its reply, and why it failed if it did.
func readACPStream(out string) Report {
	events, _ := acp.ReadRecord(strings.NewReader(out))
	tr := acp.Replay(nil, events)
	rep := Report{Output: strings.TrimSpace(tr.Reply())}
	asked := map[string]acp.Ask{}
	refused := map[string]bool{}
	output := map[string]*strings.Builder{}
	for _, e := range events {
		switch {
		case e.Session != "":
			rep.Session = e.Session
		case e.Ask != nil:
			asked[e.Ask.ID] = *e.Ask
		case e.Answer != nil:
			ask, ok := asked[e.Answer.Ask]
			if ok && ask.Permission != nil && !permissionAllowed(*ask.Permission, *e.Answer) {
				refused[ask.Permission.ToolCall.ToolCallID] = true
			}
		case e.Update != nil:
			if id, data := terminalOutput(e.Update); id != "" {
				if output[id] == nil {
					output[id] = &strings.Builder{}
				}
				output[id].WriteString(data)
			}
		case e.Sent != nil:
			// A turn after one that ended: the run goes on (RunACP's
			// carrying on after a refusal), and its end is what counts.
			rep.Done, rep.Failed = false, ""
		case e.Stop != "":
			rep.Done, rep.Failed = true, ""
			if e.Stop != "end_turn" {
				rep.Failed = stopFailure(e.Stop)
			}
		case e.Error != "":
			rep.Failed = "it failed: " + ClipHead(e.Error, 300, "…")
		}
	}
	for _, it := range tr.Items {
		if it.Tool != nil {
			rep.Calls = append(rep.Calls, acpCall(*it.Tool, output[it.Tool.ID], refused[it.Tool.ID]))
		}
	}
	return rep
}

// permissionAllowed is whether an answer let the call go ahead.
func permissionAllowed(r acp.PermissionRequest, ans acp.Answer) bool {
	if ans.Cancelled {
		return false
	}
	for _, o := range r.Options {
		if o.OptionID == ans.Option {
			return o.Kind == acp.AllowOnce || o.Kind == acp.AllowAlways
		}
	}
	return false
}

// terminalOutput is what a command printed, where the agent sends it as
// it goes rather than in the call's content: codex-acp's
// _meta.terminal_output_delta, for a client that doesn't run terminals.
func terminalOutput(update json.RawMessage) (id, data string) {
	var u struct {
		ToolCallID string `json:"toolCallId"`
		Meta       struct {
			Delta *struct {
				Data string `json:"data"`
			} `json:"terminal_output_delta"`
		} `json:"_meta"`
	}
	if json.Unmarshal(update, &u) != nil || u.Meta.Delta == nil {
		return "", ""
	}
	return u.ToolCallID, u.Meta.Delta.Data
}

// acpTools names a call's tool by its kind, where the agent doesn't name
// it in a way worth showing.
var acpTools = map[string]string{
	"read": "Read", "edit": "Edit", "delete": "Delete", "move": "Move", "search": "Search",
	"execute": "Shell", "think": "Think", "fetch": "Fetch", "switch_mode": "Mode",
}

// acpCall is a tool call as a run's report has it.
func acpCall(c acp.ToolCall, out *strings.Builder, refused bool) ToolCall {
	tc := ToolCall{Tool: firstNonEmptyString(acpTools[c.Kind], c.Name, c.Title, "Tool"), Refused: refused}
	switch {
	case c.Kind == "execute" || c.Kind == "read" && c.Input != nil:
		if cmd, ok := permissionCommand(acp.SessionUpdate{RawInput: c.Input, Kind: c.Kind, Title: &c.Title}); ok {
			tc.Input = cmd
		}
	case len(c.Locations) > 0:
		tc.Input = c.Locations[0].Path
	}
	if tc.Input == "" {
		for _, ct := range c.Content {
			if ct.Type == "diff" {
				tc.Input = ct.Path
				break
			}
		}
	}
	if tc.Input == "" {
		tc.Input = c.Title
	}
	var result []string
	for _, ct := range c.Content {
		if ct.Type == "content" && ct.Content != nil {
			result = append(result, ct.Content.String())
		}
	}
	if out != nil {
		result = append(result, out.String())
	}
	tc.Result = ClipHead(strings.TrimSpace(strings.Join(result, "\n")), resultMax, "…")
	// Its exit code: Codex's, or OpenCode's (whose command that fails
	// still ends completed).
	var exit struct {
		Code *int `json:"exit_code"`
		Meta struct {
			Exit *int `json:"exit"`
		} `json:"metadata"`
	}
	json.Unmarshal(c.Output, &exit) //nolint:errcheck // no exit code: none
	code := exit.Code
	if code == nil {
		code = exit.Meta.Exit
	}
	tc.Error = refused || c.Status == acp.StatusFailed || code != nil && *code != 0
	return tc
}

func firstNonEmptyString(s ...string) string {
	for _, v := range s {
		if v != "" {
			return v
		}
	}
	return ""
}
