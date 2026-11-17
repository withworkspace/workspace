package harness

import (
	"errors"
	"fmt"
	"os"
	"slices"
)

// DefaultHarness is what crew run when nothing configures another.
const DefaultHarness = "cursor"

// Pick is a harness, and the model to run on it ("" for the agent's own
// default), chosen for one role.
type Pick struct {
	Harness string
	Model   string
}

// ModelFor is the model to run on the named harness: the pick's model if
// that's the pick's harness, else the agent's own default. A model belongs
// to the harness it was configured for: Cursor and Claude Code name models
// differently, so picking another harness drops it.
func (p Pick) ModelFor(harnessName string) string {
	if harnessName == p.Harness {
		return p.Model
	}
	return ""
}

// String is the pick as [crew] allowed writes it: harness:model, or the
// harness alone with no model.
func (p Pick) String() string {
	if p.Model == "" {
		return p.Harness
	}
	return p.Harness + ":" + p.Model
}

// Roles is what's configured for each role agents play on a mission, from
// [crew], [briefing] and [orchestrator] in ws.toml / ws.local.toml. Empty
// fields fall back (see the For methods), so the zero value works.
type Roles struct {
	Crew, Briefing, Orchestrator, Conversation Pick
	// Team is who writes the Team page's gists, from [team].
	Team Pick
	// Judge is who judges a Work step's checks in words, from [judge].
	Judge Pick
	// CrewAllowed are the other picks crew may be launched on from a
	// mission's page ([crew] allowed); a pick with no model takes the
	// one ForCrew gives its harness, if any.
	CrewAllowed []Pick
}

// CrewPicks are what crew may be launched on from a mission's page: the
// default (ForCrew) first, then [crew] allowed's, each as it would run,
// none twice. Just the default when nothing else is allowed.
func (r Roles) CrewPicks() []Pick {
	def := r.ForCrew()
	out := []Pick{def}
	for _, p := range r.CrewAllowed {
		if p = r.crewPick(p); !slices.Contains(out, p) {
			out = append(out, p)
		}
	}
	return out
}

// AllowsCrew says whether crew may be launched on p from a mission's page:
// it's one of CrewPicks, once its model is filled in as it would run.
func (r Roles) AllowsCrew(p Pick) bool {
	return slices.Contains(r.CrewPicks(), r.crewPick(p))
}

// crewPick is p as crew would run on it: with no model, the one ForCrew
// gives its harness.
func (r Roles) crewPick(p Pick) Pick {
	if p.Model == "" {
		p.Model = r.ForCrew().ModelFor(p.Harness)
	}
	return p
}

// ForCrew picks the harness for crew doing the work: $WS_DECK_HARNESS, else
// [crew] harness, else cursor. [crew] model goes with [crew] harness.
func (r Roles) ForCrew() Pick {
	p := r.Crew
	if env := os.Getenv(EnvDefault); env != "" && env != p.Harness {
		p = Pick{Harness: env}
	}
	if p.Harness == "" {
		p.Harness = DefaultHarness
	}
	return p
}

// ForBriefing picks the harness for briefing agents: [briefing], else the
// crew's harness. A [briefing] model with no harness applies to the crew's.
func (r Roles) ForBriefing() Pick {
	if r.Briefing.Harness != "" {
		return r.Briefing
	}
	p := r.ForCrew()
	if r.Briefing.Model != "" {
		p.Model = r.Briefing.Model
	}
	return p
}

// ForOrchestrator picks the harness for the headless orchestrator, which
// must be able to run one: [orchestrator] harness, else the crew's harness
// if it can, else the first of cursor, claude and pi installed here. The model
// is [orchestrator] model, else the harness's own orchestrator default.
func (r Roles) ForOrchestrator() (Pick, Orchestrator, error) {
	if name := r.Orchestrator.Harness; name != "" {
		o, err := orchestratorNamed(name)
		if err != nil {
			return Pick{}, nil, err
		}
		return r.orchestratorPick(o), o, nil
	}
	if o, err := orchestratorNamed(r.ForCrew().Harness); err == nil {
		return r.orchestratorPick(o), o, nil
	}
	for _, name := range []string{"cursor", "claude", "pi"} {
		if o, err := orchestratorNamed(name); err == nil && o.Available() == nil {
			return r.orchestratorPick(o), o, nil
		}
	}
	return Pick{}, nil, errors.New("no harness here can run the headless orchestrator: install the Cursor CLI, Claude Code or pi, or set [orchestrator] harness")
}

