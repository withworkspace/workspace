package mission

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
)

// A mission's ball is always in one court: an agent's, yours, or outside
// (GitHub: reviewers, checks, a merge). Resolve works out whose, and says
// so in one headline: a sentence and the one action that moves it on. The
// web UI and ws mission show both read it, so they can't disagree.

// Court is who a mission waits on.
type Court string

const (
	CourtAgent   Court = "agent"   // crew, a briefing agent or the orchestrator is on it
	CourtYou     Court = "you"     // it waits on you
	CourtOutside Court = "outside" // it waits on GitHub: reviewers, checks, a merge
	CourtNone    Court = ""        // it waits on no one: it's ended or archived
)

// Kind is the situation a headline describes.
type Kind string

// Kinds, most pressing first: Resolve orders headlines this way.
const (
	// Your court, now.
	KindCrewAsks         Kind = "crew-asks"         // crew asked you something (ws mission ask)
	KindPermission       Kind = "permission"        // crew stopped to ask for a permission
	KindFlag             Kind = "flag"              // the orchestrator (or crew) flagged a question
	KindReReview         Kind = "re-review"         // a capsule is back for your re-review
	KindStepReview       Kind = "step-review"       // a Review step: what the steps before made, yours to review
	KindReplies          Kind = "replies"           // threads on a document have answers for you
	KindBriefTalk        Kind = "brief-talk"        // its briefing conversation waits for you: a question, or the brief to confirm
	KindProposed         Kind = "proposed"          // a conversation about it proposed something for you to confirm (crew, a decision, a mission…)
	KindQueued           Kind = "queued"            // crew queued for you to launch
	KindResume           Kind = "resume"            // the orchestrator suggests resuming crew
	KindToDo             Kind = "to-do"             // a Wait step on you: something you do yourself, and mark done
	KindReplied          Kind = "replied"           // crew finished its turn without asking or reporting done
	KindChecksFailing    Kind = "checks-failing"    // a PR's checks are failing
	KindChangesRequested Kind = "changes-requested" // a PR's review asks for changes
	KindConflict         Kind = "conflict"          // a PR conflicts with its base
	KindApprove          Kind = "approve"           // the current stage's document: its threads are resolved, approve it?
	KindAccept           Kind = "accept"            // crew reported the stage done; the orchestrator is off
	KindMerge            Kind = "merge"             // a PR is ready to merge
	KindLand             Kind = "land"              // a capsule's commits are ready to land, straight to the default branch
	KindWrapUp           Kind = "wrap-up"           // a conclusion is suggested
	KindStalled          Kind = "stalled"           // crew worked on it; nobody is now, nothing is pending

	// The agents' court. Of these, a mission shows one at a time, the
	// first that applies (oneBanner): crew on its route come before the
	// agents beside it.
	KindWorking   Kind = "working"    // crew working
	KindLaunching Kind = "launching"  // ws starting crew: until they're recorded, then until their first activity
	KindOwnWork   Kind = "own-work"   // crew ended a turn to wait on work they left running
	KindAnswering Kind = "answering"  // agents answering threads on a document
	KindBriefing  Kind = "briefing"   // a briefing agent writing the brief, or the planner at work on the plan
	KindDeciding  Kind = "deciding"   // the orchestrator running, or about to
	KindChecking  Kind = "checking"   // ws running and having judged a step's checks
	KindMovingOn  Kind = "moving-on"  // ws between two things it does itself: passing a step and reaching the next, taking in a merge at its Open Pull Request step, asking GitHub of a pull request it's just opened
	KindWritingPR Kind = "writing-pr" // the PR writer writing a pull request ws then opens

	// Outside.
	KindChecksRunning  Kind = "checks-running"  // a PR's checks are running
	KindAwaitingReview Kind = "awaiting-review" // a PR waits for review or a merge
	KindWaitingOn      Kind = "waiting-on"      // waiting on something outside: a Wait step (watched, or not), or what the orchestrator said (ws mission wait)

	// Your court, for work not started, paused or finished.
	KindLanded  Kind = "landed"   // every PR merged, every capsule landed: archive it
	KindEnded   Kind = "ended"    // it ended, as you said: landed, answered, handed on or dropped
	KindToBrief Kind = "to-brief" // waiting for a briefing agent
	KindReady   Kind = "ready"    // briefed, nobody has started it
	KindParked  Kind = "parked"   // paused by you

	KindArchived Kind = "archived" // history: no court
)

var kindOrder = []Kind{
	KindCrewAsks, KindPermission, KindFlag, KindReReview, KindStepReview, KindToDo, KindReplies, KindBriefTalk, KindProposed, KindQueued, KindResume, KindReplied,
	KindChecksFailing, KindChangesRequested, KindConflict, KindApprove, KindAccept, KindMerge, KindLand, KindWrapUp, KindStalled,
	KindWorking, KindLaunching, KindOwnWork, KindAnswering, KindBriefing, KindChecking, KindWritingPR, KindMovingOn, KindDeciding,
	KindChecksRunning, KindAwaitingReview, KindWaitingOn,
	KindLanded, KindEnded, KindToBrief, KindReady, KindParked, KindArchived,
}

// Rank is how pressing a kind is: lower first.
func (k Kind) Rank() int {
	if i := slices.Index(kindOrder, k); i >= 0 {
		return i
	}
	return len(kindOrder)
}

// Verb is what a headline's action does. Each place that shows a headline
// (the web UI, the CLI) does it its own way.
type Verb string

const (
	VerbNone       Verb = ""              // nothing to do
	VerbReply      Verb = "reply"         // talk to crew Target: it asked, or replied
	VerbPermission Verb = "permission"    // answer crew Target's permission prompt, in its terminal
	VerbWatch      Verb = "watch"         // look at crew Target working
	VerbAnswer     Verb = "answer"        // answer the mission's flag (Options suggest answers)
	VerbReview     Verb = "review"        // review capsule Target
	VerbReviewDoc  Verb = "review-doc"    // open the document Target (an artifact's ref) at its threads
	VerbReviewStep Verb = "review-step"   // review what a Review step looks at (Look), for its task Target; go on passes it
	VerbLaunch     Verb = "launch"        // launch new crew, offering to resume crew Target if given
	VerbLaunchNext Verb = "launch-queued" // launch the queued crew
	VerbResume     Verb = "resume"        // resume crew Target
	VerbAccept     Verb = "accept"        // accept the current stage
	VerbWait       Verb = "wait"          // a Wait step on someone else: mark it done (it's happened), or, watched, have ws look now
	VerbOpen       Verb = "open"          // open URL (a PR)
	VerbMerge      Verb = "merge"         // merge PR Target, at URL
	VerbLand       Verb = "land"          // review capsule Target's commits, and land them (ws land)
	VerbArchive    Verb = "archive"       // archive the mission
	VerbEnd        Verb = "end"           // end the mission: as its conclusion suggests, or (none suggested, every step done) as its work stands
	VerbBrief      Verb = "brief"         // start briefing it: a briefing conversation, or agent
	VerbConfirm    Verb = "confirm"       // confirm its brief and plan, both written: it starts
	VerbTalk       Verb = "talk"          // open the mission's conversation Target (t3) in the web UI
	VerbUnpark     Verb = "unpark"        // take the mission off parked
	VerbLog        Verb = "log"           // read the log: the orchestrator is at work
)

// Action is the one thing that moves a headline on.
type Action struct {
	Verb    Verb
	Label   string   // a few words: "answer c2", "open api#88"
	Target  string   // crew id, capsule (repo/name) or PR (repo#N)
	URL     string   // a PR's page
	Options []string // a flag's suggested answers
	// Look is what a Review step's review looks at: bundles to approve,
	// capsules to review, documents to read. On a flag that's a step's
	// checks, yours to say, it's the bundles that show them.
	Look []Look
}

// Headline is where a mission stands: whose court it's in, in a sentence,
// and what moves it on.
type Headline struct {
	Court  Court
	Kind   Kind
	Label  string // a few words for lists, with a glyph: "? asks"
	Text   string // one sentence: "c2 asks: which cache key?"
	Action Action
	By     string    // who it's from or about: crew id, orchestrator, repo#N
	Since  time.Time // when it began, when known
	// Checking is what's running, while ws checks a step (KindChecking).
	Checking *Checking
}

