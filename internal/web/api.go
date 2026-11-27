package web

import (
	"time"

	"github.com/withworkspace/workspace/internal/artifact"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/pulls"
	"github.com/withworkspace/workspace/internal/review"

	"github.com/withworkspace/workspace/internal/router"
	"github.com/withworkspace/workspace/internal/runs"
)

// What the API serves. web/src/api.ts mirrors these.

// Workspace is GET /api/workspace: the workspace and its missions.
type Workspace struct {
	Name string `json:"name"`
	Root string `json:"root"`
	// Org and DefaultBranch are where its repos are on GitHub, for a
	// ref's address: each under Org by its name, but for those Remotes
	// says are elsewhere (repo to owner/name).
	Org           string            `json:"org,omitempty"`
	Remotes       map[string]string `json:"remotes,omitempty"`
	DefaultBranch string            `json:"defaultBranch,omitempty"`
	Missions      []Mission         `json:"missions"`
	// Needs is what needs you: what can't move without you, and only you.
	// It's worked out from where each thing stands now, never kept
	// (ops.Needs): the missions in your court, then the conversations
	// that are your turn, then the pull requests that ask for your review
	// by name. The sidebar's Needs you, the counts beside its places, the
	// tab's title and what the desktop is told are all this.
	Needs []Need `json:"needs"`
	// Notifications is the feed of what happened (GET /api/notifications),
	// as a count: beside Needs, which is what can't move without you.
	Notifications NotificationsCount `json:"notifications"`
	// Focus is what you've pinned that stands, in your order: the
	// sidebar's Focus.
	Focus []FocusPin `json:"focus"`
	// CapsulesSeen moves when git, asked behind a read that didn't wait
	// on it, said something of a capsule other than the read was given:
	// a page showing capsules' statuses (Files' switcher) reads again.
	CapsulesSeen string `json:"capsulesSeen,omitempty"`
	// Conversations are the workspace's conversations with an agent,
	// newest first.
	Conversations []Talk `json:"conversations"`
	// Pages are the workspace's pages, the latest changed first.
	Pages []PageInfo `json:"pages"`
	// Artifacts are its artifacts, the latest version's newest first,
	// discarded ones too.
	Artifacts []ArtifactInfo `json:"artifacts"`
	// ArtifactsRepo is the repo a promoted artifact lives in ([artifacts]
	// path), offered the first time one's promoted; "" for none.
	ArtifactsRepo string `json:"artifactsRepo,omitempty"`
	// Kinds are the kinds of document the workspace has
	// ([documents.<name>]): a page can be started as one, and filed as one.
	Kinds []DocKind `json:"kinds,omitempty"`
	// Docs is when the threads on documents outside a mission last
	// changed (one opened, answered, resolved): a document that's open
	// reads its threads again.
	Docs *time.Time `json:"docs,omitempty"`
	// Reviews is how many pull requests need you, as GitHub last said:
	// absent until the Reviews page has been opened (GitHub isn't asked
	// otherwise), or where there's no GitHub.
	Reviews *ReviewsCount `json:"reviews,omitempty"`
	// Sends is how what's been said from pull requests' pages stands:
	// absent when nothing's on its way to GitHub and nothing failed.
	Sends *pulls.Sends `json:"sends,omitempty"`
	// Spaces are its spaces of documents (a repo of them, or a folder of
	// one), as config says them: a document in one is read in Documents.
	// Counts and folders aren't said here (GET /api/documents).
	Spaces []SpaceRef `json:"spaces"`
	// SpacesSeen moves when ws, reading a space again behind a read that
	// didn't wait on it, found it other than the read was given: a page
	// showing a space (Documents, a space, a document's citations and
	// what's in flight) reads it again.
	SpacesSeen string `json:"spacesSeen,omitempty"`
	// Version is the ws that serves it, as ws --version says it: the
	// daemon's, which a newer ws on the path isn't until it's restarted.
	// The web UI says which it's talking to, and tells a plugin's app as
	// it connects: the same its plugin was told when it started.
	Version string `json:"version,omitempty"`
	// GroundsSeen moves when the repos' default branches do, or the files
	// read of one (refs are checked against them) are found other than
	// they were: a page that's checked what it names checks again.
	GroundsSeen string `json:"groundsSeen,omitempty"`
	// ReadsSeen moves when a capsule's tree or summary, answered as it
	// was kept while it was read again, is found other than it was told:
	// a page showing one reads it again.
	ReadsSeen string `json:"readsSeen,omitempty"`
	// Later is a release of ws that's out and later than Version, as the
	// daemon last found (it looks once a day): none for none, or for one
	// that's installed and waiting for the daemon to restart onto it.
	Later string `json:"later,omitempty"`
	// Places is where the web UI's places go, as [ui] sidebar and menu
	// say: absent for as they come.
	Places *Places `json:"places,omitempty"`
	// Apps are the workspace's own services with a page ([run.service.<name>] page
	// in ws.toml): each a place, showing its run in a frame.
	Apps []AppInfo `json:"apps"`
}

// AppInfo is an app as a place has it: its recipe's name, what it's
// called and its icon's name ("" for the default), and whether it's
// framed or opened in a browser tab of its own.
type AppInfo struct {
	Name   string       `json:"name"`
	Title  string       `json:"title"`
	Icon   string       `json:"icon,omitempty"`
	Render RecipeRender `json:"render"`
}

// AppInfoOf is an app as ops lists it, as the wire has it.
func AppInfoOf(a ops.AppEntry) AppInfo {
	render := RenderInline
	if a.InTab {
		render = RenderTab
	}
	return AppInfo{Name: a.Name, Title: a.Title, Icon: a.Icon, Render: render}
}

// Places is where the web UI's places go: the sidebar's, in order, and
// the workspace menu's. Their ids are the web UI's.
type Places struct {
	Sidebar []string `json:"sidebar"`
	Menu    []string `json:"menu"`
}

// Need is something that needs you (ops.Need), as the web UI is given it.
type Need struct {
	// Key is the same for as long as it's the same need: what's said
	// once of it (it lights up, the desktop is told) goes by it.
	Key string `json:"key"`
	// Of is the sort of thing it's on, and Ref that thing as a ref names
	// it (a mission's slug, w4, api#89). In is the mission a conversation
	// is about, where it's about one.
	Of  NeedOf `json:"of"`
	Ref string `json:"ref"`
	In  string `json:"in,omitempty"`
	// Kind is what it needs of you: a mission's headline's kind,
	// your-turn, or your-review. Label is the headline's word for it,
	// where it has one.
	Kind  string `json:"kind"`
	Label string `json:"label,omitempty"`
	// Title is the thing's, and Text the sentence saying what it needs.
	Title string `json:"title"`
	Text  string `json:"text,omitempty"`
	// Since is when it began, where the thing knows.
	Since *time.Time `json:"since,omitempty"`
	// To is where it's dealt with, in the web UI.
	To string `json:"to"`
}

// NeedsAnnounce is a page asking which needs it should tell the desktop
// of (POST /api/needs/announce): Keys are the needs that are new to it,
// and Holding every need it has now. The daemon answers once for each
// need, whichever page asks first (ops.Announce).
type NeedsAnnounce struct {
	Keys    []string `json:"keys"`
	Holding []string `json:"holding"`
}

// NeedsAnnounced is the answer: Fresh are those of Keys no page had been
// told to announce, which this one now is.
type NeedsAnnounced struct {
	Fresh []string `json:"fresh"`
}

// NeedOf is the sort of thing a need is on.
type NeedOf string

