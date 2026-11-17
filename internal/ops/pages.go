package ops

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/withworkspace/workspace/internal/config"
	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/doc"
	"github.com/withworkspace/workspace/internal/job"
	"github.com/withworkspace/workspace/internal/page"
	"github.com/withworkspace/workspace/internal/workspace"
)

// Pages: documents you write in the web UI, their cells (each one of the
// workspace's conversations, which its agent fills), and the passes run
// over them.

// PassDefs are a workspace's passes over a page: the built-in ones, and
// its own from ws.toml ([pages.passes.<name>]). A pass that's a skill the
// workspace hasn't got says so (Unavailable). cfg nil is no config: the
// built-in ones.
func PassDefs(cfg *config.Config, ws *workspace.Workspace) []page.PassDef {
	var own []page.PassDef
	if cfg != nil {
		for name, p := range cfg.Pages.Passes {
			def := page.PassDef{Name: name, Title: p.Title, About: p.About, Looks: p.Looks, File: p.File, Skill: strings.TrimSpace(p.Skill), Args: strings.TrimSpace(p.Args), Kinds: p.Kinds, Cold: p.Cold}
			if def.Skill != "" {
				def.File, def.Unavailable = skillFile(ws, def.Skill)
			}
			own = append(own, def)
		}
	}
	return page.Passes(own)
}

// skillFile is the file of the workspace's skill named name (its
// SKILL.md), or why there's none.
func skillFile(ws *workspace.Workspace, name string) (path, unavailable string) {
	for _, s := range ws.Skills() {
		if s.Name == name || filepath.Base(filepath.Dir(s.Path)) == name {
			return s.Path, ""
		}
	}
	return "", fmt.Sprintf("the workspace has no skill named %s (ws show skills lists them)", name)
}

// Passes are the workspace's passes over a page.
func (w *Workspace) Passes() []page.PassDef { return PassDefs(w.Config(), w.WS()) }

// PassesOn are the passes a page has, by its kind (page.ForPage).
func (w *Workspace) PassesOn(id string) []page.PassDef {
	_, p, _ := w.pages().Read(id)
	kinds := map[string][]string{}
	for _, k := range w.WS().Kinds {
		kinds[k.Name] = k.Passes
	}
	return page.ForPage(w.Passes(), kinds, p.Kind)
}

func (w *Workspace) pages() page.Store { return page.Store{Root: w.Root()} }

// cells are the conversations of the workspace's pages: its own.
func (w *Workspace) cells() conversation.Store { return conversation.ForWorkspace(w.Root()) }

// NewPage starts a page: named, it opens on its name as a heading. One
// started as a kind of document is written as one, to be filed.
func (w *Workspace) NewPage(title, kind, to string) (page.Page, error) {
	if kind != "" && !slices.ContainsFunc(w.WS().Kinds, func(k *workspace.Kind) bool { return k.Name == kind }) {
		return page.Page{}, &Invalid{Reason: "the workspace has no kind of document named " + kind}
	}
	if to != "" {
		var err error
		if to, err = w.plainPlace(to); err != nil {
			return page.Page{}, err
		}
	}
	text := ""
	if t := strings.Join(strings.Fields(title), " "); t != "" {
		text = "# " + t + "\n"
	}
	p, err := w.pages().New([]byte(text))
	if err != nil {
		return page.Page{}, err
	}
	if kind != "" || to != "" {
		if err := w.pages().SetMeta(p.ID, page.Meta{Kind: kind, To: to}); err != nil {
			return page.Page{}, err
		}
		p.Kind, p.To = kind, to
	}
	return p, nil
}

// plainPlace is where a page of no kind is to be filed, checked: a folder
// of one of the workspace's repos (repo:folder/, or repo: for its top),
// or a file in one (repo:folder/name.md), as repo:path with the repo's
// canonical name.
func (w *Workspace) plainPlace(to string) (string, error) {
	repo, p, ok := strings.Cut(to, ":")
	if !ok {
		return "", &Invalid{Reason: fmt.Sprintf("%q isn't a place in a repo: it's repo:folder/", to)}
	}
	name, ok := w.WS().ResolveAlias(repo)
	if !ok {
		return "", &Invalid{Reason: "the workspace has no repo " + repo}
	}
	p = strings.TrimPrefix(strings.TrimSpace(p), "/")
	if slices.Contains(strings.Split(p, "/"), "..") {
		return "", &Invalid{Reason: fmt.Sprintf("%q leads out of %s", to, name)}
	}
	if p != "" && !strings.HasSuffix(p, "/") && !strings.HasSuffix(strings.ToLower(p), ".md") {
		p += "/"
	}
	return name + ":" + p, nil
}

// SavePage saves a page's text, edited from the version base: refused
// (page.ErrChanged) if it's changed since. It answers the version saved.
func (w *Workspace) SavePage(id, base string, text []byte) (string, error) {
	return w.pages().Write(id, base, text)
}

// RemovePage removes a page.
func (w *Workspace) RemovePage(id string) error { return w.pages().Remove(id) }

