package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/withworkspace/plugin/protocol"
	"github.com/withworkspace/workspace/internal/acp"
)

// A plugin's proposals: what a conversation's agent may propose of a
// plugin's (ws propose jira:issues), drafted by the plugin from what the
// agent wrote, kept by ws, and done by the plugin once the user accepts
// it. ws doesn't read what's written, or what's kept for each item: the
// plugin's draft says what it'll do, and it's trusted to.

// MaxProposalItems is how many items ws takes in one proposal.
const MaxProposalItems = 50

// ErrNoProposes is a plugin that doesn't propose what was named.
var ErrNoProposes = errors.New("the plugin doesn't propose that")

// Proposable is one of what a plugin proposes, as an agent's told it: by
// its name (jira:issues), what the card calls it, and when to.
type Proposable struct {
	Plugin string
	protocol.Proposes
}

// Name is how an agent names it: the plugin's name, a colon, its id.
func (p Proposable) Name() string { return p.Plugin + ":" + p.ID }

// SplitProposable is a name as an agent writes it (jira:issues): the
// plugin's name and the id, or ok false.
func SplitProposable(name string) (plugin, id string, ok bool) {
	plugin, id, ok = strings.Cut(strings.TrimSpace(name), ":")
	return plugin, id, ok && ValidName(plugin) && id != ""
}

// Proposables are what the workspace's plugins propose, as each
// registered when it last started: none of a plugin that isn't set up,
// which couldn't do it.
func Proposables(root string) []Proposable {
	var out []Proposable
	for _, name := range Installed(root) {
		reg, ok := ReadRegistration(root, name)
		if !ok || len(reg.Proposes) == 0 || protocol.ProposesProblem(reg.Proposes) != "" {
			continue
		}
		if values, err := Values(root, name); err != nil || len(Missing(reg, values)) > 0 {
			continue
		}
		for _, p := range reg.Proposes {
			out = append(out, Proposable{Plugin: name, Proposes: p})
		}
	}
	return out
}

// proposes is what a running plugin registered to propose by id, or an
// ErrNoProposes.
func (h *Host) proposes(name, id string) (*proc, protocol.Proposes, error) {
	p, err := h.ready(name)
	if err != nil {
		return nil, protocol.Proposes{}, err
	}
	p.mu.Lock()
	list := p.reg.Proposes
	p.mu.Unlock()
	i := slices.IndexFunc(list, func(x protocol.Proposes) bool { return x.ID == id })
	if i < 0 {
		var names []string
		for _, x := range list {
			names = append(names, name+":"+x.ID)
		}
		if len(names) == 0 {
			return nil, protocol.Proposes{}, fmt.Errorf("%w: %s proposes nothing", ErrNoProposes, name)
		}
		return nil, protocol.Proposes{}, fmt.Errorf("%w: %s proposes %s, not %s", ErrNoProposes, name, strings.Join(names, ", "), id)
	}
	return p, list[i], nil
}

// Draft asks a plugin to draft what an agent proposed (in.ID), as it
// wrote it: with no input, how one's written. What it answers is checked
// for what ws takes: a guide, a refusal, or items, no more than
// MaxProposalItems, each with a payload. The Proposes is what the plugin
// registered of it.
func (h *Host) Draft(ctx context.Context, name string, in protocol.ProposalDraftParams) (protocol.ProposalDraftResult, protocol.Proposes, error) {
	var out protocol.ProposalDraftResult
	p, decl, err := h.proposes(name, in.ID)
	if err != nil {
		return out, decl, err
	}
	if h.Hold != nil {
		defer h.Hold()()
	}
	ctx, cancel := context.WithTimeout(ctx, actTimeout)
	defer cancel()
	if err := h.callPlugin(ctx, p, name, protocol.MethodProposalDraft, in, &out); err != nil {
		return out, decl, err
	}
	switch {
	case len(in.Input) == 0 && strings.TrimSpace(out.Guide) == "":
		return out, decl, fmt.Errorf("%s has no guide to writing %s:%s", name, name, in.ID)
	case len(in.Input) == 0:
		return protocol.ProposalDraftResult{Guide: out.Guide}, decl, nil
	case strings.TrimSpace(out.Refused) != "":
		return protocol.ProposalDraftResult{Refused: out.Refused}, decl, nil
	case out.Proposal == nil || len(out.Proposal.Items) == 0:
		return out, decl, fmt.Errorf("%s drafted nothing of it, and didn't say why", name)
	case len(out.Proposal.Items) > MaxProposalItems:
		return protocol.ProposalDraftResult{Refused: fmt.Sprintf("that's %d at once: ws takes up to %d in a proposal, so propose them in parts", len(out.Proposal.Items), MaxProposalItems)}, decl, nil
	}
	for i, it := range out.Proposal.Items {
		if strings.TrimSpace(it.Title) == "" || len(it.Payload) == 0 {
			return out, decl, fmt.Errorf("%s drafted item %d with no title or payload", name, i+1)
		}
	}
	return protocol.ProposalDraftResult{Proposal: out.Proposal}, decl, nil
}