const (
	NeedOfMission      NeedOf = "mission"
	NeedOfConversation NeedOf = "conversation"
	NeedOfPull         NeedOf = "pull"
)

// FocusPin is a pin, as the sidebar draws it: its name, as ws writes it
// (what a ref is read back from), and what the graph says of it where it
// does: what it is (mission, talk, pull-request, thing…), what it's
// called, how it stands. A pull request ws keeps nothing of, or a file,
// has only its name.
type FocusPin struct {
	Name  string `json:"name"`
	Kind  string `json:"kind,omitempty"`
	Title string `json:"title,omitempty"`
	State string `json:"state,omitempty"`
}

// SpaceRef is a space of documents, as what links to one needs it.
type SpaceRef struct {
	Repo   string `json:"repo"`
	Folder string `json:"folder,omitempty"`
	Label  string `json:"label"`
	Color  string `json:"color"`
}

// Mission is one mission, as the missions list knows it.
type Mission struct {
	Slug     string        `json:"slug"`
	Title    string        `json:"title"`
	State    mission.State `json:"state"`
	Created  time.Time     `json:"created"`
	Updated  time.Time     `json:"updated"`            // when its record, brief or log last changed
	Flag     string        `json:"flag,omitempty"`     // the question it's flagged with
	ReReview []string      `json:"reReview,omitempty"` // capsules waiting for you to re-review
	// Headline is where it stands: whose court it's in, and what moves it
	// on. ws mission show says the same.
	Headline Headline  `json:"headline"`
	Capsules []Capsule `json:"capsules"`
	Crew     []Crew    `json:"crew"`
	// At is where it is on its route, in words: "Step 2 of 7, yours: …".
	At string `json:"at,omitempty"`
	// About is what it's about, by name: one of a plugin's things, a
	// pull request.
	About []string `json:"about,omitempty"`
}

// Headline is where a mission stands (mission.Headline): whose court it's
// in, in a sentence, and the one action that moves it on.
type Headline struct {
	Court  mission.Court `json:"court"` // agent, you, outside; "" when archived
	Kind   string        `json:"kind"`  // crew-asks, flag, merge, awaiting-review…
	Group  HeadlineGroup `json:"group"` // needs-you, in-progress, waiting, up-next, past
	Label  string        `json:"label"` // a few words, with a glyph: "? asks"
	Text   string        `json:"text"`
	Action Action        `json:"action"`
	By     string        `json:"by,omitempty"`
	Since  *time.Time    `json:"since,omitempty"`
	// Checking is what's running while ws checks a step (kind checking),
	// so the page can say how long it's been going as it goes.
	Checking *Checking `json:"checking,omitempty"`
}

// Checking is what ws is running while it checks a step (mission.Checking):
// a check that's a command (n of of), or the judge (n 0), reading the
// checks at judged.
type Checking struct {
	Step    int       `json:"step"`
	N       int       `json:"n,omitempty"`
	Of      int       `json:"of"`
	Command string    `json:"command,omitempty"`
	Line    string    `json:"line,omitempty"` // the last line it's printed
	Judged  []int     `json:"judged,omitempty"`
	Judge   string    `json:"judge,omitempty"` // who's reading, on the judge's turn
	Since   time.Time `json:"since"`
	Until   time.Time `json:"until"` // when it's stopped, if it hasn't finished
}

// Action is what moves a headline on. Verb says what (reply, answer,
// review, launch, open, merge, archive…); "" is nothing to do.
type Action struct {
	Verb    string   `json:"verb,omitempty"`
	Label   string   `json:"label"`
	Target  string   `json:"target,omitempty"` // crew id, capsule or PR
	URL     string   `json:"url,omitempty"`
	Options []string `json:"options,omitempty"` // a flag's suggested answers
	// Look is what a Review step's review looks at (verb review-step):
	// bundles to approve, capsules to review, documents to read. On a
	// step's checks that are yours to say (verb answer), it's the bundles
	// that show them.
	Look []Look `json:"look,omitempty"`
}

// Look is one thing a step of yours looks at: a verification bundle (b3),
// a capsule (repo/name), or a document (an artifact's ref), with what to
// call it. Read is a document to read beside the review, which approving
// needn't cover.
type Look struct {
	Bundle  string `json:"bundle,omitempty"`
	Capsule string `json:"capsule,omitempty"`
	Doc     string `json:"doc,omitempty"`
	Name    string `json:"name"`
	Read    bool   `json:"read,omitempty"`
	// Approved: a bundle you've approved already.
	Approved bool `json:"approved,omitempty"`
}

// Capsule is one of a mission's capsules.
type Capsule struct {
	Repo   string `json:"repo"`
	Name   string `json:"name"`
	Branch string `json:"branch,omitempty"`
	Path   string `json:"path"`
	Exists bool   `json:"exists"`
	// Head is the capsule's commit: when it moves, a review open on it has
	// new commits to show.
	Head string `json:"head,omitempty"`
}

// Crew is one agent session run on a mission.
type Crew struct {
	ID        string       `json:"id"`
	Harness   string       `json:"harness"`
	Model     string       `json:"model,omitempty"` // the one it was launched on, when picked
	Role      string       `json:"role,omitempty"`
	Capsule   string       `json:"capsule,omitempty"`
	Started   time.Time    `json:"started"`
	Ended     bool         `json:"ended"`
	Starting  bool         `json:"starting,omitempty"` // ws is starting its session: the mark the mission's headline reads (mission.Crew.Starting)
	Activity  CrewActivity `json:"activity,omitempty"` // working, waiting, permission, question
	Status    string       `json:"status,omitempty"`   // done, …
	LastReply string       `json:"lastReply,omitempty"`
	// Notice is what its harness last told you ("Claude needs your
	// permission to use Bash"), or the question it asked; Options, that
	// question's answers.
	Notice  string   `json:"notice,omitempty"`
	Options []string `json:"options,omitempty"`
	// Structured is crew over ACP: no terminal, just its conversation.
	Structured bool `json:"structured,omitempty"`
	// SwitchModel: it can be put on another model while it runs.
	SwitchModel bool `json:"switchModel,omitempty"`
	// WaitingOn is the work it ended its turn to wait on, while ws holds
	// that turn (mission.Crew.OwnWork): what it left running, in a line.
	WaitingOn string `json:"waitingOn,omitempty"`
	// Again: it reported done, and it's been at work since (sent back to
	// it, or told something), so its done isn't its latest word.
	Again bool `json:"again,omitempty"`
}

