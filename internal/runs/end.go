package runs

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Stale is why a running run's time is up, or "" if it isn't: its owner
// has gone, the place it runs in has, or its time has run out. owners
// says after owners in ws's records: nil, and none of those has ended.
func Stale(r Run, procs Procs, owners Owners, now time.Time) string {
	if _, err := os.Stat(r.Dir); errors.Is(err, os.ErrNotExist) {
		return "where it ran has gone"
	}
	switch r.Owner.Kind {
	case ByPID:
		if procs.gone(r.Owner.PID, r.Owner.Began) {
			return fmt.Sprintf("its owner (pid %d%s) has gone", r.Owner.PID, said(r.Owner.Command))
		}
	case ByLooking:
		if now.Sub(r.Looked) >= LookLinger {
			return fmt.Sprintf("nobody has looked at it for %d minutes", int(LookLinger/time.Minute))
		}
	case ByMission, ByStep, ByCrew, ByConversation:
		if owners != nil {
			if why := owners.Ended(r.Owner); why != "" {
				return why
			}
		}
	}
	if !r.Expires.IsZero() && !now.Before(r.Expires) {
		return "its time was up"
	}
	return ""
}

func said(command string) string {
	if command == "" {
		return ""
	}
	return ", " + command
}

// End stops a run, if it's running, and removes it. Its process group is
// asked to end (SIGTERM), given 5 seconds, and then made to (SIGKILL).
// Then its recipe's stop command is run, where it ran, for up to
// StopLimit. Only a folder holding a run's record, under the store's, is
// removed. The run is removed even when its stop command fails: the
// error says so.
func (s Store) End(r Run) error {
	if !s.owns(r.Path) {
		return fmt.Errorf("%s isn't a run's folder", r.Path)
	}
	s.stop(r)
	stopErr := s.runStop(r)
	if err := os.RemoveAll(r.Path); err != nil {
		return err
	}
	// The worktree's and repo's folders go with their last run.
	parent := filepath.Dir(r.Path)
	if os.Remove(parent) == nil {
		os.Remove(filepath.Dir(parent))
	}
	return stopErr
}

// StopLimit is how long a recipe's stop command has.
var StopLimit = time.Minute

// runStop runs the run's stop command, if it has one, where it ran.
func (s Store) runStop(r Run) error {
	if strings.TrimSpace(r.Stop) == "" {
		return nil
	}
	if _, err := os.Stat(r.Dir); err != nil {
		return fmt.Errorf("its stop command (%s) wasn't run: where it ran has gone", r.Stop)
	}
	ctx, cancel := context.WithTimeout(context.Background(), StopLimit)
	defer cancel()
	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", r.Stop)
	cmd.Dir = r.Dir
	cmd.Env = append(withoutKey(os.Environ(), "PORT"), "PORT="+strconv.Itoa(r.Port))
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	out, err := cmd.CombinedOutput()
	if err != nil {
		tail := strings.TrimSpace(string(out))
		if i := strings.LastIndex(tail, "\n"); i >= 0 {
			tail = tail[i+1:]
		}
		return fmt.Errorf("its stop command (%s) failed: %v %s", r.Stop, err, tail)
	}
	return nil
}

func (s Store) owns(path string) bool {
	rel, err := filepath.Rel(s.Dir(), path)
	if err != nil || strings.HasPrefix(rel, "..") || len(strings.Split(rel, string(filepath.Separator))) != 3 {
		return false
	}
	_, err = os.Stat(filepath.Join(path, recordFile))
	return err == nil
}

// stopWait is how long a run's group is given to end once asked.
var stopWait = 5 * time.Second

func (s Store) stop(r Run) {
	if len(s.snapshot().group(r.PID, r.Began)) == 0 {
		return
	}
	syscall.Kill(-r.PID, syscall.SIGTERM)
	for deadline := time.Now().Add(stopWait); time.Now().Before(deadline); time.Sleep(100 * time.Millisecond) {
		if len(s.snapshot().group(r.PID, r.Began)) == 0 {
			return
		}
	}
	syscall.Kill(-r.PID, syscall.SIGKILL)
}

// Swept is a run a sweep ended, and why.
type Swept struct {
	Run Run
	Why string
}

// Sweep ends every running run whose time is up, and removes those that
// ended by themselves more than KeepEnded ago.
func (s Store) Sweep(owners Owners, now time.Time) ([]Swept, error) {
	runs, err := s.List()
	if err != nil || len(runs) == 0 {
		return nil, err
	}
	procs := s.snapshot()
	var out []Swept
	var errs []error
	for _, r := range runs {
		why := ""
		if r.State == Running {
			why = Stale(r, procs, owners, now)
		} else if now.Sub(r.Ended) >= KeepEnded {
			why = "it ended " + r.Ended.Local().Format("15:04")
		}
		if why == "" {
			continue
		}
		err := s.End(r)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", r.Ref(), err))
		}
		if _, serr := os.Stat(r.Path); errors.Is(serr, os.ErrNotExist) {
			out = append(out, Swept{Run: r, Why: why})
		}
	}
	return out, errors.Join(errs...)
}

// Ends says when a run will be ended without being asked, as in "Ended
// when pid 4242 (zsh) exits, or in 3h58".
func Ends(r Run, now time.Time) string {
	var what string
	switch r.Owner.Kind {
	case ByPID:
		if r.Owner.Command != "" {
			what = fmt.Sprintf("when pid %d (%s) exits", r.Owner.PID, r.Owner.Command)
		} else {
			what = fmt.Sprintf("when pid %d exits", r.Owner.PID)
		}
	case ByCapsule:
		what = fmt.Sprintf("when capsule %s is burned or landed", r.Worktree)
	case ByMission:
		what = fmt.Sprintf("when mission %s ends", r.Owner.Mission)
	case ByStep:
		what = fmt.Sprintf("when step %d of mission %s passes", r.Owner.Step, r.Owner.Mission)
	case ByCrew:
		what = fmt.Sprintf("when crew %s of mission %s close", r.Owner.Crew, r.Owner.Mission)
	case ByConversation:
		what = fmt.Sprintf("when conversation %s is resolved", r.Owner.Conversation)
	case ByLooking:
		what = fmt.Sprintf("%d minutes after nobody has mission %s open", int(LookLinger/time.Minute), r.Owner.Mission)
	}
	if !r.Expires.IsZero() {
		left := r.Expires.Sub(now)
		if left < 0 {
			left = 0
		}
		in := "in " + Hours(left)
		if what == "" {
			what = in
		} else {
			what += ", or " + in
		}
	}
	if what == "" {
		return "only when it's stopped"
	}
	return what
}

// Hours is d as 3h58: hours and minutes, rounded up to the minute.
func Hours(d time.Duration) string {
	m := int((d + time.Minute - 1) / time.Minute)
	return fmt.Sprintf("%dh%02d", m/60, m%60)
}
