// Package daemon is the workspace's daemon: one per workspace, it holds what
// outlives a command. It runs crew (their pseudo-terminals and emulated
// screens, or agents held over ACP), runs jobs (the orchestrator's runs,
// conversations' answers), and hosts the web UI. Commands reach it over a
// Unix socket; inside it, the web UI reaches it directly.
//
// The protocol is newline-delimited JSON over one connection. Requests carry
// an id and get exactly one reply with that id; events (frames, raw output,
// exits) carry no id.
package daemon

import (
	"crypto/sha1"
	"encoding/hex"
	"os"
	"path/filepath"
	"time"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/withworkspace/workspace/internal/crew"
	"github.com/withworkspace/workspace/internal/job"
)

// ProtocolVersion changes whenever messages change incompatibly. A client
// refuses a server speaking another version. 2: say. 3: the daemon serves
// the workspace's records (store.sock): one without them can't.
const ProtocolVersion = 3

// Spec describes crew to launch; it travels as JSON.
type Spec = crew.Spec

// CrewInfo is running crew as listed by the server.
type CrewInfo struct {
	Spec Spec `json:"spec"`
	Pid  int  `json:"pid"`
}

type request struct {
	ID  int    `json:"id"`
	Op  string `json:"op"`
	Ver int    `json:"ver,omitempty"`

	Crew   string            `json:"crew,omitempty"`
	Spec   *Spec             `json:"spec,omitempty"`
	Key    *uv.KeyPressEvent `json:"key,omitempty"`
	Mouse  *uv.Mouse         `json:"mouse,omitempty"`
	Text   string            `json:"text,omitempty"`
	Data   []byte            `json:"data,omitempty"`
	W      int               `json:"w,omitempty"`
	H      int               `json:"h,omitempty"`
	Offset int               `json:"offset,omitempty"`
	On     bool              `json:"on,omitempty"`
	Grace  int               `json:"grace_ms,omitempty"`
	Now    bool              `json:"now,omitempty"` // restart: without waiting for what's at work
	Job    *job.Job          `json:"job,omitempty"`
}

// message is a reply (ID set) or an event (Ev set).
type message struct {
	ID    int    `json:"id,omitempty"`
	Error string `json:"error,omitempty"`

	Ver   int            `json:"ver,omitempty"`
	Pid   int            `json:"pid,omitempty"`
	Web   int            `json:"web,omitempty"`  // hello: the web UI's port; servers without one don't say
	Idle  *time.Duration `json:"idle,omitempty"` // hello: its IdleTimeout, 0 for none; servers before it don't say
	Crews []CrewInfo     `json:"crews,omitempty"`
	// hello, from a server that restarts itself (servers before it say
	// none of these): the ws it runs and where that is, whether it's to
	// restart once nothing's at work, and the ws it found in place of its
	// own.
	Build  string `json:"build,omitempty"`
	Exe    string `json:"exe,omitempty"`
	Wanted bool   `json:"wanted,omitempty"`
	Next   string `json:"next,omitempty"`
	// atwork, restart: what a restart would stop mid-flight.
	AtWork []string `json:"at_work,omitempty"`

	Ev    string `json:"ev,omitempty"`
	Crew  string `json:"crew,omitempty"`
	Frame *Frame `json:"frame,omitempty"`
	Data  []byte `json:"data,omitempty"`
	Exit  string `json:"exit,omitempty"`
	Text  string `json:"text,omitempty"`
}

// Frame is one rendering of a crew's screen, for one attached client.
type Frame = crew.Frame

// maxSocketPath is under the smallest sun_path limit we run on (104 on macOS).
const maxSocketPath = 100

// SocketPath is where a workspace's daemon listens.
func SocketPath(root string) string { return socketPath(root, "daemon") }

func socketPath(root, name string) string {
	p := filepath.Join(root, ".ws", name+".sock")
	if len(p) <= maxSocketPath {
		return p
	}
	sum := sha1.Sum([]byte(root))
	return filepath.Join(os.TempDir(), "ws-"+name+"-"+hex.EncodeToString(sum[:6])+".sock")
}

// LogPath is where an auto-started server writes its log.
func LogPath(root string) string { return filepath.Join(root, ".ws", "daemon.log") }
