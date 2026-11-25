package team

// What ws eval team needs of a gist's writing, from outside: a change read
// as the writer would be given it, and its reply read as the Team page
// reads it.

// MaterialFor is sub's change as the gist's writer is given it, read from
// bare (a clone of sub's repo): a subject of one repo, as Narrate reads it.
func MaterialFor(bare string, sub Subject) (Material, error) {
	return materialOf(bare, sub, diffInAll)
}

// ReadReply is a writer's reply as the Team page keeps it: the gist, and
// what's worth knowing (none for a change only to dependencies).
func ReadReply(reply string, m Material) (string, []Know, error) {
	return read(reply, m)
}
