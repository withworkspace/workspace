// Package turn runs one headless turn of an agent: a prompt in, a reply
// out, with what it did along the way. It's the one place that's done: a
// command in its own process group under a timeout, or an agent spoken to
// over ACP, held in this process for the turn.
package turn

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/withworkspace/workspace/internal/execlog"
	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/spend"
)

// ErrEnded is a turn's error when what it ran for ended first: the daemon
// going, with it still at it. It's not the turn failing: whoever runs it
// leaves it to be run again by the next.
var ErrEnded = errors.New("ws stopped before it was finished")

// Spec is a turn to run.
type Spec struct {
	// Life is what the turn's run for (the daemon's life): ended, the
	// turn's agent is too, with everything it started. Nil is never.
	Life    context.Context
	Launch  harness.Launch
	Env     []string      // the agent's whole environment
	Timeout time.Duration // how long it may take
	// Tee is where what it prints is also written as it goes (a run file
	// a page follows); nil for nowhere. Stderr is where what it says on
	// stderr is also written (a log); nil for nowhere.
	Tee, Stderr io.Writer
	// Root and Who name the run in the exec log.
	Root, Who string
	// Harness is the agent's harness, by name.
	Harness string
	// For is who what it spends is kept for (spend.Keep): nothing's kept
	// without a Kind.
	For spend.For
}

// waitDelay is how long a command's output may stay open once it's gone.
const waitDelay = 5 * time.Second

// Run runs a turn, and reads what it said and did. An error is the turn
// failing: it didn't start, timed out, or exited badly; what it said on
// stderr comes with it.
func Run(s Spec) (harness.Report, error) {
	life := s.Life
	if life == nil {
		life = context.Background()
	}
	if life.Err() != nil {
		return harness.Report{}, ErrEnded
	}
	ctx, cancel := context.WithTimeout(life, s.Timeout)
	defer cancel()
	var out, errOut bytes.Buffer
	stdout, stderr := io.Writer(&out), io.Writer(&errOut)
	if s.Tee != nil {
		stdout = io.MultiWriter(&out, s.Tee)
	}
	if s.Stderr != nil {
		stderr = io.MultiWriter(&errOut, s.Stderr)
	}
	l := s.Launch
	start := time.Now()
	pid := 0
	var err error
	if l.ACP != nil && len(l.Args) == 0 {
		run := *l.ACP
		run.Base = s.Env
		execlog.Start(s.Root, s.Who, run.Agent, run.Dir, s.Env, 0)
		err = harness.RunACP(ctx, run, stdout, stderr)
	} else {
		c := exec.CommandContext(ctx, l.Args[0], l.Args[1:]...)
		c.Dir = l.Dir
		c.Env = s.Env
		// Its own process group, so a timeout kills everything it
		// started (a shell, a `gh pr checks --watch`), not just the
		// agent: anything left holding its output would keep it waiting.
		c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		c.Cancel = func() error { return syscall.Kill(-c.Process.Pid, syscall.SIGKILL) }
		c.WaitDelay = waitDelay
		c.Stdout, c.Stderr = stdout, stderr
		if err := c.Start(); err != nil {
			execlog.Start(s.Root, s.Who, c.Args, c.Dir, c.Env, 0)
			execlog.Note(s.Root, s.Who, "failed to start: %v", err)
			return harness.Report{}, err
		}
		pid = c.Process.Pid
		execlog.Start(s.Root, s.Who, c.Args, c.Dir, c.Env, pid)
		err = c.Wait()
	}
	switch {
	case life.Err() != nil:
		err = ErrEnded
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		err = fmt.Errorf("it took longer than %s", s.Timeout)
	}
	result := "exit 0"
	if err != nil {
		result = err.Error()
	}
	execlog.End(s.Root, s.Who, pid, time.Since(start), fmt.Sprintf("%s · %d bytes of output", result, out.Len()))
	rep := harness.ReadStream(l.Stream, out.String())
	if rep.Session == "" {
		rep.Session = l.Conversation // a session ws named for it
	}
	if s.For.Kind != "" && len(rep.Usage) > 0 {
		source := "run " + start.UTC().Format(time.RFC3339Nano) + " " + s.Who
		if err := spend.Keep(s.Root, source, s.For, s.Harness, start, rep.Usage); err != nil {
			execlog.Note(s.Root, s.Who, "couldn't keep what it spent: %v", err)
		}
	}
	if err != nil && err != ErrEnded {
		if e := clip(errOut.String(), 500); e != "" {
			err = fmt.Errorf("%w: %s", err, e)
		}
	}
	return rep, err
}

// Converse runs a turn of a conversation on the session it has (""
// for none), and when the harness says that session's gone, once more on
// a new one: turn is given the session to run on. Any other failure is
// the turn's, and isn't retried: a turn that timed out isn't run again
// from nothing.
func Converse(session string, turn func(session string) (harness.Report, error)) (harness.Report, error) {
	rep, err := turn(session)
	if session != "" && errors.Is(err, harness.ErrSessionGone) {
		return turn("")
	}
	return rep, err
}

func clip(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) > n {
		return "…" + s[len(s)-n:]
	}
	return s
}
