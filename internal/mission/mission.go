// Package mission stores missions: durable units of work that own capsules,
// a brief, a log, and the crew (agent sessions) run for them.
//
// A mission's record (title, stored state, capsules, crew), its log and its
// decisions are kept in the workspace's records (records.go). What's
// written to be read is in <workspace-root>/.ws/missions/<slug>/:
//
//	brief.md      goal, constraints, acceptance (human-owned)
//	plan.md       its route from A to Z: Z, where it stops early, its steps
//	files/        its private files: handoffs, rendered reviews, notes
//	crew-<id>.log what crew printed
//
// The package knows nothing about git or GitHub. Callers supply capsule facts
// to DeriveState and do cwd → capsule resolution themselves.
package mission

import (
	"slices"
	"strings"
	"time"
)

// Environment variables set for crew launched on a mission. WS_MISSION also
// lets any ws command run by that crew attach work to the mission.
const (
	EnvMission = "WS_MISSION"
	EnvCrew    = "WS_CREW"
	// EnvRole names a special role a process plays for a mission:
	// RoleOrchestrator.
	EnvRole = "WS_ROLE"
)

// RoleOrchestrator is WS_ROLE for the orchestrator, which decides what
// happens next on a mission.
const RoleOrchestrator = "orchestrator"

// State is a mission's lifecycle state. Parked and Archived are stored,
// and how it ended (Mission.Ending) is kept; the rest are derived: to
// brief from its having no confirmed brief (Mission.Confirmed), the rest
// from capsule and PR facts (see DeriveState).
type State string

const (
	Draft    State = "draft"
	Active   State = "active"
	Review   State = "review"
	Landed   State = "landed"
	Parked   State = "parked"
	Archived State = "archived"
	// Briefing: to brief. Its brief and plan aren't confirmed yet
	// (Mission.ToBrief): its conversation, or a briefing crew, is turning
	// the request into them. Confirming them ends it, and starts it.
	Briefing State = "briefing"
	// How else a mission ends (ending.go), besides landed: answered (a
	// finding, and no code), handed on (its work raised as other
	// missions), dropped.
	Answered State = State(EndAnswered)
	HandedOn State = State(EndHandedOn)
	Dropped  State = State(EndDropped)
)

// Ended reports whether the state is an ending: landed, answered, handed
// on or dropped.
func (s State) Ended() bool {
	return s == Landed || s == Answered || s == HandedOn || s == Dropped
}

// MaxTitle is the longest title a mission can be given: titles name
// missions in the web UI, so they're terse, plain-English names.
const MaxTitle = 50

// TitleIsRequest reports whether the title is still the request the
// mission was raised with (or the start of it), not a name for it yet.
func (m *Mission) TitleIsRequest() bool {
	if m.Request == "" {
		return false
	}
	first, _, _ := strings.Cut(strings.TrimSpace(m.Request), "\n")
	return strings.HasPrefix(first, strings.TrimSuffix(m.Title, "…"))
}

// RoleBriefing marks crew that brief a mission rather than work on it.
const RoleBriefing = "briefing"

// RoleAnswer marks crew the user asked to answer, for them, the question
// the orchestrator flagged: they find out what's needed and answer with ws
// mission answer, and don't do the work it leads to.
const RoleAnswer = "answer"

// Capsule identifies a capsule owned by a mission. Repo and Name are its
// identity; Branch is remembered so a PR can still be found after the
// capsule's worktree is burned.
type Capsule struct {
	Repo   string `toml:"repo" json:"repo"`
	Name   string `toml:"name" json:"name"`
	Branch string `toml:"branch,omitempty" json:"branch,omitempty"`
	// SeenPR is what was last seen of its PR on GitHub (prwatch.go), so a
	// change is noticed once, whenever it's next looked at.
	SeenPR *SeenPR `toml:"seen_pr,omitempty" json:"seen_pr,omitempty"`
	// Landed is its landing, for a capsule of a repo that takes commits
	// straight to its default branch: ws land records it.
	Landed *Landing `toml:"landed,omitempty" json:"landed,omitempty"`
	// PendingLanding is persisted before a push so a retry can reconcile it.
	PendingLanding *LandingAttempt `toml:"pending_landing,omitempty" json:"pending_landing,omitempty"`
}

