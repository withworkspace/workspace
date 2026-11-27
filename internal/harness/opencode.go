package harness

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/withworkspace/workspace/internal/acp"

	_ "modernc.org/sqlite" // reading OpenCode's sessions (sessionKept)
)

// OpenCode runs OpenCode (opencode.ai) over ACP, `opencode acp`: crew,
// held by the daemon and seen in the web UI, and the headless roles
// (conversations, the orchestrator, the Team page), each held for its one
// turn.
//
// READ in OpenCode's code (v1.19.5; it couldn't be run here): initialize
// offers load and resume (resume doesn't replay), and no sign-in is
// needed (its one auth method, opencode-login, does nothing). A session's
// settings are config options: mode (its primary agents, build and plan,
// and any you define), model (provider/model) and, for a model that has
// them, its effort (category thought_level: the model's variants). A
// session is ses_…, kept in OpenCode's SQLite database. Permissions are
// OpenCode's own rules (its config's permission, each agent's own after
// it, the last rule that matches winning); by default everything's
// allowed but reading outside the session's folder, reading a .env file
// and a call repeated three times over, which are asked of the client. An
// ask offers once, always and reject; a command's ask has it in rawInput,
// an edit's has the diff. A rejection fails the call and, unless
// experimental.continue_loop_on_deny, ends the turn (end_turn). It sends
// no requests of its own: its question tool is off over ACP.
type OpenCode struct {
	// lookPath finds the CLI, and db is OpenCode's database ("" for where
	// OpenCode keeps it); tests swap both.
	lookPath func(string) (string, error)
	db       string
}

// EnvOpenCodeBin overrides the OpenCode binary (name or path).
const EnvOpenCodeBin = "WS_OPENCODE_BIN"

func init() {
	register(&OpenCode{lookPath: exec.LookPath})
}

func (o *OpenCode) Name() string { return "opencode" }

// Its crew run over ACP, held by the daemon, seen in the web UI.
func (o *OpenCode) held() (CrewVia, TranscriptVia) { return CrewACP, TranscriptACP }

func (o *OpenCode) bin() (string, error) {
	name := "opencode"
	if b := os.Getenv(EnvOpenCodeBin); b != "" {
		name = b
	}
	path, err := o.lookPath(name)
	if err != nil {
		return "", fmt.Errorf("could not find opencode CLI (looked for %s; set %s to override)", name, EnvOpenCodeBin)
	}
	return path, nil
}

// agent is OpenCode's ACP agent: `opencode acp`.
func (o *OpenCode) agent() ([]string, error) {
	bin, err := o.bin()
	if err != nil {
		return nil, err
	}
	return []string{bin, "acp"}, nil
}

func (o *OpenCode) Available() error {
	_, err := o.bin()
	return err
}

// openCodeBuild is OpenCode's own agent for doing the work: crew run in
// it, as your OpenCode config has it.
const openCodeBuild = "build"

// Start is new crew: an OpenCode session in the capsule, in its build
// agent, its opening prompt the brief. What your OpenCode config has it
// ask (by default: little) is asked of ws, which lets out crew's commands
// (CrewPolicy); the rest waits for you.
func (o *OpenCode) Start(b Brief) (Launch, error) {
	agent, err := o.agent()
	if err != nil {
		return Launch{}, err
	}
	run := ACPRun{Agent: agent, Dir: b.Dir, Mode: openCodeBuild, Allow: crewCommands, Prompt: b.Prompt}
	run.Model, run.Settings = openCodeModel(b.Model)
	return Launch{Dir: b.Dir, ACP: &run}, nil
}

// Resume goes on with crew's session: OpenCode picks it up without
// replaying it (ws has its record).
func (o *OpenCode) Resume(dir, conversation string) (Launch, error) {
	if conversation == "" {
		return Launch{}, fmt.Errorf("%w: no OpenCode session recorded for this crew", ErrNoResume)
	}
	if !o.sessionKept(conversation) {
		return Launch{}, fmt.Errorf("%w: OpenCode has no session %q", ErrNoResume, conversation)
	}
	agent, err := o.agent()
	if err != nil {
		return Launch{}, err
	}
	run := ACPRun{Agent: agent, Dir: dir, Load: conversation, Mode: openCodeBuild, Allow: crewCommands}
	return Launch{Dir: dir, Conversation: conversation, ACP: &run}, nil
}

