//go:build !darwin && !linux

package harness

import "errors"

// ReadProcs can't read processes here: the process check isn't made, and
// harnesses answer as their own payloads say.
func ReadProcs() (Procs, error) {
	return Procs{}, errors.New("reading processes isn't supported on this platform")
}