func (r Roles) orchestratorPick(o Orchestrator) Pick {
	p := Pick{Harness: o.Name(), Model: r.Orchestrator.Model}
	if p.Model == "" {
		p.Model = o.OrchestratorModel()
	}
	return p
}

func orchestratorNamed(name string) (Orchestrator, error) {
	h, err := Get(name)
	if err != nil {
		return nil, err
	}
	o, ok := h.(Orchestrator)
	if !ok {
		return nil, fmt.Errorf("harness %q can't run the headless orchestrator", name)
	}
	return o, nil
}

// ForConversation picks the harness that answers threads (and, later,
// talks with you): [conversation] harness, else the crew's if it can hold
// a conversation, else Claude Code. The model is [conversation] model, else
// the harness's own conversation default.
func (r Roles) ForConversation() (Pick, Converser, error) {
	name := r.Conversation.Harness
	if name == "" {
		name = r.ForCrew().Harness
		if _, err := converserNamed(name); err != nil {
			name = "claude"
		}
	}
	c, err := converserNamed(name)
	if err != nil {
		return Pick{}, nil, err
	}
	p := Pick{Harness: c.Name(), Model: r.Conversation.Model}
	if p.Model == "" {
		p.Model = c.ConverseModel()
	}
	return p, c, nil
}

// ForTeam picks who writes the Team page's gists: [team] harness, else
// the one conversations run on. The model is [team] model, else the
// harness's small one (what it runs the orchestrator on): a gist is a
// sentence or two from a diff, many times over.
func (r Roles) ForTeam() (Pick, Converser, error) {
	name := r.Team.Harness
	if name == "" {
		p, _, err := r.ForConversation()
		if err != nil {
			return Pick{}, nil, err
		}
		name = p.Harness
	}
	c, err := converserNamed(name)
	if err != nil {
		return Pick{}, nil, err
	}
	p := Pick{Harness: c.Name(), Model: r.Team.Model}
	if o, ok := c.(Orchestrator); ok && p.Model == "" {
		p.Model = o.OrchestratorModel()
	}
	if p.Model == "" {
		p.Model = c.ConverseModel()
	}
	return p, c, nil
}

// ForBriefingConversation picks who briefs a mission in a conversation:
// [briefing]'s harness and model, if that harness can hold one, else
// [conversation]'s.
func (r Roles) ForBriefingConversation() (Pick, Converser, error) {
	if r.Briefing.Harness != "" {
		if c, err := converserNamed(r.Briefing.Harness); err == nil {
			p := Pick{Harness: c.Name(), Model: r.Briefing.Model}
			if p.Model == "" {
				p.Model = c.ConverseModel()
			}
			return p, c, nil
		}
	}
	return r.ForConversation()
}

// ForJudge picks who judges a Work step's checks in words: an agent with
// fresh context, headless and read-only. [judge] harness, else the
// planner's (ForBriefingConversation). The model is [judge] model, else,
// on the planner's harness, the planner's model, else the harness's
// conversation default: a judgement is worth the planner's model, not the
// orchestrator's small one.
func (r Roles) ForJudge() (Pick, Converser, error) {
	if r.Judge.Harness == "" {
		p, c, err := r.ForBriefingConversation()
		if err != nil {
			return Pick{}, nil, err
		}
		if r.Judge.Model != "" {
			p.Model = r.Judge.Model
		}
		return p, c, nil
	}
	c, err := converserNamed(r.Judge.Harness)
	if err != nil {
		return Pick{}, nil, err
	}
	p := Pick{Harness: c.Name(), Model: r.Judge.Model}
	if p.Model == "" {
		p.Model = c.ConverseModel()
	}
	return p, c, nil
}

func converserNamed(name string) (Converser, error) {
	h, err := Get(name)
	if err != nil {
		return nil, err
	}
	c, ok := h.(Converser)
	if !ok {
		return nil, fmt.Errorf("harness %q can't hold a conversation yet (Claude Code can): set [conversation] harness = \"claude\"", name)
	}
	return c, nil
}
