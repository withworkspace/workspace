package harness

import (
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/acp"
)

// The process check, on a fake table: crew c2 of mission cache, whose
// harness (pid 100) started at t0. ws's hook command (pid 400) runs under
// it in a shell, as Claude Code runs a hook.
var (
	t0       = time.Date(2026, 11, 12, 18, 0, 0, 0, time.UTC)
	crewVars = []string{"PATH=/usr/bin", "WS_MISSION=cache", "WS_CREW=c2"}
)

func at(min int) time.Time { return t0.Add(time.Duration(min) * time.Minute) }

func fakeProcs() Procs {
	return Procs{List: []Proc{
		{PID: 1, PPID: 0, Start: at(-600), Name: "launchd"},
		{PID: 90, PPID: 1, Start: at(-60), Name: "workspace", Args: []string{"workspace", "daemon", "run"}, Env: []string{"WS_MISSION=", "WS_CREW="}},
		{PID: 100, PPID: 90, Start: at(0), Name: "claude", Args: []string{"claude", "--resume", "x"}, Env: crewVars},
		// A command still running in a shell the harness ran: the shell
		// (Claude Code's wrapper) and the command under it.
		{PID: 110, PPID: 100, Start: at(5), Name: "zsh", Args: []string{"/bin/zsh", "-c", "source snap.sh && eval 'just test' < /dev/null && pwd -P >| /tmp/cwd"}, Env: []string{}},
		{PID: 111, PPID: 110, Start: at(5), Name: "just", Args: []string{"just", "test"}, Env: crewVars},
		{PID: 112, PPID: 111, Start: at(5), Name: "go", Args: []string{"go", "test", "./..."}, Env: crewVars},
		// An MCP server, the harness's direct child, not a shell, and its
		// child; a language server started mid-session.
		{PID: 120, PPID: 100, Start: at(0), Name: "node", Args: []string{"node", "mcp-server.js"}, Env: crewVars},
		{PID: 121, PPID: 120, Start: at(0), Name: "zsh", Args: []string{"zsh", "-c", "helper"}, Env: crewVars},
		{PID: 130, PPID: 100, Start: at(20), Name: "gopls", Args: []string{"gopls", "serve"}, Env: crewVars},
		// ws's hook command, in a shell of the harness's.
		{PID: 399, PPID: 100, Start: at(30), Name: "sh", Args: []string{"/bin/sh", "-c", "ws mission hook stop"}, Env: []string{}},
		{PID: 400, PPID: 399, Start: at(30), Name: "ws", Args: []string{"ws", "mission", "hook", "stop"}, Env: crewVars},
		// Detached: an orphan with the crew's names, started after the
		// session, still in the group of the shell it was started from
		// (gone), and the server it started.
		{PID: 200, PPID: 1, PGID: 199, Start: at(10), Name: "node", Args: []string{"node", "-e", "setTimeout(()=>{},5000)"}, Env: crewVars},
		{PID: 201, PPID: 200, PGID: 199, Start: at(10), Name: "esbuild", Args: []string{"esbuild", "--service"}, Env: crewVars},
		// A tool's daemon that a command of the crew's started: it put
		// itself in a group of its own, as daemons do.
		{PID: 220, PPID: 1, PGID: 220, Start: at(15), Name: "turbo", Args: []string{"turbo", "daemon"}, Env: crewVars},
		{PID: 221, PPID: 220, PGID: 220, Start: at(15), Name: "node", Args: []string{"node", "worker.js"}, Env: crewVars},
		// Orphans that aren't its: without the names, another crew's, its
		// environment hidden, started before the session.
		{PID: 210, PPID: 1, Start: at(10), Name: "node", Args: []string{"node", "dev-server.js"}, Env: []string{"PATH=/usr/bin"}},
		{PID: 211, PPID: 1, Start: at(10), Name: "node", Args: []string{"node", "theirs.js"}, Env: []string{"WS_MISSION=cache", "WS_CREW=c3"}},
		{PID: 212, PPID: 1, Start: at(10), Name: "sleep", Args: []string{"/bin/sleep", "45"}, Env: []string{}},
		{PID: 213, PPID: 1, Start: at(-5), Name: "node", Args: []string{"node", "old.js"}, Env: crewVars},
		// Re-parented elsewhere than launchd (a Linux subreaper): still
		// nobody's descendant of the harness.
		{PID: 300, PPID: 1, Start: at(-600), Name: "systemd", Args: []string{"systemd", "--user"}, Env: []string{}},
		{PID: 301, PPID: 300, Start: at(12), Name: "python3", Args: []string{"python3", "-m", "http.server"}, Env: crewVars},
	}}
}

var cacheC2 = ProcCrew{Mission: "cache", Crew: "c2", Harness: 100, Hook: 400}

