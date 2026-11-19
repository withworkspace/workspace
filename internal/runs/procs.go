package runs

import (
	"syscall"
	"time"
	"unicode"

	"github.com/withworkspace/workspace/internal/harness"
)

// Procs is the user's processes at one moment, as runs ask after them.
// Known is false when they can't be read (in a sandbox, or on a system ws
// can't read them on): then a run's owner hasn't gone by them, and its
// process is asked after by its pid alone.
type Procs struct {
	Known bool
	ByPID map[int]harness.Proc
}

// ReadProcs reads the user's processes.
func ReadProcs() (Procs, error) {
	ps, err := harness.ReadProcs()
	if err != nil {
		return Procs{}, err
	}
	return ProcsOf(ps.List), nil
}

// ProcsOf is a Procs of these processes.
func ProcsOf(list []harness.Proc) Procs {
	by := make(map[int]harness.Proc, len(list))
	for _, p := range list {
		by[p.PID] = p
	}
	return Procs{Known: true, ByPID: by}
}

// sameStart is whether two starts are one process's: the kernel's, read
// at two times, to the second.
func sameStart(a, b time.Time) bool {
	d := a.Sub(b)
	return d < time.Second && d > -time.Second
}

// started is when pid started, if it's running and that can be read.
func (p Procs) started(pid int) (time.Time, bool) {
	if !p.Known {
		return time.Time{}, false
	}
	pr, ok := p.ByPID[pid]
	return pr.Start, ok
}

// gone reports whether the process pid, which began then, has exited: no
// process has its pid, or one that began at another time has. When the
// processes can't be read, nothing has gone.
func (p Procs) gone(pid int, began time.Time) bool {
	if !p.Known || pid <= 0 {
		return false
	}
	pr, ok := p.ByPID[pid]
	if !ok {
		return true
	}
	return !began.IsZero() && !sameStart(pr.Start, began)
}

// group is the processes still running in the process group a run's
// leader (pid, which began then) made: those in it that began no earlier
// than it did, so a group id given out again isn't taken for it. When
// the processes can't be read, it's the leader, if its pid is there.
func (p Procs) group(pid int, began time.Time) []int {
	if pid <= 0 {
		return nil
	}
	if !p.Known {
		if syscall.Kill(pid, 0) == nil {
			return []int{pid}
		}
		return nil
	}
	var out []int
	for _, pr := range p.ByPID {
		if pr.PGID == pid && (began.IsZero() || !pr.Start.Before(began.Add(-time.Second))) {
			out = append(out, pr.PID)
		}
	}
	return out
}

// FindOwner is the process a run is for when nobody says: the nearest
// process above from (the caller's parent) that has been running for
// OwnerAge or more. None (a zero Owner) when there's no such process or
// the processes can't be read.
func FindOwner(p Procs, from int, now time.Time) Owner {
	if !p.Known {
		return Owner{}
	}
	seen := map[int]bool{}
	for pid := from; pid > 1 && !seen[pid]; {
		seen[pid] = true
		pr, ok := p.ByPID[pid]
		if !ok {
			break
		}
		if now.Sub(pr.Start) >= OwnerAge {
			return Owner{Kind: ByPID, PID: pr.PID, Began: pr.Start, Command: nameOf(pr.Name)}
		}
		pid = pr.PPID
	}
	return Owner{}
}

// OwnerOf is the process pid as a run's owner.
func OwnerOf(p Procs, pid int) (Owner, bool) {
	if !p.Known {
		if syscall.Kill(pid, 0) != nil {
			return Owner{}, false
		}
		return Owner{Kind: ByPID, PID: pid}, true
	}
	pr, ok := p.ByPID[pid]
	if !ok {
		return Owner{}, false
	}
	return Owner{Kind: ByPID, PID: pid, Began: pr.Start, Command: nameOf(pr.Name)}, true
}

// nameOf is a process's name as it's worth saying: none when it's no
// word at all (Claude Code names its process for its version, 2.1.328).
func nameOf(comm string) string {
	for _, c := range comm {
		if unicode.IsLetter(c) {
			return comm
		}
	}
	return ""
}
