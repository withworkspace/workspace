package daemon

import (
	"sync"

	"github.com/withworkspace/workspace/internal/crew"
)

// Host runs crew in the workspace's daemon from another process, over its
// socket, as a crew.Host. It starts the server only to launch crew.
type Host struct {
	root string

	mu     sync.Mutex
	client *Client
}

var _ crew.Host = (*Host)(nil)

// NewHost is a Host for the workspace at root.
func NewHost(root string) *Host { return &Host{root: root} }

func (h *Host) connect(start bool) (*Client, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.client != nil {
		select {
		case <-h.client.Lost():
			h.client = nil
		default:
			return h.client, nil
		}
	}
	var c *Client
	var err error
	if start {
		c, err = Connect(h.root)
	} else {
		c, err = Dial(h.root)
	}
	if err != nil {
		return nil, err
	}
	h.client = c
	return c, nil
}

// Launch starts crew on the server, starting the server if it isn't
// running.
func (h *Host) Launch(s crew.Spec) (int, error) {
	c, err := h.connect(true)
	if err != nil {
		return 0, err
	}
	return c.Start(s)
}

// Stop stops crew the server runs, by key.
func (h *Host) Stop(key string) error {
	c, err := h.connect(false)
	if err != nil {
		return crew.ErrNotRunning // no server: nothing's running
	}
	return c.Stop(key)
}

// Say gives crew the server runs a message.
func (h *Host) Say(key, text string) (string, error) {
	c, err := h.connect(false)
	if err != nil {
		return "", crew.ErrNotRunning // no server: nothing's running
	}
	return c.Say(key, text)
}

// Leave disconnects; crew keep running on the server.
func (h *Host) Leave() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.client != nil {
		h.client.Close()
		h.client = nil
	}
}