// ConverseModel and OrchestratorModel are "": OpenCode runs on whichever
// providers you've signed in to, so ws can't name a model sure to be
// there. They run on your OpenCode default unless a role's model says.
func (o *OpenCode) ConverseModel() string     { return "" }
func (o *OpenCode) OrchestratorModel() string { return "" }

// openCodeHeadless is the agent OpenCode's headless roles run in: ws's
// own, defined for the run (openCodeConfig) rather than OpenCode's build
// or plan. Rules in your config come before an agent's own, so its rules
// are the last word whatever yours say.
const openCodeHeadless = "ws-headless"

// openCodeConfig is the OpenCode config a headless run is given
// (OPENCODE_CONFIG_CONTENT, merged over yours): the ws-headless agent, and
// a refused call failing alone rather than ending the turn.
//
// The agent may read and search (but not a .env file: nobody's there to
// say yes, as OpenCode would ask), anywhere, as Claude Code's and Codex's
// headless runs may; every shell command is asked of ws, whose policy lets
// out the role's (CommandPolicy) and refuses the rest. Everything else
// (editing, fetching, subagents, todos) is denied: a subagent's asks
// would never reach ws. Alone, everything is. A conversation that runs
// things (runs) has its edits asked of ws too, which allows them in its
// folders (ConversePolicy), and fetches. The JSON is written out, not
// marshalled from a map: the last rule that matches wins, so the order of
// its keys is what it means.
func openCodeConfig(alone, runs bool) string {
	perm := `{"*":"deny","read":{"*":"allow","*.env":"deny","*.env.*":"deny","*.env.example":"allow"},` +
		`"glob":"allow","grep":"allow","external_directory":"allow","bash":"ask"}`
	switch {
	case alone:
		perm = `{"*":"deny"}`
	case runs:
		perm = strings.TrimSuffix(perm, "}") + `,"edit":"ask","webfetch":"allow"}`
	}
	return `{"agent":{"` + openCodeHeadless + `":{"mode":"primary","description":"ws's headless runs: reading, and the commands ws lets out.",` +
		`"permission":` + perm + `}},"experimental":{"continue_loop_on_deny":true}}`
}

// EnvOpenCodeConfig is OpenCode's own variable for config given inline.
const EnvOpenCodeConfig = "OPENCODE_CONFIG_CONTENT"

// headless is a headless run: in ws-headless, its commands allow. One
// that runs things (folders, not nil: a conversation's) is answered by
// ConversePolicy, its edits judged by folders.
func (o *OpenCode) headless(dir, session, model, prompt string, allow []string, alone bool, folders func() []string) (Launch, error) {
	agent, err := o.agent()
	if err != nil {
		return Launch{}, err
	}
	if session != "" && !o.sessionKept(session) {
		return Launch{}, fmt.Errorf("%w: OpenCode has no session %s", ErrSessionGone, session)
	}
	runs := folders != nil && !alone
	run := ACPRun{Agent: agent, Env: []string{EnvOpenCodeConfig + "=" + openCodeConfig(alone, runs)}, Dir: dir, Load: session,
		Mode: openCodeHeadless, Prompt: prompt, Allow: allow}
	if alone {
		run.Allow = nil
	}
	if runs {
		run.Runs, run.Folders = true, folders
	}
	run.Model, run.Settings = openCodeModel(model)
	return acpLaunch(run, "")
}

// Converse runs one turn of a conversation in ws-headless, its commands
// those a conversation may run, going on with its session. One that runs
// things (r.Sandbox) runs any command and edits in its folders, as ws
// answers (ConversePolicy): OpenCode has no sandbox, and asks ws before
// each command and edit, as ws's agent config says.
func (o *OpenCode) Converse(r ConverseRun) (Launch, error) {
	var folders func() []string
	if r.Sandbox {
		folders = r.Folders
		if folders == nil {
			folders = func() []string { return nil }
		}
	}
	l, err := o.headless(r.Dir, r.Session, r.Model, r.Prompt, converseCommands(r), r.Alone, folders)
	if l.ACP != nil {
		l.ACP.Images = r.Images
	}
	return l, err
}

