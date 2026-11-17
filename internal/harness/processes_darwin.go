package harness

import (
	"bytes"
	"encoding/binary"
	"os"
	"time"

	"golang.org/x/sys/unix"
)

// szomb is a process that has exited and not been reaped (sys/proc.h).
const szomb = 5

// ReadProcs reads the user's processes: kern.proc.all for each one's pid,
// parent and start, and kern.procargs2 (KERN_PROCARGS2) for the arguments
// and environment of those the check looks at.
//
// VERIFIED (macOS 26.5): another process of the same user can read a
// process's environment this way, but macOS hides it for its own
// binaries: /bin/sh and /bin/sleep come back with none, node, python3 and
// a Go binary with theirs. Their arguments it doesn't hide. A process
// detached from a shell that has exited is re-parented to launchd, and
// stays in the shell's process group; OrbStack's helper, which its CLI
// leaves running, leads a group of its own.
func ReadProcs() (Procs, error) {
	kps, err := unix.SysctlKinfoProcSlice("kern.proc.all")
	if err != nil {
		return Procs{}, err
	}
	uid := uint32(os.Getuid())
	var list []Proc
	for _, kp := range kps {
		if kp.Eproc.Ucred.Uid != uid || kp.Proc.P_stat == szomb {
			continue
		}
		st := kp.Proc.P_starttime
		list = append(list, Proc{
			PID:   int(kp.Proc.P_pid),
			PPID:  int(kp.Eproc.Ppid),
			PGID:  int(kp.Eproc.Pgid),
			Start: time.Unix(st.Sec, int64(st.Usec)*int64(time.Microsecond)),
			Name:  unix.ByteSliceToString(kp.Proc.P_comm[:]),
		})
	}
	return Procs{List: list, read: readProcArgs}, nil
}

// readProcArgs reads a process's arguments and environment: nothing when
// it's gone, or isn't the user's.
func readProcArgs(p *Proc) {
	buf, err := unix.SysctlRaw("kern.procargs2", p.PID)
	if err != nil {
		return
	}
	p.Args, p.Env = parseProcArgs2(buf)
}

// parseProcArgs2 reads KERN_PROCARGS2's layout: argc (an int32), the
// executable's path, NULs to align, then argc arguments and the
// environment, each NUL-terminated, the environment ending at an empty
// string (what follows it is the system's).
func parseProcArgs2(buf []byte) (args, env []string) {
	if len(buf) < 4 {
		return nil, nil
	}
	argc := int(binary.LittleEndian.Uint32(buf))
	rest := buf[4:]
	if i := bytes.IndexByte(rest, 0); i >= 0 {
		rest = rest[i:]
	} else {
		return nil, nil
	}
	for len(rest) > 0 && rest[0] == 0 {
		rest = rest[1:]
	}
	next := func() (string, bool) {
		i := bytes.IndexByte(rest, 0)
		if i < 0 {
			s := string(rest)
			rest = nil
			return s, s != ""
		}
		s := string(rest[:i])
		rest = rest[i+1:]
		return s, true
	}
	args = []string{}
	for range argc {
		s, ok := next()
		if !ok {
			break
		}
		args = append(args, s)
	}
	for {
		s, ok := next()
		if !ok || s == "" {
			break
		}
		env = append(env, s)
	}
	return args, env
}