// MissionDetail is GET /api/missions/{slug}: everything a mission's page
// view shows. Capsules' PRs are as GitHub last said: it's slow, so it's
// asked behind the read, and the mission's event moves when it answers
// with something different.
type MissionDetail struct {
	Slug    string        `json:"slug"`
	Title   string        `json:"title"`
	State   mission.State `json:"state"`
	Created time.Time     `json:"created"`
	Request string        `json:"request,omitempty"` // what was asked for, when a briefing agent made it
	From    string        `json:"from,omitempty"`    // the mission/crew that raised it
	About   []string      `json:"about,omitempty"`   // what it's about, by name
	// Conversation is the conversation it came out of (w3), and its title.
	Conversation      string `json:"conversation,omitempty"`
	ConversationTitle string `json:"conversationTitle,omitempty"`
	// Talks are the conversations about it (its Talk tab), newest first.
	Talks []Talk `json:"talks"`
	// Planner is the conversation its planner is in (t1): the one it was
	// briefed in, carried on through it, which its brief's own page talks
	// in. "" when it was briefed without one.
	Planner string `json:"planner,omitempty"`
	Brief   string `json:"brief"`
	// BriefUnwritten: the brief is empty, or still the template it was
	// made with, so a first draft is nothing to compare with it.
	BriefUnwritten bool `json:"briefUnwritten,omitempty"`
	// Plan is its plan (plan.md), its route in place of the brief's
	// stages: "" for none. PlanProblem is why it doesn't read, when it
	// was changed by hand and doesn't.
	Plan        string      `json:"plan,omitempty"`
	PlanProblem string      `json:"planProblem,omitempty"`
	Flag        *Proposal   `json:"flag,omitempty"`     // the oldest of Tasks
	Tasks       []Task      `json:"tasks,omitempty"`    // what it waits on you for, oldest first
	Dispatch    *Proposal   `json:"dispatch,omitempty"` // crew waiting for you to launch
	Conclude    *Conclusion `json:"conclude,omitempty"` // how it ends, suggested
	// Ending is how it ended, once you said; EndedCapsules are capsules
	// that ended alone, before it.
	Ending        *Conclusion    `json:"ending,omitempty"`
	EndedCapsules []EndedCapsule `json:"endedCapsules,omitempty"`
	// At is where it is on its route, in words; Z what has to be true for
	// it to end there, each part checked when it ends.
	At       string     `json:"at,omitempty"`
	Z        []string   `json:"z,omitempty"`
	ReReview []ReReview `json:"reReview,omitempty"` // capsules waiting for you to re-review
	// Headlines is where it stands, most pressing first: the first is the
	// mission's.
	Headlines []Headline `json:"headlines"`
	// Cards is the headlines as the page draws them, the lead first: each
	// with its title, its sentence, the one action meant to move it and
	// what comes after (mission.Cards).
	Cards []mission.Card `json:"cards"`
	// Runs are the orchestrator's latest runs on it, newest first.
	Runs      []OrchestratorRun `json:"runs"`
	Stages    []Stage           `json:"stages"`
	Capsules  []CapsuleDetail   `json:"capsules"`
	Crew      []CrewDetail      `json:"crew"`
	Artifacts []Artifact        `json:"artifacts"`
	// Bundles are its verification bundles, oldest first (its Verify tab).
	Bundles []Bundle `json:"bundles"`
	// Decisions are what's been decided on it and why, newest first: those
	// that stand, then those superseded or withdrawn.
	Decisions []Decision `json:"decisions"`
	Events    []Event    `json:"events"` // the log, oldest first
	// GitHub: GitHub has answered for its capsules' repos, so a capsule
	// with no PR has none (as last asked).
	GitHub bool `json:"github"`
	// CrewPicks are what Launch crew may start crew on.
	CrewPicks CrewPicks `json:"crewPicks"`
	// LevelOn is what crew run on at each level of a Work step, by level
	// (rote, normal, hard), as [crew.levels] has it.
	LevelOn map[string]string `json:"levelOn,omitempty"`
}

// Bundle is a verification bundle, as the mission's Verify tab shows it:
// what was handed over to show work is as its checks say.
type Bundle struct {
	ID string `json:"id"`
	// Step is its step's number as the plan has it now (0: no longer
	// there, or it was of none), and StepName how that's listed.
	Step     int    `json:"step,omitempty"`
	StepName string `json:"stepName,omitempty"`
	// Crew made it, on By (claude on sonnet); Own: they did the work it
	// shows.
	Crew string `json:"crew"`
	By   string `json:"by,omitempty"`
	Own  bool   `json:"own,omitempty"`
	// Capsule and Commit are what it's a bundle of. Stale: its capsule
	// has moved on since, to Head.
	Capsule string    `json:"capsule,omitempty"`
	Commit  string    `json:"commit,omitempty"`
	Stale   bool      `json:"stale,omitempty"`
	Head    string    `json:"head,omitempty"`
	At      time.Time `json:"at"`
	// Replaces is the bundle it took the place of; ReplacedBy, the one
	// that took its.
	Replaces   string `json:"replaces,omitempty"`
	ReplacedBy string `json:"replacedBy,omitempty"`
	// Verdict is yours of it: approved, or sent back with Notes, what you
	// said wasn't right ("" until you say), and VerdictAt when. Looked are
	// its checks you've marked as looked at, by their keys in its list.
	Verdict   BundleVerdict `json:"verdict,omitempty"`
	VerdictAt *time.Time    `json:"verdictAt,omitempty"`
	Notes     []BundleNote  `json:"notes,omitempty"`
	Looked    []string      `json:"looked,omitempty"`
	// Asked: the mission is at a step of yours (AskedAt) that looks at
	// it, so approving it passes that step: a Review step, or its own
	// step with its checks yours to say.
	Asked       bool          `json:"asked,omitempty"`
	AskedAt     int           `json:"askedAt,omitempty"`
	Summary     string        `json:"summary"`
	Try         []BundleTry   `json:"try,omitempty"`
	Checks      []BundleCheck `json:"checks"`
	NotVerified []string      `json:"notVerified,omitempty"`
	// Noticed is what its crew came across that isn't one of its checks.
	Noticed []string `json:"noticed,omitempty"`
	// Ran are its step's checks that are commands, as ws last ran them in
	// its capsule at its commit.
	Ran []BundleRan `json:"ran,omitempty"`
}

// BundleNote is what you said isn't right of a bundle: of a check, by
// its number, or (0) of it as a whole.
type BundleNote struct {
	Check int    `json:"check"`
	Text  string `json:"text"`
}

// BundleTry is something to do to see it yourself: Command where it
// starts with one; Look, what to look for. Recipe is the recipe a command
// that's `ws run <recipe>` alone asks for: the page starts it in the
// bundle's capsule with a button, where the capsule has it.
type BundleTry struct {
	Text    string `json:"text"`
	Command string `json:"command,omitempty"`
	Recipe  string `json:"recipe,omitempty"`
	Look    bool   `json:"look,omitempty"`
}

// BundleCheck is one of a bundle's checks: what it came to, and what
// shows it.
type BundleCheck struct {
	N       int              `json:"n"`
	Check   string           `json:"check,omitempty"` // the step's, "" for one of the bundle's own
	Verdict CheckVerdict     `json:"verdict"`         // met, unmet, can't tell
	Said    string           `json:"said"`
	Points  []string         `json:"points,omitempty"` // facts of the same kind, listed under what it said
	Shows   []BundleEvidence `json:"shows,omitempty"`
	Lines   []string         `json:"lines,omitempty"`
	Try     []BundleTry      `json:"try,omitempty"`
	Asks    []string         `json:"asks,omitempty"`
	// Judged is what the judge last said of it, once ws had its step's
	// checks judged.
	Judged *BundleRan `json:"judged,omitempty"`
}

// BundleEvidence is a file that shows a check, served from URL: an image,
// a video, or text.
type BundleEvidence struct {
	Name    string          `json:"name"`
	URL     string          `json:"url"`
	Kind    EvidenceKind    `json:"kind"` // image, video or text
	Size    int64           `json:"size,omitempty"`
	Caption string          `json:"caption,omitempty"`
	Before  *BundleEvidence `json:"before,omitempty"`
	MadeBy  string          `json:"madeBy,omitempty"`
}

// BundleRan is what a check of the bundle's step came to when ws ran it
// (a command) or had it judged.
type BundleRan struct {
	N        int          `json:"n,omitempty"` // its number in the step
	Check    string       `json:"check"`
	Result   CheckVerdict `json:"result"`
	Evidence string       `json:"evidence,omitempty"`
	By       string       `json:"by"`
	// Kept is the run ws kept of a command it ran, by its ref: all it
	// printed and its reports are on the Runs page (/runs?show=).
	Kept string `json:"kept,omitempty"`
}

