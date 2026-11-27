package plugin

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/withworkspace/plugin/protocol"
	"github.com/withworkspace/workspace/internal/acp"
	"github.com/withworkspace/workspace/internal/store"
)

// A kind's things can live in a repo (protocol.Remote): a folder each,
// which ws finds in the workspace's spaces (internal/space). One found is
// kept as one of the plugin's things, beside those it told ws of: named
// by its folder (records/sketches/release-flow/), called what its
// thing.toml says, with what the plugin said its main file is found by.
// So it's found and drawn as the plugin's other things are, and forgotten
// with the plugin. It's ws's, though, not the plugin's: thing/list doesn't
// list it, and thing/put and thing/remove don't reach it.

// Found is a thing ws found in a space, as it's kept for its kind's
// plugin.
type Found struct {
	// Kind is the plugin's ref it's one of, and Name its name: its
	// folder, as ws names one.
	Kind, Name string
	// Version is what it was read at: its manifest, its main file, and
	// its kind as the plugin registered it. One kept at the version it's
	// at now needn't be read again.
	Version string
	Changed time.Time
	// Fields are its values: its title, from its manifest, and what the
	// plugin said its main file is found by.
	Fields map[string]string
}

// Finding is a thing ws found in a space as it's kept: whose kind it's
// one of, and the version it was read at.
type Finding struct {
	Plugin, Kind, Name, Version string
}

// Findings are the things ws keeps that it found in a space, by plugin,
// kind and name.
func Findings(root string) ([]Finding, error) {
	db, err := store.Open(root)
	if err != nil {
		return nil, err
	}
	var out []Finding
	err = db.Read(context.Background(), func(tx *store.Tx) error {
		rows, err := tx.Query(`SELECT plugin, kind, ref, found FROM plugin_things WHERE found != '' ORDER BY plugin, kind, ref`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var f Finding
			if err := rows.Scan(&f.Plugin, &f.Kind, &f.Name, &f.Version); err != nil {
				return err
			}
			out = append(out, f)
		}
		return rows.Err()
	})
	return out, err
}

// FoundNamed is the thing ws found in a space by that name, if it keeps
// one: whose kind it's one of.
func FoundNamed(root, name string) (Finding, bool, error) {
	db, err := store.Open(root)
	if err != nil {
		return Finding{}, false, err
	}
	f := Finding{Name: name}
	ok := false
	err = db.Read(context.Background(), func(tx *store.Tx) error {
		switch err := tx.QueryRow(`SELECT plugin, kind, found FROM plugin_things WHERE ref = ? AND found != '' LIMIT 1`, name).Scan(&f.Plugin, &f.Kind, &f.Version); {
		case errors.Is(err, sql.ErrNoRows):
			return nil
		case err != nil:
			return err
		}
		ok = true
		return nil
	})
	return f, ok, err
}

// KeepFound keeps things ws found in the spaces as the plugin name's,
// each in place of what was kept by its name, and forgets those gone (by
// kind and name). A thing of a kind the plugin hasn't registered, or one
// whose things don't live in a repo, isn't kept; neither is one with no
// title.
func KeepFound(root, name string, things []Found, gone []Finding) error {
	db, err := store.Open(root)
	if err != nil {
		return err
	}
	return db.Write(context.Background(), func(tx *store.Tx) error {
		for _, g := range gone {
			if err := forgetThing(tx, name, g.Kind, g.Name); err != nil {
				return err
			}
		}
		if len(things) == 0 {
			return nil
		}
		kept, err := refsIn(tx)
		if err != nil {
			return err
		}
		for _, t := range things {
			i := slices.IndexFunc(kept, func(k KeptRef) bool { return k.Plugin == name && k.Kind == t.Kind })
			if i < 0 || kept[i].Remote == nil {
				continue
			}
			kind := kept[i].Ref
			fields := Tidy(kind, t.Fields)
			if fields[kind.Typed(protocol.FieldTitle)] == "" {
				continue
			}
			told, err := json.Marshal(fields)
			if err != nil {
				return err
			}
			if _, err := tx.Exec(`INSERT INTO plugin_things (plugin, kind, ref, changed, fields, found) VALUES (?, ?, ?, ?, ?, ?)
				ON CONFLICT (plugin, kind, ref) DO UPDATE SET changed = excluded.changed, fields = excluded.fields, found = excluded.found`,
				name, t.Kind, t.Name, t.Changed.UTC().Format(store.Stamp), string(told), t.Version); err != nil {
				return err
			}
			if _, err := indexThing(tx, name, kind, Record{Kind: t.Kind, Ref: t.Name, Changed: t.Changed, Fields: fields, Found: t.Version}, kept); err != nil {
				return err
			}
		}
		return nil
	})
}