// Landing is a capsule's commits pushed to its repo's default branch, with
// no pull request: when, and the commit the branch was left at.
type Landing struct {
	At   time.Time `toml:"at" json:"at"`
	Head string    `toml:"head" json:"head"`
}

// Crew records one agent session run for a mission.
type Crew struct {
	ID           string     `toml:"id" json:"id"`
	Harness      string     `toml:"harness" json:"harness"`
	Conversation string     `toml:"conversation,omitempty" json:"conversation,omitempty"`
	Started      time.Time  `toml:"started" json:"started"`
	Ended        *time.Time `toml:"ended,omitempty" json:"ended,omitempty"`
	Exit         string     `toml:"exit,omitempty" json:"exit,omitempty"`
	// Stopped: it was stopped (by you, or its host shutting down) rather
	// than ending by itself.
	Stopped bool `toml:"stopped,omitempty" json:"stopped,omitempty"`
	// Interrupted: of those, its host shut down under it (the daemon
	// restarted, was upgraded or closed) while nobody had asked it to stop.
	// The next daemon resumes it if it was still at its work, and looks at
	// it once (ResumeCrew and ClearInterrupted clear it).
	Interrupted bool `toml:"interrupted,omitempty" json:"interrupted,omitempty"`
	// PID of the crew process while it runs, so a later deck can tell a
	// crew that is still running from one whose deck died without recording
	// its end.
	PID int `toml:"pid,omitzero" json:"pid,omitzero"`
	// Capsule (repo/name) the crew worked in, so resuming goes back there.
	Capsule string `toml:"capsule,omitempty" json:"capsule,omitempty"`
	// Role is "" for crew working on the mission, RoleBriefing, or
	// RoleAnswer.
	Role string `toml:"role,omitempty" json:"role,omitempty"`
	// Model is the model it was launched on when one was asked for (a
	// pick on the mission's page); "" for the one its role gives it.
	// Resuming it over ACP runs it on that model again (CLI harnesses
	// resume their session without naming one).
	Model string `toml:"model,omitempty" json:"model,omitempty"`

	// Status is what the crew last said about itself: CrewAsking (needs the
	// user), CrewDone (finished its part), or "". Note is the question or
	// the summary. Both clear when the user next talks to the crew.
	Status   string     `toml:"status,omitempty" json:"status,omitempty"`
	Note     string     `toml:"note,omitempty" json:"note,omitempty"`
	StatusAt *time.Time `toml:"status_at,omitempty" json:"status_at,omitempty"`
	// Starting is since when ws has been starting this crew's session: set
	// as they're recorded by a launch (Mission.Launching), and cleared once
	// ws has handed them their prompt and, where ws's hooks are in their
	// session, the session has reported (Handed, AwaitsReport); or before
	// that, at anything they do (a turn, a status, their end). While it's
	// set they're starting, not working.
	Starting *time.Time `toml:"starting,omitempty" json:"starting,omitempty"`
	// AwaitsReport and Handed are how far a start has got, and mean
	// nothing once Starting's cleared. AwaitsReport: ws installed its hooks
	// in the session it's starting, so the crew will report that it's up
	// (and what they do in it), and they haven't yet. Handed: ws has handed
	// them their prompt, or had none to hand them.
	AwaitsReport bool `toml:"awaits_report,omitempty" json:"awaits_report,omitempty"`
	Handed       bool `toml:"handed,omitempty" json:"handed,omitempty"`
	// Activity is what the agent last reported through its own hooks, which
	// ws installs when it launches crew: ActivityWorking once it's sent a
	// message, ActivityWaiting once it has finished its turn,
	// ActivityPermission when it has stopped to ask for one. "" for a
	// harness without hooks: the deck then judges from the screen.
	Activity   string     `toml:"activity,omitempty" json:"activity,omitempty"`
	ActivityAt *time.Time `toml:"activity_at,omitempty" json:"activity_at,omitempty"`
	// LastReply is the start of what the agent last said: its turn's final
	// message. Notice is its last notification ("Claude needs your
	// permission to use Bash"), or with ActivityQuestion the question it
	// asked you, and Options the answers it offered.
	LastReply string   `toml:"last_reply,omitempty" json:"last_reply,omitempty"`
	Notice    string   `toml:"notice,omitempty" json:"notice,omitempty"`
	Options   []string `toml:"options,omitempty" json:"options,omitempty"`
	// Pending is the work it left running when its last turn ended, as its
	// harness said, while it waits on it (pending.go): nil when there was
	// none, or the harness couldn't tell. It clears when it's next prompted;
	// it's of the moment, so ws export leaves it out.
	Pending *Pending `toml:"-" json:"pending,omitempty"`
	// ResumeWith is a message to send when this crew is next resumed
	// (ws mission resume-with); it's used once.
	ResumeWith string `toml:"resume_with,omitempty" json:"resume_with,omitempty"`
	// Conversing: you've sent this crew a message since ws last did (its
	// opening prompt, a nudge). When its turn ends, it's answering you, so
	// it's your court; otherwise the orchestrator looks.
	Conversing bool `toml:"conversing,omitempty" json:"conversing,omitempty"`
	// Dispatch is what this crew was dispatched to do, when the orchestrator
	// (or the user) dispatched it with ws mission dispatch.
	Dispatch string `toml:"dispatch,omitempty" json:"dispatch,omitempty"`
	// HandedOff is when the crew last left a handoff with ws mission done.
	// It's kept at HandoffRef(ID).
	HandedOff *time.Time `toml:"handed_off,omitempty" json:"handed_off,omitempty"`
	// Orchestrated identifies the crew event (a run ending, a done or ask) the
	// orchestrator last looked at, so each is handled once. Times alone can't tell events apart:
	// they're stored to the second.
	Orchestrated string `toml:"orchestrated,omitempty" json:"orchestrated,omitempty"`
}