func pids(f *Found) []int {
	var out []int
	for _, r := range f.Running {
		out = append(out, r.PID)
	}
	slices.Sort(out)
	return out
}

func TestProcs_CrewWork(t *testing.T) {
	f := fakeProcs().CrewWork(cacheC2)
	if f == nil {
		t.Fatal("no check made")
	}
	// The shell running just test (named as ps names its command), the
	// detached node and the python3 re-parented to a subreaper. Not the MCP
	// server or its shell, gopls, the hook or its shell, the orphans that
	// aren't the crew's, the daemon, nor what runs under work already
	// found.
	if got := pids(f); !reflect.DeepEqual(got, []int{110, 200, 301}) {
		t.Fatalf("found %v", got)
	}
	for _, r := range f.Running {
		switch r.PID {
		case 110:
			if !r.Shell || r.What != "just test" {
				t.Errorf("the shell: %+v", r)
			}
		case 200:
			if r.Shell || r.What != "node -e setTimeout(()=>{},5000)" {
				t.Errorf("the orphan: %+v", r)
			}
		}
	}
}

func TestProcs_CrewWork_CantCheck(t *testing.T) {
	procs := fakeProcs()
	for name, c := range map[string]ProcCrew{
		"no harness pid":          {Mission: "cache", Crew: "c2", Hook: 400},
		"the harness has gone":    {Mission: "cache", Crew: "c2", Harness: 999, Hook: 400},
		"the hook isn't under it": {Mission: "cache", Crew: "c2", Harness: 120, Hook: 400},
		"no names to look for":    {Harness: 100, Hook: 400},
	} {
		if f := procs.CrewWork(c); f != nil {
			t.Errorf("%s: checked, %v", name, pids(f))
		}
	}
	// From the daemon, for crew over ACP: no hook, the harness is the
	// agent's process.
	if f := procs.CrewWork(ProcCrew{Mission: "cache", Crew: "c2", Harness: 100}); f == nil || !reflect.DeepEqual(pids(f), []int{110, 200, 301, 399}) {
		t.Errorf("from the daemon: %v", f)
	}
}

// A shell whose arguments read as a harness quotes a command: Claude Code
// writes a quote in eval '…' as '"'"'.
func TestProcs_ShellText(t *testing.T) {
	procs := Procs{List: []Proc{
		{PID: 100, PPID: 1, Start: at(0), Name: "claude"},
		{PID: 110, PPID: 100, Start: at(1), Name: "zsh", Args: []string{"/bin/zsh", "-c", `eval 'echo '"'"'hi'"'"' && sleep 30' < /dev/null`}},
		{PID: 120, PPID: 100, Start: at(2), Name: "bash", Args: []string{"bash", "-lc", "make serve"}},
	}}
	f := procs.CrewWork(ProcCrew{Mission: "m", Crew: "c1", Harness: 100})
	got := map[int]Running{}
	for _, r := range f.Running {
		got[r.PID] = r
	}
	if listedAs([]Work{{What: "echo 'hi' && sleep 30", From: FromHarness}}, got[110]) != 0 {
		t.Errorf("the quoted command isn't found in %q", got[110].text)
	}
	if got[120].What != "make serve" { // no child: the command it was given
		t.Errorf("a shell's command: %+v", got[120])
	}
}

// Reading processes lazily: only those the check looks at.
func TestProcs_ReadsWhatItLooksAt(t *testing.T) {
	procs := fakeProcs()
	full := map[int]Proc{}
	for i := range procs.List {
		full[procs.List[i].PID] = procs.List[i]
		procs.List[i].Args, procs.List[i].Env = nil, nil
	}
	var read []int
	procs.read = func(p *Proc) {
		read = append(read, p.PID)
		p.Args, p.Env = full[p.PID].Args, full[p.PID].Env
	}
	if got := pids(procs.CrewWork(cacheC2)); !reflect.DeepEqual(got, []int{110, 200, 301}) {
		t.Fatalf("found %v", got)
	}
	slices.Sort(read)
	// The harness's shells and their children, and what started after the
	// session outside the harness.
	if want := []int{110, 111, 200, 201, 210, 211, 212, 220, 221, 301}; !reflect.DeepEqual(read, want) {
		t.Errorf("read %v, want %v", read, want)
	}
}

