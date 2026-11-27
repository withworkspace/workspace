package web

import (
	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/ops"
)

// The values a wire field can take, where the store keeps it as a plain
// string. Each is a type so that web/src/wire.gen.ts gives the web UI the
// union of its constants (scripts/wiretypes), not just string. Where the
// store names the values, these are its names, so they can't drift apart.
// A value the store gains goes here too, or the web UI won't know of it.

// RecipeKind is what a service is for: an app, what's being built, or a
// tool that helps build it (a recipe's kind).
type RecipeKind string

const (
	KindApp  RecipeKind = ops.RecipeApp
	KindTool RecipeKind = ops.RecipeTool
)

// RecipeRender is how the web UI shows a service: framed, or in a browser
// tab of its own (a recipe's render).
type RecipeRender string

const (
	RenderInline RecipeRender = ops.RenderInline
	RenderTab    RecipeRender = ops.RenderTab
)

// HeadlineGroup is which group of the missions list a headline puts its
// mission in (mission.GroupNames, as words joined by hyphens).
type HeadlineGroup string

const (
	GroupNeedsYou   HeadlineGroup = "needs-you"
	GroupInProgress HeadlineGroup = "in-progress"
	GroupWaiting    HeadlineGroup = "waiting"
	GroupUpNext     HeadlineGroup = "up-next"
	GroupPast       HeadlineGroup = "past"
)

// CrewActivity is what running crew is doing, as its hooks last said.
type CrewActivity string

const (
	CrewWorking    CrewActivity = mission.ActivityWorking
	CrewWaiting    CrewActivity = mission.ActivityWaiting
	CrewPermission CrewActivity = mission.ActivityPermission
	CrewQuestion   CrewActivity = mission.ActivityQuestion
)

// CrewEnded is how crew ended, beyond its exit.
type CrewEnded string

const (
	CrewInterrupted CrewEnded = mission.EndedInterrupted
	CrewHandedBack  CrewEnded = mission.EndedHandedBack
	CrewStopped     CrewEnded = mission.EndedStopped
)

// CheckVerdict is how a check of what a step asked for came out: met,
// unmet, the judge can't tell, or yours to say.
type CheckVerdict string

const (
	CheckMet      CheckVerdict = mission.CheckMet
	CheckUnmet    CheckVerdict = mission.CheckUnmet
	CheckCantTell CheckVerdict = mission.CheckCantTell
	CheckYours    CheckVerdict = mission.CheckYours
)

// BundleVerdict is yours of a verification bundle.
type BundleVerdict string

const (
	BundleApproved BundleVerdict = mission.BundleApproved
	BundleSentBack BundleVerdict = mission.BundleSentBack
)

// EvidenceKind is what a bundle's evidence is, as the web UI shows it.
type EvidenceKind string

const (
	EvidenceImage EvidenceKind = "image"
	EvidenceVideo EvidenceKind = "video"
	EvidenceText  EvidenceKind = "text"
)

// DecisionState is where a mission's decision stands.
type DecisionState string

const (
	DecisionCurrent    DecisionState = mission.DecisionCurrent
	DecisionSuperseded DecisionState = mission.DecisionSuperseded
	DecisionWithdrawn  DecisionState = mission.DecisionWithdrawn
)

// CitationHow is how a document names another: a link, or a ref in its
// words.
type CitationHow string

const (
	CiteLink CitationHow = "link"
	CiteRef  CitationHow = "ref"
)

// Dispatch is whether the orchestrator launches crew itself, or asks you
// first.
type Dispatch string

const (
	DispatchAuto Dispatch = "auto"
	DispatchAsk  Dispatch = "ask"
)

// SettingKind is what kind of value a setting takes, which says how the
// Settings page edits it: picks, a list of harness:model; level, a model
// for a level of crew's Work steps (on crew's harness, or harness:model);
// color, #rrggbb; places, the sidebar's, by id; tint, a whole number from
// 0 to 100.
type SettingKind string