// CrewPicks are the harnesses and models new crew may be launched on from
// a mission's page: Default, what Launch crew starts them on, and Allowed,
// the others [crew] allowed lists (none: just the default).
type CrewPicks struct {
	Default Pick   `json:"default"`
	Allowed []Pick `json:"allowed"`
}

// Pick is a harness, and the model to run on it ("" for the agent's own).
type Pick struct {
	Harness string `json:"harness"`
	Model   string `json:"model,omitempty"`
}

// DecisionAsked is the question a decision answers: who asked (crew by
// id, "orchestrator" or "ws") and what.
type DecisionAsked struct {
	By   string `json:"by"`
	Text string `json:"text"`
}

// Decision is what was decided on a mission, and why.
type Decision struct {
	ID    string    `json:"id"`
	What  string    `json:"what"`
	Why   string    `json:"why,omitempty"`
	By    string    `json:"by"`
	From  string    `json:"from,omitempty"`
	About []string  `json:"about,omitempty"`
	At    time.Time `json:"at"`
	Step  int       `json:"step,omitempty"` // the plan's step it was made at; 0 for none, or not known
	// Asked is the question it answers, for one made by answering.
	Asked        *DecisionAsked `json:"asked,omitempty"`
	State        DecisionState  `json:"state"` // current, superseded, withdrawn
	SupersededBy string         `json:"supersededBy,omitempty"`
	WithdrawnWhy string         `json:"withdrawnWhy,omitempty"`
}

// ReReview is a capsule back with you: crew have addressed a review that
// asked for changes.
type ReReview struct {
	Capsule  string `json:"capsule"`
	Review   string `json:"review"`
	Answered int    `json:"answered"`
	Total    int    `json:"total"`
}

// Proposal is something raised for you, by whom.
type Proposal struct {
	Text    string   `json:"text"`
	By      string   `json:"by,omitempty"`
	Capsule string   `json:"capsule,omitempty"`
	Options []string `json:"options,omitempty"`
}

// Conclusion is how a mission ends, suggested or made (mission.Conclusion):
// its ending ("" for a conclusion from before endings, which only says it
// looks finished), what it came to, Z as checked, and where it was handed
// on to.
type Conclusion struct {
	Ending mission.Ending `json:"ending,omitempty"`
	Text   string         `json:"text"`
	Z      []ZCheck       `json:"z,omitempty"`
	To     []string       `json:"to,omitempty"`
	By     string         `json:"by,omitempty"`
	At     string         `json:"at,omitempty"`
}

// ZCheck is a part of Z, met or not, and how.
type ZCheck struct {
	Z   string `json:"z"`
	Met bool   `json:"met"`
	How string `json:"how,omitempty"`
}

// EndedCapsule is a capsule that ended alone: handed on (to a mission), or
// dropped.
type EndedCapsule struct {
	Repo   string        `json:"repo"`
	Name   string        `json:"name"`
	Ending CapsuleEnding `json:"ending"`
	To     string        `json:"to,omitempty"`
	Why    string        `json:"why,omitempty"`
	By     string        `json:"by,omitempty"`
	At     string        `json:"at"`
}

// Stage is one of the brief's stages, or its plan's steps, and where it's
// got to.
type Stage struct {
	N       int           `json:"n"`
	Title   string        `json:"title"` // a few words
	Text    string        `json:"text"`  // the whole stage, on one line
	State   ops.StepState `json:"state"` // accepted, current, todo
	Crew    []string      `json:"crew,omitempty"`
	CheckIn bool          `json:"checkIn,omitempty"`
	Why     string        `json:"why,omitempty"` // why it was accepted
	// Who is who's on a step of the plan (crew, you, CI, outside, the
	// planner), and WhoSays that as the plan says it; Makes what it makes.
	// None for a brief's stage.
	// Type is its kind of step (Work, Review, Open Pull Request…).
	Type    StepType `json:"type,omitempty"`
	Who     StepWho  `json:"who,omitempty"`
	WhoSays string   `json:"whoSays,omitempty"`
	// In is where it works, as the plan says: a repo, or one of the
	// mission's capsules (repo/name). The Work tab lists a capsule's steps
	// by it.
	In    string `json:"in,omitempty"`
	Makes string `json:"makes,omitempty"`
	// Level is a Work step's level (rote, normal or hard), and LevelOn
	// what its crew run on at it ([crew.levels]); neither for a step that
	// names its model (on:), whose WhoSays says it.
	Level   string `json:"level,omitempty"`
	LevelOn string `json:"levelOn,omitempty"`
	// OpensPR: the step makes a pull request, which is opened through ws.
	OpensPR bool `json:"opensPR,omitempty"`
	// A Work step's expansion: the step whose crew it carries on from,
	// its limit as written and when its time runs out (or ran out), and
	// its checks, each with what the latest round came to (Round, its
	// number; 0 before any).
	CarryOn   int          `json:"carryOn,omitempty"`
	Limit     string       `json:"limit,omitempty"`
	Deadline  *time.Time   `json:"deadline,omitempty"`
	OutOfTime bool         `json:"outOfTime,omitempty"`
	Checks    []StageCheck `json:"checks,omitempty"`
	Round     int          `json:"round,omitempty"`
	// The orchestrator's decisions on it: when it applies (OnlyIf) and
	// where it runs (InEach), as the plan says; whether it waits for a
	// call (Held), was skipped, or was started again (Attempts); and its
	// runs, one per repo.
	OnlyIf    string     `json:"onlyIf,omitempty"`
	InEach    string     `json:"inEach,omitempty"`
	Held      bool       `json:"held,omitempty"`
	Skipped   bool       `json:"skipped,omitempty"`
	Attempts  int        `json:"attempts,omitempty"`
	Instances []StageRun `json:"runs,omitempty"`
	// A Wait step's expansion: who it waits on (you, or who or what, in
	// the plan's words; "" for one that doesn't say), and how it's passed
	// (Passes: watched, ws running its checks; done, you marking it done;
	// said, anyone saying it's happened). Watch is since when it's waited,
	// and ws's last look at its checks, whose results are on Checks.
	On     string      `json:"on,omitempty"`
	Passes WaitPasses  `json:"passes,omitempty"`
	Watch  *StageWatch `json:"watch,omitempty"`
}

// StageWatch is a Wait step being waited for.
type StageWatch struct {
	Since  time.Time  `json:"since"`
	Looked *time.Time `json:"looked,omitempty"` // when ws last ran its checks
	Looks  int        `json:"looks,omitempty"`
	// Came is what the last look came to, in a few words: met, not yet,
	// ws couldn't tell.
	Came string `json:"came,omitempty"`
}

// StageRun is one of a step's runs, in a repo.
type StageRun struct {
	Repo    string `json:"repo"`
	Capsule string `json:"capsule,omitempty"`
	Done    bool   `json:"done,omitempty"`
}

// StageCheck is one of a Work step's checks, and what it last came to.
type StageCheck struct {
	Text string `json:"text"`
	Run  bool   `json:"run,omitempty"` // a command ws runs, not words the judge judges
	// Result is met, unmet or can't tell in the latest round ("" before
	// one), Evidence what showed it, and By who said.
	Result   CheckVerdict `json:"result,omitempty"`
	Evidence string       `json:"evidence,omitempty"`
	By       string       `json:"by,omitempty"`
	// Kept is the run ws kept of a command it ran, by its ref.
	Kept string `json:"kept,omitempty"`
}

