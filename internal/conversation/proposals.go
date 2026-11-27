package conversation

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/withworkspace/plugin/protocol"
	"github.com/withworkspace/workspace/internal/automation"
	"github.com/withworkspace/workspace/internal/events"
	"github.com/withworkspace/workspace/internal/store"
)

// A conversation's agent changes nothing, but proposes: a decision, a mission (briefed, or an idea to brief later), a
// document of one of the workspace's kinds or an edit to one, an artifact or a version of one, a replan by the mission's
// planner, each shown as a button under its reply, for you to confirm or decline. Confirming does it through ws's verbs,
// as you.

// Proposal kinds.
const (
	ProposeDecision = "decision"
	ProposeMission  = "mission"
	ProposeBrief    = "brief"    // an edit to the mission's brief
	ProposeDispatch = "dispatch" // crew to start on the mission
	ProposeComment  = "comment"  // a review comment on a pull request's lines
	ProposePlan     = "plan"     // an edit to the mission's plan
	ProposeDocument = "document" // a document of one of the workspace's kinds, added where its kind says, or (To) an edit to one
	ProposeArtifact = "artifact" // an artifact, or a version of one: an HTML file, run in a sandboxed frame
	ProposeReplan   = "replan"   // the mission's planner asked to revise its plan, as ws mission replan asks it
	ProposePlugin   = "plugin"   // something of a plugin's (issues to create), drafted by the plugin, done by it once accepted
	// ProposeAutomation is an automation (internal/automation): its script
	// in Text, what it is and would have done lately in Automation; added,
	// on, once accepted.
	ProposeAutomation = "automation"
)

// Proposal states.
const (
	ProposalOpen     = "open"
	ProposalAccepted = "accepted"
	ProposalDeclined = "declined"
	// ProposalSuperseded: a later one of the agent's took its place (its
	// Result is the later one's number): the same brief or plan drafted
	// again, a mission or document of the same title proposed again.
	ProposalSuperseded = "superseded"
	// ProposalWithdrawn: the agent took it back, as what the user said
	// since ruled it out (Because says how).
	ProposalWithdrawn = "withdrawn"
)

// supersedes says whether proposal p, made after q, takes its place: the
// brief drafted again (with its plan, the plan too), the plan again, a
// mission or a document of the same title (and kind) proposed again, or
// an edit to the same document drafted again.
func (p Proposal) supersedes(q Proposal) bool {
	same := func(a, b string) bool {
		return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b)) && strings.TrimSpace(a) != ""
	}
	switch p.Kind {
	case ProposeBrief:
		return q.Kind == ProposeBrief || (p.Plan != "" && q.Kind == ProposePlan)
	case ProposePlan:
		return q.Kind == ProposePlan
	case ProposeMission:
		return q.Kind == ProposeMission && same(p.Title, q.Title)
	case ProposePlugin:
		return q.Kind == ProposePlugin && q.Plugin == p.Plugin && q.Proposes == p.Proposes && same(p.Title, q.Title) && !q.anyMade()
	case ProposeDocument:
		if p.To != "" {
			return q.Kind == ProposeDocument && q.To == p.To
		}
		return q.Kind == ProposeDocument && q.To == "" && p.DocKind == q.DocKind && same(p.Title, q.Title)
	case ProposeAutomation:
		return q.Kind == ProposeAutomation && p.Automation != nil && q.Automation != nil && same(p.Automation.Name, q.Automation.Name)
	}
	return false
}

// ProposedFile is one file of an artifact proposed with files: its path
// and its size.
type ProposedFile struct {
	Path string `json:"path"`
	Size int    `json:"size"`
}

