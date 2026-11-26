package ops

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/withworkspace/workspace/internal/execlog"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/prompt"
	"github.com/withworkspace/workspace/internal/runs"
)

// What's running while ws checks a step: each check that's a command, and
// then the judge, kept on the mission as it begins (mission.Checking) so
// the headline says it, and its start, end and result logged to exec.log
// and the mission's log, where the step's Lately list finds them.

// checkLineEvery is how often the last line a check has printed is kept,
// at most: each is a write of the mission's record.
var checkLineEvery = 2 * time.Second

// checkRound is a round of step st's checks, crew's done, as ws runs it.
type checkRound struct {
	w      *Workspace
	slug   string
	st     mission.StageState
	crew   string
	judged []int // the checks the judge judges, from 1
}

func (w *Workspace) checkRound(slug string, st mission.StageState, crewID string, judged []int) *checkRound {
	ns := make([]int, len(judged))
	for k, i := range judged {
		ns[k] = i + 1
	}
	return &checkRound{w: w, slug: slug, st: st, crew: crewID, judged: ns}
}

// log notes what happened to the step's checks on the mission's log.
func (r *checkRound) log(format string, a ...any) {
	r.w.Missions.AppendLogAfter(r.slug, mission.Event{Kind: mission.EventChecks, Crew: "ws", Text: fmt.Sprintf(format, a...)})
}

// command runs check i (from 0), a command, in dir: what's running is
// kept on the mission, with the last line it prints as it goes, and its
// start and end logged.
func (r *checkRound) command(i int, c mission.Check, capsule, dir string, env []string) mission.CheckResult {
	n, of := i+1, len(r.st.Step.Checks)
	since := time.Now()
	r.w.Missions.SetChecking(r.slug, mission.Checking{Step: r.st.N, Crew: r.crew, Of: of, N: n, Command: c.Command,
		Judged: r.judged, Since: since, Until: since.Add(checkTimeout)})
	r.log("step %d, check %d of %d started: `%s`", r.st.N, n, of, c.Command)
	who := fmt.Sprintf("check %s step %d, %d of %d", r.slug, r.st.N, n, of)
	pid := 0
	res := r.w.runCheckTask(capsule, dir, c, env, runs.CheckOf{Mission: r.slug, Step: r.st.N, N: n}, checkTimeout, checkHooks{
		Started: func(p int) {
			pid = p
			execlog.Start(r.w.Root(), who, []string{"sh", "-c", c.Command}, dir, env, p)
		},
		Line: func(line string) { r.w.Missions.CheckingLine(r.slug, since, line) },
	})
	ended := checkEnded(res)
	if pid == 0 {
		execlog.Start(r.w.Root(), who, []string{"sh", "-c", c.Command}, dir, env, 0)
	}
	execlog.End(r.w.Root(), who, pid, time.Since(since), ended)
	r.log("step %d, check %d of %d: `%s` %s", r.st.N, n, of, c.Command, ended)
	return res
}

// metBefore is check i (from 0), a command ws ran in an earlier round
// (before, with what it came to there) on the files the capsule's at now:
// met as it was, logged, and not run again. It keeps that round's run.
func (r *checkRound) metBefore(i int, c mission.Check, before mission.CheckRound, was mission.CheckResult) mission.CheckResult {
	n, of := i+1, len(r.st.Step.Checks)
	at := ""
	if before.Commit != "" {
		at = " at " + shortCommit(before.Commit)
	}
	r.log("step %d, check %d of %d: `%s` was met in round %d%s, and the files are the same now: not run again", r.st.N, n, of, c.Command, before.N, at)
	return mission.CheckResult{Check: c.Text, Result: mission.CheckMet, By: "ws", Run: was.Run,
		Evidence: fmt.Sprintf("met in round %d%s, and the capsule's files are the same now, all committed: not run again", before.N, at)}
}

// checkEnded is how a check that's a command ended, in a line: "passed in
// 4s: met".
func checkEnded(r mission.CheckResult) string {
	return firstLine(r.Evidence) + ": " + r.Result
}

