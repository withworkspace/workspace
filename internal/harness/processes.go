package harness

import (
	"slices"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/acp"
)

// The process check: what an agent left running that its harness doesn't
// say, read from the user's processes at the end of a turn. It finds two
// kinds of work. A command still running in a shell the harness ran for it
// (a shell that is the harness's direct child): a background command the
// harness holds, a Cursor background shell, a Claude Code task. And work
// the agent detached itself (nohup … &): nobody's descendant any more, it
// carries the crew's names (WS_MISSION, WS_CREW, which every process crew
// start inherits), started after the crew's session did, and is still in
// the group of the shell it was started from. The harness's adapter says
// what each is to it (ProcessReader). See
// workspace-records:docs/orchestrator-nudges-crew-waiting-on-background-work.md.
//
// It's a heuristic. MCP servers, language servers and a harness's own
// helpers are its direct children started without a shell, so they don't
// count, but one started through a shell (sh -c …) does. A tool that
// daemonizes itself (setsid, or node's spawn with detached) leads a
// process group of its own, so it doesn't count, nor does work an agent
// started with setsid; a daemon started some other way (Gradle's) does.
// macOS hides the environment of its own binaries (/bin/sh, /bin/sleep),
// so such a process detached is missed: the crew is nudged as before.
// Either way the worst is a nudge that comes as before, or late (the
// backstop: mission.HoldMax).

// Proc is one of the user's processes, as the process check reads it.
type Proc struct {
	PID   int
	PPID  int
	PGID  int // its process group
	Start time.Time
	// Name is its executable's name (ps's comm).
	Name string
	// Args are its arguments, and Env its environment (KEY=VALUE): nil
	// when the system hides them, or they haven't been read.
	Args []string
	Env  []string
}

// Procs is the user's processes at one moment (ReadProcs).
type Procs struct {
	List []Proc
	// read reads a process's arguments and environment, which the check
	// reads only for the processes it looks at (a system call or a file
	// each): nil when List has them already.
	read func(p *Proc)
}

// ProcCrew is crew whose work the process check looks for.
type ProcCrew struct {
	// Mission and Crew are their names, as their processes carry them
	// (WS_MISSION, WS_CREW).
	Mission, Crew string
	// Harness is the pid of the agent's process ws started (mission.Crew
	// PID; for crew over ACP, the agent's).
	Harness int
	// Hook is the pid of ws's hook command, when the check runs in it,
	// under the harness: it and its ancestors aren't work. 0 when the
	// check runs elsewhere (the daemon).
	Hook int
}

// Found is what the process check found running for crew's agent at the
// end of a turn.
type Found struct {
	Running []Running
}

// Running is a piece of it: a process.
type Running struct {
	PID  int
	What string // what it runs
	// Shell: it's a shell the harness ran for a command, and the command
	// still runs. Otherwise the agent detached it.
	Shell bool
	// text is the command text of it and what runs under it, read as
	// a harness quotes it, for finding it in the harness's own list.
	text string
}

// shells are the shells a harness runs commands in, by their executable's
// name.
var shells = []string{"sh", "bash", "zsh", "dash", "ksh", "mksh", "fish", "ash", "csh", "tcsh"}

// CrewWork is what crew's agent left running, as the processes show it:
// nil when the check can't be made (crew's harness isn't running, or isn't
// above the hook command).
func (t Procs) CrewWork(c ProcCrew) *Found {
	if c.Mission == "" || c.Crew == "" || c.Harness <= 0 {
		return nil
	}
	byPID := map[int]*Proc{}
	kids := map[int][]*Proc{}
	for i := range t.List {
		p := &t.List[i]
		byPID[p.PID] = p
	}
	for i := range t.List {
		p := &t.List[i]
		if p.PPID != p.PID {
			kids[p.PPID] = append(kids[p.PPID], p)
		}
	}
	for _, ks := range kids {
		slices.SortFunc(ks, func(a, b *Proc) int { return a.Start.Compare(b.Start) })
	}
	h, ok := byPID[c.Harness]
	if !ok {
		return nil
	}
	hook := map[int]bool{} // the hook command and its ancestors
	for pid := c.Hook; pid > 1 && !hook[pid]; {
		hook[pid] = true
		p, ok := byPID[pid]
		if !ok {
			break
		}
		pid = p.PPID
	}
	if c.Hook > 0 && !hook[c.Harness] {
		return nil // the record's harness isn't the one running the hook
	}
	under := map[int]bool{c.Harness: true}
	for queue := []int{c.Harness}; len(queue) > 0; queue = queue[1:] {
		for _, k := range kids[queue[0]] {
			if !under[k.PID] {
				under[k.PID] = true
				queue = append(queue, k.PID)
			}
		}
	}

	found := &Found{}
	for _, sh := range kids[c.Harness] {
		if hook[sh.PID] || !isShellProc(*sh) {
			continue
		}
		t.fill(sh)
		what := shellCommand(sh.Args)
		texts := []string{strings.Join(sh.Args, " ")}
		for i, k := range kids[sh.PID] {
			t.fill(k)
			if i == 0 && len(k.Args) > 0 {
				what = strings.Join(k.Args, " ") // as ps names it
			}
			texts = append(texts, strings.Join(k.Args, " "))
		}
		found.Running = append(found.Running, Running{PID: sh.PID, What: procWhat(what, sh.Name), Shell: true, text: unquote(strings.Join(texts, "\n"))})
	}

	carries := map[int]bool{} // detached work: the crew's, nobody's descendant
	for i := range t.List {
		p := &t.List[i]
		if under[p.PID] || hook[p.PID] || p.Start.Before(h.Start) {
			continue
		}
		t.fill(p)
		if carriesCrew(p.Env, c.Mission, c.Crew) {
			carries[p.PID] = true
		}
	}
	for i := range t.List {
		p := &t.List[i]
		// The top of what was detached (what runs under it is part of it),
		// unless it put itself in a group of its own: a daemon.
		if carries[p.PID] && !carries[p.PPID] && p.PGID != p.PID {
			found.Running = append(found.Running, Running{PID: p.PID, What: procWhat(strings.Join(p.Args, " "), p.Name), text: unquote(strings.Join(p.Args, " "))})
		}
	}
	return found
}

