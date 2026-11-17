package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// Model is one a harness's agent can run on, as ws's config names it for
// that harness: what goes in a role's model.
type Model struct {
	ID    string `json:"id"`
	Name  string `json:"name,omitempty"`
	About string `json:"about,omitempty"`
	// Default is the agent's own default, where it says.
	Default bool `json:"default,omitempty"`
}

// ModelLister is a harness that can say which models its agent offers.
// Each asks its CLI (a second or so, no session): `cursor-agent
// --list-models`, `pi --list-models`, `codex debug models` (through
// codex-acp). Claude Code has no such command, so it lists its aliases.
type ModelLister interface {
	Models(ctx context.Context) ([]Model, error)
}

// modelsTimeout bounds asking a CLI for its models.
const modelsTimeout = 20 * time.Second

// runModelsCLI runs a CLI for its models; tests replace it.
var runModelsCLI = func(ctx context.Context, argv []string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, modelsTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, argv[0], argv[1:]...).Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok && len(ee.Stderr) > 0 {
			return nil, fmt.Errorf("%s: %w: %s", strings.Join(argv, " "), err, ClipHead(strings.TrimSpace(string(ee.Stderr)), 300, "…"))
		}
		return nil, fmt.Errorf("%s: %w", strings.Join(argv, " "), err)
	}
	return out, nil
}

// cursorModelLine is a line of `cursor-agent --list-models`: "id - Name",
// marked "(current, default)" where it is.
var cursorModelLine = regexp.MustCompile(`^(\S+) - (.+?)(?:\s+\(([^)]*)\))?$`)

// Models are Cursor's, by `cursor-agent --list-models`: its CLI's names
// (claude-haiku-5-5-medium), which those the account's plan doesn't
// include are among (VERIFIED: it lists them on a Free plan too).
func (c *Cursor) Models(ctx context.Context) ([]Model, error) {
	bin, err := c.bin()
	if err != nil {
		return nil, err
	}
	out, err := runModelsCLI(ctx, []string{bin, "--list-models"})
	if err != nil {
		return nil, err
	}
	var models []Model
	for _, line := range strings.Split(string(out), "\n") {
		m := cursorModelLine.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			continue
		}
		models = append(models, Model{ID: m[1], Name: m[2], Default: strings.Contains(m[3], "default")})
	}
	return models, nil
}

// Models are Cursor's, as ACP names them: each model once, its effort
// dropped (cursorACPModel).
func (c *CursorACP) Models(ctx context.Context) ([]Model, error) {
	cur, err := c.cli()
	if err != nil {
		return nil, err
	}
	all, err := cur.Models(ctx)
	if err != nil {
		return nil, err
	}
	var models []Model
	seen := map[string]bool{}
	for _, m := range all {
		id := cursorACPModel(m.ID)
		if seen[id] {
			continue
		}
		seen[id] = true
		name := m.Name
		if id != m.ID {
			name = id // its CLI name says the effort it's no longer at
		}
		models = append(models, Model{ID: id, Name: name, Default: m.Default})
	}
	return models, nil
}

// Models are Claude Code's aliases, each for its latest model; it takes a
// model's full name too, and has no command to list them.
func (c *Claude) Models(ctx context.Context) ([]Model, error) {
	return []Model{
		{ID: "opus", Name: "Opus", About: "the latest Opus: the most capable"},
		{ID: "sonnet", Name: "Sonnet", About: "the latest Sonnet: capable and quicker"},
		{ID: "haiku", Name: "Haiku", About: "the latest Haiku: small and quick"},
	}, nil
}

// Models are pi's, by `pi --list-models`: provider/model, from whichever
// providers you've signed in to.
func (p *Pi) Models(ctx context.Context) ([]Model, error) {
	bin, err := p.bin()
	if err != nil {
		return nil, err
	}
	out, err := runModelsCLI(ctx, []string{bin, "--list-models"})
	if err != nil {
		return nil, err
	}
	var models []Model
	for i, line := range strings.Split(string(out), "\n") {
		f := strings.Fields(line)
		if i == 0 || len(f) < 2 {
			continue // its header, or a blank line
		}
		about := ""
		if len(f) > 2 {
			about = f[2] + " context"
		}
		models = append(models, Model{ID: f[0] + "/" + f[1], Name: f[1], About: about})
	}
	return models, nil
}

// Models are Codex's, by its catalog (`codex debug models`, run as
// `codex-acp cli debug models`, so it's the adapter's own Codex): each
// model, then each at each effort it has, as ws names them
// ("gpt-6.1-sol[medium]"). Hidden ones (its auto reviewer) are left out.
func (c *Codex) Models(ctx context.Context) ([]Model, error) {
	agent, err := c.agent()
	if err != nil {
		return nil, err
	}
	out, err := runModelsCLI(ctx, append(agent, "cli", "debug", "models"))
	if err != nil {
		return nil, err
	}
	var cat struct {
		Models []struct {
			Slug       string `json:"slug"`
			Name       string `json:"display_name"`
			About      string `json:"description"`
			Visibility string `json:"visibility"`
			DefaultLvl string `json:"default_reasoning_level"`
			Levels     []struct {
				Effort string `json:"effort"`
				About  string `json:"description"`
			} `json:"supported_reasoning_levels"`
		} `json:"models"`
	}
	if err := json.Unmarshal(out, &cat); err != nil {
		return nil, fmt.Errorf("reading Codex's models: %w", err)
	}
	var models []Model
	for _, m := range cat.Models {
		if m.Visibility == "hide" {
			continue
		}
		about := m.About
		if m.DefaultLvl != "" {
			about += " Its default effort: " + m.DefaultLvl + "."
		}
		models = append(models, Model{ID: m.Slug, Name: m.Name, About: strings.TrimSpace(about)})
		for _, l := range m.Levels {
			models = append(models, Model{ID: m.Slug + "[" + l.Effort + "]", Name: m.Name + ", " + l.Effort, About: l.About})
		}
	}
	return models, nil
}