// Key identifies a headline across re-reads: the same key is the same
// news, so it's announced once. Records keep times to the second, so the
// key does too.
func (h Headline) Key() string {
	return fmt.Sprintf("%s|%s|%d", h.Kind, h.By, h.Since.Unix())
}

// Group is how lists of missions group them.
type Group int

const (
	GroupNeedsYou   Group = iota // your court, now
	GroupInProgress              // the agents' court
	GroupWaiting                 // outside
	GroupUpNext                  // your court, for work not started or paused
	GroupPast                    // landed or archived
)

// GroupNames are the groups' headings.
var GroupNames = [...]string{"Needs you", "In progress", "Waiting", "Up next", "Past"}

// Group is the group a mission with this headline belongs in.
func (h Headline) Group() Group {
	switch h.Kind {
	case KindLanded, KindEnded, KindArchived:
		return GroupPast
	case KindToBrief, KindReady, KindParked:
		return GroupUpNext
	}
	switch h.Court {
	case CourtAgent:
		return GroupInProgress
	case CourtOutside:
		return GroupWaiting
	}
	return GroupNeedsYou
}

// Snapshot is what Resolve needs to know about a mission.
type Snapshot struct {
	M     *Mission
	State State  // as DeriveState works it out
	Brief string // for the stages
	// Events is the log's tail, oldest first.
	Events []Event
	// PRs are the capsules' PRs, open or merged, when GitHub was asked.
	PRs []PRFacts
	// Direct is the capsules of repos that take commits straight to their
	// default branch, with no pull request, that have commits to land.
	Direct []DirectFacts
	// ReReview is capsules back with you to re-review.
	ReReview []ReReview
	// Threads is the documents whose threads wait on you (someone
	// answered) or on an agent (it's answering).
	Threads DocThreads
	// CapsulesLanded: every capsule with work has merged its PR or landed
	// (CapsulesLanded), as far as GitHub was asked.
	CapsulesLanded bool
	// OrchestratorOff is [orchestrator] enabled = false.
	OrchestratorOff bool
	Now             time.Time
	// Seen is what a viewer of crew terminals (the deck) knows beyond the
	// records; zero elsewhere.
	Seen Seen
}

// PRFacts is what's known about a capsule's PR.
type PRFacts struct {
	Capsule string // repo/name
	Ref     string // repo#N
	URL     string
	Merged  bool
	// Review is GitHub's review decision: APPROVED, CHANGES_REQUESTED,
	// REVIEW_REQUIRED, or "" (none needed).
	Review string
	// Checks is the checks' rollup: failure, pending, success, or "" (none).
	Checks string
	// Conflict: it doesn't merge cleanly onto its base.
	Conflict bool
	// MergeUnknown: GitHub is still working out whether it merges cleanly.
	MergeUnknown bool
	// Unasked: ws opened it, and GitHub hasn't been asked how it stands
	// since. No review and no checks, of such a one, is nothing known, not
	// a pull request that needs neither.
	Unasked bool
}

// DirectFacts is a capsule of a repo with no pull requests that has commits
// its default branch hasn't: they land when you say (ws land), and you're
// the only gate there is.
type DirectFacts struct {
	Capsule string // repo/name
	Branch  string // the default branch they'd land on
	Ahead   int    // commits to land
	Dirty   bool   // it has changes not committed too
	// Approved: your latest review of it approves it as it is now.
	Approved bool
}

// ReReview is a capsule whose latest review asked for changes, that crew
// have since moved on: it's back with you.
type ReReview struct {
	Capsule  string
	Review   string // the review's id
	Answered int    // its comments answered since
	Total    int
}

// DocThreads is where the threads on a mission's documents stand, by
// document: those waiting on you, and those an agent is answering.
type DocThreads struct {
	// Briefing is its briefing conversation, while it's open.
	Briefing           *BriefingTalk
	Replies, Answering []DocReplies
	// Approve is the current stage's documents you've commented on, with
	// every thread resolved, not approved as they are now.
	Approve []DocReplies
	// Blocking is the Change threads open on the current stage's
	// documents: the stage isn't accepted while there are any.
	Blocking []string
	// Proposed is the open conversations about it with proposals of their
	// agent's waiting for you to confirm, but those its briefing and its
	// planner's headlines say already.
	Proposed []TalkProposed
}

// TalkProposed is a conversation's open proposals: their kinds, oldest
// first.
type TalkProposed struct {
	ID    string
	Kinds []string
}

// proposedWords says what a conversation proposed: "crew and a decision",
// "2 decisions".
func proposedWords(kinds []string) (string, int) {
	words := map[string][2]string{
		"decision": {"a decision", "decisions"}, "mission": {"a mission", "missions"}, "dispatch": {"crew", "crew"},
		"document": {"a document", "documents"}, "comment": {"a review comment", "review comments"},
		"brief": {"an edit to the brief", "edits to the brief"}, "plan": {"an edit to the plan", "edits to the plan"},
		"replan": {"a replan", "replans"}, "artifact": {"an artifact", "artifacts"},
	}
	var order []string
	n := map[string]int{}
	for _, k := range kinds {
		if n[k] == 0 {
			order = append(order, k)
		}
		n[k]++
	}
	var said []string
	for _, k := range order {
		w, ok := words[k]
		if !ok {
			w = [2]string{"a " + k, k + "s"}
		}
		if n[k] == 1 || k == "dispatch" {
			said = append(said, w[0])
		} else {
			said = append(said, fmt.Sprintf("%d %s", n[k], w[1]))
		}
	}
	switch len(said) {
	case 0:
		return "", 0
	case 1:
		return said[0], len(kinds)
	}
	return strings.Join(said[:len(said)-1], ", ") + " and " + said[len(said)-1], len(kinds)
}

// BriefingTalk is where a mission's briefing conversation stands: its
// agent answering, or waiting for you (it asked, or proposed the brief).
type BriefingTalk struct {
	ID        string
	Answering bool
	Waiting   bool
	Proposed  bool // a brief waiting to be confirmed
	// Once it's briefed, it's the planner's conversation: PlanWhy is what
	// makes a revision the planner proposes yours to confirm, while one's
	// open.
	PlanWhy string
}

// DocReplies is a document with threads in one court.
type DocReplies struct {
	Doc     string // the artifact's ref
	Name    string // how a sentence names it: its path
	Waiting int    // how many
	Threads []string
}

// Seen is what the deck knows from crew terminals.
type Seen struct {
	// Running is the crew whose sessions are running. Nil means a crew
	// record without an end is running.
	Running map[string]bool
	// Still is how long each running crew's screen has been still.
	Still map[string]time.Duration
}

// StillAfter is how long crew's screen must stay still before crew that
// say they're working (or say nothing) count as waiting: an approval
// prompt mid-turn, which not every agent reports.
var StillAfter = 5 * time.Second

// CrewState is where one crew stands.
type CrewState string

const (
	CrewStarting   CrewState = "starting" // ws is starting its session: no activity of its own yet
	CrewWorking    CrewState = "working"
	CrewWaiting    CrewState = "waiting" // its turn is over: it said nothing about itself
	CrewAsks       CrewState = "asking"
	CrewPermission CrewState = "permission"
	CrewHandedBack CrewState = "done" // reported done, or finished briefing
	CrewEnded      CrewState = "ended"
)

func (s Snapshot) now() time.Time {
	if s.Now.IsZero() {
		return time.Now()
	}
	return s.Now
}

func (s Snapshot) running(c Crew) bool {
	if s.Seen.Running != nil {
		return s.Seen.Running[c.ID]
	}
	return c.Ended == nil
}