// judging keeps that the judge (who) has started reading its checks, and
// logs it. It returns when.
func (r *checkRound) judging(who string) time.Time {
	since := time.Now()
	r.w.Missions.SetChecking(r.slug, mission.Checking{Step: r.st.N, Crew: r.crew, Of: len(r.st.Step.Checks), Judged: r.judged,
		Judge: who, Since: since, Until: since.Add(judgeTimeout)})
	r.log("step %d: %s started reading %s", r.st.N, who, mission.CheckNumbers(r.judged))
	return since
}

// judgedThem logs what the judge came to, or why it couldn't be asked.
func (r *checkRound) judgedThem(since time.Time, who string, verdicts []prompt.Verdict, err error) {
	took := ""
	if !since.IsZero() {
		took = " in " + mission.Elapsed(time.Since(since))
	}
	if err != nil {
		r.log("step %d: the judge couldn't judge %s%s: %v", r.st.N, mission.CheckNumbers(r.judged), took, err)
		return
	}
	var said []string
	for k, v := range verdicts {
		if k < len(r.judged) {
			said = append(said, fmt.Sprintf("check %d %s", r.judged[k], v.Result))
		}
	}
	text := fmt.Sprintf("step %d: %s finished%s: %s", r.st.N, who, took, strings.Join(said, ", "))
	execlog.Note(r.w.Root(), "judge "+r.slug, "%s", text)
	r.log("%s", text)
}

// checkHooks follow a check's command as it runs: Started with its pid
// once it has, and Line with the last line it has printed, as that
// changes (every checkLineEvery at most).
type checkHooks struct {
	Started func(pid int)
	Line    func(line string)
}

// lastLine is a command's output: all of it, kept as it always was, and
// the last line it has printed so far.
type lastLine struct {
	mu      sync.Mutex
	out     bytes.Buffer
	partial []byte // the line it's printing, not ended yet
	last    string // the last whole line that wasn't blank
}

// lineMax is how much of a line is kept as the last.
const lineMax = 240

func (l *lastLine) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.out.Write(p)
	data := append(l.partial, p...)
	if cut := bytes.LastIndexAny(data, "\n\r"); cut >= 0 {
		for _, line := range bytes.FieldsFunc(data[:cut], func(r rune) bool { return r == '\n' || r == '\r' }) {
			if s := cleanLine(line); s != "" {
				l.last = s
			}
		}
		data = data[cut+1:]
	}
	if len(data) > 4*lineMax {
		data = data[len(data)-4*lineMax:]
	}
	l.partial = append([]byte(nil), data...)
	return len(p), nil
}

// Line is the last line printed so far: the one it's printing, if it
// has started one.
func (l *lastLine) Line() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	if s := cleanLine(l.partial); s != "" {
		return s
	}
	return l.last
}

// String is all it printed.
func (l *lastLine) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.out.String()
}

// ansi is a terminal's escape: colours, the cursor.
var ansi = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)

// cleanLine is a line as the headline says it: no colours, no control
// characters, trimmed and clipped.
func cleanLine(b []byte) string {
	s := ansi.ReplaceAllString(string(b), "")
	s = strings.Map(func(r rune) rune {
		if r == '\t' {
			return ' '
		}
		if r < ' ' || r == 0x7f {
			return -1
		}
		return r
	}, s)
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > lineMax {
		s = string(r[:lineMax-1]) + "…"
	}
	return s
}

// follow calls line with out's last line every checkLineEvery while it
// changes, until the returned stop is called.
func (l *lastLine) follow(line func(string)) (stop func()) {
	if line == nil {
		return func() {}
	}
	done, finished := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(finished)
		t := time.NewTicker(checkLineEvery)
		defer t.Stop()
		said := ""
		for {
			select {
			case <-done:
				return
			case <-t.C:
				if now := l.Line(); now != said {
					said = now
					line(now)
				}
			}
		}
	}()
	return func() {
		close(done)
		<-finished
	}
}