// CapsuleDetail is a capsule with its git status and PR.
type CapsuleDetail struct {
	Capsule
	Dirty  bool `json:"dirty,omitempty"`
	Ahead  int  `json:"ahead,omitempty"`
	Behind int  `json:"behind,omitempty"`
	// Unread: whether it's dirty, and how far ahead and behind it is,
	// haven't been read yet and are being: the mission's event comes
	// when they have.
	Unread bool `json:"unread,omitempty"`
	PR     *PR  `json:"pr,omitempty"`
	// Direct: its repo has no pull requests, and its commits land straight
	// on the default branch when you say. ToLand is how many it has to
	// land; Landed is when they did.
	Direct bool       `json:"direct,omitempty"`
	ToLand int        `json:"toLand,omitempty"`
	Landed *time.Time `json:"landed,omitempty"`
	// Runs are what's running in it (ws run), and Recipes the names of
	// what its ws.repo.toml says it can run; Tasks are those of them that
	// end by themselves (tests, lint), whose result is kept.
	Runs    []RunDetail `json:"runs,omitempty"`
	Recipes []string    `json:"recipes,omitempty"`
	Tasks   []string    `json:"tasks,omitempty"`
}

// RunDetail is a run (ws run) as a capsule's page shows it.
type RunDetail struct {
	Ref    string `json:"ref"`
	Name   string `json:"name"`
	Recipe string `json:"recipe,omitempty"`
	// Type is service, for one kept up until it's ended, or task, for one
	// that ends by itself and is kept for what it came to. A task has no
	// link or port. N is which run of the task it is, and NRef names that
	// run alone (Ref with #n: Ref is always the task's latest). Result is
	// what it came to; Took how long it ran, in milliseconds; Commit where
	// its worktree stood, Dirty with changes not committed; Why, why ws
	// stopped it; Past how many earlier runs are kept behind it; Reports
	// the files kept with it; and Check the step's check it was run for.
	Type    runs.Type   `json:"type"`
	N       int         `json:"n,omitempty"`
	NRef    string      `json:"nref,omitempty"`
	Result  RunResult   `json:"result,omitempty"`
	Took    int64       `json:"took,omitempty"`
	Commit  string      `json:"commit,omitempty"`
	Dirty   bool        `json:"dirty,omitempty"`
	Why     string      `json:"why,omitempty"`
	Past    int         `json:"past,omitempty"`
	Reports []RunReport `json:"reports,omitempty"`
	Check   *RunCheck   `json:"check,omitempty"`
	// Link is where it's reached: by its name, when the workspace has
	// names (ws router), else Local, at 127.0.0.1 and its port.
	Link  string `json:"link"`
	Local string `json:"local"`
	Port  int    `json:"port"`
	// State is running, exited (Status its exit status) or gone.
	State  runs.State `json:"state"`
	Status int        `json:"status,omitempty"`
	// Up: something listens on its port.
	Up bool `json:"up,omitempty"`
	// Ends says when it's ended without being asked, in words, while it
	// runs; Ended is when it ended, once it has.
	Ends    string     `json:"ends,omitempty"`
	Ended   *time.Time `json:"ended,omitempty"`
	Owner   string     `json:"owner"`
	By      string     `json:"by,omitempty"`
	Command string     `json:"command"`
	Started time.Time  `json:"started"`
	// Where it runs: its repo, and its worktree said as a name has it (a
	// capsule's name, ground or silo); Dir is the folder.
	Repo     string `json:"repo"`
	Worktree string `json:"worktree"`
	Dir      string `json:"dir,omitempty"`
	// PID leads its process group, while it runs.
	PID int `json:"pid,omitempty"`
	// Who it's for, beyond Owner's kind: a mission's slug (and its step's
	// number, or its crew's id), a conversation's id, or a process.
	Mission      string `json:"mission,omitempty"`
	Step         int    `json:"step,omitempty"`
	Crew         string `json:"crew,omitempty"`
	Conversation string `json:"conversation,omitempty"`
	OwnerPID     int    `json:"ownerPid,omitempty"`
	// App is the app whose page it's for, for one owned by looking at it.
	App          string `json:"app,omitempty"`
	OwnerCommand string `json:"ownerCommand,omitempty"`
	// Also is how many others it's for besides Owner: those who asked
	// for it while it ran. Ends says when the last of them goes.
	Also int `json:"also,omitempty"`
	// Needs are what it was started needing, and NeededBy the running
	// runs that need it, by their refs: it isn't ended while there are any.
	Needs    []RunNeed `json:"needs,omitempty"`
	NeededBy []string  `json:"neededBy,omitempty"`
}

// RunResult is what a task came to.
type RunResult string

const (
	RunRunning RunResult = "running"
	RunPassed  RunResult = "passed"
	RunFailed  RunResult = "failed"
	// RunStopped is one that didn't end by itself: ws stopped it, or it
	// was killed.
	RunStopped RunResult = "stopped"
)

// RunReport is a report kept with a task: what its recipe calls it, the
// format it's in, the path its command wrote it at, and how many files
// were kept (0: it wasn't written).
type RunReport struct {
	Name   string `json:"name"`
	Format string `json:"format,omitempty"`
	Path   string `json:"path"`
	Files  int    `json:"files"`
}

// RunCheck is the check of a mission's step a task was run for.
type RunCheck struct {
	Mission string `json:"mission"`
	Step    int    `json:"step"`
	N       int    `json:"n"`
}

// RunNeed is one of a run's needs: another run, or an address outside ws.
type RunNeed struct {
	// Name is what the run calls it (api).
	Name string `json:"name"`
	// Run is the run that meets it, by its ref: none for an address
	// outside ws.
	Run string `json:"run,omitempty"`
	// Address is what the run was started with for it.
	Address string `json:"address"`
	// Env is the variable Address was handed over in, and Mount the path
	// of the run's own name the need answers at.
	Env   string `json:"env,omitempty"`
	Mount string `json:"mount,omitempty"`
	// Folder is a need of a repo's folder, not a run: Repo's Worktree
	// (said as a name has it), at Dir. Address is then "".
	Folder   bool   `json:"folder,omitempty"`
	Repo     string `json:"repo,omitempty"`
	Worktree string `json:"worktree,omitempty"`
	Dir      string `json:"dir,omitempty"`
}

// RunDetailOf is a run of the workspace at root as a page shows it.
func RunDetailOf(r runs.Run, root string, now time.Time) RunDetail {
	d := RunDetail{Ref: r.Ref(), Name: r.Name, Recipe: r.Recipe, Type: runs.Service, Link: router.RunAddress(root, r), Local: r.Link(), Port: r.Port, State: r.State,
		Owner: string(r.Owner.Kind), By: r.By, Command: r.Command, Started: r.Created,
		Repo: r.Repo, Worktree: runs.Label(r.Worktree), Dir: r.Dir,
		Mission: r.Owner.Mission, Step: r.Owner.Step, Crew: r.Owner.Crew, Conversation: r.Owner.Conversation, App: r.Owner.App,
		OwnerPID: r.Owner.PID, OwnerCommand: r.Owner.Command, Also: len(r.Also), NeededBy: r.NeededBy}
	for _, n := range r.Needs {
		rn := RunNeed{Name: n.Name, Run: n.Run, Address: n.Addr, Env: n.Env, Mount: n.Mount}
		if n.IsFolder() {
			rn.Folder, rn.Repo, rn.Worktree, rn.Dir = true, n.Repo, runs.Label(n.Worktree), n.Dir
		}
		d.Needs = append(d.Needs, rn)
	}
	if r.IsTask() {
		d.Type, d.Link, d.Local = runs.Task, "", ""
		d.N, d.NRef, d.Commit, d.Dirty, d.Why, d.Past = r.N, r.NRef(), r.Commit, r.Dirty, r.Why, r.Past
		d.Took = r.Took().Milliseconds()
		switch {
		case r.State == runs.Running:
			d.Result = RunRunning
		case r.Passed() && r.Why == "":
			d.Result = RunPassed
		case r.State == runs.Exited && r.Why == "":
			d.Result = RunFailed
		default:
			d.Result = RunStopped
		}
		for _, rp := range r.Reports {
			d.Reports = append(d.Reports, RunReport{Name: rp.Name, Format: rp.Format, Path: rp.Path, Files: len(r.ReportFiles(rp.Name))})
		}
		if c := r.Check; c != nil {
			d.Check = &RunCheck{Mission: c.Mission, Step: c.Step, N: c.N}
		}
	}
	switch r.State {
	case runs.Running:
		d.Ends, d.PID = runs.Ends(r, now), r.PID
		d.Up = !r.IsTask() && r.Listening()
	case runs.Exited:
		d.Status = r.Status
		fallthrough
	default:
		ended := r.Ended
		d.Ended = &ended
	}
	return d
}

