package web

import (
	"io"
	"time"

	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/doc"
	"github.com/withworkspace/workspace/internal/ops"
	pages "github.com/withworkspace/workspace/internal/page"
	"github.com/withworkspace/workspace/internal/pulls"
	"github.com/withworkspace/workspace/internal/review"
	"github.com/withworkspace/workspace/internal/runs"
	"github.com/withworkspace/workspace/internal/team"
)

// App is what the web UI asks of the workspace: every write a page makes
// goes through it, so a command and a page doing the same thing can't
// drift apart. *ops.Workspace is one.
type App interface {
	Missions
	Documents
	Files
	Settings
	Processing
	Acting
	Crewing
	Posting
	Conversing
	Paging
	Reviewing
	Artifacting
	Focusing
	Running
}

// Running is runs on a mission's page (internal/runs): a capsule's recipe
// started, a run stopped, and which missions someone has open, for the
// runs started from them.
type Running interface {
	RunRecipe(slug, repo, capsule, recipe string, by ops.Actor) (runs.Run, error)
	StopRun(slug, ref string) error
	LookedAt(slugs map[string]bool) error
	PointSilo(repo, capsule string, out io.Writer) error
}

// Focusing is your pins: what's pinned, and pinning, unpinning and
// putting them in order, each as you.
type Focusing interface {
	Focus() ([]ops.FocusItem, error)
	Pin(a ops.Actor, name string) (ops.Pinned, error)
	Unpin(a ops.Actor, name string) (ops.Unpinned, error)
	ArrangeFocus(a ops.Actor, names []string) error
}

// Artifacting is an artifact's lifetime: promoting, discarding and
// restoring one, and clearing the expired, each as you.
type Artifacting interface {
	PromoteArtifact(p ops.ArtifactPromote, by ops.Actor) (ops.ArtifactDone, error)
	DiscardArtifact(ref, reason string, by ops.Actor) (ops.ArtifactDone, error)
	RestoreArtifact(ref string, by ops.Actor) (ops.ArtifactDone, error)
	ClearExpiredArtifacts(by ops.Actor) (ops.ArtifactsCleared, error)
}

// Conversing is threads on documents, talk, asks from an ask bar, what
// was decided in them, and the documents' kept versions and drafts.
type Conversing interface {
	KeepVersion(sc doc.Scope, text []byte) (string, error)
	OpenThread(sc doc.Scope, ref string, text []byte, t conversation.DocThread, by ops.Actor) (*conversation.Conversation, error)
	Reply(sc doc.Scope, id string, text []byte, body, commit string, by ops.Actor) (*conversation.Conversation, error)
	SetThreadStatus(sc doc.Scope, id, status string, by ops.Actor) (*conversation.Conversation, error)
	DeclineSuggestion(sc doc.Scope, id string, by ops.Actor) (*conversation.Conversation, error)
	Approve(sc doc.Scope, ref string, text []byte, version string, by ops.Actor) (string, error)
	Decide(slug, id, what, why string, about []string, by ops.Actor) (string, error)
	SkipDraft(sc doc.Scope, id string, by ops.Actor) error
	StartTalk(sc doc.Scope, body string, by ops.Actor) (*conversation.Conversation, error)
	SayInTalk(sc doc.Scope, id, body string, by ops.Actor) (*conversation.Conversation, error)
	SetTalkStatus(sc doc.Scope, id, status string, by ops.Actor) (*conversation.Conversation, error)
	Ask(a ops.Asked, by ops.Actor) (*conversation.Conversation, error)
	KeepAsk(sc doc.Scope, id string, by ops.Actor) (*conversation.Conversation, error)
	SaveDraft(sc doc.Scope, d doc.Draft) error
	DropDraft(sc doc.Scope, ref string) error
}

