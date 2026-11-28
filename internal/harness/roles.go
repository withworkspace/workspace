package harness

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
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
	// PRWriter is who writes the pull requests ws opens, from [pr_writer].
	PRWriter Pick
	// Lessons is who reads crew's handoffs for lessons, from [lessons].
	Lessons Pick
	// Automation is who answers an automation's ask, from [automations].
	Automation Pick
	// CrewAllowed are the other picks crew may be launched on from a
	// mission's page ([crew] allowed); a pick with no model takes the
	// one ForCrew gives its harness, if any.
	CrewAllowed []Pick
	// CrewLevels are what crew run on at each level of a plan's Work
	// step ([crew.levels]): by level, as configured, harness:model or a
	// model alone. A level not set falls back (ForLevel).
	CrewLevels map[string]string
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
	for _, l := range levelNames {
		if p := r.ForLevel(l); !slices.Contains(out, p) {
			out = append(out, p)
		}
	}
	return out
}

// levelNames are the levels a Work step may be at, least first (as
// mission.Levels has them).
var levelNames = []string{"rote", "normal", "hard"}

// ForLevel is what crew run on for a Work step at level: [crew.levels]'s
// pick for it, else, for rote, crew's harness on its small model (haiku
// on Claude Code; what it orchestrates on, for another), and for normal
// and hard, crew's own (ForCrew). A harness with no small model of its
// own runs rote on crew's own too.
func (r Roles) ForLevel(level string) Pick {
	if raw := r.CrewLevels[level]; raw != "" {
		if name, model, ok := strings.Cut(raw, ":"); ok {
			if _, err := Get(name); err == nil {
				return r.crewPick(Pick{Harness: name, Model: model})
			}
		}
		return Pick{Harness: r.ForCrew().Harness, Model: raw}
	}
	crew := r.ForCrew()
	if level != "rote" {
		return crew
	}
	h, err := Get(crew.Harness)
	if err != nil {
		return crew
	}
	small := ""
	if s, ok := h.(SmallModeler); ok {
		small = s.SmallModel()
	}
	if o, ok := h.(Orchestrator); ok && small == "" {
		small = o.OrchestratorModel()
	}
	if small == "" {
		return crew
	}
	return Pick{Harness: crew.Harness, Model: small}
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
	return Pick{}, nil, errors.New("no harness can run the orchestrator: install the Cursor CLI, Claude Code or pi, or set [orchestrator] harness")
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
		return nil, fmt.Errorf("harness %q cannot run the orchestrator", name)
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
// harness's small one (smallModel): a gist is a sentence or two from a
// diff, many times over.
func (r Roles) ForTeam() (Pick, Converser, error) {
	name := r.Team.Harness
	if name == "" {
		p, _, err := r.ForConversation()
		if err != nil {
			return Pick{}, nil, err
		}
		name = p.Harness
	}
	return pickOn(name, r.Team.Model, smallModel)
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
// planner's (ForBriefingConversation). The model is [judge] model, else
// the harness's small one (smallModel): it reads a step's work against
// its checks, which ws's evals found the small models judge as well as
// the large, and no better for more effort.
func (r Roles) ForJudge() (Pick, Converser, error) {
	name := r.Judge.Harness
	if name == "" {
		p, _, err := r.ForBriefingConversation()
		if err != nil {
			return Pick{}, nil, err
		}
		name = p.Harness
	}
	return pickOn(name, r.Judge.Model, smallModel)
}

// ForPRWriter picks who writes a pull request's title and description:
// headless, read-only, with fresh context. [pr_writer] harness, else
// conversations' (ForConversation). The model is [pr_writer] model, else
// the one the harness runs the orchestrator on (orchestratorModel): ws's
// evals found the small models write them as well as the large, and
// Claude Code's Haiku better at the high effort it orchestrates at.
func (r Roles) ForPRWriter() (Pick, Converser, error) {
	name := r.PRWriter.Harness
	if name == "" {
		p, _, err := r.ForConversation()
		if err != nil {
			return Pick{}, nil, err
		}
		name = p.Harness
	}
	return pickOn(name, r.PRWriter.Model, orchestratorModel)
}

// ForAutomation picks who answers an automation's ask: one turn, no
// tools, fresh context. [automations] harness, else conversations'
// (ForConversation). The model is [automations] model, else the
// harness's small one (smallModel): an ask is a short question with its
// facts given, as a judge's is, and each spends from a daily budget.
func (r Roles) ForAutomation() (Pick, Converser, error) {
	name := r.Automation.Harness
	if name == "" {
		p, _, err := r.ForConversation()
		if err != nil {
			return Pick{}, nil, err
		}
		name = p.Harness
	}
	return pickOn(name, r.Automation.Model, smallModel)
}

// ForLessons picks who reads a crew's handoff for what crew keep running
// into where they work (internal/lessons): one turn, no tools, fresh
// context. [lessons] harness, else conversations' (ForConversation). The
// model is [lessons] model, else the harness's small one (smallModel):
// it sorts a few lines against what's known, once for each handoff, and
// ws's offline pass over recorded handoffs found the small models find
// the same lessons the large do.
func (r Roles) ForLessons() (Pick, Converser, error) {
	name := r.Lessons.Harness
	if name == "" {
		p, _, err := r.ForConversation()
		if err != nil {
			return Pick{}, nil, err
		}
		name = p.Harness
	}
	return pickOn(name, r.Lessons.Model, smallModel)
}

// pickOn is the named harness, on model, else on the one dflt gives it.
func pickOn(name, model string, dflt func(Converser) string) (Pick, Converser, error) {
	c, err := converserNamed(name)
	if err != nil {
		return Pick{}, nil, err
	}
	p := Pick{Harness: c.Name(), Model: model}
	if p.Model == "" {
		p.Model = dflt(c)
	}
	return p, c, nil
}

// SmallModeler is a harness whose small model, for roles that don't think
// at length, isn't the one it orchestrates on: Claude Code orchestrates at
// a higher effort.
type SmallModeler interface {
	SmallModel() string
}

// smallModel is a harness's small model: its own say (SmallModeler), else
// what it orchestrates on.
func smallModel(c Converser) string {
	if s, ok := c.(SmallModeler); ok && s.SmallModel() != "" {
		return s.SmallModel()
	}
	return orchestratorModel(c)
}

// orchestratorModel is what a harness runs the orchestrator on, or its
// conversation default if it runs none.
func orchestratorModel(c Converser) string {
	if o, ok := c.(Orchestrator); ok && o.OrchestratorModel() != "" {
		return o.OrchestratorModel()
	}
	return c.ConverseModel()
}

func converserNamed(name string) (Converser, error) {
	h, err := Get(name)
	if err != nil {
		return nil, err
	}
	c, ok := h.(Converser)
	if !ok {
		return nil, fmt.Errorf("harness %q cannot hold conversations: set [conversation] harness = \"claude\"", name)
	}
	return c, nil
}