// What each harness makes of what the check found.
func TestAtStop_WithProcesses(t *testing.T) {
	found := &Found{Running: []Running{
		{PID: 110, What: "just test", Shell: true, text: "/bin/zsh -c source snap.sh && eval 'just test' < /dev/null\njust test"},
		{PID: 115, What: "afplay done.aiff", Shell: true, text: "/bin/sh -c afplay done.aiff"},
		{PID: 200, What: "node server.js"},
	}}
	check := func() *Found { return found }
	tasks := `{"background_tasks":[{"id":"b1","status":"running","command":"just test"},{"id":"b2","status":"running","command":"npm run build"}]}`
	detached := Work{What: "node server.js", From: FromProcesses, PID: 200, Wakes: WakeNo}
	cases := []struct {
		name, harness, payload string
		want                   Pending
	}{
		// Claude Code lists its tasks: a shell gives the one it runs its
		// pid; another shell of its (a hook beside ws's) isn't work.
		{"claude", "claude", tasks, Pending{Known: true, Work: []Work{
			{What: "just test", From: FromHarness, PID: 110, Wakes: WakeYes},
			{What: "npm run build", From: FromHarness, Wakes: WakeYes},
			detached,
		}}},
		// Without background_tasks, its shells are work it can't say it
		// wakes the agent for.
		{"claude without background_tasks", "claude", `{}`, Pending{Known: true, Work: []Work{
			{What: "just test", From: FromProcesses, PID: 110, Wakes: WakeUnknown},
			{What: "afplay done.aiff", From: FromProcesses, PID: 115, Wakes: WakeUnknown},
			detached,
		}}},
		{"cursor in a terminal", "cursor", `{"status":"completed"}`, Pending{Known: true, Work: []Work{
			{What: "just test", From: FromProcesses, PID: 110, Wakes: WakeYes},
			{What: "afplay done.aiff", From: FromProcesses, PID: 115, Wakes: WakeYes},
			detached,
		}}},
		{"pi", "pi", `{}`, Pending{Known: true, Work: []Work{
			{What: "just test", From: FromProcesses, PID: 110, Wakes: WakeNo},
			{What: "afplay done.aiff", From: FromProcesses, PID: 115, Wakes: WakeNo},
			detached,
		}}},
		// A harness without an adapter for it isn't checked.
		{"shell", "shell", `{}`, Pending{}},
		{"no such harness", "no-such-harness", `{}`, Pending{}},
	}
	for _, tc := range cases {
		if got := AtStop(tc.harness, payload(t, tc.payload), check); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s:\n got %+v\nwant %+v", tc.name, got, tc.want)
		}
	}
	// No check made: the payload alone.
	if got := AtStop("pi", payload(t, `{}`), func() *Found { return nil }); got.Known {
		t.Errorf("pi unchecked: %+v", got)
	}
	if got := AtStop("claude", payload(t, tasks), nil); len(got.Work) != 2 || got.Work[0].PID != 0 {
		t.Errorf("claude unchecked: %+v", got)
	}
	// Nothing found: nothing running.
	if got := AtStop("pi", payload(t, `{}`), func() *Found { return &Found{} }); !reflect.DeepEqual(got, Pending{Known: true}) {
		t.Errorf("pi, nothing found: %+v", got)
	}
}

// Over ACP: the transcript's calls, with what the check found, for a turn
// that ended end_turn; a turn stopped any other way is unknown (d4).
func TestAtTurnEnd_WithProcesses(t *testing.T) {
	call := acp.ToolCall{ID: "call_1", Kind: "execute", Title: "`sleep 45`", Status: acp.StatusInProgress}
	found := &Found{Running: []Running{
		{PID: 110, What: "sleep 45", Shell: true, text: "zsh -c sleep 45; true\nsleep 45"},
		{PID: 200, What: "node server.js"},
	}}
	checked := 0
	check := func() *Found { checked++; return found }
	got := AtTurnEnd("cursor-acp", acp.Turn{Stop: "end_turn", Unfinished: []acp.ToolCall{call}}, check)
	want := Pending{Known: true, Work: []Work{
		{What: "sleep 45", From: FromHarness, PID: 110, Wakes: WakeNo, Call: "call_1"},
		{What: "node server.js", From: FromProcesses, PID: 200, Wakes: WakeNo},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("cursor-acp:\n got %+v\nwant %+v", got, want)
	}
	// Codex: only work it detached is found (its commands are its calls).
	if got := AtTurnEnd("codex", acp.Turn{Stop: "end_turn"}, func() *Found { return &Found{Running: found.Running[1:]} }); !reflect.DeepEqual(got, Pending{Known: true, Work: want.Work[1:]}) {
		t.Errorf("codex: %+v", got)
	}
	checked = 0
	for _, turn := range []acp.Turn{{Stop: "cancelled", Unfinished: []acp.ToolCall{call}}, {Stop: "refusal"}} {
		if got := AtTurnEnd("codex", turn, check); got.Known {
			t.Errorf("a turn ended %s: %+v", turn.Stop, got)
		}
	}
	if checked != 0 {
		t.Errorf("checked processes %d times for turns that were stopped", checked)
	}
}