const (
	KindHarness SettingKind = "harness"
	KindModel   SettingKind = "model"
	KindChoice  SettingKind = "choice"
	KindBool    SettingKind = "bool"
	KindNumber  SettingKind = "number"
	KindText    SettingKind = "text"
	KindPicks   SettingKind = "picks"
	KindLevel   SettingKind = "level"
	KindColor   SettingKind = "color"
	KindPlaces  SettingKind = "places"
	KindTint    SettingKind = "tint"
)

// ThreadStatus is whether a conversation or thread is still open.
type ThreadStatus string

const (
	ThreadOpen     ThreadStatus = conversation.Open
	ThreadResolved ThreadStatus = conversation.Resolved
)

// FindingState is where a review pass's finding stands.
type FindingState string

const (
	FindingOpen      FindingState = conversation.FindingOpen
	FindingDone      FindingState = conversation.FindingDone
	FindingDismissed FindingState = conversation.FindingDismissed
)

// SourceState is how a cell's source has moved on since the cell was run:
// changed, or gone.
type SourceState string

const (
	SourceChanged SourceState = conversation.SourceChanged
	SourceGone    SourceState = conversation.SourceGone
)

// StepType is a kind of plan step, as a plan writes it.
type StepType string

const (
	StepWork   StepType = mission.StepWork
	StepVerify StepType = mission.StepVerify
	StepReview StepType = mission.StepReview
	StepDecide StepType = mission.StepDecide
	StepOpenPR StepType = mission.StepOpenPR
	StepLand   StepType = mission.StepLand
	StepWait   StepType = mission.StepWait
	StepPlan   StepType = mission.StepPlan
)

// TaskVerb is what a task of yours asks of you.
type TaskVerb string

const (
	TaskAnswer  TaskVerb = mission.TaskAnswer
	TaskReview  TaskVerb = mission.TaskReview
	TaskApprove TaskVerb = mission.TaskApprove
	TaskLand    TaskVerb = mission.TaskLand
	TaskDo      TaskVerb = mission.TaskDo
	TaskChecks  TaskVerb = mission.TaskChecks
	TaskTime    TaskVerb = mission.TaskTime
	TaskReply   TaskVerb = mission.TaskReply
)

// CapsuleEnding is how a capsule ended alone: handed on to a mission, or
// dropped.
type CapsuleEnding string

const (
	CapsuleHandedOn CapsuleEnding = CapsuleEnding(mission.EndHandedOn)
	CapsuleDropped  CapsuleEnding = CapsuleEnding(mission.EndDropped)
)

// SettingSource is where a setting's value comes from: ws's default, the
// team's ws.toml, or yours.
type SettingSource string

const (
	FromDefault SettingSource = "default"
	FromTeam    SettingSource = "team"
	FromYours   SettingSource = "yours"
)

// DismissWhy is why a finding was dismissed: it's wrong, the page already
// says it, or it's so but not worth acting on.
type DismissWhy string

const (
	DismissWrong   DismissWhy = conversation.DismissWrong
	DismissAlready DismissWhy = conversation.DismissAlready
	DismissMinor   DismissWhy = conversation.DismissMinor
)

// StepWho is who's on a step: crew, you, ws itself, CI, someone outside,
// or the planner.
type StepWho string

const (
	WhoCrew    StepWho = mission.WhoCrew
	WhoYou     StepWho = mission.WhoYou
	WhoWS      StepWho = mission.WhoWS
	WhoCI      StepWho = mission.WhoCI
	WhoOutside StepWho = mission.WhoOutside
	WhoPlanner StepWho = mission.WhoPlanner
)

// WaitPasses is how a Wait step is passed: watched (ws running its
// checks), done (you marking it done), or said (anyone saying it's
// happened).
type WaitPasses string

const (
	PassesWatched WaitPasses = "watched"
	PassesDone    WaitPasses = "done"
	PassesSaid    WaitPasses = "said"
)
