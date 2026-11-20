package ops

// CapsuleStamp names what git's answers about a worktree are built from,
// as it is now, without asking git (capsuleStamp): for what keeps those
// answers outside ops, the web UI's kept reads of a capsule's files. ""
// for a folder that isn't a worktree, whose answers can't be kept by it.
func CapsuleStamp(dir string) string { return capsuleStamp(dir) }