// ErrNoRead is a plugin that doesn't say what a file of that kind is
// found by: a kind it didn't register, one whose things don't live in a
// repo, or one it doesn't read. What's found is found by its title.
var ErrNoRead = errors.New("the plugin doesn't read that kind's files")

type noRead struct{ why string }

func (e *noRead) Error() string        { return e.why }
func (e *noRead) Is(target error) bool { return target == ErrNoRead }

// ErrCantRead is a plugin saying it can't read a file it was asked to: the
// file's wrong, not the plugin. What's found is found by its title.
var ErrCantRead = errors.New("the plugin can't read that file")

type cantRead struct{ why string }

func (e *cantRead) Error() string        { return e.why }
func (e *cantRead) Is(target error) bool { return target == ErrCantRead }

// ReadRemote asks a plugin what a thing's main file is found by
// (remote/read): a value for each of its kind's fields the file has one
// for. The plugin's started if it isn't running. ErrNoRead is a kind the
// plugin doesn't read, and ErrCantRead a file it says it can't; anything
// else is the plugin not being there to ask.
func (h *Host) ReadRemote(ctx context.Context, name string, in protocol.RemoteReadParams) (map[string]string, error) {
	p, err := h.ready(name)
	if err != nil {
		return nil, err
	}
	p.mu.Lock()
	i := slices.IndexFunc(p.reg.Refs, func(r protocol.Ref) bool { return r.Kind == in.Kind })
	reads := i >= 0 && p.reg.Refs[i].Remote != nil && p.reg.Refs[i].Remote.Reads
	p.mu.Unlock()
	if !reads {
		return nil, &noRead{fmt.Sprintf("%s doesn't read a %s's file", name, in.Kind)}
	}
	if h.Hold != nil {
		defer h.Hold()()
	}
	ctx, cancel := context.WithTimeout(ctx, showTimeout)
	defer cancel()
	var out protocol.RemoteReadResult
	err = p.conn.Call(ctx, protocol.MethodRemoteRead, in, &out)
	var re *acp.RPCError
	switch {
	case errors.As(err, &re) && re.Code == protocol.CodeMethodNotFound:
		return nil, &noRead{fmt.Sprintf("%s doesn't read a %s's file", name, in.Kind)}
	case errors.As(err, &re):
		return nil, &cantRead{fmt.Sprintf("%s: %s", name, re.Message)}
	case errors.Is(err, acp.ErrClosed):
		return nil, fmt.Errorf("%s stopped while it was reading %s: see %s", name, in.Name, LogPath(h.Root, name))
	case errors.Is(err, context.DeadlineExceeded):
		return nil, fmt.Errorf("%s didn't say what %s is found by in %s", name, in.Name, showTimeout)
	case err != nil:
		return nil, err
	}
	return out.Fields, nil
}

type remoteReadParams struct {
	Name string                    `json:"name"`
	In   protocol.RemoteReadParams `json:"in"`
}

// ErrNoRead and ErrCantRead, across the socket.
const (
	codeNoRead   = protocol.CodeNotFound - 3
	codeCantRead = protocol.CodeNotFound - 4
)

// serveRemotes answers the socket's remote-read, or ok false for another
// method.
func (h *Host) serveRemotes(ctx context.Context, method string, params json.RawMessage) (any, bool, error) {
	if method != "remote-read" {
		return nil, false, nil
	}
	var p remoteReadParams
	if err := json.Unmarshal(params, &p); err != nil || !ValidName(p.Name) {
		return nil, true, &acp.RPCError{Code: acp.CodeInvalidParams, Message: "which plugin's?"}
	}
	fields, err := h.ReadRemote(ctx, p.Name, p.In)
	switch {
	case errors.Is(err, ErrNoRead):
		return nil, true, &acp.RPCError{Code: codeNoRead, Message: err.Error()}
	case errors.Is(err, ErrCantRead):
		return nil, true, &acp.RPCError{Code: codeCantRead, Message: err.Error()}
	case err != nil:
		return nil, true, err
	}
	return protocol.RemoteReadResult{Fields: fields}, true, nil
}