// Crew works out where crew stands, and since when.
func (s Snapshot) Crew(c Crew) (CrewState, time.Time) {
	switch {
	case !s.running(c):
		if c.Ended != nil {
			return CrewEnded, *c.Ended
		}
		return CrewEnded, c.Started
	case c.Starting != nil:
		// A mark ws set as it recorded them, and clears: not a guess from
		// what they last did, which for crew resumed is from before.
		return CrewStarting, *c.Starting
	case c.HandedBack(s.M) && !c.Conversing && !c.InTurnSinceDone():
		// Crew who've reported done and are answering you since are shown
		// as any crew you're talking to: at work, then waiting with what
		// they said.
		return CrewHandedBack, timeOr(c.StatusAt, c.Started)
	case c.Status == CrewAsking:
		return CrewAsks, timeOr(c.StatusAt, c.Started)
	case c.Activity == ActivityQuestion:
		return CrewAsks, timeOr(c.ActivityAt, c.Started)
	case c.Activity == ActivityPermission:
		return CrewPermission, timeOr(c.ActivityAt, c.Started)
	}
	still, known := s.Seen.Still[c.ID]
	if !known {
		still = -1
	}
	working, since := Working(c.Activity, timeOr(c.ActivityAt, c.Started), still, s.now())
	if working {
		return CrewWorking, since
	}
	return CrewWaiting, since
}

// Working says whether running crew is working, and since when, from what
// its hooks last reported (activity, at) and how long its screen has been
// still (negative when that isn't known). Crew that say they're working
// but whose screen has been still a while are waiting; crew without hooks
// are judged by the screen alone, and without one, taken to be working.
func Working(activity string, at time.Time, still time.Duration, now time.Time) (bool, time.Time) {
	switch activity {
	case ActivityWorking:
		if still >= StillAfter {
			return false, now.Add(-still)
		}
		return true, at
	case ActivityWaiting, ActivityPermission:
		return false, at
	}
	if still < 0 {
		return true, at
	}
	return still < StillAfter, now.Add(-still)
}

func timeOr(t *time.Time, or time.Time) time.Time {
	if t != nil {
		return *t
	}
	return or
}