// Crew statuses.
const (
	CrewAsking = "asking"
	CrewDone   = "done"

	// What crew report through their hooks (Crew.Activity).
	ActivityWorking    = "working"
	ActivityWaiting    = "waiting"
	ActivityPermission = "permission"
	// ActivityQuestion: it asked you something with its own question tool
	// (Claude Code's AskUserQuestion), and waits for your answer.
	ActivityQuestion = "question"
)

// Mission is the contents of mission.toml plus its slug.
type Mission struct {
	Slug    string    `toml:"-" json:"-"`
	Title   string    `toml:"title" json:"title"`
	Created time.Time `toml:"created" json:"created"`
	State   State     `toml:"state,omitempty" json:"state,omitempty"` // stored override: "", parked or archived
	// Confirmed is when its brief (and plan) were confirmed, the go-ahead,
	// and ConfirmedBy who confirmed them: you, or a briefing crew you
	// were briefing it with. Nil for an idea (one raised from a Request):
	// it's to brief.
	Confirmed   *time.Time `toml:"confirmed,omitempty" json:"confirmed,omitempty"`
	ConfirmedBy string     `toml:"confirmed_by,omitempty" json:"confirmed_by,omitempty"`
	// Request is what the user asked for, verbatim, when a briefing crew
	// created the mission from it.
	Request string `toml:"request,omitempty" json:"request,omitempty"`
	// From is the mission/crew ("parent/c2") that raised this one, when
	// crew created it from inside another mission.
	From string `toml:"from,omitempty" json:"from,omitempty"`
	// Conversation is the conversation it came out of (w3), when you
	// confirmed it there.
	Conversation string `toml:"conversation,omitempty" json:"conversation,omitempty"`
	// About is what it's about, by name, as ws names things: one of a
	// plugin's things (RAM-123), a pull request (demo#12). It's the
	// mission's to say: whoever makes it, or works on it, says so (ws
	// mission about), and the graph has it from here.
	About    []string  `toml:"about,omitempty" json:"about,omitempty"`
	Capsules []Capsule `toml:"capsules,omitempty" json:"capsules,omitempty"`
	Crew     []Crew    `toml:"crew,omitempty" json:"crew,omitempty"`
	// Tasks are what the mission waits on the user for, oldest first: its
	// questions, and the steps of its route that are theirs (tasks.go).
	// TaskSeq numbers them.
	Tasks   []Task `toml:"tasks,omitempty" json:"tasks,omitempty"`
	TaskSeq int    `toml:"task_seq,omitempty" json:"task_seq,omitempty"`
	// Replies are replies drafted on the mission's pull requests, posted
	// or waiting to be (prreplies.go).
	Replies []Reply `toml:"replies,omitempty" json:"replies,omitempty"`
	// OldFlag is the one question a mission kept before tasks, read only to
	// become a task as it's loaded (migrateFlag).
	OldFlag *Proposal `toml:"flag,omitempty" json:"flag,omitempty"`
	// Answer is the user's answer to the last task they answered (ws
	// mission answer, or from the web UI), which the orchestrator's next
	// run acts on.
	Answer *Answer `toml:"answer,omitempty" json:"answer,omitempty"`
	// Conclude is an ending suggested (ws mission conclude): how it ends,
	// and Z checked. Ending it is the user's call.
	Conclude *Conclusion `toml:"conclude,omitempty" json:"conclude,omitempty"`
	// Ending is how it ended, once the user said (ws mission end): its
	// state is then the ending's.
	Ending *Conclusion `toml:"ending,omitempty" json:"ending,omitempty"`
	// EndedCapsules are capsules that ended alone, before the mission:
	// handed on to another mission, or dropped. They're no longer among
	// its Capsules.
	EndedCapsules []EndedCapsule `toml:"ended_capsules,omitempty" json:"ended_capsules,omitempty"`
	// Dispatch is crew the orchestrator wants launched, waiting for the user
	// ([orchestrator] dispatch = "ask", or no daemon to launch on).
	Dispatch *Dispatch `toml:"dispatch,omitempty" json:"dispatch,omitempty"`
	// Dispatches counts crew the orchestrator dispatched since the user last
	// stepped in; past [orchestrator] max_dispatches it must flag instead.
	Dispatches int `toml:"dispatches,omitempty" json:"dispatches,omitempty"`
	// Artifacts point at what the mission produced or relies on: files (on
	// a default branch, in a capsule, or private to the mission), PRs and
	// issues, and links. Capsules' own PRs are found, not recorded.
	Artifacts []Artifact `toml:"artifacts,omitempty" json:"artifacts,omitempty"`
	// Bundles are its verification bundles, oldest first (bundle.go): what
	// was handed over to show its work is as its checks say.
	Bundles []Bundle `toml:"bundles,omitempty" json:"bundles,omitempty"`
	// Stages is the progress of its route (stages.go): its plan's steps,
	// or without a plan, the brief's stages. The route itself is plan.md's,
	// or the brief's.
	Stages []StageRecord `toml:"stages,omitempty" json:"stages,omitempty"`
	// Waiting says the mission waits on something outside it that ws can't
	// see (a teammate, a deploy, another team's PR), as the orchestrator
	// placed it (ws mission wait). Anything happening clears it.
	Waiting *Proposal `toml:"waiting,omitempty" json:"waiting,omitempty"`
	// Orchestrating is the orchestrator's run, from when it's started until
	// it finishes: while it's set (and not past Until), the mission is in
	// the orchestrator's court.
	Orchestrating *OrchestratorRun `toml:"orchestrating,omitempty" json:"orchestrating,omitempty"`
	// Launching is ws starting crew, from when it begins (a launch asked
	// for, a brief confirmed, a step reached) until the crew are recorded:
	// nothing else says so in between. Crew recorded while it's set are
	// the crew it's starting: they take it over (Crew.Starting).
	Launching *Launching `toml:"launching,omitempty" json:"launching,omitempty"`
	// Planning is the planner's turn in its conversation (briefing the
	// mission, or coming back to its plan): set as ws asks for the turn,
	// cleared once its reply is recorded and the run that had it is over.
	Planning *PlannerTurn `toml:"planning,omitempty" json:"planning,omitempty"`
	// Checking is what ws is running while it checks a step crew reported
	// done (checking.go): set as each check begins, cleared when the
	// round is recorded.
	Checking *Checking `toml:"checking,omitempty" json:"checking,omitempty"`
	// WritingPRs are the pull requests the PR writer is writing, a capsule
	// each: set once ws has found one can be opened, cleared once it's
	// opened, or couldn't be.
	WritingPRs []WritingPR `toml:"writing_prs,omitempty" json:"writingPRs,omitempty"`

	// Plan is its plan (plan.md beside the brief), read as it's loaded,
	// and PlanErr why it couldn't be: a plan changed by hand outside ws
	// that doesn't read. Neither is kept in its record.
	Plan    *Plan `toml:"-" json:"-"`
	PlanErr error `toml:"-" json:"-"`
}