// Cell is a cell of a page, by its id: its conversation, and for a block
// of an outline (w12-b1), the block's key. conversation.ErrNotFound for
// one that isn't that page's.
func (w *Workspace) Cell(pageID, id string) (*conversation.Conversation, string, error) {
	if _, _, err := w.pages().Read(pageID); err != nil {
		return nil, "", err
	}
	outline, key := conversation.SplitBlockID(id)
	c, err := w.cells().Load(outline)
	if err != nil {
		return nil, "", err
	}
	if c.Kind != conversation.Cell || c.Anchor.Page != pageID {
		return nil, "", errors.Join(conversation.ErrNotFound, errors.New(c.ID+" isn't a cell of "+pageID))
	}
	if _, ok := c.BlockOf(key); key != "" && !ok {
		return nil, "", errors.Join(conversation.ErrNotFound, errors.New(c.ID+" planned no block "+key))
	}
	return c, key, nil
}

// CellAsk is you asking for a cell: what for (Prompt), and its goal (Ask
// for a question answered beside the page), what words of yours it's made
// around (Holds), or the pass's finding it asks about (Finding: it has no
// place in the page).
type CellAsk struct {
	Prompt, Goal, Holds, Finding string
	Ask                          bool
}

// OpenCell numbers a cell of a page, for the page to write into its text.
// It's filled once the page says so (FillCell), when the text it's in is
// saved for its agent to read.
func (w *Workspace) OpenCell(pageID string, a CellAsk) (*conversation.Conversation, error) {
	if _, _, err := w.pages().Read(pageID); err != nil {
		return nil, err
	}
	if a.Ask {
		a.Goal = conversation.GoalAsk
	}
	return w.cells().OpenCell(pageID, a.Prompt, a.Goal, a.Holds, a.Finding, w.now())
}

// FillCell has a cell's agent fill it (or write it again), if what's last
// said in it is yours. It answers the cell as it is now: marked as being
// answered.
func (w *Workspace) FillCell(c *conversation.Conversation) *conversation.Conversation {
	w.answered(job.Answer, doc.Scope{}, c)
	if again, err := w.cells().Load(c.ID); err == nil {
		return again
	}
	return c
}

// CellSay is you saying something in a cell: a change you'd have made
// (Edited, the cell as it reads in the page when you've edited it by
// hand), or, Ask, a question about it to be answered beside it.
type CellSay struct {
	Body, Edited string
	Ask          bool
}

// SayInCell is you saying something in a cell (said of a block of an
// outline, it's said in the outline's conversation, of that block: its
// one agent answers). Its agent answers, and it answers the cell as it is
// then.
func (w *Workspace) SayInCell(c *conversation.Conversation, key string, s CellSay) (*conversation.Conversation, error) {
	c, err := w.cells().Say(c.ID, conversation.Turn{By: conversation.You, At: w.now(), Body: s.Body, Edited: strings.TrimSpace(s.Edited), Ask: s.Ask, Block: key})
	if err != nil {
		return nil, err
	}
	return w.FillCell(c), nil
}

// PlacedCell is the page saying it's put an outline's blocks in: once.
func (w *Workspace) PlacedCell(c *conversation.Conversation) error {
	if c.Placed != nil {
		return nil
	}
	return w.cells().MarkPlaced(c.ID, w.now())
}

// Pass is a pass over a page, by name: its definition, its conversation
// (nil if it's never been run), and the page's text.
func (w *Workspace) Pass(pageID, name string) (page.PassDef, *conversation.Conversation, []byte, error) {
	text, _, err := w.pages().Read(pageID)
	if err != nil {
		return page.PassDef{}, nil, nil, err
	}
	def, ok := page.PassNamed(w.PassesOn(pageID), name)
	if !ok {
		return def, nil, nil, errors.Join(conversation.ErrNotFound, errors.New(pageID+" has no pass named "+name))
	}
	return def, w.cells().PassOn(pageID, def.Name), text, nil
}

// RunPass runs a pass over a page, the first time or again: its agent
// reads the page as it's saved now. One that's running isn't started
// twice. It answers the pass's conversation as it is then.
func (w *Workspace) RunPass(pageID, name string) (page.PassDef, *conversation.Conversation, []byte, error) {
	def, c, text, err := w.Pass(pageID, name)
	if err == nil && def.Unavailable != "" {
		err = errors.New(def.Title + " can't be run: " + def.Unavailable)
	}
	if err != nil {
		return def, nil, nil, err
	}
	switch {
	case c == nil:
		c, err = w.cells().OpenPass(pageID, def.Name, w.now())
	case !c.Answering:
		c, err = w.cells().Say(c.ID, conversation.Turn{By: conversation.You, At: w.now(), Body: "Run it again."})
	}
	if err != nil {
		return def, nil, nil, err
	}
	return def, w.FillCell(c), text, nil
}

// SettleFinding is you marking a pass's finding done or dismissed (and
// why), or opening it again.
func (w *Workspace) SettleFinding(pageID, name, key, state, why string) (page.PassDef, *conversation.Conversation, []byte, error) {
	def, c, text, err := w.Pass(pageID, name)
	if err == nil && c == nil {
		err = errors.Join(conversation.ErrNotFound, errors.New(def.Name+" hasn't been run over this page"))
	}
	if err != nil {
		return def, nil, nil, err
	}
	c, err = w.cells().SettleFinding(c.ID, key, state, why, w.now())
	return def, c, text, err
}