// Resolve works out where the mission stands: every headline that applies
// in your court and outside, and of those in the agents' court the one
// most pressing (oneBanner), all most pressing first. The first is the
// mission's; there's always one.
func Resolve(s Snapshot) []Headline {
	m := s.M
	if s.State == Archived {
		return []Headline{{Kind: KindArchived, Label: "archived", Text: "Archived"}}
	}
	if e := m.Ending; e != nil && s.State.Ended() {
		// It ended, as you said: what's left of it is history.
		return []Headline{endedHeadline(*e)}
	}
	var out []Headline
	add := func(h Headline) { out = append(out, h) }

	// Crew.
	var working, starting []Crew
	var workingSince time.Time
	busyIn := map[string]bool{} // capsules crew are at: their PRs are theirs
	for _, c := range m.Crew {
		st, since := s.Crew(c)
		if st != CrewEnded && st != CrewHandedBack && c.Capsule != "" {
			busyIn[c.Capsule] = true
		}
		switch st {
		case CrewAsks:
			question, options := c.Note, []string(nil)
			if c.Status != CrewAsking { // its question tool
				question, options = firstNonEmpty(c.Notice, "a question"), c.Options
			}
			add(Headline{Court: CourtYou, Kind: KindCrewAsks, Label: "? asks", By: c.ID, Since: since,
				Text:   c.ID + " asks: " + question,
				Action: Action{Verb: VerbReply, Label: "answer " + c.ID, Target: c.ID, Options: options}})
		case CrewPermission:
			text := c.ID + " needs your permission"
			if c.Notice != "" {
				text += ": " + c.Notice
			}
			add(Headline{Court: CourtYou, Kind: KindPermission, Label: "? permission", By: c.ID, Since: since, Text: text,
				Action: Action{Verb: VerbPermission, Label: "go to " + c.ID, Target: c.ID}})
		case CrewWaiting:
			if s.orchestrating() && c.Status == "" && !c.Conversing {
				continue // its turn ended after ws's message: the orchestrator's looking at it first
			}
			if work, ok := c.OwnWork(s.now()); ok {
				add(ownWorkHeadline(c, work))
				continue
			}
			text := c.ID + " is waiting for you"
			if c.Conversing {
				text = c.ID + " replied"
			}
			if said := firstLine(c.LastReply, 120); said != "" {
				text += ": " + said
			}
			add(Headline{Court: CourtYou, Kind: KindReplied, Label: "◐ waiting", By: c.ID, Since: since, Text: text,
				Action: Action{Verb: VerbReply, Label: "reply to " + c.ID, Target: c.ID}})
		case CrewStarting:
			starting = append(starting, c)
		case CrewWorking:
			if len(working) == 0 || since.Before(workingSince) {
				workingSince = since
			}
			working = append(working, c)
		}
	}
	if len(working) > 0 {
		add(workingHeadline(s, working, workingSince))
	}
	// ws starting crew: those recorded, until their first activity; and
	// before any are, the launch itself. Both are marks ws set and clears
	// (Mission.Launching, Crew.Starting), so there's no gap to guess across.
	if len(starting) > 0 {
		add(startingHeadline(starting))
	}
	if l := m.Launching; l != nil {
		text := "Starting crew"
		if l.What != "" {
			text += " on " + l.What
		}
		add(Headline{Court: CourtAgent, Kind: KindLaunching, Label: "… starting", By: "ws", Since: l.Since, Text: text,
			Action: Action{Label: "nothing to do"}})
	}
	// onIt: crew are at work on it, or about to be.
	onIt := len(working) > 0 || len(starting) > 0 || m.Launching != nil

	// Raised for you.
	// Each task of yours, oldest first: the queue. A question crew you
	// asked are answering is theirs for now.
	answering := slices.ContainsFunc(slices.Concat(working, starting), func(c Crew) bool { return c.Role == RoleAnswer })
	for _, t := range m.Tasks {
		if answering && t.Question() {
			continue
		}
		if t.Verb == TaskReview && t.Step > 0 {
			add(stepReviewHeadline(s, t.Step, t.Text, strconv.Itoa(t.N), firstNonEmpty(t.By, "ws"), t.At))
			continue
		}
		if t.Verb == TaskDo && t.Step > 0 {
			add(Headline{Court: CourtYou, Kind: KindToDo, Label: "→ to do", By: firstNonEmpty(t.By, "ws"), Since: t.At,
				Text:   t.Text,
				Action: Action{Verb: VerbAnswer, Label: "mark it done", Options: t.Options, Target: strconv.Itoa(t.N)}})
			continue
		}
		flag := Headline{Court: CourtYou, Kind: KindFlag, Label: "! asks you", By: firstNonEmpty(t.By, "orchestrator"), Since: t.At,
			Text:   flagWho(t.By) + ": " + t.Text,
			Action: Action{Verb: VerbAnswer, Label: "answer", Options: t.Options, Target: strconv.Itoa(t.N)}}
		if t.Verb == TaskChecks && t.Step > 0 {
			// Its checks are yours to say: what its crew handed over to
			// show them is the thing to look at.
			states, _ := m.StageProgress(s.Brief)
			flag.Action.Look = m.ChecksLooks(states, t.Step)
			if i := slices.IndexFunc(flag.Action.Look, func(l Look) bool { return !l.Approved }); i >= 0 {
				flag.Action.Label = "verify " + flag.Action.Look[i].Name
			} else if len(flag.Action.Look) > 0 {
				// You've approved what shows them, and the step still
				// waits (ws didn't pass it then: the capsule had moved
				// on, or the step couldn't be passed): there's nothing
				// left to approve, so passing it is what moves it on.
				flag.Text += fmt.Sprintf(" You've approved %s already, so there's nothing left to approve: %s to go on.", lookNames(flag.Action.Look), strings.ToLower(ChecksPass))
				flag.Action.Label = strings.ToLower(ChecksPass)
			}
		}
		add(flag)
	}
	for _, rr := range s.ReReview {
		add(Headline{Court: CourtYou, Kind: KindReReview, Label: "↻ re-review", By: rr.Capsule,
			Text: fmt.Sprintf("%s is ready for your re-review: %s asked for changes; %d of %d comments answered since",
				rr.Capsule, rr.Review, rr.Answered, rr.Total),
			Action: Action{Verb: VerbReview, Label: "review " + rr.Capsule, Target: rr.Capsule}})
	}
	for _, r := range s.Threads.Answering {
		comments := "a comment"
		if r.Waiting > 1 {
			comments = fmt.Sprintf("%d comments", r.Waiting)
		}
		add(Headline{Court: CourtAgent, Kind: KindAnswering, Label: "… answering", By: r.Name,
			Text:   fmt.Sprintf("an agent is answering %s on %s (%s)", comments, r.Name, strings.Join(r.Threads, ", ")),
			Action: Action{Verb: VerbReviewDoc, Label: "read " + r.Name, Target: r.Doc}})
	}
	for _, r := range s.Threads.Replies {
		replies := "a reply"
		if r.Waiting > 1 {
			replies = fmt.Sprintf("%d replies", r.Waiting)
		}
		add(Headline{Court: CourtYou, Kind: KindReplies, Label: "↩ replies", By: r.Name,
			Text:   fmt.Sprintf("%s on %s, waiting for you (%s)", replies, r.Name, strings.Join(r.Threads, ", ")),
			Action: Action{Verb: VerbReviewDoc, Label: "read the replies on " + r.Name, Target: r.Doc}})
	}
	// Its briefing conversation, until ws is starting it: confirmed, what
	// its agent last said isn't waiting on you any more.
	if b := s.Threads.Briefing; b != nil && s.State == Briefing && m.Launching == nil {
		switch {
		case b.Answering:
			add(Headline{Court: CourtAgent, Kind: KindBriefing, Label: "● briefing", By: b.ID,
				Text:   "The briefing agent is at work, in " + b.ID,
				Action: Action{Verb: VerbTalk, Label: "open " + b.ID, Target: b.ID}})
		case b.Proposed:
			add(Headline{Court: CourtYou, Kind: KindBriefTalk, Label: "↩ brief", By: b.ID,
				Text:   "The brief's proposed, in " + b.ID + ": confirm it, and the mission starts",
				Action: Action{Verb: VerbTalk, Label: "read the brief", Target: b.ID}})
		case b.Waiting:
			add(Headline{Court: CourtYou, Kind: KindBriefTalk, Label: "↩ briefing", By: b.ID,
				Text:   "The briefing agent is waiting for you, in " + b.ID,
				Action: Action{Verb: VerbTalk, Label: "answer it", Target: b.ID}})
		}
	}
	if b := s.Threads.Briefing; b != nil && m.Plan != nil && s.State != Briefing && s.State != Parked {
		// Briefed, its briefing conversation is the planner's.
		switch {
		case b.Answering:
			add(Headline{Court: CourtAgent, Kind: KindBriefing, Label: "● planning", By: "planner",
				Text:   "The planner is at work on the plan, in " + b.ID,
				Action: Action{Verb: VerbTalk, Label: "open " + b.ID, Target: b.ID}})
		case b.PlanWhy != "":
			add(Headline{Court: CourtYou, Kind: KindBriefTalk, Label: "↩ plan", By: "planner",
				Text:   "The planner proposes a revision of the plan, in " + b.ID + ": " + strings.TrimSuffix(b.PlanWhy, "."),
				Action: Action{Verb: VerbTalk, Label: "read the revision", Target: b.ID}})
		}
	}
	for _, p := range s.Threads.Proposed {
		what, n := proposedWords(p.Kinds)
		them := "it"
		if n > 1 {
			them = "them"
		}
		add(Headline{Court: CourtYou, Kind: KindProposed, Label: "↩ to confirm", By: p.ID,
			Text:   fmt.Sprintf("%s proposes %s: confirm or decline %s", p.ID, what, them),
			Action: Action{Verb: VerbTalk, Label: "read " + p.ID, Target: p.ID}})
	}
	if !onIt {
		for _, r := range s.Threads.Approve {
			add(Headline{Court: CourtYou, Kind: KindApprove, Label: "✓ approve", By: r.Name,
				Text:   r.Name + " has no open threads: approve it?",
				Action: Action{Verb: VerbReviewDoc, Label: "approve " + r.Name, Target: r.Doc}})
		}
	}
	if d := m.Dispatch; d != nil {
		text := "Next crew is queued"
		if d.Capsule != "" {
			text += " in " + d.Capsule
		}
		add(Headline{Court: CourtYou, Kind: KindQueued, Label: "→ next crew", By: firstNonEmpty(d.By, "orchestrator"), Since: d.At,
			Text:   text + ": " + d.Text,
			Action: Action{Verb: VerbLaunchNext, Label: "launch the queued crew"}})
	}
	for _, c := range slices.Backward(m.Crew) {

		if c.ResumeWith == "" || c.Done(m) {
			continue
		}
		st, _ := s.Crew(c)
		if st == CrewEnded && c.Conversation != "" {
			add(Headline{Court: CourtYou, Kind: KindResume, Label: "↻ resume", By: c.ID,
				Text:   "Resume " + c.ID + " with: " + c.ResumeWith,
				Action: Action{Verb: VerbResume, Label: "resume " + c.ID, Target: c.ID}})
		} else if st == CrewHandedBack || st == CrewWaiting {
			add(Headline{Court: CourtYou, Kind: KindResume, Label: "↻ resume", By: c.ID,
				Text:   c.ID + " should carry on: " + c.ResumeWith,
				Action: Action{Verb: VerbReply, Label: "tell " + c.ID, Target: c.ID}})
		}
		break
	}
	if c := m.Conclude; c != nil && c.Ending != "" {
		add(concludeHeadline(*c))
	} else if c != nil && s.State != Landed {
		// Suggested before missions had endings: it only looks finished.
		add(Headline{Court: CourtYou, Kind: KindWrapUp, Label: "✓ wrap up", By: firstNonEmpty(c.By, "orchestrator"), Since: c.At,
			Text:   "Looks finished: " + c.Text,
			Action: Action{Verb: VerbArchive, Label: "archive it"}})
	}

	// ws, checking a step its crew reported done: until it's checked,
	// it's neither the orchestrator's nor yours.
	checking := false
	if st, id, ok := m.UncheckedDone(s.Brief); ok {
		checking = true
		add(checkingHeadline(st, id, m.Checking, s.now()))
	}

	// The PR writer, writing a pull request ws opens once it's written.
	for _, p := range m.WritingPRs {
		text := "The PR writer is writing " + p.Capsule + "'s pull request: ws opens it once it's written"
		if p.Step > 0 {
			text += fmt.Sprintf(", for step %d", p.Step)
		}
		add(Headline{Court: CourtAgent, Kind: KindWritingPR, Label: "… writing", By: "the PR writer", Since: p.Since, Text: text,
			Action: Action{Label: "nothing to do"}})
	}

	// ws moving the plan on from a step it's passing, until what the next
	// begins with says so itself (Mission.Moving).
	if mv := m.Moving; mv != nil {
		add(Headline{Court: CourtAgent, Kind: KindMovingOn, Label: "… moving on", By: "ws", Since: mv.Since,
			Text:   fmt.Sprintf("ws is moving the plan on from step %d", mv.Step),
			Action: Action{Label: "nothing to do"}})
	}

	// The orchestrator.
	// Its run is marked underway as the event it's for is taken (ops.Notify),
	// so there's no gap to guess across.
	if r := m.Orchestrating; s.orchestrating() {
		add(Headline{Court: CourtAgent, Kind: KindDeciding, Label: "◆ deciding", By: "orchestrator", Since: r.Since,
			Text:   "The orchestrator is looking at " + firstNonEmpty(r.For, "the mission"),
			Action: Action{Verb: VerbLog, Label: "read the log"}})
	} else if s.OrchestratorOff && !checking {
		if h, ok := s.acceptHeadline(); ok {
			add(h)
		}
	}

	// The plan's step waits on CI or someone outside ws.
	stepWaits := false
	if st := s.waitingStep(); st != nil && !onIt && s.State != Parked && !s.orchestrating() {
		stepWaits = true
		add(s.stepWaitHeadline(*st))
	}

	// Placed outside by the orchestrator: waiting on something ws can't see.
	// Not at a step that waits: the step says what it waits on, and how
	// it's passed, and "nothing for you to do" said of the same wait would
	// stand in front of marking it done.
	if w := m.Waiting; w != nil && !stepWaits {
		add(Headline{Court: CourtOutside, Kind: KindWaitingOn, Label: "⧗ waiting", By: firstNonEmpty(w.By, "orchestrator"), Since: w.At,
			Text:   "Waiting on " + w.Text + " · nothing for you to do",
			Action: Action{Label: "nothing to do"}})
	}

	// PRs: crew at work in a capsule have its PR in hand. One that's
	// merged, at its Open Pull Request step, which ws hasn't passed yet,
	// is ws's.
	for _, pr := range s.PRs {
		if pr.Merged && s.State != Parked {
			if h, ok := s.mergedHeadline(pr); ok {
				add(h)
			}
		}
		if pr.Merged || busyIn[pr.Capsule] || s.State == Parked {
			continue
		}
		add(prHeadline(pr))
	}

	// Capsules with no pull request to wait on: theirs to land is yours,
	// once crew are out of them. One back for re-review says so already.
	for _, d := range s.Direct {
		again := slices.ContainsFunc(s.ReReview, func(rr ReReview) bool { return rr.Capsule == d.Capsule })
		if d.Ahead == 0 || busyIn[d.Capsule] || again || s.State == Parked {
			continue
		}
		add(landHeadline(d))
	}

	if len(out) == 0 {
		out = append(out, s.restingHeadline())
	}
	slices.SortStableFunc(out, func(a, b Headline) int { return a.Kind.Rank() - b.Kind.Rank() })
	return oneBanner(out)
}