// Launching is ws starting crew on a mission, before the crew are
// recorded: a mark ws sets as it begins and clears itself, with no time
// after which it stops counting.
type Launching struct {
	What  string    `toml:"what,omitempty" json:"what,omitempty"` // what they're for: "step 3", or ""
	Since time.Time `toml:"since" json:"since"`
}

// WritingPR is the PR writer writing the pull request of one of a
// mission's capsules, before ws opens it: what ws was asked to open, so
// it's opened as asked once it's written, by whoever asked.
type WritingPR struct {
	Capsule string    `toml:"capsule" json:"capsule"` // repo/name
	Step    int       `toml:"step,omitempty" json:"step,omitempty"`
	Draft   bool      `toml:"draft,omitempty" json:"draft,omitempty"`
	By      string    `toml:"by" json:"by"` // who asked: you, ws, or crew's id
	Since   time.Time `toml:"since" json:"since"`
}

// WritingPR is the PR writer's mark on capsule's pull request, if it's
// writing it.
func (m *Mission) WritingPR(capsule string) (WritingPR, bool) {
	for _, p := range m.WritingPRs {
		if p.Capsule == capsule {
			return p, true
		}
	}
	return WritingPR{}, false
}

// PlannerTurn is a turn of the planner's underway in its conversation.
type PlannerTurn struct {
	In    string    `toml:"in" json:"in"` // the conversation: t1
	Since time.Time `toml:"since" json:"since"`
}