// Proposal is something the agent proposed in a conversation.
type Proposal struct {
	N    int       `json:"n"`
	Kind string    `json:"kind"`
	By   string    `json:"by"`
	At   time.Time `json:"at"`
	// A decision: what, and why.
	What string `json:"what,omitempty"`
	Why  string `json:"why,omitempty"`
	// A mission: its title, and its brief ("": an idea, to brief later),
	// and the capsules (repo/branch) to lift for it. An edit to the brief:
	// the brief as it should read (Brief), and as it read (Was).
	Title    string   `json:"title,omitempty"`
	Brief    string   `json:"brief,omitempty"`
	Was      string   `json:"was,omitempty"`
	Capsules []string `json:"capsules,omitempty"`
	// A mission: what it's about, by names ws reads (one of a plugin's
	// things, a pull request), as its record will say once it's created.
	About []string `json:"about,omitempty"`
	// Briefing a mission: existing work to build on (repo/branch or
	// repo#PR, docked), and a better slug for it.
	Docks []string `json:"docks,omitempty"`
	Slug  string   `json:"slug,omitempty"`
	// A dispatch: what the crew's to do, the capsule (repo/name) it works
	// in ("" for one no crew is in), and what it should read first. A
	// replan: what to change (Text), asked of the planner as you; accepted,
	// its Result is the planner's conversation.
	Text    string   `json:"text,omitempty"`
	Capsule string   `json:"capsule,omitempty"`
	Reads   []string `json:"reads,omitempty"`
	// A review comment: the pull request (api#89), the lines it's on (Path,
	// Line to EndLine, on Side: "new", or "old" for lines the PR removes),
	// its kind, what it says (Text) and a suggested replacement for the
	// lines. Accepting adds it to your draft review of the PR.
	PR         string  `json:"pr,omitempty"`
	Path       string  `json:"path,omitempty"`
	Line       int     `json:"line,omitempty"`
	EndLine    int     `json:"endLine,omitempty"`
	Side       string  `json:"side,omitempty"`
	NoteKind   string  `json:"noteKind,omitempty"`
	Suggestion *string `json:"suggestion,omitempty"`
	// Excerpt is the lines it's on, as they read when it was proposed.
	Excerpt []string `json:"excerpt,omitempty"`
	// An edit to the mission's plan: the plan as it should read (Plan), and
	// as it read (Was). Briefing a mission, its brief comes with its plan
	// (Plan, and PlanWas as the plan read), confirmed together.
	Plan    string `json:"plan,omitempty"`
	PlanWas string `json:"planWas,omitempty"`
	// A revision of the plan the planner proposed because it's the user's
	// to confirm (Why says what makes it theirs), and the planner's step it
	// was written at (Step; 0 for none): accepting it passes that step.
	Step int `json:"step,omitempty"`
	// A document: the kind it's of (its name in ws.toml), its text (Text),
	// the guide it was written to (the kind's, as ws.toml names it, which
	// the agent was shown before it could propose), and, as worked out when
	// it was proposed, its title (its first heading), where it would go
	// (Path, repo:path) and the first line of the commit that adds it
	// (Subject). Accepting works the last three out again, from the text as
	// it's accepted.
	//
	// An edit to a document that's there (To, its ref, repo:path, as Path
	// is too) is the whole of it as it should read (Text), with the whole
	// of it as it read when it was proposed (Was), which accepting merges
	// from, as a page's save does, if it's changed since: what changed
	// meanwhile stands. Subject is then the commit's that changes it, and
	// Title its first heading: it isn't moved, whatever that says.
	DocKind string `json:"docKind,omitempty"`
	Guide   string `json:"guide,omitempty"`
	Subject string `json:"subject,omitempty"`
	// An artifact: its page (HTML), or, for one that's more than its page,
	// the commit that keeps its files until it's added (Tree, in the repo
	// beside the artifacts) and what they are (Files); and its note (what
	// it tries out, or what changed and why). A new one has its title
	// (Title); a version of one names the artifact (To, a3), the version
	// it's based on (On), and the artifact's latest version as it was
	// proposed (Latest): one added since makes it out of date, though it's
	// still added, based on On. As worked out when it was proposed: where
	// it would go (Path), its size (Size), and what in it won't load in
	// the frame (Warnings). Accepting adds it as you, from this
	// conversation, and Result is the version it made (a3.v1).
	HTML   string         `json:"html,omitempty"`
	Tree   string         `json:"tree,omitempty"`
	Files  []ProposedFile `json:"files,omitempty"`
	Note   string         `json:"note,omitempty"`
	To     string         `json:"to,omitempty"`
	On     int            `json:"on,omitempty"`
	Latest int            `json:"latest,omitempty"`
	Size   int            `json:"size,omitempty"`
	// Automation is a proposed automation, as its card shows it (its
	// script is Text).
	Automation *automation.Proposed `json:"automation,omitempty"`
	Warnings   []string             `json:"warnings,omitempty"`

	// A plugin's: the plugin (Plugin), which of what it proposes (Proposes,
	// its id), what the card calls that (Label), what the agent wrote
	// (Input, in the plugin's own shape), and what the plugin drafted of
	// it: Title for the whole, and its Items, each accepted on its own or
	// with the rest. Result, once it's settled, is what its items made.
	Plugin   string          `json:"plugin,omitempty"`
	Proposes string          `json:"proposes,omitempty"`
	Label    string          `json:"label,omitempty"`
	Icon     string          `json:"icon,omitempty"`
	Input    json.RawMessage `json:"input,omitempty"`
	Items    []PluginItem    `json:"items,omitempty"`

	// Revises is the proposal this one revises, by its number: the agent
	// asked to change it, it proposed it again, as it should now be, and
	// that one's superseded by this. 0: it revises none.
	Revises int `json:"revises,omitempty"`

	State string `json:"state"`
	// Because is why the agent withdrew it.
	Because string `json:"because,omitempty"`
	// Result is what accepting it made: a mission's slug, a decision's
	// ref in its mission (slug:d3), a document's ref (repo:path, or
	// repo/capsule:path where it was left in a capsule), or an artifact's
	// version (a3.v1); "" for a decision kept with a workspace
	// conversation until it becomes a mission.
	Result    string     `json:"result,omitempty"`
	SettledBy string     `json:"settledBy,omitempty"`
	SettledAt *time.Time `json:"settledAt,omitempty"`
}

