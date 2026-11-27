package runs

import "golang.org/x/sys/unix"

// cloneFile makes to a clone of the file at from: APFS shares their
// blocks until either is changed, so a pass kept or put back costs next
// to nothing. It fails where the volume can't (HFS+, another volume), and
// the caller copies.
func cloneFile(from, to string) error { return unix.Clonefile(from, to, unix.CLONE_NOFOLLOW) }