// OrchestratorRun is the orchestrator at work on a mission.
type OrchestratorRun struct {
	For   string    `toml:"for,omitempty" json:"for,omitempty"` // what it's looking at: "c2 (done)", "your answer"
	Since time.Time `toml:"since" json:"since"`
	// Until is when it must have finished, or died: past it, the run no
	// longer counts.
	Until time.Time `toml:"until" json:"until"`
}

// Proposal is something raised for the user: why, who by, when.
type Proposal struct {
	Text string    `toml:"text" json:"text"`
	By   string    `toml:"by,omitempty" json:"by,omitempty"`
	At   time.Time `toml:"at" json:"at"`
	// Options are the answers a flag suggests, so the user can answer
	// with one key (they can always write their own).
	Options []string `toml:"options,omitempty" json:"options,omitempty"`
	// CheckIn, on ws's own check-in flag from before tasks, is the stage
	// it was for.
	CheckIn int `toml:"check_in,omitempty" json:"check_in,omitempty"`
}

// Answer is the user's answer to a flag: what was asked, and what they
// said.
type Answer struct {
	Question string    `toml:"question" json:"question"`
	Text     string    `toml:"text" json:"text"`
	At       time.Time `toml:"at" json:"at"`
	// By is "you", or the crew (RoleAnswer) that answered for you.
	By string `toml:"by,omitempty" json:"by,omitempty"`
	// Step is the step of yours it was asked at, and Verb what it asked
	// (a TaskReview: what's to change in what you reviewed); 0 and "" for
	// a question.
	Step int    `toml:"step,omitempty" json:"step,omitempty"`
	Verb string `toml:"verb,omitempty" json:"verb,omitempty"`
}