// oneBanner keeps one headline of the agents' court, the first: several
// agents at once (crew working, the planner answering, the orchestrator
// looking) are one thing to you, that it's in hand, and the most pressing
// says who has it. What's yours and what's outside all stay, in order.
func oneBanner(hs []Headline) []Headline {
	shown := false
	return slices.DeleteFunc(hs, func(h Headline) bool {
		if h.Court != CourtAgent {
			return false
		}
		drop := shown
		shown = true
		return drop
	})
}

// workingHeadline is crew at work: one, or several.
func workingHeadline(s Snapshot, working []Crew, since time.Time) Headline {
	c := working[0]
	h := Headline{Court: CourtAgent, Kind: KindWorking, Label: "● working", By: c.ID, Since: since,
		Action: Action{Verb: VerbWatch, Label: "watch " + c.ID, Target: c.ID}}
	if s.State == Briefing || c.Role == RoleBriefing {
		h.Kind, h.Label = KindBriefing, "● briefing"
		h.Text = c.ID + " is writing the brief and lifting capsules"
		return h
	}
	if len(working) > 1 {
		var ids []string
		for _, w := range working {
			ids = append(ids, w.ID)
		}
		h.Text = strings.Join(ids[:len(ids)-1], ", ") + " and " + ids[len(ids)-1] + " are working"
		return h
	}
	switch {
	case c.Role == RoleAnswer:
		h.Text = c.ID + " is finding out the answer to the question for you"
	case c.Dispatch != "":
		h.Text = c.ID + " is working: " + firstLine(c.Dispatch, 120)
	default:
		h.Text = c.ID + " is working"
		if c.Capsule != "" {
			h.Text += " in " + c.Capsule
		}
	}
	return h
}

// startingHeadline is crew ws is starting: recorded, their session coming
// up, and nothing of their own done yet.
func startingHeadline(starting []Crew) Headline {
	c := starting[0]
	h := Headline{Court: CourtAgent, Kind: KindLaunching, Label: "… starting", By: c.ID, Since: *c.Starting,
		Action: Action{Verb: VerbWatch, Label: "watch " + c.ID, Target: c.ID}}
	if len(starting) > 1 {
		var ids []string
		for _, s := range starting {
			ids = append(ids, s.ID)
		}
		h.Text = "Starting " + strings.Join(ids[:len(ids)-1], ", ") + " and " + ids[len(ids)-1]
		return h
	}
	h.Text = "Starting " + c.ID
	switch {
	case c.Role == RoleBriefing:
		h.Text += ", to brief it"
	case c.Role == RoleAnswer:
		h.Text += ", to find out the answer to the question for you"
	case c.Dispatch != "":
		h.Text += ": " + firstLine(c.Dispatch, 120)
	case c.Capsule != "":
		h.Text += " in " + c.Capsule
	}
	return h
}

// ownWorkHeadline is crew whose turn is held on the work they left
// running (Crew.OwnWork): the agent's court, with what they wait on.
func ownWorkHeadline(c Crew, work []Work) Headline {
	h := Headline{Court: CourtAgent, Kind: KindOwnWork, Label: "⧗ background work", By: c.ID, Since: c.Pending.At,
		Text:   c.ID + " is waiting on its own work: " + WorkWhat(work),
		Action: Action{Verb: VerbWatch, Label: "watch " + c.ID, Target: c.ID}}
	if len(c.Pending.Running()) == 0 {
		h.Text = c.ID + "'s work has ended, and its harness is to wake it: " + WorkWhat(work)
	}
	return h
}

// orchestrating is whether the orchestrator's run is underway.
func (s Snapshot) orchestrating() bool {
	r := s.M.Orchestrating
	return r != nil && s.now().Before(r.Until)
}

// acceptHeadline asks you to accept the current stage, with the
// orchestrator off: its crew reported done and none are still at it, and
// no change you asked for on its documents is open.
func (s Snapshot) acceptHeadline() (Headline, bool) {
	states, _ := s.M.StageProgress(s.Brief)
	cur, ok := CurrentStage(states)
	if !ok || cur.Record == nil || len(cur.Record.Crew) == 0 || len(s.Threads.Blocking) > 0 {
		return Headline{}, false
	}
	if cur.Step != nil && cur.Step.Type == StepVerify {
		// Its verifier's done is its bundle, which ws settles the step on:
		// passed, or the work sent back. It isn't yours to accept.
		return Headline{}, false
	}
	var reported *Crew
	for _, id := range cur.Record.Crew {
		for i, c := range s.M.Crew {
			if c.ID != id || c.Role != "" {
				continue
			}
			st, _ := s.Crew(c)
			if st != CrewEnded && st != CrewHandedBack {
				return Headline{}, false // still at it
			}
			if c.Status == CrewDone {
				reported = &s.M.Crew[i]
			}
		}
	}
	if reported == nil {
		return Headline{}, false
	}
	stage := "its part"
	label := "accept the work"
	if !cur.Implicit {
		stage = fmt.Sprintf("%s %d", cur.Noun(), cur.N)
		label = fmt.Sprintf("accept %s %d", cur.Noun(), cur.N)
	}
	return Headline{Court: CourtYou, Kind: KindAccept, Label: "✓ accept", By: reported.ID, Since: timeOr(reported.StatusAt, reported.Started),
		Text:   fmt.Sprintf("%s reports %s done: %s. Accept it?", reported.ID, stage, firstLine(reported.Note, 120)),
		Action: Action{Verb: VerbAccept, Label: label}}, true
}

// concludeHeadline is an ending suggested, for you to confirm: how it
// ends, and Z as checked.
func concludeHeadline(c Conclusion) Headline {
	text := "Ready to end, " + c.Ending.Words()
	if z := c.ZWords(); z != "" {
		text += " (" + z + ")"
	}
	return Headline{Court: CourtYou, Kind: KindWrapUp, Label: "✓ wrap up", By: firstNonEmpty(c.By, "orchestrator"), Since: c.At,
		Text:   text + c.came(),
		Action: Action{Verb: VerbEnd, Label: "end it, " + c.Ending.Words()}}
}

// endedHeadline is a mission that ended, as you said.
func endedHeadline(c Conclusion) Headline {
	text := "Ended, " + c.Ending.Words()
	if z := c.ZWords(); z != "" {
		text += " (" + z + ")"
	}
	if len(c.To) > 0 {
		text += " to " + strings.Join(c.To, ", ")
	}
	return Headline{Court: CourtYou, Kind: KindEnded, Label: c.Ending.Words(), By: firstNonEmpty(c.By, "you"), Since: c.At,
		Text:   text + c.came(),
		Action: Action{Verb: VerbArchive, Label: "archive it"}}
}