// PR is a capsule's pull request.
type PR struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	URL    string `json:"url"`
	Merged bool   `json:"merged,omitempty"`
	Checks string `json:"checks,omitempty"`
}

// CrewDetail is crew with how it went.
type CrewDetail struct {
	Crew
	EndedAt *time.Time `json:"endedAt,omitempty"`
	Exit    string     `json:"exit,omitempty"`
	// How it ended, beyond its exit: "interrupted" (a daemon restart
	// stopped it), "handed-back" (ws closed its session once it had
	// handed its work back), "stopped" (you stopped it), or "".
	How      CrewEnded `json:"how,omitempty"`
	Note     string    `json:"note,omitempty"`     // with Status: what it said
	Dispatch string    `json:"dispatch,omitempty"` // what it was dispatched for
	Handoff  string    `json:"handoff,omitempty"`  // its handoff's artifact reference
}

// Artifact is something the mission made or relies on, and where it is now.
type Artifact struct {
	Ref   string    `json:"ref"`
	Kind  string    `json:"kind"`
	Label string    `json:"label,omitempty"`
	State string    `json:"state"` // draft, in review, landed, private, missing; open, merged; link
	URL   string    `json:"url,omitempty"`
	Path  string    `json:"path,omitempty"`
	By    string    `json:"by,omitempty"` // the crew that recorded it; "" for you
	Added time.Time `json:"added"`
	// Readable: it's a file here, so GET .../artifact shows it.
	Readable bool `json:"readable"`
	// Threads is where the threads on it stand, for a document with open
	// ones.
	Threads *ArtifactThreads `json:"threads,omitempty"`
	// Versions is how many versions it has, for an artifact (a3): it's
	// opened at /artifacts/a3.
	Versions int `json:"versions,omitempty"`
}

// ArtifactInfo is an artifact (a versioned HTML prototype) as the list,
// its refs and the sidebar show it: what it is, where it stands, its
// latest version, and where its versions came from.
type ArtifactInfo struct {
	ID     string          `json:"id"` // a3
	Title  string          `json:"title"`
	Status artifact.Status `json:"status"` // live, promoted, discarded
	// Gone: its folder isn't where it was, and it wasn't discarded.
	Gone bool `json:"gone,omitempty"`
	// Folder is its folder's name in .ws/artifacts, on this machine, and
	// Where the folder as it's said to people
	// (.ws/artifacts/2026-11-12-horizon-glow/).
	Folder string `json:"folder"`
	Where  string `json:"where"`
	// Home is where it lives in a repo, for everyone with it, promoted
	// there: none for one on this machine only. Here: promoted to stay on
	// this machine, whatever [artifacts] says.
	Home     *ArtifactHome `json:"home,omitempty"`
	Here     bool          `json:"here,omitempty"`
	Made     time.Time     `json:"made"`
	Versions int           `json:"versions"` // how many
	// Latest is its latest version; nil for one with none.
	Latest *ArtifactVersion `json:"latest,omitempty"`
	// From is where its versions came from, the latest first, each once: a
	// conversation (w9) or a mission (its slug), with its title then.
	From      []ArtifactFrom      `json:"from,omitempty"`
	Promoted  *artifact.Promotion `json:"promoted,omitempty"`
	Discarded *artifact.Discard   `json:"discarded,omitempty"`
	// Expires is when a live one expires ([artifacts] expire_after after
	// its last version, or its restoring); none for one that doesn't.
	// Expired: by now, until the daemon discards it, or Clear expired
	// does.
	Expires *time.Time `json:"expires,omitempty"`
	Expired bool       `json:"expired,omitempty"`
	// Erased: discarded, and its files deleted for good. Restorable:
	// discarded, and it can be put back.
	Erased     bool `json:"erased,omitempty"`
	Restorable bool `json:"restorable,omitempty"`
}

// ArtifactHome is where a promoted artifact lives in a repo: its folder
// there as it's said to people (records:artifacts/2026-11-12-horizon-glow/),
// the version that's there, and the commit that put it there.
type ArtifactHome struct {
	Where   string    `json:"where"`
	Repo    string    `json:"repo"`
	Version int       `json:"version"`
	Commit  string    `json:"commit,omitempty"`
	At      time.Time `json:"at"`
}

// ArtifactFrom is where versions of an artifact came from.
type ArtifactFrom struct {
	From  string `json:"from"`
	Title string `json:"title,omitempty"`
}

// ArtifactVersion is one version: what it's based on, who added it,
// where it was made, and its note.
type ArtifactVersion struct {
	N         int       `json:"n"`
	BasedOn   int       `json:"basedOn,omitempty"`
	At        time.Time `json:"at"`
	By        string    `json:"by"`
	From      string    `json:"from,omitempty"`
	Proposal  int       `json:"proposal,omitempty"`
	FromTitle string    `json:"fromTitle,omitempty"`
	Note      string    `json:"note,omitempty"`
	// Files is how many files it has, and Size what they come to, in
	// bytes; 0 while there are none to read (only on an artifact's page).
	Files int `json:"files,omitempty"`
	Size  int `json:"size,omitempty"`
}

// ArtifactDone is what promoting, discarding or restoring an artifact
// did: the artifact as it is now; the folder in a repo it was put in,
// taken out of or put back in, where it lives in one (Home), what landing
// that did, and whether it's the first time it was put there; and what it
// was promoted into (a mission started from it, or one it was recorded
// on).
type ArtifactDone struct {
	Artifact ArtifactInfo `json:"artifact"`
	Home     string       `json:"home,omitempty"`
	Landed   string       `json:"landed,omitempty"`
	First    bool         `json:"first,omitempty"`
	Into     string       `json:"into,omitempty"`
	Started  string       `json:"started,omitempty"`
	Recorded string       `json:"recorded,omitempty"`
}

// ArtifactsCleared is what Clear expired did: the refs of those it
// discarded.
type ArtifactsCleared struct {
	Local []string `json:"local"`
}

// ArtifactDetail is GET /api/artifacts/{id}: an artifact, with All its
// versions, oldest first, and the folder in a repo it has, or is offered,
// to live in (HomeOffered: where it lives, or, for one that lives in
// none, the folder [artifacts] names for it; none where there's none to
// offer).
type ArtifactDetail struct {
	ArtifactInfo
	All         []ArtifactVersion `json:"all"`
	HomeOffered string            `json:"homeOffered,omitempty"`
}