// Dispatch is new crew to launch on a mission, with what it's to do
// from.
type Dispatch struct {
	Text    string    `toml:"text" json:"text"`
	Capsule string    `toml:"capsule,omitempty" json:"capsule,omitempty"` // repo/name; "" picks one
	Harness string    `toml:"harness,omitempty" json:"harness,omitempty"`
	By      string    `toml:"by,omitempty" json:"by,omitempty"`
	At      time.Time `toml:"at" json:"at"`
	// Stage is the stage of the brief's plan it's for (from 1); 0 for
	// whichever is current when it's launched.
	Stage int `toml:"stage,omitempty" json:"stage,omitempty"`
	// Model is what it runs on, when a step of the plan says; "" for the
	// crew's.
	Model string `toml:"model,omitempty" json:"model,omitempty"`
}

// Agent is what the crew run on: its harness, and the model it was
// launched on when one was asked for (claude on sonnet).
func (c Crew) Agent() string {
	if c.Model == "" {
		return c.Harness
	}
	return c.Harness + " on " + c.Model
}

// Artifact points at something a mission produced or relies on. Ref says
// where, in one of these forms (the cli package parses and resolves them):
//
//	repo:path            a file on the repo's default branch
//	repo/capsule:path    a file in one of the mission's capsules
//	mission:path         a file in the mission's own files/ folder
//	repo#N               a PR or issue
//	https://…            a link
//
// Where it is now (draft, in review, landed…) isn't stored: it's worked
// out from git and GitHub each time.
type Artifact struct {
	Ref   string `toml:"ref,omitempty" json:"ref,omitempty"`
	Kind  string `toml:"kind,omitempty" json:"kind,omitempty"` // proposal, adr, note, doc, pr, link…
	Label string `toml:"label,omitempty" json:"label,omitempty"`
	Crew  string `toml:"crew,omitempty" json:"crew,omitempty"` // who recorded it; "" for the user
	// Stage is the text of the stage it was made for: the recording crew's,
	// or the current one. Matched to the brief's stages by text, as stage
	// records are; "" when the brief has none.
	Stage string    `toml:"stage,omitempty" json:"stage,omitempty"`
	Added time.Time `toml:"added" json:"added"`
}

// MissionFilesDir is the folder, inside a mission's own, for its private
// files (mission:path artifacts).
const MissionFilesDir = "files"

// CrewByID is the mission's record of one crew.
func (m *Mission) CrewByID(id string) (Crew, bool) {
	for _, c := range m.Crew {
		if c.ID == id {
			return c, true
		}
	}
	return Crew{}, false
}

// HasCapsule reports whether the mission owns repo/name.
func (m *Mission) HasCapsule(repo, name string) bool {
	return slices.ContainsFunc(m.Capsules, func(c Capsule) bool {
		return c.Repo == repo && c.Name == name
	})
}

// AddCapsule adds repo/name if it isn't already owned. It reports whether
// anything changed.
func (m *Mission) AddCapsule(repo, name string) bool {
	return m.AddCapsuleBranch(repo, name, "")
}

