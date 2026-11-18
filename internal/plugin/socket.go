package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/withworkspace/plugin/protocol"
	"github.com/withworkspace/workspace/internal/acp"
)

// A command reaches the workspace's plugins through a socket the daemon
// serves, as it reaches the records through one. What crosses it is
// JSON-RPC, as between ws and a plugin: "run" is a command, whose output
// comes back as "output" notifications before its answer.

// SocketPath is where the daemon for the workspace at root answers for its
// plugins.
func SocketPath(root string) string {
	return socketPath(root, filepath.Join(root, ".ws", "plugins.sock"), "plugins")
}

type runParams struct {
	Name     string   `json:"name"`
	Args     []string `json:"args"`
	Dir      string   `json:"dir"`
	Terminal Terminal `json:"terminal"`
}

type outputParams struct {
	Stream string `json:"stream"`
	Data   string `json:"data"`
}

type nameParams struct {
	Name string `json:"name"`
}

type showParams struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
	Ref  string `json:"ref"`
	Full bool   `json:"full,omitempty"`
}

// The daemon's answers for a thing a plugin hasn't (ErrNoThing), and for
// a plugin with nothing to say of one (ErrNoShow), across the socket.
const (
	codeNoThing = protocol.CodeNotFound
	codeNoShow  = protocol.CodeNotFound - 1
)

// Listen binds the socket commands reach the workspace's plugins through.
func (h *Host) Listen() (net.Listener, error) {
	path := SocketPath(h.Root)
	if c, err := net.DialTimeout("unix", path, time.Second); err == nil {
		c.Close()
		return nil, fmt.Errorf("something is already listening on %s", path)
	}
	os.Remove(path) // left by a daemon that died
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	os.Chmod(path, 0o600) //nolint:errcheck // the folder's the workspace's own
	return ln, nil
}

// Serve answers commands on ln until it's closed.
func (h *Host) Serve(ln net.Listener) {
	for {
		nc, err := ln.Accept()
		if err != nil {
			return
		}
		go h.serve(nc)
	}
}

func (h *Host) serve(nc net.Conn) {
	// The handler's given the connection it answers on, which exists only
	// once it's been handed the handler.
	ready := make(chan struct{})
	var c *acp.Conn
	c = acp.NewConn(nc, nc, func(ctx context.Context, method string, params json.RawMessage) (any, error) {
		<-ready
		switch method {
		case "run":
			var p runParams
			if err := json.Unmarshal(params, &p); err != nil {
				return nil, &acp.RPCError{Code: acp.CodeInvalidParams, Message: err.Error()}
			}
			exit, err := h.Run(ctx, p.Name, p.Args, p.Dir, p.Terminal, func(stream, data string) {
				c.Notify("output", outputParams{Stream: stream, Data: data}) //nolint:errcheck // the command's gone
			})
			if err != nil {
				return nil, err
			}
			return map[string]int{"exit": exit}, nil
		case "list":
			return h.Statuses(), nil
		case "show":
			var p showParams
			if err := json.Unmarshal(params, &p); err != nil || !ValidName(p.Name) {
				return nil, &acp.RPCError{Code: acp.CodeInvalidParams, Message: "which plugin's thing?"}
			}
			shown, err := h.Show(ctx, p.Name, p.Kind, p.Ref, p.Full)
			if errors.Is(err, ErrNoThing) {
				return nil, &acp.RPCError{Code: codeNoThing, Message: err.Error()}
			}
			if errors.Is(err, ErrNoShow) {
				return nil, &acp.RPCError{Code: codeNoShow, Message: err.Error()}
			}
			if err != nil {
				return nil, err
			}
			return shown, nil
		case "refresh", "config", "stop":
			var p nameParams
			if err := json.Unmarshal(params, &p); err != nil || !ValidName(p.Name) {
				return nil, &acp.RPCError{Code: acp.CodeInvalidParams, Message: "which plugin?"}
			}
			switch method {
			case "refresh":
				st, err := h.Refresh(p.Name)
				if err != nil {
					return nil, err
				}
				return st, nil
			case "config":
				return nil, h.ConfigChanged(p.Name)
			}
			h.Stop(p.Name)
			return nil, nil
		}
		return nil, &acp.RPCError{Code: acp.CodeMethodNotFound, Message: "no such method: " + method}
	})
	close(ready)
	<-c.Done()
	nc.Close()
}

