package process

import "github.com/withworkspace/workspace/internal/store"

// The records once kept read-backs of the process: what an agent
// understood of it, for the user to confirm. They went with the read-back,
// and the part stays so a file that had them drops them: its first step
// made the table, and does nothing now.
func init() {
	store.Define("process", store.Retired, store.SQL(`DROP TABLE IF EXISTS process_readbacks`))
}