// ArtifactFound is an artifact in one of the workspace's repos that no
// artifact on this machine lives at: someone else's, or another
// machine's, as ws found it there. It's named by its folder, which is its
// name on every machine.
type ArtifactFound struct {
	// Name is its folder as ws names one
	// (records/artifacts/2026-11-20-horizon-glow/), and Where the same as
	// it's said to people (records:artifacts/2026-11-20-horizon-glow/).
	Name  string `json:"name"`
	Where string `json:"where"`
	Repo  string `json:"repo"`
	// Label is what ws calls the repo, and Group the folders it's grouped
	// in there, under where promoted artifacts live (kept, 2026/q4): none
	// for one straight in it. They're what the list is laid out by.
	Label string `json:"label"`
	Group string `json:"group,omitempty"`
	Title string `json:"title"`
	// Version is what its folder holds, as git names it, and Frame where
	// that runs: another of each when any of its files changes.
	Version string `json:"version"`
	Frame   string `json:"frame"`
	// Changed is when it last changed in its repo, and ChangedBy who by.
	Changed   time.Time `json:"changed"`
	ChangedBy string    `json:"changedBy,omitempty"`
	// Promoted says who promoted the version that's there, when, and
	// which of its versions it is, where its record there says.
	Promoted *artifact.Promotion `json:"promoted,omitempty"`
}

// ArtifactFoundDetail is GET /api/artifacts/found/detail?name=…: one of
// them, with what its record says each version up to the one there
// changed (oldest first), and its files.
type ArtifactFoundDetail struct {
	ArtifactFound
	All   []ArtifactVersion   `json:"all"`
	Files []ArtifactFoundFile `json:"files"`
	Size  int                 `json:"size"`
}

// ArtifactFoundFile is one of a found artifact's files: its path in the
// artifact's folder, and its size.
type ArtifactFoundFile struct {
	Path string `json:"path"`
	Size int    `json:"size"`
}

// ArtifactsFound is GET /api/artifacts/found: every one of them, the
// latest changed first.
type ArtifactsFound struct {
	Found []ArtifactFound `json:"found"`
}

// ArtifactSource is GET /api/artifacts/{id}/source: one of a version's
// files (its page, unless another's asked for), as text, and the
// version's files. Binary: it isn't text, and Text is empty.
type ArtifactSource struct {
	Version int            `json:"version"`
	File    string         `json:"file"`
	Text    string         `json:"text"`
	Binary  bool           `json:"binary,omitempty"`
	Size    int            `json:"size"`
	Files   []ArtifactFile `json:"files"`
}

// ArtifactFile is one of a version's files: its path in the version, and
// its size.
type ArtifactFile struct {
	Path string `json:"path"`
	Size int    `json:"size"`
}

// ArtifactCompare is GET /api/artifacts/compare: two versions' files, a's
// against b's: each file that differs, as a review's file is diffed, by
// path. None: the two are the same.
type ArtifactCompare struct {
	A     string                `json:"a"` // a3.v1
	B     string                `json:"b"`
	Files []ArtifactCompareFile `json:"files"`
}

// ArtifactCompareFile is one file that differs between two versions: its
// diff (added, deleted or modified; Binary for one that isn't text, which
// has no hunks), and its lines in b, to show the unchanged ones between
// the changes.
type ArtifactCompareFile struct {
	File  review.File `json:"file"`
	Lines []string    `json:"lines"`
}

// ArtifactThreads is a document's open threads, and how many wait on you.
type ArtifactThreads struct {
	Open    int `json:"open"`
	Waiting int `json:"waiting"`
}

// Event is a line of the mission's log, in words (mission.Sentence).
type Event struct {
	Time time.Time `json:"time"`
	Kind string    `json:"kind"`
	Text string    `json:"text"`
	By   string    `json:"by,omitempty"` // who: a crew id, "orchestrator", or "" (you, or ws)
	// To is whom a message to crew is for (resume_with, told): By is then
	// who said it ("you", "orchestrator", "ws"), "" for one logged before
	// ws said.
	To string `json:"to,omitempty"`
}

// ArtifactContent is GET /api/missions/{slug}/artifact?ref=: a readable
// artifact's text.
type ArtifactContent struct {
	Artifact
	Text     string `json:"text"`
	Markdown bool   `json:"markdown"`
	// Version names Text (conversation.Version); Annotatable says it's a
	// document you can comment on.
	Version     string `json:"version"`
	Annotatable bool   `json:"annotatable"`
	// Stage is the stage of the brief's plan it was made for, if it was.
	Stage *DocStage `json:"stage,omitempty"`
	// Editable says it can be edited here (it's in a capsule, or the
	// mission's own); Uneditable says why not, if it can't.
	Editable   bool   `json:"editable"`
	Uneditable string `json:"uneditable,omitempty"`
	// In is the worktree its text was read from, for a file in a repo: its
	// links to other files open them there, and its images are read from
	// beside it.
	In *DocIn `json:"in,omitempty"`
	// Mission is the mission that records it, for a document opened as a
	// file (not as that mission's): its threads are there.
	Mission *DocMission `json:"mission,omitempty"`
	// Saves is how an edit to it is saved, for a document outside a
	// mission: where it's written depends on where it is and how its repo
	// lands.
	Saves *DocSaves `json:"saves,omitempty"`
	// DocKind is which of the workspace's kinds of document it is, if it's
	// one ([documents.<name>]).
	DocKind *DocKind `json:"docKind,omitempty"`
	// Page is the page it was written in, for a document filed from one.
	Page string `json:"page,omitempty"`
	// Media is its content type, for an image or a video, shown as it is
	// from Raw (GET .../artifact/raw?ref=), with no Text.
	Media string `json:"media,omitempty"`
	Raw   string `json:"raw,omitempty"`
}

// DocKind is a kind of document the workspace has, as one of its
// documents is of it: what one's called and where they go, the file that
// says how one's written, and the first line of a commit that changes this
// one, from the kind's message ("" if it has none).
type DocKind struct {
	Name    string `json:"name"`
	Title   string `json:"title"`
	Where   string `json:"where"`
	Guide   string `json:"guide,omitempty"`
	Subject string `json:"subject,omitempty"`
	// Passes are the workspace's own passes that are the kind's: tabs on a
	// page of the kind, and on no other.
	Passes []string `json:"passes,omitempty"`
}

// DocSaves is how saving a document outside a mission goes. How is:
//
//	capsule  it's in a capsule: written and committed there, as you
//	land     on the default branch of a repo with no pull requests: a
//	         commit of yours, landed straight there (Verify must pass)
//	lift     on the default branch of a repo that takes pull requests: a
//	         capsule is lifted for it, with your commit, and left to you
type DocSaves struct {
	How    ops.SaveHow `json:"how"`
	Branch string      `json:"branch"`           // the default branch
	Verify string      `json:"verify,omitempty"` // what must pass before it lands
	// Subject is the commit's first line, unless you give one.
	Subject string `json:"subject"`
}

// DocMission is the mission that records a document, and the ref it has
// there.
type DocMission struct {
	Slug  string `json:"slug"`
	Title string `json:"title"`
	Ref   string `json:"ref"`
}

// DocIn is where a document is: a repo, one of its worktrees (".ground",
// or a capsule's name) and its path there.
type DocIn struct {
	Repo     string `json:"repo"`
	Worktree string `json:"worktree"`
	Path     string `json:"path"`
}