// PluginItem is one thing a plugin's proposal does, as the plugin drafted
// it, and what came of it: open (State ""), made (what it made or
// changed, by name, in Made), or declined or withdrawn with the rest.
// Problem is why it wasn't done the last time it was accepted: it's open
// still, to accept again.
type PluginItem struct {
	protocol.ProposalItem
	State   string `json:"state,omitempty"`
	Made    string `json:"made,omitempty"`
	Say     string `json:"say,omitempty"`
	Open    string `json:"open,omitempty"`
	Problem string `json:"problem,omitempty"`
}

// The states of a plugin's proposal's item, past open ("").
const (
	ItemMade = "made"
	// ItemLeft is an item the user didn't accept with the rest, declined
	// or withdrawn with what was left open.
	ItemLeft = "left"
)

// ItemOutcome is what came of an item asked to be done: as the plugin
// answered (protocol.AcceptedItem), or left.
type ItemOutcome struct {
	N       int    `json:"n"`
	State   string `json:"state,omitempty"`
	Made    string `json:"made,omitempty"`
	Say     string `json:"say,omitempty"`
	Open    string `json:"open,omitempty"`
	Problem string `json:"problem,omitempty"`
}

// itemsDone is what came of a plugin's proposal's items, once asked.
type itemsDone struct {
	N     int           `json:"n"`
	Items []ItemOutcome `json:"items"`
	At    time.Time     `json:"at"`
}

// OpenItems are the numbers of a plugin's proposal's items still open.
func (p Proposal) OpenItems() []int {
	var out []int
	for i, it := range p.Items {
		if it.State == "" {
			out = append(out, i+1)
		}
	}
	return out
}

// MadeByItem is what each item made, by number.
func (p Proposal) MadeByItem() map[int]string {
	out := map[int]string{}
	for i, it := range p.Items {
		if it.State == ItemMade && it.Made != "" {
			out[i+1] = it.Made
		}
	}
	return out
}