// waitingStep is the plan's current step when it waits on CI or someone
// outside ws.
func (s Snapshot) waitingStep() *StageState {
	if s.M.Plan == nil {
		return nil
	}
	states, _ := s.M.StageProgress(s.Brief)
	cur, ok := CurrentStage(states)
	if !ok || cur.Step == nil || (cur.Step.Who != WhoCI && cur.Step.Who != WhoOutside) {
		return nil
	}
	return &cur
}

// stepWaitHeadline is the plan at a step that waits on someone else (CI,
// or anyone outside ws), and says how it ends, which is one of three:
//
//   - watched: it has checks, which ws runs every so often and passes the
//     step on. Outside's court; you can have ws look now, or mark it done.
//   - not watched: nobody's looking, and it's passed when someone says
//     it's happened. Outside's court, with marking it done offered. It's
//     yours once it looks to have happened (every pull request merged,
//     every capsule landed), or has waited a while (StaleWait): to say
//     whether it has.
func (s Snapshot) stepWaitHeadline(st StageState) Headline {
	who := waitsOn(*st.Step, "")
	what := sentenceOf(st.Plain())
	by := fmt.Sprintf("step %d", st.N)
	done := Action{Verb: VerbWait, Label: fmt.Sprintf("mark step %d done", st.N)}
	var since time.Time
	watch := st.Watch()
	if watch != nil {
		since = watch.Since
	}
	if st.Step.Watched() {
		looked := "it hasn't looked yet"
		if watch != nil && watch.Looked != nil {
			looked = fmt.Sprintf("it last looked %s ago: %s", Elapsed(s.now().Sub(*watch.Looked)), watch.Summary())
		}
		return Headline{Court: CourtOutside, Kind: KindWaitingOn, Label: "⧗ watching", By: by, Since: since,
			Text:   fmt.Sprintf("Step %d %s: %s ws is watching for it (%s), and passes the step once it's happened; %s", st.N, who, what, checkCommands(st.Step.Checks), looked),
			Action: done}
	}
	if s.CapsulesLanded {
		return Headline{Court: CourtYou, Kind: KindAccept, Label: "✓ accept", By: by, Since: since,
			Text:   fmt.Sprintf("Step %d %s: %s Every pull request has merged, and every capsule landed: mark it done, if that's what it waits for", st.N, who, what),
			Action: done}
	}
	if !since.IsZero() && s.now().Sub(since) >= StaleWait {
		return Headline{Court: CourtYou, Kind: KindAccept, Label: "? happened", By: by, Since: since,
			Text:   fmt.Sprintf("Step %d has waited %s%s: %s Nobody's watching for it: has it happened? Mark it done if it has", st.N, waitedFor(s.now().Sub(since)), strings.TrimPrefix(who, "waits"), what),
			Action: done}
	}
	return Headline{Court: CourtOutside, Kind: KindWaitingOn, Label: "⧗ waiting", By: by, Since: since,
		Text:   fmt.Sprintf("Step %d %s: %s Nobody's watching for it: mark it done when it's happened", st.N, who, what),
		Action: done}
}

// checkCommands is a step's checks that are commands, as a sentence names
// them: `gh release view v1.4.0`.
func checkCommands(checks []Check) string {
	var out []string
	for _, c := range checks {
		if c.Run() {
			out = append(out, "`"+c.Command+"`")
		}
	}
	return strings.Join(out, ", ")
}

// waitedFor is how long a step has waited, in whole days or hours: "2
// days", "a day", "5 hours".
func waitedFor(d time.Duration) string {
	switch days, hours := int(d/(24*time.Hour)), int(d/time.Hour); {
	case days > 1:
		return fmt.Sprintf("%d days", days)
	case days == 1:
		return "a day"
	case hours == 1:
		return "an hour"
	}
	return fmt.Sprintf("%d hours", int(d/time.Hour))
}

// mergedHeadline is a pull request GitHub says has merged, which ws last
// saw open, while the plan's at an Open Pull Request step: ws takes the
// merge in when it next looks (ops.WatchPRs, which a page's news of it
// starts), passing the step once each of its pull requests has merged.
// Until then it's ws's, not a mission nobody's working on.
func (s Snapshot) mergedHeadline(pr PRFacts) (Headline, bool) {
	st, ok := s.currentStep()
	if !ok || st.Step.Type != StepOpenPR {
		return Headline{}, false
	}
	seen := slices.ContainsFunc(s.M.Capsules, func(c Capsule) bool {
		return c.Repo+"/"+c.Name == pr.Capsule && c.SeenPR != nil && !c.SeenPR.Merged && !c.SeenPR.Closed &&
			fmt.Sprintf("%s#%d", c.Repo, c.SeenPR.Number) == pr.Ref
	})
	if !seen {
		return Headline{}, false
	}
	text := fmt.Sprintf("%s has merged: ws is passing step %d and moving the plan on", pr.Ref, st.N)
	if len(st.Instances()) > 1 {
		text = fmt.Sprintf("%s has merged: ws is taking it in at step %d, which passes once each of its pull requests has merged", pr.Ref, st.N)
	}
	return Headline{Court: CourtAgent, Kind: KindMovingOn, Label: "… moving on", By: "ws", Text: text,
		Action: Action{Label: "nothing to do"}}, true
}

// waitsOn is what a step that's CI's or outside ws's waits on, after
// verb: "waits on CI", or a Wait step, whose words say what for, "waits".
func waitsOn(st Step, verb string) string {
	if verb == "" {
		verb = "waits"
	}
	switch {
	case st.Who == WhoOutside && st.Whom == "" && st.Type == StepWait:
		return verb
	case st.Who == WhoOutside && st.Whom != "" && st.Type == StepWait:
		return verb + " on " + st.Whom
	}
	return verb + " on " + st.WhoSays()
}

// sentenceOf is s with a full stop, if it ends without one.
func sentenceOf(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || strings.ContainsAny(s[len(s)-1:], ".?!") {
		return s
	}
	return s + "."
}

// prHeadline is where an open PR stands.
func prHeadline(pr PRFacts) Headline {
	h := Headline{By: pr.Ref, Action: Action{Verb: VerbOpen, Label: "open " + pr.Ref, Target: pr.Ref, URL: pr.URL}}
	switch {
	case pr.Unasked:
		// ws's, for the moment it takes: neither ready to merge (which
		// no review and no checks would otherwise read as) nor a mission
		// nobody's on.
		h.Court, h.Kind, h.Label, h.By = CourtAgent, KindMovingOn, "… moving on", "ws"
		h.Text = pr.Ref + " is open: ws is asking GitHub how its checks and review stand"
	case pr.Checks == "failure":
		h.Court, h.Kind, h.Label = CourtYou, KindChecksFailing, "✗ checks"
		h.Text = "Checks are failing on " + pr.Ref
	case pr.Review == "CHANGES_REQUESTED":
		h.Court, h.Kind, h.Label = CourtYou, KindChangesRequested, "✗ changes"
		h.Text = pr.Ref + ": a reviewer asked for changes"
	case pr.Conflict:
		// Whatever its checks and review say, it can't be merged as it is.
		h.Court, h.Kind, h.Label = CourtYou, KindConflict, "✗ conflict"
		h.Text = pr.Ref + " conflicts with its base"
	case pr.Checks == "pending":
		h.Court, h.Kind, h.Label = CourtOutside, KindChecksRunning, "◌ checks"
		h.Text = pr.Ref + "'s checks are running · nothing for you to do"
	case pr.Review == "REVIEW_REQUIRED":
		h.Court, h.Kind, h.Label = CourtOutside, KindAwaitingReview, "in review"
		h.Text = pr.Ref + " is waiting for review · nothing for you to do"
	case pr.MergeUnknown:
		// Nothing else holds it up, but it may turn out to conflict: it's
		// not yours to merge until GitHub has said.
		h.Court, h.Kind, h.Label = CourtOutside, KindWaitingOn, "⧗ waiting"
		h.Text = pr.Ref + ": GitHub is working out whether it merges cleanly · nothing for you to do"
	default:
		h.Court, h.Kind, h.Label = CourtYou, KindMerge, "✓ merge"
		var why string
		switch {
		case pr.Review == "APPROVED" && pr.Checks == "success":
			why = "it's approved and its checks pass"
		case pr.Review == "APPROVED":
			why = "it's approved, and has no checks"
		case pr.Checks == "success":
			why = "its checks pass, and it needs no review"
		default:
			why = "it needs no review, and has no checks"
		}
		h.Text = pr.Ref + " is ready to merge: " + why
		h.Action = Action{Verb: VerbMerge, Label: "merge " + pr.Ref, Target: pr.Ref, URL: pr.URL}
	}
	return h
}