// Paging is pages, their cells and the passes over them.
type Paging interface {
	Passes() []pages.PassDef
	PassesOn(id string) []pages.PassDef
	NewPage(title, kind, to string) (pages.Page, error)
	SavePage(id, base string, text []byte) (string, error)
	RemovePage(id string) error
	Cell(pageID, id string) (*conversation.Conversation, string, error)
	OpenCell(pageID string, a ops.CellAsk) (*conversation.Conversation, error)
	FillCell(c *conversation.Conversation) *conversation.Conversation
	SayInCell(c *conversation.Conversation, key string, s ops.CellSay) (*conversation.Conversation, error)
	PlacedCell(c *conversation.Conversation) error
	Pass(pageID, name string) (pages.PassDef, *conversation.Conversation, []byte, error)
	RunPass(pageID, name string) (pages.PassDef, *conversation.Conversation, []byte, error)
	SettleFinding(pageID, name, key, state, why string) (pages.PassDef, *conversation.Conversation, []byte, error)
	// A review's passes: over a pull request, or a mission's capsule.
	ReviewPassesOf(of ops.ReviewOf) []pages.PassDef
	ReviewedPR(repo string, number int) bool
	AutoPasses() string
	PassMarks(heads map[string]string) map[string][]ops.PassMark
	ReviewPass(of ops.ReviewOf, name string) (pages.PassDef, *conversation.Conversation, error)
	RunReviewPass(of ops.ReviewOf, name string) (pages.PassDef, *conversation.Conversation, error)
	SettleReviewFinding(of ops.ReviewOf, name, key, state, why string) (pages.PassDef, *conversation.Conversation, error)
	TakeReviewFinding(of ops.ReviewOf, name, key string) (pages.PassDef, *conversation.Conversation, error)
}

// Reviewing is your review of a mission's capsule or a pull request, as
// you write it and once it's submitted.
type Reviewing interface {
	Reviews(of ops.ReviewOf) (review.Store, string)
	DiscardReview(of ops.ReviewOf) error
	AddNote(of ops.ReviewOf, d review.Diffed, n review.NewNote) error
	EditNote(of ops.ReviewOf, id, kind, body string, suggestion *string) error
	DeleteNote(of ops.ReviewOf, id string) error
	SetViewed(of ops.ReviewOf, path, blob string, viewed bool) error
	Respond(of ops.ReviewOf, id string, what review.Response, body, commit string, by ops.Actor) (*review.Review, error)
	SubmitReview(of ops.ReviewOf, d review.Diffed, verdict, summary string, by ops.Actor) (*review.Review, error)
}

var _ App = (*ops.Workspace)(nil)

// Files is the workspace's repos and their worktrees, where a mission's
// capsule is, and landing a capsule.
type Files interface {
	Repos() []ops.Repo
	Worktrees(repo string) ([]ops.Worktree, error)
	Worktree(repo, name string) (dir, baseRef string, err error)
	CapsuleDir(slug, repo, name string) (dir, baseRef string, err error)
	Landing(repo, name string) (*ops.Landing, error)
	LandCapsule(repo, name string) (*ops.LandOutcome, error)
	CodeSearching
}

// Crewing is crew's pages (the crew the daemon holds), sending threads to
// them, settling what a conversation's agent proposed, and asking about or
// docking a pull request.
type Crewing interface {
	Crew() ops.CrewPages
	SendToCrew(slug, ref string, ids []string) (string, error)
	SettleProposal(slug, id string, n int, accept bool, e ops.ProposalEdits) (said, slugAfter string, err error)
	AsksPRs() bool
	AskPR(repo string, number int, q ops.PRQuestion) (*conversation.Conversation, error)
	DockPR(repo string, number int) (string, error)
}

// Posting is the Reviews and Team pages' writes: a pull request's review
// and replies posted to GitHub, and the Team page's seen, gists and
// accounts.
type Posting interface {
	SubmitPR(repo string, number int, head, verdict, summary string) (*pulls.Submitted, error)
	RespondPR(repo string, number int, thread string, what review.Response, body string) error
	CommentPR(repo string, number int, body string) error
	MarkTeamSeen() error
	Narrate(snap *team.Snapshot, strands []string, since time.Time) int
	Summarise(snap *team.Snapshot, since time.Time, key string) bool
}