// made is what a plugin's proposal's items made, in order.
func (p Proposal) made() []string {
	var out []string
	for _, it := range p.Items {
		if it.State == ItemMade && it.Made != "" {
			out = append(out, it.Made)
		}
	}
	return out
}

// anyMade is whether any of a plugin's proposal's items was done.
func (p Proposal) anyMade() bool {
	return slices.ContainsFunc(p.Items, func(it PluginItem) bool { return it.State == ItemMade })
}

// applyItems is what came of items happening to a proposal.
func (p *Proposal) applyItems(out []ItemOutcome) {
	for _, o := range out {
		if o.N < 1 || o.N > len(p.Items) {
			continue
		}
		it := &p.Items[o.N-1]
		if it.State != "" {
			continue // done already, or left
		}
		it.State, it.Made, it.Say, it.Open, it.Problem = o.State, o.Made, o.Say, o.Open, o.Problem
	}
}

// ItemsDone records what came of a plugin's proposal's items, asked to be
// done (or left), and settles it once none is open: accepted, with what
// they made, if any was done; declined otherwise. An item that wasn't
// done (a Problem, no State) stays open.
func (s Store) ItemsDone(id string, n int, out []ItemOutcome, by string, at time.Time) (*Conversation, error) {
	if by != You && !IsAutomation(by) {
		return nil, errors.New("only the user accepts or declines a proposal, or an automation they let do it")
	}
	return s.update(id, func(tx *store.Tx, c *Conversation) error {
		p := c.Proposal(n)
		switch {
		case p == nil:
			return fmt.Errorf("%w: %s has no proposal %d", ErrNotFound, id, n)
		case p.Kind != ProposePlugin:
			return fmt.Errorf("proposal %d in %s isn't a plugin's", n, id)
		case p.State != ProposalOpen:
			return fmt.Errorf("proposal %d in %s is %s already", n, id, p.State)
		}
		if err := s.add(tx, c, line{Items: &itemsDone{N: n, Items: out, At: at}}); err != nil {
			return err
		}
		p = c.Proposal(n)
		if len(p.OpenItems()) > 0 {
			return nil
		}
		state := ProposalDeclined
		if p.anyMade() {
			state = ProposalAccepted
		}
		return s.add(tx, c, line{Proposal: &proposalSettled{N: n, State: state, Result: strings.Join(p.made(), " "), By: by, At: at}})
	})
}

// ByAutomation is who an automation is, accepting what it was let do
// without asking (act): automation:<name>.
func ByAutomation(name string) string { return "automation:" + name }

// IsAutomation is whether by is an automation's.
func IsAutomation(by string) bool {
	return strings.HasPrefix(by, "automation:") && len(by) > len("automation:")
}

// ProposalOrigin is the Origin of the turn ws writes for you settling
// proposal n: "proposal:3".
func ProposalOrigin(n int) string { return "proposal:" + strconv.Itoa(n) }

// OriginProposal is the proposal a turn was written for settling (its
// Origin's number), or 0 for a turn that wasn't.
func (t Turn) OriginProposal() int {
	n, ok := strings.CutPrefix(t.Origin, "proposal:")
	if !ok {
		return 0
	}
	i, _ := strconv.Atoi(n)
	return i
}

// Wakes is whether settling a proposal of a kind, accepted or declined
// (state), wakes the conversation's agent to answer at once. Declining
// anything does: a decline is feedback, usually, for it to take in.
// Accepting a brief or a plan does: what comes of it is the agent's to
// say. Accepting anything else (a mission, a decision, a document, crew, a
// replan, which the planner takes up, a review comment, an artifact) is
// done with, and needs nothing of it: the turn written for it is quiet,
// and read on the agent's next turn.
func Wakes(kind, state string) bool {
	if state == ProposalDeclined {
		return true
	}
	return kind == ProposeBrief || kind == ProposePlan
}

// proposalSettled is a proposal accepted (with what it made) or declined,
// or a decision kept with the conversation moved to its mission.
type proposalSettled struct {
	N       int       `json:"n"`
	State   string    `json:"state"`
	Result  string    `json:"result,omitempty"`
	Because string    `json:"because,omitempty"`
	By      string    `json:"by"`
	At      time.Time `json:"at"`
}

