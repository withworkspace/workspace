package harness

import (
	"bytes"
	"errors"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// clockTicks is the kernel's USER_HZ, what /proc/<pid>/stat counts a
// start in: 100 on every architecture Go builds ws for.
const clockTicks = 100

// ReadProcs reads the user's processes from /proc: each one's parent and
// start from /proc/<pid>/stat, and the arguments and environment of those
// the check looks at from /proc/<pid>/cmdline and /proc/<pid>/environ,
// which the same user can read.
//
// VERIFIED (Alpine 3.20 in a container, arm64): a user's detached
// /bin/sleep has its environment in /proc/<pid>/environ, unlike on macOS.
// An orphan is re-parented to pid 1, or to a subreaper (systemd --user, a
// container's init): the check doesn't look at which.
func ReadProcs() (Procs, error) {
	boot, err := bootTime()
	if err != nil {
		return Procs{}, err
	}
	ents, err := os.ReadDir("/proc")
	if err != nil {
		return Procs{}, err
	}
	uid := uint32(os.Getuid())
	var list []Proc
	for _, e := range ents {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if st, ok := info.Sys().(*syscall.Stat_t); !ok || st.Uid != uid {
			continue
		}
		data, err := os.ReadFile("/proc/" + e.Name() + "/stat")
		if err != nil {
			continue
		}
		p, ok := parseProcStat(data, boot)
		if !ok || p.PID != pid {
			continue
		}
		list = append(list, p)
	}
	return Procs{List: list, read: readProcFiles}, nil
}

// parseProcStat reads /proc/<pid>/stat: "pid (comm) state ppid pgrp …",
// the start (field 22) in clock ticks after boot. A process that has exited
// and not been reaped (Z, X) isn't one.
func parseProcStat(data []byte, boot time.Time) (Proc, bool) {
	open, end := bytes.IndexByte(data, '('), bytes.LastIndexByte(data, ')')
	if open < 0 || end < open || end+2 > len(data) {
		return Proc{}, false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data[:open])))
	if err != nil {
		return Proc{}, false
	}
	f := strings.Fields(string(data[end+2:])) // from field 3, the state
	if len(f) < 20 || f[0] == "Z" || f[0] == "X" {
		return Proc{}, false
	}
	ppid, err1 := strconv.Atoi(f[1])
	pgid, err2 := strconv.Atoi(f[2])
	ticks, err3 := strconv.ParseInt(f[19], 10, 64)
	if err1 != nil || err2 != nil || err3 != nil {
		return Proc{}, false
	}
	return Proc{PID: pid, PPID: ppid, PGID: pgid, Start: boot.Add(time.Duration(ticks) * time.Second / clockTicks), Name: string(data[open+1 : end])}, true
}

// bootTime is when the system booted: /proc/stat's btime.
func bootTime() (time.Time, error) {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return time.Time{}, err
	}
	for _, line := range strings.Split(string(data), "\n") {
		if v, ok := strings.CutPrefix(line, "btime "); ok {
			s, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
			if err != nil {
				return time.Time{}, err
			}
			return time.Unix(s, 0), nil
		}
	}
	return time.Time{}, errors.New("/proc/stat has no btime")
}

// readProcFiles reads a process's arguments and environment: nothing when
// it's gone, or isn't the user's.
func readProcFiles(p *Proc) {
	dir := "/proc/" + strconv.Itoa(p.PID) + "/"
	if data, err := os.ReadFile(dir + "cmdline"); err == nil {
		p.Args = nulStrings(data)
	}
	if data, err := os.ReadFile(dir + "environ"); err == nil {
		p.Env = nulStrings(data)
	}
}

// nulStrings splits NUL-terminated strings.
func nulStrings(data []byte) []string {
	out := []string{}
	for _, s := range bytes.Split(bytes.TrimRight(data, "\x00"), []byte{0}) {
		if len(s) > 0 {
			out = append(out, string(s))
		}
	}
	return out
}