// landHeadline is a capsule with commits to land: yours to review, and
// then to land.
func landHeadline(d DirectFacts) Headline {
	commits := fmt.Sprintf("%d commits", d.Ahead)
	if d.Ahead == 1 {
		commits = "1 commit"
	}
	if d.Branch == "" {
		d.Branch = "the default branch"
	}
	h := Headline{Court: CourtYou, Kind: KindLand, Label: "✓ land", By: d.Capsule,
		Action: Action{Verb: VerbLand, Label: "review and land " + d.Capsule, Target: d.Capsule}}
	switch {
	case d.Dirty:
		h.Text = fmt.Sprintf("%s has %s to land on %s, and changes not committed yet: there's no pull request, so it lands when you say", d.Capsule, commits, d.Branch)
	case d.Approved:
		h.Text = fmt.Sprintf("%s is approved: land its %s on %s", d.Capsule, commits, d.Branch)
		h.Action.Label = "land " + d.Capsule
	default:
		h.Text = fmt.Sprintf("%s has %s to land on %s: there's no pull request, so review them, and it lands when you say", d.Capsule, commits, d.Branch)
	}
	return h
}

// restingHeadline is where a mission stands when nobody is on it and
// nothing is raised: it's yours, for one of a few reasons.
func (s Snapshot) restingHeadline() Headline {
	m := s.M
	switch {
	case s.State == Landed:
		text := "Every PR has merged"
		if landed := slices.ContainsFunc(m.Capsules, func(c Capsule) bool { return c.Landed != nil }); landed {
			text = "Everything has landed"
		}
		return Headline{Court: CourtYou, Kind: KindLanded, Label: "landed", Text: text + ": archive it when you're ready",
			Action: Action{Verb: VerbArchive, Label: "archive it"}}
	case s.State == Parked:
		return Headline{Court: CourtYou, Kind: KindParked, Label: "parked", Text: "Parked: nothing moves until you unpark it",
			Action: Action{Verb: VerbUnpark, Label: "unpark it"}}
	case s.State == Briefing && m.Plan != nil && !IsUnwrittenBrief(s.Brief, m.Title):
		// Briefed and planned, and not confirmed: briefing it again would
		// start over what's written.
		return Headline{Court: CourtYou, Kind: KindToBrief, Label: "to confirm",
			Text:   "Its brief and plan are written: confirm them to start it",
			Action: Action{Verb: VerbConfirm, Label: "confirm and start"}}
	case s.State == Briefing:
		text := "Waiting to be briefed"
		if r := firstLine(m.Request, 120); r != "" {
			text += ": " + r
		}
		return Headline{Court: CourtYou, Kind: KindToBrief, Label: "to brief", Text: text,
			Action: Action{Verb: VerbBrief, Label: "brief it"}}
	case s.yourStep() != nil:
		// The plan's at a step of yours, and nothing's asking you of it
		// (ws's question was answered, or another took its place).
		st := s.yourStep()
		if st.Step.Type == StepReview {
			return stepReviewHeadline(s, st.N, fmt.Sprintf("Step %d is yours, to review: %s", st.N, strings.TrimSuffix(st.Plain(), ".")), "", "ws", time.Time{})
		}
		text := fmt.Sprintf("Step %d is yours: %s Go on when you're satisfied", st.N, strings.TrimSuffix(st.Plain(), "."))
		action := Action{Verb: VerbAccept, Label: fmt.Sprintf("go on: pass step %d", st.N)}
		if st.Step.WaitOnYou() {
			// Something you do yourself, and nothing's asking you of it.
			return Headline{Court: CourtYou, Kind: KindToDo, Label: "→ to do",
				Text:   fmt.Sprintf("Step %d is yours, to do: %s Mark it done when you have", st.N, sentenceOf(st.Plain())),
				Action: Action{Verb: VerbAccept, Label: fmt.Sprintf("mark step %d done", st.N)}}
		}
		if st.Step.Who == WhoPlanner {
			// The planner had its turn and didn't write the steps after it
			// (it asked something, or couldn't): it's yours to say more, or
			// write them yourself and go on.
			text = fmt.Sprintf("Step %d is the planner's: %s Write the steps after it, then go on", st.N, strings.TrimSuffix(st.Plain(), "."))
			if b := s.Threads.Briefing; b != nil {
				text = fmt.Sprintf("Step %d is the planner's: %s It hasn't written the steps after it: answer it in %s, or write them yourself and go on", st.N, sentenceOf(st.Plain()), b.ID)
				action = Action{Verb: VerbTalk, Label: "open " + b.ID, Target: b.ID}
			}
		}
		return Headline{Court: CourtYou, Kind: KindAccept, Label: "✓ yours", Text: text, Action: action}
	case s.atZ():
		// Every step of the plan is passed, and nothing's suggested how it
		// ends: ending it is a press, landed where its work has merged.
		if s.CapsulesLanded {
			return Headline{Court: CourtYou, Kind: KindWrapUp, Label: "✓ wrap up",
				Text:   "Every step of the plan is done, and its work has merged",
				Action: Action{Verb: VerbEnd, Label: "end it, landed"}}
		}
		return Headline{Court: CourtYou, Kind: KindWrapUp, Label: "✓ wrap up",
			Text:   "Every step of the plan is done",
			Action: Action{Verb: VerbEnd, Label: "end it"}}
	case s.State == Review:
		// Every capsule has a PR, but GitHub wasn't asked how they stand.
		return Headline{Court: CourtOutside, Kind: KindAwaitingReview, Label: "in review", Text: "Its PRs are up, waiting for review and merge · nothing for you to do",
			Action: Action{Label: "nothing to do"}}
	case s.checksUnmet() != nil:
		return *s.checksUnmet()
	case !hadWorkCrew(m):
		text := "Ready: nobody has started it"
		if st, ok := s.currentStep(); ok && st.Held() {
			text = fmt.Sprintf("Ready: step %d waits for your call, as it %s", st.N, st.HeldWhy())
		}
		return Headline{Court: CourtYou, Kind: KindReady, Label: "ready", Text: text,
			Action: Action{Verb: VerbLaunch, Label: "launch crew"}}
	}
	text := "Nobody's working on it"
	if st, ok := s.currentStep(); ok && st.Step.Who == WhoCrew {
		text = fmt.Sprintf("Nobody's on step %d, crew's: %s", st.N, st.Short())
	}
	if said := s.orchestratorSaid(); said != "" {
		text += "; the orchestrator said: " + said
	} else if states, _ := m.StageProgress(s.Brief); len(states) > 0 {
		if _, ok := CurrentStage(states); !ok {
			text = "Every stage is accepted, and nobody's working on it"
			if m.Plan != nil {
				text = "Every step of the plan is accepted, and nobody's working on it"
			}
		}
	}
	h := Headline{Court: CourtYou, Kind: KindStalled, Label: "idle", Text: text,
		Action: Action{Verb: VerbLaunch, Label: "launch crew"}}
	running := func(id string) bool {
		return slices.ContainsFunc(m.Crew, func(c Crew) bool { return c.ID == id && s.running(c) })
	}
	if c, ok := m.LastResumableCrew(running); ok {
		// Launching offers to resume it first.
		h.Action = Action{Verb: VerbLaunch, Label: "resume " + c.ID + " or launch crew", Target: c.ID}
	}
	return h
}