// Repo is one of the workspace's repos, as ws labels it: its display
// name (Label) and colour (CSS), its group and whether that's hidden,
// whether it's cloned, its capsules, and its aliases (what else it's
// called, which name it in what's written).
type Repo struct {
	Name   string `json:"name"`
	Label  string `json:"label"`
	Color  string `json:"color"`
	Group  string `json:"group,omitempty"`
	Hidden bool   `json:"hidden,omitempty"`
	Cloned bool   `json:"cloned"`
	// Documents: a space of documents, whole: its home is Documents.
	Documents bool              `json:"documents,omitempty"`
	Capsules  []string          `json:"capsules"`
	Aliases   []string          `json:"aliases"`
	Branches  map[string]string `json:"branches,omitempty"` // each capsule's branch
	// Landing is how work lands in it: "pr", or "direct" (straight to its
	// default branch).
	Landing ops.RepoLanding `json:"landing,omitempty"`
	// Yours: it's in ws.local.toml alone, not the team's ws.toml.
	Yours bool `json:"yours,omitempty"`
	// GitHub is where it is on GitHub, as owner/name: "" for nowhere.
	GitHub string `json:"github,omitempty"`
	// Cloning: it was just added, and is being cloned; Problem is why it
	// couldn't be, if it couldn't.
	Cloning bool   `json:"cloning,omitempty"`
	Problem string `json:"problem,omitempty"`
	// TeamGroup is the team's group for it, where yours (Group) is over
	// it; GroupYours: Group is yours (ws.local.toml), not the team's.
	TeamGroup  string `json:"teamGroup,omitempty"`
	GroupYours bool   `json:"groupYours,omitempty"`
	// GroupLabel is what its group's called where it's shown (ws.toml's
	// display_name, or the group's name), and GroupOrder where the group
	// comes among them, lowest first.
	GroupLabel string `json:"groupLabel,omitempty"`
	GroupOrder int    `json:"groupOrder,omitempty"`
}

// Worktree is a repo's ground (Name ".ground": its default branch,
// checked out) or one of its capsules, with its branch and where it
// stands against its upstream.
type Worktree struct {
	Name   string `json:"name"`
	Ground bool   `json:"ground,omitempty"`
	Branch string `json:"branch"`
	Dirty  bool   `json:"dirty,omitempty"`
	Ahead  int    `json:"ahead,omitempty"`
	Behind int    `json:"behind,omitempty"`
	// Unread: whether it's dirty, or where it stands, hasn't been read
	// yet and is being: the workspace's capsulesSeen moves when it has.
	Unread bool `json:"unread,omitempty"`
}

// DocSave is an edited document to save (POST .../artifact/save): its
// text, edited from version Base, and the commit's subject, if given.
type DocSave struct {
	Ref     string `json:"ref"`
	Base    string `json:"base"`
	Text    string `json:"text"`
	Message string `json:"message,omitempty"`
}

// DocSaved is what saving did: what to say, the document's version after,
// the commit (in a capsule), and whether it was merged with changes made
// meanwhile. Conflict, instead, is an edit that clashes with those
// changes: nothing's written.
type DocSaved struct {
	Message  string       `json:"message,omitempty"`
	Version  string       `json:"version"`
	Commit   string       `json:"commit,omitempty"`
	Merged   bool         `json:"merged,omitempty"`
	Conflict *DocConflict `json:"conflict,omitempty"`
	// Ref is where the document is now, when saving moved it: a document
	// on a default branch, saved into a capsule lifted for it.
	Ref string `json:"ref,omitempty"`
}

// DocConflict is an edit merged with what's there now, where the two clash:
// each clash is marked (<<<<<<< your edit … >>>>>>> current), to
// settle and save again from Version.
type DocConflict struct {
	Text    string `json:"text"`
	Clashes int    `json:"clashes"`
}

// DocStage is the stage a document was made for: approving it offers to
// accept the stage, while it's the current one.
type DocStage struct {
	N     int           `json:"n"`
	Title string        `json:"title"`
	State ops.StepState `json:"state"` // todo, current, accepted
}

// Task is something a mission waits on you for: a question, or a step of
// its route that's yours (to review, answer, land, or go on from).
type Task struct {
	N       int       `json:"n"`
	Verb    TaskVerb  `json:"verb"`
	Text    string    `json:"text"`
	Step    int       `json:"step,omitempty"`
	Options []string  `json:"options,omitempty"`
	By      string    `json:"by,omitempty"`
	At      time.Time `json:"at"`
}

// ActRequest is POST /api/missions/{slug}/act: an action on a mission, as
// its headline offers it. Verb is answer (Text is the answer: one of the
// flag's options, or your own words), dismiss (the flag, or else queued
// crew), accept (the current stage; Text is why, optionally), archive,
// unpark, launch (new crew on the current stage: on Harness and Model,
// one of the mission's CrewPicks, or the default if neither is given),
// launch-queued, brief (start a briefing agent), resume (Crew, ended) or
// stop (Crew, running).
type ActRequest struct {
	Verb string `json:"verb"`
	Text string `json:"text,omitempty"`
	// Task is the task answer answers (the number its headline's action
	// targets); 0, the oldest.
	Task int    `json:"task,omitempty"`
	Crew string `json:"crew,omitempty"`
	// Step is the step of the plan level puts at another level (Text).
	Step int `json:"step,omitempty"`
	// Harness and Model are what launch starts crew on: one of the
	// mission's CrewPicks (neither: the default).
	Harness string `json:"harness,omitempty"`
	Model   string `json:"model,omitempty"`
	// How it ends, for end: its ending (none: as suggested), each part of
	// Z met or not ("<n>: <how>"), and the missions it's handed on to.
	// Capsule is the one that ends alone, for hand-on and drop (To, for
	// hand-on, a mission there is).
	Ending  string   `json:"ending,omitempty"`
	Met     []string `json:"met,omitempty"`
	Unmet   []string `json:"unmet,omitempty"`
	To      []string `json:"to,omitempty"`
	Capsule string   `json:"capsule,omitempty"`
	// Title and Draft are a pull request's, for open-pr (Text its
	// description, Capsule where it's opened from).
	Title string `json:"title,omitempty"`
	Draft bool   `json:"draft,omitempty"`
	// BurnAll, for archive: burn every capsule, even one whose work
	// hasn't merged (archiving is refused otherwise).
	BurnAll bool `json:"burnAll,omitempty"`
	// Bundle is the verification bundle bundle-approve, bundle-send-back
	// and bundle-looked are about; Notes, what's not right of it, for
	// sending it back; Check and Looked, the check of it (its key in the
	// bundle's list) marked as looked at, or not.
	Bundle string       `json:"bundle,omitempty"`
	Notes  []BundleNote `json:"notes,omitempty"`
	Check  string       `json:"check,omitempty"`
	Looked bool         `json:"looked,omitempty"`
	// PR is the pull request merge, merge-ready and merge-off are about,
	// as repo#N.
	PR string `json:"pr,omitempty"`
}

// MergeRequest asks for a pull request to merge (POST
// /api/prs/{repo}/{number}/merge): now (the default), once it can
// ("ready"), or no longer by itself ("off"), at the commit the page shows
// (Head; "" for the one ws last showed).
type MergeRequest struct {
	When string `json:"when,omitempty"`
	Head string `json:"head,omitempty"`
}

// MergeResult is what came of it: said, and GitHub's status (merged,
// pending, enqueued; on or off for merging by itself).
type MergeResult struct {
	Message string `json:"message"`
	Status  string `json:"status"`
}

// ActResult says what an action did.
type ActResult struct {
	Message string `json:"message"`
	// Path is a page to go to for what it started: a briefing conversation.
	Path string `json:"path,omitempty"`
}