// Client is a command's connection to the workspace's plugins, in its
// daemon.
type Client struct {
	nc   net.Conn
	conn *acp.Conn
	out  func(stream, data string)
}

// Dial connects to the daemon for the workspace at root, which must be
// running.
func Dial(root string) (*Client, error) {
	nc, err := net.DialTimeout("unix", SocketPath(root), 2*time.Second)
	if err != nil {
		return nil, fmt.Errorf("the workspace's daemon isn't answering for its plugins: %w", err)
	}
	c := &Client{nc: nc}
	c.conn = acp.NewConn(nc, nc, func(ctx context.Context, method string, params json.RawMessage) (any, error) {
		if method == "output" && c.out != nil {
			var o outputParams
			if json.Unmarshal(params, &o) == nil {
				c.out(o.Stream, o.Data)
			}
		}
		return nil, nil
	})
	return c, nil
}

// Close ends the connection.
func (c *Client) Close() { c.nc.Close() }

func (c *Client) call(ctx context.Context, method string, params, result any) error {
	err := c.conn.Call(ctx, method, params, result)
	var re *acp.RPCError
	switch {
	case errors.As(err, &re):
		return errors.New(re.Message) // what the daemon said, without the code
	case errors.Is(err, acp.ErrClosed):
		return errors.New("the workspace's daemon went away")
	}
	return err
}

// Run runs a plugin's command in the daemon (Host.Run), with out given
// its output as it comes. One command runs on a client at a time.
func (c *Client) Run(ctx context.Context, name string, args []string, dir string, term Terminal, out func(stream, data string)) (int, error) {
	c.out = out
	var answer struct {
		Exit int `json:"exit"`
	}
	err := c.call(ctx, "run", runParams{Name: name, Args: args, Dir: dir, Terminal: term}, &answer)
	if errors.Is(err, context.Canceled) {
		return 130, nil
	}
	if err != nil {
		return 1, err
	}
	return answer.Exit, nil
}

// Show asks a plugin what one of its things is, in the daemon (Host.Show).
// One it hasn't is ErrNoThing, and a plugin with nothing to say of one,
// ErrNoShow.
func (c *Client) Show(ctx context.Context, name, kind, ref string, full bool) (Shown, error) {
	var out Shown
	err := c.conn.Call(ctx, "show", showParams{Name: name, Kind: kind, Ref: ref, Full: full}, &out)
	var re *acp.RPCError
	switch {
	case errors.As(err, &re) && re.Code == codeNoThing:
		return out, &noThing{re.Message}
	case errors.As(err, &re) && re.Code == codeNoShow:
		return out, &noShow{re.Message}
	case errors.As(err, &re):
		return out, errors.New(re.Message)
	case errors.Is(err, acp.ErrClosed):
		return out, errors.New("the workspace's daemon went away")
	}
	return out, err
}

// Statuses are the workspace's plugins, as the daemon has them.
func (c *Client) Statuses(ctx context.Context) ([]Status, error) {
	var out []Status
	return out, c.call(ctx, "list", nil, &out)
}

// Refresh starts a plugin afresh, and says where it stands.
func (c *Client) Refresh(ctx context.Context, name string) (Status, error) {
	var st Status
	return st, c.call(ctx, "refresh", nameParams{Name: name}, &st)
}

// ConfigChanged tells a running plugin its config changed.
func (c *Client) ConfigChanged(ctx context.Context, name string) error {
	return c.call(ctx, "config", nameParams{Name: name}, nil)
}

// Stop stops a plugin.
func (c *Client) Stop(ctx context.Context, name string) error {
	return c.call(ctx, "stop", nameParams{Name: name}, nil)
}