// AddCapsuleBranch is AddCapsule that also records the capsule's branch. For
// an already-owned capsule it fills in a missing branch. A capsule that had
// ended alone is the mission's again.
func (m *Mission) AddCapsuleBranch(repo, name, branch string) bool {
	m.EndedCapsules = slices.DeleteFunc(m.EndedCapsules, func(e EndedCapsule) bool { return e.Repo == repo && e.Name == name })
	for i, c := range m.Capsules {
		if c.Repo == repo && c.Name == name {
			if c.Branch == "" && branch != "" {
				m.Capsules[i].Branch = branch
				return true
			}
			return false
		}
	}
	m.Capsules = append(m.Capsules, Capsule{Repo: repo, Name: name, Branch: branch})
	return true
}

// RemoveCapsule drops repo/name. It reports whether anything changed.
func (m *Mission) RemoveCapsule(repo, name string) bool {
	before := len(m.Capsules)
	m.Capsules = slices.DeleteFunc(m.Capsules, func(c Capsule) bool {
		return c.Repo == repo && c.Name == name
	})
	return len(m.Capsules) != before
}

// PRState is the GitHub PR state of a capsule's branch, as far as callers know.
type PRState int

const (
	PRNone PRState = iota
	PROpen
	PRMerged
)

// CapsuleFacts is what DeriveState needs to know about one capsule.
type CapsuleFacts struct {
	Exists bool
	PR     PRState
	// Landed: its commits were pushed straight to the default branch (a
	// repo that takes them so). It counts as a PR that merged does.
	Landed bool
}

// DeriveState works out a mission's state. A stored Parked or Archived
// wins, then how it ended, once it has, then to brief, while its brief
// isn't confirmed. A mission with a plan is
// otherwise where it is on its plan (Snapshot.Position): active once a
// step has started or a capsule has work, else draft; its pull requests
// are facts its steps read, not its state. Without a plan, only capsules
// that still exist, have a PR or have landed count:
//
//	none count                      → draft
//	every one merged, or landed     → landed
//	every one has a PR, or landed   → review
//	anything else                   → active
//
// A capsule of a repo with no pull requests is never in review: it's being
// worked on, and then it has landed. Capsules that ended alone aren't the
// mission's, and don't count.
func DeriveState(m *Mission, facts func(Capsule) CapsuleFacts) State {
	if m.State == Parked || m.State == Archived {
		return m.State
	}
	if m.Ending != nil && m.Ending.Ending != "" {
		return State(m.Ending.Ending)
	}
	if m.ToBrief() {
		return Briefing
	}
	counted, withPR, merged := capsuleCounts(m, facts)
	if m.Plan != nil {
		if counted == 0 && !slices.ContainsFunc(m.Stages, func(r StageRecord) bool { return r.Started != nil || r.Accepted != nil }) {
			return Draft
		}
		return Active
	}
	switch {
	case counted == 0:
		return Draft
	case merged == counted:
		return Landed
	case withPR == counted:
		return Review
	default:
		return Active
	}
}

// capsuleCounts is how many of the mission's capsules count (they exist,
// have a PR or have landed), how many of those have a PR or have landed,
// and how many merged or landed.
func capsuleCounts(m *Mission, facts func(Capsule) CapsuleFacts) (counted, withPR, merged int) {
	for _, c := range m.Capsules {
		f := facts(c)
		if !f.Exists && f.PR == PRNone && !f.Landed {
			continue
		}
		counted++
		if f.PR != PRNone || f.Landed {
			withPR++
		}
		if f.PR == PRMerged || f.Landed {
			merged++
		}
	}
	return counted, withPR, merged
}

// CapsulesLanded reports whether the mission's work has all left: at
// least one of its capsules counts, and every one that does has merged
// its PR or landed. It's what a landed ending needs, and a step that
// waits on a merge reads.
func CapsulesLanded(m *Mission, facts func(Capsule) CapsuleFacts) bool {
	counted, _, merged := capsuleCounts(m, facts)
	return counted > 0 && merged == counted
}

// ToBrief reports whether the mission is to brief: an idea, raised from
// words (Request) whose brief and plan aren't confirmed yet. Nothing
// starts until they are.
func (m *Mission) ToBrief() bool { return m.Request != "" && m.Confirmed == nil }
