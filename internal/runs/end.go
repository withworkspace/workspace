package runs

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// placeGone is why a run whose folder has gone is ended: nothing that
// needs it keeps it then.
const placeGone = "where it ran has gone"

// Stale is why a running run's time is up, or "" if it isn't: everyone
// it's for has gone, the place it runs in has, or its time has run out.
// owners says after owners in ws's records: nil, and none of those has
// ended. A run that a running run needs (r.NeededBy, as List finds it)
// isn't stale while its place is there.
func Stale(r Run, procs Procs, owners Owners, now time.Time) string {
	why := staleAlone(r, procs, owners, now)
	if why != placeGone && len(r.NeededBy) > 0 {
		return ""
	}
	return why
}

// staleAlone is why r would be ended were nothing to need it.
func staleAlone(r Run, procs Procs, owners Owners, now time.Time) string {
	if _, err := os.Stat(r.Dir); errors.Is(err, os.ErrNotExist) {
		return placeGone
	}
	for _, n := range r.Needs {
		if _, err := os.Stat(n.Dir); n.IsFolder() && errors.Is(err, os.ErrNotExist) {
			return fmt.Sprintf("%s, the folder it was started on for %s, has gone", Label(n.Worktree), n.Name)
		}
	}
	var gone []string
	for _, o := range r.Owners() {
		why := ownerGone(r, o, procs, owners, now)
		if why == "" {
			gone = nil
			break
		}
		gone = append(gone, why)
	}
	if len(gone) > 0 {
		return strings.Join(gone, ", and ")
	}
	if !r.Expires.IsZero() && !now.Before(r.Expires) {
		return "its time was up"
	}
	return ""
}

// ownerGone is why o, one of r's owners, has gone, or "" while it hasn't.
func ownerGone(r Run, o Owner, procs Procs, owners Owners, now time.Time) string {
	switch o.Kind {
	case ByPID:
		if procs.gone(o.PID, o.Began) {
			return fmt.Sprintf("its owner (pid %d%s) has gone", o.PID, said(o.Command))
		}
	case ByLooking:
		if now.Sub(r.Looked) >= LookLinger {
			return fmt.Sprintf("nobody has looked at it for %d minutes", int(LookLinger/time.Minute))
		}
	case ByMission, ByStep, ByCrew, ByConversation:
		if owners != nil {
			return owners.Ended(o)
		}
	case ByRun:
		// What needs it is the sweep's to say (Sweep): alone, it's needed
		// only while the run it was started for is still being started.
		if !now.Before(r.Hold) {
			return "nothing needs it now"
		}
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
// ended by themselves more than KeepEnded ago. A run isn't ended while a
// run that's staying needs it, whoever it's for and whatever its time:
// the runs that need it are counted afresh each sweep, from their
// records, so nothing's kept that could be left wrong by a ws that
// stopped half way.
func (s Store) Sweep(owners Owners, now time.Time) ([]Swept, error) {
	runs, err := s.List()
	if err != nil || len(runs) == 0 {
		return nil, err
	}
	procs := s.snapshot()
	why := map[string]string{}
	byRef := map[string]Run{}
	var stay []string
	for _, r := range runs {
		if r.State != Running {
			continue
		}
		byRef[r.Ref()] = r
		if why[r.Ref()] = staleAlone(r, procs, owners, now); why[r.Ref()] == "" {
			stay = append(stay, r.Ref())
		}
	}
	// What's staying keeps what it needs, and that keeps what it needs.
	for len(stay) > 0 {
		ref := stay[0]
		stay = stay[1:]
		for _, n := range byRef[ref].Needs {
			if w, ok := why[n.Run]; ok && w != "" && w != placeGone {
				why[n.Run] = ""
				stay = append(stay, n.Run)
			}
		}
	}
	var out []Swept
	var errs []error
	for _, r := range runs {
		w := ""
		if r.State == Running {
			w = why[r.Ref()]
		} else if now.Sub(r.Ended) >= KeepEnded {
			w = "it ended " + r.Ended.Local().Format("15:04")
		}
		if w == "" {
			continue
		}
		err := s.End(r)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", r.Ref(), err))
		}
		if _, serr := os.Stat(r.Path); errors.Is(serr, os.ErrNotExist) {
			out = append(out, Swept{Run: r, Why: w})
		}
	}
	return out, errors.Join(errs...)
}

// Ends says when a run will be ended without being asked, as in "when
// pid 4242 (zsh) exits, or in 3h58". With several owners, it's when the
// last of them has gone; and one a running run needs isn't ended while
// that needs it.
func Ends(r Run, now time.Time) string {
	var whens []string
	for _, o := range r.Owners() {
		if w := endsWith(r, o); w != "" && !slices.Contains(whens, w) {
			whens = append(whens, w)
		}
	}
	what := strings.Join(whens, ", and ")
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
	needed := ""
	if len(r.NeededBy) > 0 {
		needed = strings.Join(r.NeededBy, " and ")
	}
	byRun := slices.ContainsFunc(r.Owners(), func(o Owner) bool { return o.Kind == ByRun })
	switch {
	case needed != "" && what == "":
		return "when " + needed + " " + has(len(r.NeededBy)) + " ended"
	case needed != "":
		return what + ", and not while " + needed + " " + need(len(r.NeededBy)) + " it"
	case what == "" && byRun:
		return "within seconds, as nothing needs it now"
	case what == "":
		return "only when it's stopped"
	}
	return what
}

func has(n int) string {
	if n == 1 {
		return "has"
	}
	return "have"
}

func need(n int) string {
	if n == 1 {
		return "needs"
	}
	return "need"
}

// endsWith is when o, one of r's owners, goes, in words that follow
// "ends": "" for an owner that says nothing of when (none, or the runs
// that need it, which Ends names).
func endsWith(r Run, o Owner) string {
	switch o.Kind {
	case ByPID:
		if o.Command != "" {
			return fmt.Sprintf("when pid %d (%s) exits", o.PID, o.Command)
		}
		return fmt.Sprintf("when pid %d exits", o.PID)
	case ByCapsule:
		return fmt.Sprintf("when capsule %s is burned or landed", r.Worktree)
	case ByMission:
		return fmt.Sprintf("when mission %s ends", o.Mission)
	case ByStep:
		return fmt.Sprintf("when step %d of mission %s passes", o.Step, o.Mission)
	case ByCrew:
		return fmt.Sprintf("when crew %s of mission %s close", o.Crew, o.Mission)
	case ByConversation:
		return fmt.Sprintf("when conversation %s is resolved", o.Conversation)
	case ByLooking:
		if o.App != "" {
			return fmt.Sprintf("%d minutes after nobody has %s open", int(LookLinger/time.Minute), o.App)
		}
		return fmt.Sprintf("%d minutes after nobody has mission %s open", int(LookLinger/time.Minute), o.Mission)
	}
	return ""
}

// Hours is d as 3h58: hours and minutes, rounded up to the minute.
func Hours(d time.Duration) string {
	m := int((d + time.Minute - 1) / time.Minute)
	return fmt.Sprintf("%dh%02d", m/60, m%60)
}