// Trusted: a conversation's agent on OpenCode runs things, with no
// sandbox, ws answering what it asks (ConversePolicy), as on Cursor.
// READ (OpenCode's code, v1.19.5; it couldn't be run here): an edit
// asked for carries its file as rawInput's filepath and its diff, which
// ConversePolicy places (permissionPaths).
func (o *OpenCode) Trusted() bool { return true }

// Orchestrate runs the orchestrator in ws-headless, its commands those
// it may run.
func (o *OpenCode) Orchestrate(r OrchestratorRun) (Launch, error) {
	return o.headless(r.Dir, "", r.Model, r.Prompt, orchestratorCommands(r), false, nil)
}

// openCodeModel is a model as ws's config names it for OpenCode: its own
// provider/model, or with one of the model's variants (its effort) in
// brackets, as Codex's are, "anthropic/claude-sonnet-5-5[high]": the
// model, and the setting for its effort.
func openCodeModel(m string) (string, []acp.Setting) { return codexModel(m) }

func (o *OpenCode) acpModel(m string) (string, []acp.Setting) { return openCodeModel(m) }

// Models are OpenCode's, by `opencode models`: provider/model, one a
// line, from whichever providers you've signed in to (and OpenCode's own
// free ones).
func (o *OpenCode) Models(ctx context.Context) ([]Model, error) {
	bin, err := o.bin()
	if err != nil {
		return nil, err
	}
	out, err := runModelsCLI(ctx, []string{bin, "models"})
	if err != nil {
		return nil, err
	}
	var models []Model
	for line := range strings.SplitSeq(string(out), "\n") {
		id := strings.TrimSpace(line)
		if id == "" || strings.ContainsAny(id, " \t{}\"") || !strings.Contains(id, "/") {
			continue
		}
		_, name, _ := strings.Cut(id, "/")
		models = append(models, Model{ID: id, Name: name})
	}
	return models, nil
}

// dbPath is where OpenCode keeps its database: $OPENCODE_DB (a path, or
// one in its data folder), else opencode.db in its data folder
// ($XDG_DATA_HOME/opencode, else ~/.local/share/opencode). "" where it
// can't say.
func (o *OpenCode) dbPath() string {
	if o.db != "" {
		return o.db
	}
	data := os.Getenv("XDG_DATA_HOME")
	if data == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		data = filepath.Join(home, ".local", "share")
	}
	data = filepath.Join(data, "opencode")
	db := os.Getenv("OPENCODE_DB")
	switch {
	case db == "":
		return filepath.Join(data, "opencode.db")
	case db == ":memory:" || filepath.IsAbs(db):
		return db
	}
	return filepath.Join(data, db)
}

// errSessionUnknown is a database that can't say whether it has a session.
var errSessionUnknown = errors.New("can't tell")

// sessionKept is whether OpenCode still has a session: a row of its id in
// its database's session table. Where it can't tell (no database where it
// looked, one it can't read), it says yes, and resuming says if not. In
// memory (OPENCODE_DB=:memory:), none outlives its run.
func (o *OpenCode) sessionKept(id string) bool {
	path := o.dbPath()
	if path == ":memory:" {
		return false
	}
	kept, err := sessionInDB(path, id)
	if err != nil {
		return true
	}
	return kept
}

// sessionInDB looks for a session in OpenCode's database, read only.
func sessionInDB(path, id string) (bool, error) {
	if path == "" {
		return false, errSessionUnknown
	}
	if _, err := os.Stat(path); err != nil {
		return false, errSessionUnknown
	}
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro&_pragma=busy_timeout(2000)")
	if err != nil {
		return false, err
	}
	defer db.Close()
	var one int
	switch err := db.QueryRow("SELECT 1 FROM session WHERE id = ?", id).Scan(&one); { //nolint:noctx // a read-only lookup: busy_timeout bounds it
	case errors.Is(err, sql.ErrNoRows):
		return false, nil
	case err != nil:
		return false, err // not the database it was: can't tell
	}
	return true, nil
}
