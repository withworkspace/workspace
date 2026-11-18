package daemon

import (
	"errors"
	"os"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/release"
)

// A daemon keeps the ws it started from: installing another over it (an
// upgrade, a rebuild) changes the commands run from then on, crew's among
// them, and not the daemon they talk to. So a server that knows its own
// binary (Exe) watches it, and once it's been replaced, restarts onto the
// new one when nothing's at work: no crew mid-turn, no agent's run. What
// it hangs up then (crew waiting for you) the next daemon reopens as it
// was. `ws daemon restart` asks for the same, and with --now doesn't wait.
//
// The server only shuts down, saying it's to be started again
// (Restarting): whoever ran Serve does that, once what it holds is closed.

// exeTick is how often a server looks at its binary and, while a restart's
// wanted, at what's at work.
var exeTick = 5 * time.Second

// quietTicks is how many looks in a row must find nothing at work before a
// restart nobody said was to be now: what follows the end of a turn (the
// orchestrator's run, a check) starts a moment after it.
const quietTicks = 2

// AtWork is what a restart would stop mid-flight, each as it's said: crew
// mid-turn, an agent's run.
func (s *Server) AtWork() []string {
	var out []string
	if s.CrewAtWork != nil {
		out = s.CrewAtWork()
	}
	if s.Jobs != nil && s.Jobs.Busy() {
		out = append(out, "an agent's run (the orchestrator's, a check's or a conversation's answer)")
	}
	return out
}

// Restart shuts the server down to be started again on the ws now at Exe.
// With now it does so at once, hanging up what's at work. Without, it
// waits until nothing is, and says what it's waiting for: none means it's
// restarting.
func (s *Server) Restart(now bool) (waits []string, err error) {
	if s.Exe == "" {
		return nil, errors.New("this daemon wasn't started as one that restarts itself")
	}
	if !now {
		if waits = s.AtWork(); len(waits) > 0 {
			s.mu.Lock()
			s.restartWanted = true
			s.mu.Unlock()
			s.Log.Printf("to restart once nothing's at work: waiting for %s", strings.Join(waits, ", "))
			return waits, nil
		}
	}
	s.mu.Lock()
	if s.closing {
		s.mu.Unlock()
		return nil, errors.New("the daemon is shutting down")
	}
	s.restarting = true
	s.mu.Unlock()
	go s.Shutdown("restarting")
	return nil, nil
}

// Restarting is whether the server shut down to be started again: asked
// once Serve has returned.
func (s *Server) Restarting() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.restarting
}

// watchExe restarts the server once its binary has been replaced, or a
// restart's been asked for, and nothing's at work. was is its binary as
// the server started.
func (s *Server) watchExe(was os.FileInfo, every time.Duration) {
	// seen is a replacement as it was last look: one that's the same a
	// look later is all there, not half written.
	var seen os.FileInfo
	quiet := 0
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-s.done:
			return
		case <-t.C:
		}
		s.mu.Lock()
		wanted := s.restartWanted
		s.mu.Unlock()
		if !wanted {
			now, err := os.Stat(s.Exe)
			switch {
			case err != nil || sameBinary(was, now):
				seen = nil
				continue
			case seen == nil || !sameBinary(seen, now):
				seen = now
				continue
			}
			version, err := release.VersionOf(s.Exe)
			if err != nil {
				// Stay on the ws that runs, and look again when this one's
				// replaced in its turn.
				s.Log.Printf("%s has been replaced by a ws that doesn't run (%v): staying on %s", s.Exe, err, s.Build)
				was, seen = now, nil
				continue
			}
			s.mu.Lock()
			s.restartWanted, s.next = true, version
			s.mu.Unlock()
			s.Log.Printf("%s has been replaced (ws %s, this is %s): restarting onto it once nothing's at work", s.Exe, version, s.Build)
		}
		if len(s.AtWork()) > 0 {
			quiet = 0
			continue
		}
		if quiet++; quiet < quietTicks {
			continue
		}
		if _, err := s.Restart(true); err != nil {
			return
		}
	}
}

// sameBinary is whether two looks at a path found the same file,
// unchanged. An install renames a new file over the old (another file); a
// build may write in place (another time, or size).
func sameBinary(a, b os.FileInfo) bool {
	return os.SameFile(a, b) && a.ModTime().Equal(b.ModTime()) && a.Size() == b.Size()
}
