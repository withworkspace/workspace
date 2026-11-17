// Package git runs git: the one place ws starts it. Most callers want one
// of the shorthands; Cmd is for the rest (an environment, input, a
// context).
package git

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
)

// Error is git failing: the subcommand, and what it said on stderr. It
// unwraps to the *exec.ExitError, for its exit code.
type Error struct {
	Args   []string
	Stderr string
	Err    error
}

func (e *Error) Error() string {
	sub := "git"
	if len(e.Args) > 0 {
		sub += " " + e.Args[0]
	}
	if e.Stderr != "" {
		return sub + ": " + e.Stderr
	}
	return sub + ": " + e.Err.Error()
}

func (e *Error) Unwrap() error { return e.Err }

// ExitCode is the code git exited with, or -1 when err isn't git exiting.
func ExitCode(err error) int {
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode()
	}
	return -1
}

// Cmd is one run of git.
type Cmd struct {
	Ctx  context.Context // nil for none
	Dir  string          // "" for the current directory
	Args []string
	// ReadOnly runs it without the optional locks (the index's, which
	// crew's own git commands need while ws reads their capsule), and
	// with paths unquoted.
	ReadOnly bool
	Env      []string // added to ws's own environment
	Stdin    io.Reader
}

// started counts the git processes this process has started: what a
// view costs is measured by it (Started).
var started atomic.Int64

// Started is how many git processes this process has started.
func Started() int64 { return started.Load() }

func (c Cmd) command() *exec.Cmd {
	started.Add(1)
	var args []string
	if c.ReadOnly {
		args = append(args, "--no-optional-locks", "-c", "core.quotePath=false")
	}
	args = append(args, c.Args...)
	ctx := c.Ctx
	if ctx == nil {
		ctx = context.Background()
	}
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = c.Dir
	if len(c.Env) > 0 {
		cmd.Env = append(os.Environ(), c.Env...)
	}
	cmd.Stdin = c.Stdin
	return cmd
}

// Command is it as a process not yet started, for one that's talked to
// as it runs (cat-file --batch).
func (c Cmd) Command() *exec.Cmd { return c.command() }

// Output runs it and returns what it printed on stdout, which is kept even
// when it fails (some commands say how in their exit code, and print all
// the same). A failure is an *Error.
func (c Cmd) Output() ([]byte, error) {
	cmd := c.command()
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return out, &Error{Args: c.Args, Stderr: strings.TrimSpace(stderr.String()), Err: err}
	}
	return out, nil
}

// Combined runs it and returns stdout and stderr together, as a person
// would see them; a failure is git's own error, for the caller to put the
// output with.
func (c Cmd) Combined() (string, error) {
	out, err := c.command().CombinedOutput()
	return string(out), err
}

// Output is what git prints in dir.
func Output(dir string, args ...string) (string, error) {
	out, err := Cmd{Dir: dir, Args: args}.Output()
	return string(out), err
}

// Line is what git prints in dir, trimmed: a commit, a branch, a path.
func Line(dir string, args ...string) (string, error) {
	out, err := Output(dir, args...)
	return strings.TrimSpace(out), err
}

// Lines is what git prints in dir, a line each, without empty ones; none
// if it fails.
func Lines(dir string, args ...string) []string {
	out, err := Output(dir, args...)
	if err != nil {
		return nil
	}
	var lines []string
	for _, l := range strings.Split(out, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			lines = append(lines, l)
		}
	}
	return lines
}

// Ok is whether git succeeds in dir: a check (show-ref --verify,
// check-ignore, merge-base --is-ancestor).
func Ok(dir string, args ...string) bool {
	_, err := Cmd{Dir: dir, Args: args}.Output()
	return err == nil
}

// Read is Output for reading a worktree others are working in: see
// Cmd.ReadOnly.
func Read(ctx context.Context, dir string, args ...string) ([]byte, error) {
	return Cmd{Ctx: ctx, Dir: dir, Args: args, ReadOnly: true}.Output()
}

// ReadLine is Read, trimmed.
func ReadLine(ctx context.Context, dir string, args ...string) (string, error) {
	out, err := Read(ctx, dir, args...)
	return strings.TrimSpace(string(out)), err
}

// Run runs git in dir for what it does, not what it prints. A failure
// carries what git said.
func Run(dir string, args ...string) error {
	out, err := Cmd{Dir: dir, Args: args}.Combined()
	if err != nil {
		if msg := strings.TrimSpace(out); msg != "" {
			return &Error{Args: args, Stderr: msg, Err: err}
		}
		return &Error{Args: args, Err: err}
	}
	return nil
}

// Combined is what git prints in dir, stdout and stderr together, as a
// person would see them.
func Combined(dir string, args ...string) (string, error) {
	return Cmd{Dir: dir, Args: args}.Combined()
}