// ReadRemote asks a plugin what a thing's main file is found by, in the
// daemon (Host.ReadRemote).
func (c *Client) ReadRemote(ctx context.Context, name string, in protocol.RemoteReadParams) (map[string]string, error) {
	var out protocol.RemoteReadResult
	err := c.conn.Call(ctx, "remote-read", remoteReadParams{Name: name, In: in}, &out)
	var re *acp.RPCError
	switch {
	case errors.As(err, &re) && re.Code == codeNoRead:
		return nil, &noRead{re.Message}
	case errors.As(err, &re) && re.Code == codeCantRead:
		return nil, &cantRead{re.Message}
	case errors.As(err, &re):
		return nil, errors.New(re.Message)
	case errors.Is(err, acp.ErrClosed):
		return nil, errors.New("the workspace's daemon went away")
	case err != nil:
		return nil, err
	}
	return out.Fields, nil
}

// ErrNoExport is a plugin that doesn't give that kind's things as a
// folder is to hold them: a kind it didn't register, one whose things
// don't live in a repo, or one it doesn't export. ws puts none of them in
// a repo.
var ErrNoExport = errors.New("the plugin doesn't give that kind's things for a repo")

type noExport struct{ why string }

func (e *noExport) Error() string        { return e.why }
func (e *noExport) Is(target error) bool { return target == ErrNoExport }

// ExportRemote asks a plugin for one of its own things as its folder in a
// repo is to hold it (remote/export): to promote it, or to save it. With
// in.Current, the thing has changed in its repo since the plugin last had
// it, and what comes back is the two brought together. ErrNoExport is a
// kind the plugin doesn't export, and ErrNoThing a name it has nothing by.
func (h *Host) ExportRemote(ctx context.Context, name string, in protocol.RemoteExportParams) (protocol.RemoteExported, error) {
	var out protocol.RemoteExported
	p, err := h.ready(name)
	if err != nil {
		return out, err
	}
	p.mu.Lock()
	i := slices.IndexFunc(p.reg.Refs, func(r protocol.Ref) bool { return r.Kind == in.Kind })
	exports := i >= 0 && p.reg.Refs[i].Remote != nil && p.reg.Refs[i].Remote.Exports
	p.mu.Unlock()
	if !exports {
		return out, &noExport{fmt.Sprintf("%s cannot save a %s to a repo", name, in.Kind)}
	}
	if h.Hold != nil {
		defer h.Hold()()
	}
	ctx, cancel := context.WithTimeout(ctx, actTimeout)
	defer cancel()
	err = p.conn.Call(ctx, protocol.MethodRemoteExport, in, &out)
	var re *acp.RPCError
	switch {
	case errors.As(err, &re) && re.Code == protocol.CodeMethodNotFound:
		return out, &noExport{fmt.Sprintf("%s cannot save a %s to a repo", name, in.Kind)}
	case errors.As(err, &re) && re.Code == protocol.CodeNotFound:
		return out, &noThing{fmt.Sprintf("%s has no %s called %s", name, in.Kind, in.Ref)}
	}
	return out, h.said(name, "giving "+in.Ref+" for its repo", err)
}

// SavedRemote tells a plugin that one of its things is in a repo, as it
// was last exported (remote/saved): where, and at what version.
func (h *Host) SavedRemote(ctx context.Context, name string, in protocol.RemoteSavedParams) error {
	p, err := h.ready(name)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, showTimeout)
	defer cancel()
	err = p.conn.Call(ctx, protocol.MethodRemoteSaved, in, nil)
	var re *acp.RPCError
	if errors.As(err, &re) && re.Code == protocol.CodeMethodNotFound {
		return nil // it keeps nothing of it
	}
	return h.said(name, "hearing "+in.Ref+" is in its repo", err)
}

// said is what went wrong asking a plugin something, as a person would
// want it said: nil for nothing.
func (h *Host) said(name, doing string, err error) error {
	var re *acp.RPCError
	switch {
	case err == nil:
		return nil
	case errors.As(err, &re):
		return fmt.Errorf("%s: %s", name, re.Message)
	case errors.Is(err, acp.ErrClosed):
		return fmt.Errorf("%s stopped while %s: see %s", name, doing, LogPath(h.Root, name))
	case errors.Is(err, context.DeadlineExceeded):
		return fmt.Errorf("%s timed out %s", name, doing)
	}
	return err
}