// fill reads a process's arguments and environment, once.
func (t Procs) fill(p *Proc) {
	if t.read != nil && p.Args == nil && p.Env == nil {
		t.read(p)
		if p.Args == nil {
			p.Args = []string{} // read: hidden
		}
	}
}

// isShellProc is whether a process is a shell, by its executable's name.
func isShellProc(p Proc) bool {
	name := p.Name
	if name == "" && len(p.Args) > 0 {
		name = p.Args[0]
	}
	name = strings.TrimPrefix(name[strings.LastIndex(name, "/")+1:], "-") // a login shell's argv[0] is -zsh
	return slices.Contains(shells, name)
}

// shellCommand is the command a shell was given (-c, -lc, …): "" if none.
func shellCommand(args []string) string {
	for i := 1; i+1 < len(args); i++ {
		if a := args[i]; strings.HasPrefix(a, "-") && !strings.HasPrefix(a, "--") && strings.Contains(a, "c") {
			return args[i+1]
		}
	}
	return ""
}

// carriesCrew is whether an environment carries crew's names.
func carriesCrew(env []string, slug, crewID string) bool {
	return slices.Contains(env, "WS_MISSION="+slug) && slices.Contains(env, "WS_CREW="+crewID)
}

// procWhatMax keeps what's kept of a command to what's worth showing.
const procWhatMax = 200

// procWhat is a process's command, in a line, or its name.
func procWhat(what, name string) string {
	what = strings.Join(strings.Fields(what), " ")
	if what == "" {
		what = name
	}
	if r := []rune(what); len(r) > procWhatMax {
		what = string(r[:procWhatMax-1]) + "…"
	}
	return what
}

// unquote undoes the two usual ways of writing a single quote within a
// single-quoted shell word: closing the word, writing the quote
// double-quoted or escaped, and reopening it. Claude Code does the first
// when it wraps a command in eval '…'.
func unquote(s string) string {
	return strings.NewReplacer(`'"'"'`, `'`, `'\''`, `'`).Replace(s)
}

// ProcessReader is a harness whose turns' ends the process check
// completes: commands still running in shells it ran, and work its agent
// detached. A harness that isn't one isn't checked, and answers as its
// own payload or transcript says.
type ProcessReader interface {
	// Shells is what a command still running in a shell the harness ran
	// is to it, given its own answer (own): whether it wakes the agent when
	// that ends, and whether own lists all such commands already, so that
	// a shell only gives a piece of own its pid.
	Shells(own Pending) (wakes Wake, listed bool)
}

// AtStop is the named harness's answer at the end of a terminal crew's
// turn: its stop hook's payload (PendingAtStop), with what the process
// check finds, for a harness that is a ProcessReader. check makes it (nil,
// or nil back: it can't be made).
func AtStop(name string, payload map[string]any, check func() *Found) Pending {
	return withProcesses(name, PendingAtStop(name, payload), check)
}

// AtTurnEnd is the named harness's answer at the end of a turn of its crew
// over ACP: ws's transcript (PendingAtTurnEnd), with what the process
// check finds, as AtStop. A turn that ended other than with end_turn
// answers unknown, processes or not: it was stopped rather than ended to
// wait (decision d4). So does one an error cut off (TurnDropped): the
// call it left unfinished is the one it was cut off in, not work it's
// waiting on.
func AtTurnEnd(name string, t acp.Turn, check func() *Found) Pending {
	if _, dropped := TurnDropped(name, t); dropped {
		return Pending{}
	}
	own := PendingAtTurnEnd(name, t)
	if t.Err != nil || t.Stop != "end_turn" {
		return own
	}
	return withProcesses(name, own, check)
}

// withProcesses completes a harness's own answer with what the process
// check found. A command in one of its shells that its own answer lists
// (found by its command, without a pid yet) gives that piece its pid, so ws
// sees it end; one it doesn't list is work of its own, unless the harness
// lists them all. Work the agent detached is work nothing wakes it for:
// the harness doesn't know it's running.
func withProcesses(name string, own Pending, check func() *Found) Pending {
	r, ok := registry[name].(ProcessReader)
	if !ok || check == nil {
		return own
	}
	found := check()
	if found == nil {
		return own
	}
	wakes, listed := r.Shells(own)
	out := Pending{Known: true, Work: append([]Work(nil), own.Work...)}
	for _, f := range found.Running {
		if !f.Shell {
			out.Work = append(out.Work, Work{What: f.What, From: FromProcesses, PID: f.PID, Wakes: WakeNo})
			continue
		}
		if i := listedAs(out.Work, f); i >= 0 {
			out.Work[i].PID = f.PID
			continue
		}
		if !listed {
			out.Work = append(out.Work, Work{What: f.What, From: FromProcesses, PID: f.PID, Wakes: wakes})
		}
	}
	return out
}

// listedAs is the piece of a harness's list of work, without a pid yet,
// whose command a shell runs: -1 if none is.
func listedAs(ws []Work, f Running) int {
	for i, w := range ws {
		if w.From == FromHarness && w.PID == 0 && strings.TrimSpace(w.What) != "" && strings.Contains(f.text, unquote(w.What)) {
			return i
		}
	}
	return -1
}
