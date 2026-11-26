package harness

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/acp"
)

// Model is one a harness's agent can run on, as ws's config names it for
// that harness: what goes in a role's model.
type Model struct {
	ID    string `json:"id"`
	Name  string `json:"name,omitempty"`
	About string `json:"about,omitempty"`
	// Default is the agent's own default, where it says.
	Default bool `json:"default,omitempty"`

	// Where it sits among the others, where its name says (a picker's
	// tree): its maker, then the model (its family and version, shown as
	// Label: "Claude Opus 5.5"), then what this one chooses of that
	// model: an effort ("" is the model's own), thinking, fast. Those a
	// name says nothing of are left empty, and it's a model of its own.
	Vendor   string `json:"vendor,omitempty"`
	Family   string `json:"family,omitempty"`
	Version  string `json:"version,omitempty"`
	Label    string `json:"label,omitempty"`
	Effort   string `json:"effort,omitempty"`
	Thinking bool   `json:"thinking,omitempty"`
	Fast     bool   `json:"fast,omitempty"`
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
		models = append(models, placed(Model{ID: m[1], Name: m[2], Default: strings.Contains(m[3], "default")}))
	}
	return models, nil
}

// Models are Cursor's as its ACP offers them: a session's model setting,
// one value per model at the one effort ACP runs it at
// ("claude-haiku-5-5[thinking=true,context=300k,reasoning_effort=high]"),
// each a model of its own named by its bare name, which is what it's
// picked by (acp.ConfigOption.Pick). Its effort, thinking and fast are
// said, not chosen: ACP takes no other (see CursorACP). Not the CLI's
// list: of its 58 names once efforts are dropped, 20 (every -thinking
// one, and older spellings) aren't ACP's, and 11 of ACP's aren't among
// them (cursor-agent 2026.11.05).
//
// It opens a session to ask, no turn run (a few seconds), takes how each
// is shown from the CLI's list (`--list-models`), and removes the
// folder Cursor keeps it in, as a session that ran nothing can't be
// loaded anyway (sessionKept).
func (c *CursorACP) Models(ctx context.Context) ([]Model, error) {
	agent, err := c.agent()
	if err != nil {
		return nil, err
	}
	dir, err := os.MkdirTemp("", "ws-cursor-models-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	s, err := openACPSession(ctx, acp.Command{Args: agent, Dir: dir}, cursorLogin, dir)
	if err != nil {
		return nil, fmt.Errorf("asking Cursor's ACP for its models: %w", err)
	}
	if sd := c.sessionsDir(); sd != "" && s.ID != "" && !strings.ContainsAny(s.ID, `/\`) && !strings.HasPrefix(s.ID, ".") && !c.sessionKept(s.ID) {
		os.RemoveAll(filepath.Join(sd, s.ID))
	}
	opt := s.Option("model")
	if opt == nil {
		return nil, errors.New("Cursor's ACP offered no model setting")
	}
	// ACP names a model by its id alone; the CLI's list has it as it's
	// shown ("Claude Haiku 5.5"), where the names meet.
	labels := map[string]string{}
	if cur, err := c.cli(); err == nil {
		if all, err := cur.Models(ctx); err == nil {
			for _, m := range all {
				if id := cursorACPModel(m.ID); labels[id] == "" && m.Label != "" {
					labels[id] = m.Label
				}
			}
		}
	}
	var models []Model
	for _, v := range opt.Values {
		m := cursorACPOffered(v)
		if l := labels[m.ID]; l != "" && m.Vendor != "" {
			m.Label = l
		} else if m.Vendor != "" {
			m.Label = idLabel(m.ID)
		}
		models = append(models, m)
	}
	return models, nil
}

// cursorACPOffered is one of the models Cursor's ACP offers, as a Model:
// its bare name, its place in the tree, and what its value fixes.
func cursorACPOffered(v acp.ConfigValue) Model {
	bare, params, _ := strings.Cut(v.Value, "[")
	if bare == "default" {
		return Model{ID: "auto", Name: v.Name, Default: true, About: "Cursor picks the model."}
	}
	m := placed(Model{ID: bare})
	m.Name, m.Effort, m.Thinking, m.Fast = v.Name, "", false, false
	context := ""
	for _, p := range strings.Split(strings.TrimSuffix(params, "]"), ",") {
		k, val, _ := strings.Cut(p, "=")
		switch k {
		case "effort", "reasoning_effort", "reasoning":
			if val == "extra-high" {
				val = "xhigh"
			}
			if isModelEffort(val) {
				m.Effort = val
			}
		case "thinking":
			m.Thinking = val == "true"
		case "fast":
			m.Fast = val == "true"
		case "context":
			context = val
		}
	}
	var said []string
	if m.Effort != "" {
		said = append(said, m.Effort+" effort")
	}
	if m.Thinking {
		said = append(said, "thinking")
	}
	if m.Fast {
		said = append(said, "fast")
	}
	if context != "" {
		said = append(said, context+" context")
	}
	if len(said) > 0 {
		m.About = "Over ACP, Cursor runs it only as: " + strings.Join(said, ", ") + "."
	}
	return m
}

// idLabel is a model's id as it might be shown, for one no list shows:
// "claude-sonnet-4-6" is "Claude Sonnet 4.6", "glm-5p3-flash" "GLM 5p3
// Flash".
func idLabel(id string) string {
	var words []string
	for _, w := range strings.Split(id, "-") {
		n := len(words)
		switch {
		case n > 0 && isDigits(w) && isVersion(words[n-1]):
			words[n-1] += "." + w // 4-6 is 4.6
		case strings.EqualFold(w, "gpt") || strings.EqualFold(w, "glm"):
			words = append(words, strings.ToUpper(w))
		case w != "":
			words = append(words, strings.ToUpper(w[:1])+w[1:])
		}
	}
	return strings.Join(words, " ")
}

func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}

// isVersion is a version's first part: digits, then at most a dot's more.
func isVersion(s string) bool {
	return isDigits(strings.ReplaceAll(s, ".", "")) && strings.Count(s, ".") == 0
}

// openACPSession starts an agent, signs in (auth, if any), opens a session
// in dir and ends the agent: the session as it was offered. Tests replace
// it.
var openACPSession = func(ctx context.Context, cmd acp.Command, auth, dir string) (acp.Session, error) {
	ctx, cancel := context.WithTimeout(ctx, modelsTimeout)
	defer cancel()
	cl, err := acp.Start(ctx, cmd, acp.Callbacks{})
	if err != nil {
		return acp.Session{}, err
	}
	defer cl.Close(2 * time.Second) //nolint:errcheck // done with it
	if auth != "" {
		if err := cl.Authenticate(ctx, auth); err != nil {
			return acp.Session{}, fmt.Errorf("signing in (%s): %w", auth, err)
		}
	}
	return cl.NewSession(ctx, acp.SessionParams{Dir: dir})
}

// Models are Claude Code's aliases, each for its latest model, then each
// at each effort it takes (ClaudeEfforts), as ws names them
// ("haiku[high]"): one model in a picker, its efforts chosen under it. It
// takes a model's full name too, and has no command to list them.
//
// VERIFIED (claude 2.1.332): opus, sonnet and haiku each run at every
// effort, xhigh and max too.
func (c *Claude) Models(ctx context.Context) ([]Model, error) {
	var models []Model
	for _, m := range []Model{
		{ID: "opus", Name: "Opus", About: "the latest Opus: the most capable"},
		{ID: "sonnet", Name: "Sonnet", About: "the latest Sonnet: capable and quicker"},
		{ID: "haiku", Name: "Haiku", About: "the latest Haiku: small and quick"},
	} {
		m.Label = m.Name
		models = append(models, m)
		for _, e := range ClaudeEfforts {
			one := m
			one.ID, one.Name, one.Effort = m.ID+"["+e+"]", m.Name+", "+e, e
			models = append(models, one)
		}
	}
	return models, nil
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
		// Each is the model at an effort: one model in a picker.
		at := placed(Model{ID: m.Slug})
		at.Label = m.Name
		one := at
		one.ID, one.Name, one.About = m.Slug, m.Name, strings.TrimSpace(about)
		models = append(models, one)
		for _, l := range m.Levels {
			one := at
			one.ID, one.Name, one.About, one.Effort = m.Slug+"["+l.Effort+"]", m.Name+", "+l.Effort, l.About, l.Effort
			models = append(models, one)
		}
	}
	return models, nil
}