type remoteExportParams struct {
	Name string                      `json:"name"`
	In   protocol.RemoteExportParams `json:"in"`
}

type remoteSavedParams struct {
	Name string                     `json:"name"`
	In   protocol.RemoteSavedParams `json:"in"`
}

// ErrNoExport, across the socket.
const codeNoExport = protocol.CodeNotFound - 5

// serveExports answers the socket's remote-export and remote-saved, or ok
// false for another method.
func (h *Host) serveExports(ctx context.Context, method string, params json.RawMessage) (any, bool, error) {
	switch method {
	case "remote-export":
		var p remoteExportParams
		if err := json.Unmarshal(params, &p); err != nil || !ValidName(p.Name) {
			return nil, true, &acp.RPCError{Code: acp.CodeInvalidParams, Message: "which plugin's?"}
		}
		out, err := h.ExportRemote(ctx, p.Name, p.In)
		switch {
		case errors.Is(err, ErrNoExport):
			return nil, true, &acp.RPCError{Code: codeNoExport, Message: err.Error()}
		case errors.Is(err, ErrNoThing):
			return nil, true, &acp.RPCError{Code: codeNoThing, Message: err.Error()}
		case err != nil:
			return nil, true, err
		}
		return out, true, nil
	case "remote-saved":
		var p remoteSavedParams
		if err := json.Unmarshal(params, &p); err != nil || !ValidName(p.Name) {
			return nil, true, &acp.RPCError{Code: acp.CodeInvalidParams, Message: "which plugin's?"}
		}
		return struct{}{}, true, h.SavedRemote(ctx, p.Name, p.In)
	}
	return nil, false, nil
}

// ExportRemote asks a plugin for one of its things as its folder in a repo
// is to hold it, in the daemon (Host.ExportRemote).
func (c *Client) ExportRemote(ctx context.Context, name string, in protocol.RemoteExportParams) (protocol.RemoteExported, error) {
	var out protocol.RemoteExported
	err := c.conn.Call(ctx, "remote-export", remoteExportParams{Name: name, In: in}, &out)
	var re *acp.RPCError
	switch {
	case errors.As(err, &re) && re.Code == codeNoExport:
		return out, &noExport{re.Message}
	case errors.As(err, &re) && re.Code == codeNoThing:
		return out, &noThing{re.Message}
	case errors.As(err, &re):
		return out, errors.New(re.Message)
	case errors.Is(err, acp.ErrClosed):
		return out, errors.New("the workspace's daemon went away")
	}
	return out, err
}

// SavedRemote tells a plugin one of its things is in a repo, in the
// daemon (Host.SavedRemote).
func (c *Client) SavedRemote(ctx context.Context, name string, in protocol.RemoteSavedParams) error {
	return c.call(ctx, "remote-saved", remoteSavedParams{Name: name, In: in}, nil)
}

// KeepAlias keeps that a plugin's own name for one of its things (ref,
// sketch-12) is the name of a thing in a repo now (name, its folder): ws
// put it there for the plugin.
func KeepAlias(root, plugin, kind, ref, name string) error {
	db, err := store.Open(root)
	if err != nil {
		return err
	}
	return db.Write(context.Background(), func(tx *store.Tx) error {
		_, err := tx.Exec(`INSERT INTO plugin_aliases (plugin, kind, ref, name) VALUES (?, ?, ?, ?)
			ON CONFLICT (plugin, kind, ref) DO UPDATE SET name = excluded.name`, plugin, kind, ref, name)
		return err
	})
}

// AliasOf is the thing in a repo that a plugin's own name for one of its
// things names now, if ws put it there: its folder.
func AliasOf(root, plugin, kind, ref string) (name string, ok bool, err error) {
	db, err := store.Open(root)
	if err != nil {
		return "", false, err
	}
	err = db.Read(context.Background(), func(tx *store.Tx) error {
		switch err := tx.QueryRow(`SELECT name FROM plugin_aliases WHERE plugin = ? AND kind = ? AND ref = ?`, plugin, kind, ref).Scan(&name); {
		case errors.Is(err, sql.ErrNoRows):
			return nil
		case err != nil:
			return err
		}
		ok = true
		return nil
	})
	return name, ok, err
}