// Became is the mission a conversation became: the latest mission
// proposal accepted in it, or "".
func (c *Conversation) Became() string {
	for _, p := range slices.Backward(c.Proposals) {
		if p.Kind == ProposeMission && p.State == ProposalAccepted {
			return p.Result
		}
	}
	return ""
}

// Proposal is the proposal numbered n, or nil.
func (c *Conversation) Proposal(n int) *Proposal {
	for i := range c.Proposals {
		if c.Proposals[i].N == n {
			return &c.Proposals[i]
		}
	}
	return nil
}

// Propose records something proposed in an open conversation.
func (s Store) Propose(id string, p Proposal) (*Conversation, error) {
	p.What, p.Why, p.Title, p.Text = strings.TrimSpace(p.What), strings.TrimSpace(p.Why), strings.TrimSpace(p.Title), strings.TrimSpace(p.Text)
	p.Note, p.To = strings.TrimSpace(p.Note), strings.TrimSpace(p.To)
	if p.Kind != ProposeBrief {
		p.Brief = strings.TrimSpace(p.Brief)
	}
	switch {
	case p.Kind == ProposeDecision && p.What == "":
		return nil, errors.New("say what was decided")
	case p.Kind == ProposeMission && p.Title == "":
		return nil, errors.New("say what the mission is")
	case p.Kind == ProposeBrief && strings.TrimSpace(p.Brief) == "":
		return nil, errors.New("the brief proposed is empty")
	case p.Kind == ProposeBrief && p.Brief == p.Was && p.Plan == "":
		// With a plan, the brief as it is is what's confirmed with it.
		return nil, errors.New("the brief proposed is the brief as it is: propose the plan, and the brief comes with it")
	case p.Kind == ProposeDispatch && p.Text == "":
		return nil, errors.New("say what the crew's to do")
	case p.Kind == ProposeComment && (p.PR == "" || p.Path == "" || p.Line < 1):
		return nil, errors.New("a comment is on lines of a pull request: its ref, a path and a line")
	case p.Kind == ProposeComment && p.Text == "" && p.Suggestion == nil:
		return nil, errors.New("say something, or suggest a change")
	case p.Kind == ProposePlan && strings.TrimSpace(p.Plan) == "":
		return nil, errors.New("the plan proposed is empty")
	case p.Kind == ProposePlan && p.Plan == p.Was:
		return nil, errors.New("the plan proposed is the plan as it is")
	case p.Kind == ProposeDocument && (p.DocKind == "" || p.Text == ""):
		return nil, errors.New("a document is of one of the workspace's kinds, and has something in it")
	case p.Kind == ProposeDocument && p.To != "" && p.Text == strings.TrimSpace(p.Was):
		return nil, errors.New("the document proposed is the document as it is: change something in it")
	case p.Kind == ProposeArtifact && strings.TrimSpace(p.HTML) == "" && p.Tree == "":
		return nil, errors.New("an artifact is an HTML page, and this one has nothing in it")
	case p.Kind == ProposeArtifact && p.To == "" && p.Title == "":
		return nil, errors.New("a new artifact has a title")
	case p.Kind == ProposeArtifact && p.Note == "":
		return nil, errors.New("an artifact's version has a note: what it tries out, or what changed and why")
	case p.Kind == ProposeAutomation && (p.Automation == nil || p.Automation.Name == "" || strings.TrimSpace(p.Text) == ""):
		return nil, errors.New("an automation has a name and its script")
	case p.Kind == ProposeReplan && p.Text == "":
		return nil, errors.New("say what to change in the plan, or what doesn't fit it")
	case p.Kind == ProposePlugin && (p.Plugin == "" || p.Proposes == "" || len(p.Items) == 0):
		return nil, errors.New("a plugin's proposal is of something it proposes, and has something in it to do")
	case p.Kind != ProposeDecision && p.Kind != ProposeMission && p.Kind != ProposeBrief && p.Kind != ProposeDispatch && p.Kind != ProposeComment && p.Kind != ProposePlan && p.Kind != ProposeDocument && p.Kind != ProposeArtifact && p.Kind != ProposeReplan && p.Kind != ProposePlugin && p.Kind != ProposeAutomation:
		return nil, fmt.Errorf("a proposal is a decision, a mission, an edit to the brief or the plan, a dispatch, a replan, a review comment, a document, an artifact, an automation or a plugin's, not %q", p.Kind)
	case (p.Kind == ProposeBrief || p.Kind == ProposeDispatch || p.Kind == ProposePlan || p.Kind == ProposeReplan) && s.Workspace():
		return nil, fmt.Errorf("a %s is a mission's: this conversation is the workspace's", p.Kind)
	}
	c, err := s.update(id, func(tx *store.Tx, c *Conversation) error {
		if c.Status != Open {
			return fmt.Errorf("%s is resolved", id)
		}
		p.N, p.State = len(c.Proposals)+1, ProposalOpen
		if p.Revises != 0 {
			if err := c.revisable(p); err != nil {
				return err
			}
		}
		earlier := append([]Proposal(nil), c.Proposals...)
		if err := s.add(tx, c, line{Propose: &p}); err != nil {
			return err
		}
		if err := s.event(tx, c, events.Event{Kind: events.ProposalMade, By: byOf(p.By), At: p.At, Proposal: p.N, ProposalKind: p.kindNamed(), Title: oneLine(p.Says())}); err != nil {
			return err
		}
		// What it takes the place of is nothing to confirm now.
		at := p.At
		if at.IsZero() {
			at = time.Now()
		}
		for _, q := range earlier {
			if q.State == ProposalOpen && (q.N == p.Revises || p.supersedes(q)) {
				if err := s.add(tx, c, line{Proposal: &proposalSettled{N: q.N, State: ProposalSuperseded, Result: strconv.Itoa(p.N), By: WS, At: at}}); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	what := p.Says()
	if p.Revises != 0 {
		what = fmt.Sprintf("%s, revising proposal %d", what, p.Revises)
	}
	return c, s.log(c, p.By, fmt.Sprintf("%s %s proposes %s", id, p.By, oneLine(what)), p.At)
}

// Says is what a proposal is, in a line: "a mission: Token cache".
func (p Proposal) Says() string {
	what := p.What
	switch p.Kind {
	case ProposeDecision:
		what = "a decision: " + p.What
	case ProposeMission:
		what = "a mission: " + p.Title
	case ProposeBrief:
		what = "an edit to the brief"
		if p.Plan != "" {
			what = "the brief, with its plan"
		}
	case ProposePlan:
		what = "an edit to the plan"
	case ProposeDispatch:
		what = "a dispatch: " + p.Text
	case ProposeReplan:
		what = "a replan: " + p.Text
	case ProposeComment:
		what = fmt.Sprintf("a comment on %s %s:%d: %s", p.PR, p.Path, p.Line, p.Text)
	case ProposeDocument:
		what = fmt.Sprintf("a document (%s), %s: %s", p.DocKind, p.Path, p.Title)
		if p.To != "" {
			what = fmt.Sprintf("an edit to the document %s (%s): %s", p.To, p.DocKind, p.Title)
		}
	case ProposeArtifact:
		what = "an artifact: " + p.Title
		if p.To != "" {
			what = fmt.Sprintf("a version of %s, on v%d: %s", p.To, p.On, p.Note)
		}
	case ProposeAutomation:
		if p.Automation != nil {
			what = fmt.Sprintf("an automation, %s: when %s", p.Automation.Name, p.Automation.When)
		}
	case ProposePlugin:
		what = fmt.Sprintf("%s:%s (%s): %s", p.Plugin, p.Proposes, firstNonEmptyString(p.Label, "a plugin's"), p.Title)
		if len(p.Items) > 1 {
			what += fmt.Sprintf(", %d items", len(p.Items))
		}
	}
	return what
}

// revisable says why the agent's proposal p can't revise the one it
// names (p.Revises), or nil: that one's open, and of what p is.
func (c *Conversation) revisable(p Proposal) error {
	q := c.Proposal(p.Revises)
	if q == nil {
		return fmt.Errorf("there's no proposal %d in %s to revise: leave --revises off to propose it as a new one", p.Revises, c.ID)
	}
	switch q.State {
	case ProposalOpen:
	case ProposalAccepted:
		return fmt.Errorf("proposal %d was accepted, so there's nothing to revise: change what it made (%s) as you would anything that's there, or propose it as a new one", q.N, firstNonEmptyString(q.Result, "see ws show "+c.ID))
	case ProposalSuperseded:
		return fmt.Errorf("proposal %d was revised already, as proposal %s: revise that one", q.N, q.Result)
	default:
		return fmt.Errorf("proposal %d is %s: propose it as a new one, without --revises", q.N, q.State)
	}
	same := q.Kind == p.Kind || (p.Kind == ProposeBrief && p.Plan != "" && q.Kind == ProposePlan)
	switch {
	case !same:
		return fmt.Errorf("proposal %d is %s, and this is %s: a revision is of the same kind, so propose this as a new one (and withdraw %d if it's ruled out: ws conversation withdraw %d --why \"…\")", q.N, q.Says(), p.Says(), q.N, q.N)
	case p.Kind == ProposeDocument && q.To != p.To:
		return fmt.Errorf("proposal %d is %s, and this is %s: a revision is of the same document", q.N, q.Says(), p.Says())
	case p.Kind == ProposeDocument && p.To == "" && q.DocKind != p.DocKind:
		return fmt.Errorf("proposal %d is a document of the kind %s, and this is a %s: a revision is of the same kind", q.N, q.DocKind, p.DocKind)
	case p.Kind == ProposeArtifact && q.To != p.To:
		return fmt.Errorf("proposal %d is %s, and this is %s: a revision is of the same artifact", q.N, q.Says(), p.Says())
	case p.Kind == ProposePlugin && (q.Plugin != p.Plugin || q.Proposes != p.Proposes):
		return fmt.Errorf("proposal %d is %s, and this is %s: a revision is of the same plugin's same kind of proposal", q.N, q.Says(), p.Says())
	case p.Kind == ProposePlugin && q.anyMade():
		return fmt.Errorf("proposal %d had items done already (%s): propose what's left as a new one, naming those by what they made, and withdraw %d", q.N, strings.Join(q.made(), ", "), q.N)
	case p.Kind == ProposeComment && q.PR != p.PR:
		return fmt.Errorf("proposal %d is a comment on %s, and this is on %s: a revision is on the same pull request", q.N, q.PR, p.PR)
	}
	return nil
}

func firstNonEmptyString(s ...string) string {
	for _, x := range s {
		if strings.TrimSpace(x) != "" {
			return x
		}
	}
	return ""
}

// Withdraw records the agent taking back a proposal of its own that's
// still open, as what the user said since rules it out: why, in a
// sentence. Nothing's done, and the user isn't asked any more.
func (s Store) Withdraw(id string, n int, because string, at time.Time) (*Conversation, error) {
	because = strings.TrimSpace(because)
	if because == "" {
		return nil, errors.New("say why it's withdrawn (--why), in a sentence: the user reads it on the card")
	}
	c, err := s.update(id, func(tx *store.Tx, c *Conversation) error {
		p := c.Proposal(n)
		switch {
		case p == nil:
			return fmt.Errorf("%w: %s has no proposal %d", ErrNotFound, id, n)
		case p.State == ProposalSuperseded:
			return fmt.Errorf("proposal %d was revised as proposal %s: withdraw that one", n, p.Result)
		case p.State != ProposalOpen:
			return fmt.Errorf("proposal %d is %s already: there's nothing to withdraw", n, p.State)
		}
		if p.Kind == ProposePlugin && p.anyMade() {
			// What was done stands: it's accepted, with what's left withdrawn.
			return s.add(tx, c, line{Proposal: &proposalSettled{N: n, State: ProposalAccepted, Result: strings.Join(p.made(), " "), Because: because, By: Agent, At: at}})
		}
		return s.add(tx, c, line{Proposal: &proposalSettled{N: n, State: ProposalWithdrawn, Because: because, By: Agent, At: at}})
	})
	if err != nil {
		return nil, err
	}
	return c, s.log(c, Agent, fmt.Sprintf("%s %s withdraws proposal %d: %s", id, Agent, n, oneLine(because)), at)
}

// Open is the proposals still open in a conversation, oldest first.
func (c *Conversation) Open() []Proposal {
	var out []Proposal
	for _, p := range c.Proposals {
		if p.State == ProposalOpen {
			out = append(out, p)
		}
	}
	return out
}

// SettleProposal records you accepting a proposal (with what it made) or
// declining it. An accepted decision can be settled again with where it
// moved to, once its conversation becomes a mission.
func (s Store) SettleProposal(id string, n int, state, result, by string, at time.Time) (*Conversation, error) {
	if by != You {
		return nil, errors.New("only the user accepts or declines a proposal")
	}
	if state != ProposalAccepted && state != ProposalDeclined {
		return nil, fmt.Errorf("a proposal is accepted or declined, not %q", state)
	}
	return s.update(id, func(tx *store.Tx, c *Conversation) error {
		p := c.Proposal(n)
		switch {
		case p == nil:
			return fmt.Errorf("%w: %s has no proposal %d", ErrNotFound, id, n)
		case p.State == ProposalOpen && p.Kind == ProposePlugin && state == ProposalDeclined && p.anyMade():
			// What was done stands: it's accepted, with what's left declined.
			state, result = ProposalAccepted, strings.Join(p.made(), " ")
		case p.State == ProposalOpen:
		case p.State == ProposalAccepted && state == ProposalAccepted && p.Kind == ProposeDecision && p.Result == "" && result != "":
			// moved to its mission
		default:
			return fmt.Errorf("proposal %d in %s is %s already", n, id, p.State)
		}
		wasOpen := p.State == ProposalOpen
		if err := s.add(tx, c, line{Proposal: &proposalSettled{N: n, State: state, Result: result, By: by, At: at}}); err != nil {
			return err
		}
		if !wasOpen {
			return nil // settled already, and moved: said when it was
		}
		kind := events.ProposalAccepted
		if state == ProposalDeclined {
			kind = events.ProposalDeclined
		}
		return s.event(tx, c, events.Event{Kind: kind, By: events.ByYou, At: at, Proposal: n, ProposalKind: p.kindNamed(), Title: oneLine(p.Says()), Text: result})
	})
}

// kindNamed is the proposal's kind as an event names it: a plugin's as
// plugin:what (jira:issues).
func (p *Proposal) kindNamed() string {
	if p.Kind == ProposePlugin && p.Plugin != "" {
		return p.Plugin + ":" + p.Proposes
	}
	return p.Kind
}

// LatestOpen is the latest proposal of a kind still open, or nil.
func (c *Conversation) LatestOpen(kind string) *Proposal {
	for i := len(c.Proposals) - 1; i >= 0; i-- {
		if p := &c.Proposals[i]; p.Kind == kind && p.State == ProposalOpen {
			return p
		}
	}
	return nil
}

// LatestPlan is the plan last proposed in a conversation and still open:
// on its own, or with a brief. "" for none.
func (c *Conversation) LatestPlan() string {
	for _, p := range slices.Backward(c.Proposals) {

		if p.State == ProposalOpen && p.Plan != "" && (p.Kind == ProposePlan || p.Kind == ProposeBrief) {
			return p.Plan
		}
	}
	return ""
}

// KeptDecisions are the decisions accepted in a conversation that aren't
// any mission's yet.
func (c *Conversation) KeptDecisions() []Proposal {
	var out []Proposal
	for _, p := range c.Proposals {
		if p.Kind == ProposeDecision && p.State == ProposalAccepted && p.Result == "" {
			out = append(out, p)
		}
	}
	return out
}