// Position is where the mission is on its route, in words: the step of its
// plan (or the stage of its brief) it's at, and who's on it; at the end of
// its plan; or how it ended. Whose turn it is stays the headline's. "" for
// a mission with no route to speak of (a brief without stages), or not
// under way (briefing, archived).
func (s Snapshot) Position() string {
	m := s.M
	if e := m.Ending; e != nil && s.State.Ended() {
		w := "Ended, " + e.Ending.Words()
		if z := e.ZWords(); z != "" {
			w += " (" + z + ")"
		}
		return w
	}
	if s.State == Briefing || s.State == Archived {
		return ""
	}
	states, _ := m.StageProgress(s.Brief)
	if len(states) == 0 || (len(states) == 1 && states[0].Implicit) {
		return ""
	}
	cur, ok := CurrentStage(states)
	if !ok {
		switch {
		case m.Plan == nil:
			return "Every stage accepted"
		case m.Conclude != nil && m.Conclude.Ending != "":
			return "Every step done: ending it, " + m.Conclude.Ending.Words() + ", is suggested"
		}
		return "Every step done: at Z, if what Z says is true"
	}
	head := fmt.Sprintf("Stage %d of %d", cur.N, len(states))
	if cur.Step != nil {
		head = fmt.Sprintf("Step %d of %d", cur.N, len(states))
	}
	switch cur.Who() {
	case WhoYou:
		head += ", yours"
	case WhoPlanner:
		head += ", the planner's"
	case WhoWS:
		head += ", ws's"
	case WhoCI, WhoOutside:
		head += ", " + waitsOn(*cur.Step, "waiting")
	}
	short := cur.Short()
	if strings.HasPrefix(cur.Plain(), short+"?") {
		short += "?" // a question stays one
	}
	text := head + ": " + short
	if cur.Who() == WhoCrew && cur.Record != nil {
		var on []string
		for _, id := range cur.Record.Crew {
			if c, ok := m.CrewByID(id); ok && c.Role == "" && s.running(c) {
				on = append(on, id)
			}
		}
		if len(on) > 0 {
			text += " (" + strings.Join(on, ", ") + ")"
		}
	}
	return text
}

// currentStep is the plan's current step, if the mission has a plan and
// a step of it isn't passed.
func (s Snapshot) currentStep() (StageState, bool) {
	if s.M.Plan == nil {
		return StageState{}, false
	}
	states, _ := s.M.StageProgress(s.Brief)
	cur, ok := CurrentStage(states)
	if !ok || cur.Step == nil {
		return StageState{}, false
	}
	return cur, true
}

// atZ reports whether every step of the mission's plan is passed, and
// nothing has said how it ends.
func (s Snapshot) atZ() bool {
	if s.M.Plan == nil || len(s.M.Plan.Steps) == 0 || s.M.Ending != nil || s.M.Conclude != nil {
		return false
	}
	states, _ := s.M.StageProgress(s.Brief)
	_, ok := CurrentStage(states)
	return !ok
}

// yourStep is the plan's current step when it's yours to pass: a
// checkpoint, or the planner's step once the planner's had its turn at it
// and nothing else is happening (it's at work, or proposing a revision,
// which say so).
func (s Snapshot) yourStep() *StageState {
	if s.M.Plan == nil {
		return nil
	}
	states, _ := s.M.StageProgress(s.Brief)
	cur, ok := CurrentStage(states)
	if !ok || cur.Step == nil || (cur.Step.Who != WhoYou && cur.Step.Who != WhoPlanner) {
		return nil
	}
	if _, asked := s.M.StepTask(cur.N); asked {
		return nil // its task says so
	}
	return &cur
}

// checksUnmet is the headline of a mission resting at a Work step whose
// checks came round not all met, with nobody on it and nothing asked of
// it (the orchestrator noted and left it, or its question was dismissed):
// it's yours, to pass as it is. ws passes such a step only once every
// check is met, and nobody else can, so resting here without saying so
// would leave the mission with no way on.
func (s Snapshot) checksUnmet() *Headline {
	st, round, ok := s.M.ChecksUnmet(s.Brief)
	if !ok {
		return nil
	}
	return &Headline{Court: CourtYou, Kind: KindAccept, Label: "✓ yours", By: "ws", Since: round.At,
		Text: fmt.Sprintf("Step %d's checks, round %d: %s. Nobody's on it: pass it as it is, or resume %s with what's left to put right",
			st.N, round.N, round.Summary(), round.Crew),
		Action: Action{Verb: VerbAccept, Label: fmt.Sprintf("pass step %d as it is", st.N)}}
}

// orchestratorSaid is what the orchestrator last said, if its run is the
// latest thing that happened to crew: its note (what it found and decided)
// if it left one, else the gist of its report.
func (s Snapshot) orchestratorSaid() string {
	report, run := "", ""
	for _, e := range slices.Backward(s.Events) {

		switch {
		case e.Kind == EventOrchestrateRun:
			return report // the run's start: no note in it
		case e.Kind == EventOrchestrate && run == "":
			report, run = firstLine(e.Text, 160), e.Run
		case e.Kind == EventNote && e.Run != "" && e.Run == run:
			// Its note, if it left one: asked to start it "Orchestrator: ".
			return firstLine(strings.TrimPrefix(e.Text, "Orchestrator: "), 160)
		case e.Kind == EventCrewStarted, e.Kind == EventCrewEnded, e.Kind == EventCrewDone, e.Kind == EventCrewAsked:
			return report
		}
	}
	return report
}

// hadWorkCrew reports whether any crew (not just a briefing one) has worked
// on the mission.
func hadWorkCrew(m *Mission) bool {
	return slices.ContainsFunc(m.Crew, func(c Crew) bool { return c.Role != RoleBriefing })
}

// stepReviewHeadline is Review step n, yours: what the steps before it
// made, to review. Its action is the review of what it looks at (Look),
// for its task (task; "" when it has none open), and going on: your
// review approving what the plan put under review, or going on, passes
// the step; asking for changes sends it back to the crew who made it, and
// it's yours again after. It's labelled by what's to approve, or with
// none, what's to read.
func stepReviewHeadline(s Snapshot, n int, text, task, by string, since time.Time) Headline {
	states, _ := s.M.StageProgress(s.Brief)
	looks := s.M.Looks(states, n)
	named := looks
	if i := slices.IndexFunc(looks, func(l Look) bool { return l.Read }); i > 0 {
		named = looks[:i]
	}
	// A bundle you've approved already isn't what's to approve.
	named = slices.DeleteFunc(slices.Clone(named), func(l Look) bool { return l.Approved })
	label := fmt.Sprintf("go on: pass step %d", n)
	switch {
	case len(named) > 0 && named[0].Bundle != "":
		label = "verify " + named[0].Name
	case len(named) == 1 && named[0].Capsule != "":
		label = "review " + named[0].Name
	case len(named) == 1:
		label = "read " + named[0].Name
	case len(named) > 1:
		label = fmt.Sprintf("review step %d's work", n)
	}
	return Headline{Court: CourtYou, Kind: KindStepReview, Label: "✓ review", By: by, Since: since, Text: text,
		Action: Action{Verb: VerbReviewStep, Label: label, Target: task, Look: looks}}
}

// lookNames lists what's looked at, by name: b1, or b1 and b2.
func lookNames(looks []Look) string {
	names := make([]string, len(looks))
	for i, l := range looks {
		names[i] = l.Name
	}
	if len(names) < 2 {
		return strings.Join(names, "")
	}
	return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
}

func flagWho(by string) string {
	switch by {
	case "orchestrator", "":
		return "The orchestrator asks"
	case "you":
		return "You flagged"
	}
	return by + " asks"
}

func firstNonEmpty(s, or string) string {
	if strings.TrimSpace(s) == "" {
		return or
	}
	return s
}

// firstLine is the first line of s with something to say: without
// markdown's marks, skipping a lone "…" (where a long report was cut), cut
// to max runes.
func firstLine(s string, max int) string {
	for l := range strings.SplitSeq(s, "\n") {
		l = strings.NewReplacer("**", "", "__", "").Replace(l)
		if l = strings.TrimSpace(strings.TrimLeft(l, "#*->⏺ ")); l != "" && l != "…" && l != "..." {
			if r := []rune(l); len(r) > max {
				return string(r[:max-1]) + "…"
			}
			return l
		}
	}
	return ""
}