// Accept has a plugin do items of what it drafted, as the user accepted
// them: what each made, or why it wasn't done, in the order asked. An
// item it says nothing of isn't done.
func (h *Host) Accept(ctx context.Context, name string, in protocol.ProposalAcceptParams) (protocol.ProposalAcceptResult, error) {
	var out protocol.ProposalAcceptResult
	p, _, err := h.proposes(name, in.ID)
	if err != nil {
		return out, err
	}
	if h.Hold != nil {
		defer h.Hold()()
	}
	ctx, cancel := context.WithTimeout(ctx, actTimeout)
	defer cancel()
	if err := h.callPlugin(ctx, p, name, protocol.MethodProposalAccept, in, &out); err != nil {
		return out, err
	}
	return out, nil
}

// callPlugin asks a running plugin something, saying what went wrong as
// a person would want it said.
func (h *Host) callPlugin(ctx context.Context, p *proc, name, method string, in, out any) error {
	err := p.conn.Call(ctx, method, in, out)
	var re *acp.RPCError
	switch {
	case errors.As(err, &re):
		return fmt.Errorf("%s: %s", name, re.Message)
	case errors.Is(err, acp.ErrClosed):
		return fmt.Errorf("%s stopped mid-action: see %s", name, LogPath(h.Root, name))
	case errors.Is(err, context.DeadlineExceeded):
		return fmt.Errorf("%s did not finish within %s", name, actTimeout)
	}
	return err
}

type draftParams struct {
	Name string                       `json:"name"`
	In   protocol.ProposalDraftParams `json:"in"`
}

// Drafted is a draft as the daemon answers it: the plugin's answer, and
// what it registered of what was proposed.
type Drafted struct {
	protocol.ProposalDraftResult
	Proposes protocol.Proposes `json:"proposes"`
}

type acceptParams struct {
	Name string                        `json:"name"`
	In   protocol.ProposalAcceptParams `json:"in"`
}

// codeNoProposes is ErrNoProposes, across the socket.
const codeNoProposes = protocol.CodeNotFound - 2

// serveProposals answers the socket's draft and accept, or ok false for
// another method.
func (h *Host) serveProposals(ctx context.Context, method string, params json.RawMessage) (any, bool, error) {
	switch method {
	case "draft":
		var p draftParams
		if err := json.Unmarshal(params, &p); err != nil || !ValidName(p.Name) {
			return nil, true, &acp.RPCError{Code: acp.CodeInvalidParams, Message: "which plugin's?"}
		}
		out, decl, err := h.Draft(ctx, p.Name, p.In)
		if errors.Is(err, ErrNoProposes) {
			return nil, true, &acp.RPCError{Code: codeNoProposes, Message: err.Error()}
		}
		if err != nil {
			return nil, true, err
		}
		return Drafted{ProposalDraftResult: out, Proposes: decl}, true, nil
	case "accept":
		var p acceptParams
		if err := json.Unmarshal(params, &p); err != nil || !ValidName(p.Name) {
			return nil, true, &acp.RPCError{Code: acp.CodeInvalidParams, Message: "which plugin's?"}
		}
		out, err := h.Accept(ctx, p.Name, p.In)
		if errors.Is(err, ErrNoProposes) {
			return nil, true, &acp.RPCError{Code: codeNoProposes, Message: err.Error()}
		}
		return out, true, err
	}
	return nil, false, nil
}

// Draft has a plugin draft what an agent proposed, in the daemon
// (Host.Draft).
func (c *Client) Draft(ctx context.Context, name string, in protocol.ProposalDraftParams) (Drafted, error) {
	var out Drafted
	return out, c.proposalCall(ctx, "draft", draftParams{Name: name, In: in}, &out)
}

// Accept has a plugin do items of what it drafted, in the daemon
// (Host.Accept).
func (c *Client) Accept(ctx context.Context, name string, in protocol.ProposalAcceptParams) (protocol.ProposalAcceptResult, error) {
	var out protocol.ProposalAcceptResult
	return out, c.proposalCall(ctx, "accept", acceptParams{Name: name, In: in}, &out)
}

func (c *Client) proposalCall(ctx context.Context, method string, in, out any) error {
	err := c.conn.Call(ctx, method, in, out)
	var re *acp.RPCError
	switch {
	case errors.As(err, &re) && re.Code == codeNoProposes:
		return fmt.Errorf("%w: %s", ErrNoProposes, strings.TrimPrefix(re.Message, ErrNoProposes.Error()+": "))
	case errors.As(err, &re):
		return errors.New(re.Message)
	case errors.Is(err, acp.ErrClosed):
		return errors.New("the workspace's daemon went away")
	}
	return err
}
