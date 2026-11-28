package web

import (
	"fmt"
	"strings"

	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/ops"
)

// Acting is what a mission's page does to it: what its headline asks of
// you (POST /api/missions/{slug}/act).
type Acting interface {
	AnswerTask(slug string, n int, by ops.Actor, text string) (ops.Answered, error)
	RecordDecision(slug string, d ops.Deciding, by ops.Actor) (ops.Decided, error)
	WithdrawDecision(slug, id, why string, by ops.Actor) (ops.DecisionWithdrawn, error)
	Dismiss(slug string, by ops.Actor) (ops.Dismissed, error)
	AcceptStage(slug string, by ops.Actor, why string) (ops.Accepted, error)
	CheckWait(slug string, by ops.Actor) (string, error)
	SetState(slug string, state mission.State, by ops.Actor) (ops.StateSet, error)
	Archive(slug string, burnAll bool, by ops.Actor) (ops.StateSet, error)
	Reopen(slug string, from mission.State, by ops.Actor) (ops.StateSet, error)
	RetryPutAway(slug string, id int64, by ops.Actor) (string, error)
	DropPutAway(slug string, id int64, by ops.Actor) (string, error)
	SetBrief(slug, text string, by ops.Actor) (ops.BriefSet, error)
	SetPlan(slug, text string, by ops.Actor) (ops.PlanSet, error)
	SetStepLevel(slug string, n int, level string, by ops.Actor) (ops.LevelSet, error)
	AskPlanner(slug, why string, by ops.Actor) (ops.AskedPlanner, error)
	SetTitle(slug, title string, by ops.Actor) error
	Launch(slug string, d ops.Dispatch, by ops.Actor) (ops.Launched, error)
	LaunchPicked(slug string, p harness.Pick, by ops.Actor) (ops.Launched, error)
	LaunchQueued(slug string, by ops.Actor) (ops.Launched, error)
	Brief(slug string, by ops.Actor) (string, error)
	ConfirmBrief(slug string, by ops.Actor) (string, error)
	StopCrew(slug, id string, by ops.Actor) (string, error)
	End(slug string, as *ops.EndAs, by ops.Actor) (ops.Concluded, error)
	Unend(slug string, by ops.Actor) (ops.StateSet, error)
	HandOnCapsule(slug, capsule, what, to string, by ops.Actor) (ops.CapsuleEnded, error)
	DropCapsule(slug, capsule, why string, by ops.Actor) (ops.CapsuleEnded, error)
	OpenPR(slug, capsule string, o ops.NewPullRequest, by ops.Actor) (ops.PROpened, error)
	WritePR(slug, capsule string, by ops.Actor) (ops.NewPullRequest, error)
	ApproveBundle(slug, id string, by ops.Actor) (ops.BundleDone, error)
	SendBackBundle(slug, id string, notes []mission.BundleNote, by ops.Actor) (ops.BundleDone, error)
	MarkBundleLooked(slug, id, key string, looked bool, by ops.Actor) error
	MergeMissionPR(slug, target, when string, by ops.Actor) (ops.Merged, error)
}

// ActOn does what a mission's headline asks of you, from a page, and says
// what it did. Crew start in the workspace's daemon (stop hangs one up
// there); talking to them is their crew page's.
func ActOn(a Acting, slug string, req ActRequest) (string, error) {
	text, you := strings.TrimSpace(req.Text), ops.AsYou
	switch req.Verb {
	case "answer":
		return said(a.AnswerTask(slug, req.Task, you, text))
	case "dismiss":
		return said(a.Dismiss(slug, you))
	case "accept":
		return said(a.AcceptStage(slug, you, text))
	case "look":
		// A watched Wait step: ws looks now, not when it next would.
		return a.CheckWait(slug, you)
	case "archive":
		// Refused while a capsule's work hasn't merged, unless you've
		// agreed to burn it anyway (BurnAll).
		return said(a.Archive(slug, req.BurnAll, you))
	case "put-away-retry":
		// What archiving it left to do, and couldn't: tried again, or
		// let go of, the capsule left as it is.
		return a.RetryPutAway(slug, req.Item, you)
	case "put-away-drop":
		return a.DropPutAway(slug, req.Item, you)
	case "park":
		return said(a.SetState(slug, mission.Parked, you))
	case "unarchive":
		// Undoing an archive from the web UI, a moment after.
		return said(a.Reopen(slug, mission.Archived, you))
	case "unpark":
		return said(a.Reopen(slug, mission.Parked, you))
	case "edit-brief":
		return said(a.SetBrief(slug, req.Text, you))
	case "edit-plan":
		return said(a.SetPlan(slug, req.Text, you))
	case "level":
		// A step ahead at another level, kept as your decision.
		return said(a.SetStepLevel(slug, req.Step, text, you))
	case "replan":
		// Asking the planner for a change: it revises the plan in its
		// conversation, applying it or proposing it to you there.
		return said(a.AskPlanner(slug, req.Text, you))
	case "title":
		if err := a.SetTitle(slug, text, you); err != nil {
			return "", err
		}
		return "Renamed.", nil
	case "launch":
		// On the default, or a harness and model picked from those the
		// mission's page offers: anything else is refused.
		return said(a.LaunchPicked(slug, harness.Pick{Harness: strings.TrimSpace(req.Harness), Model: strings.TrimSpace(req.Model)}, you))
	case "launch-queued":
		return said(a.LaunchQueued(slug, you))
	case "resume":
		return said(a.Launch(slug, ops.Dispatch{Resume: req.Crew}, you))
	case "ask-agent":
		l, err := a.Launch(slug, ops.Dispatch{Role: mission.RoleAnswer}, you)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Asked %s to answer.", l.ID), nil
	case "brief":
		return a.Brief(slug, you)
	case "confirm":
		// Its brief and plan written: confirming them starts it.
		started, err := a.ConfirmBrief(slug, you)
		if err != nil {
			return "", err
		}
		return "Confirmed: " + started + ".", nil
	case "stop":
		return a.StopCrew(slug, req.Crew, you)
	case "end":
		// As its suggested ending, or (with an ending) as you say.
		if req.Ending == "" {
			return said(a.End(slug, nil, you))
		}
		return said(a.End(slug, &ops.EndAs{Ending: req.Ending, Text: req.Text, Met: req.Met, Unmet: req.Unmet, To: req.To}, you))
	case "unend":
		// Taking an ending back, a moment after or later.
		if _, err := a.Unend(slug, you); err != nil {
			return "", err
		}
		return "Reopened.", nil
	case "hand-on":
		to := ""
		if len(req.To) > 0 {
			to = req.To[0]
		}
		return said(a.HandOnCapsule(slug, req.Capsule, text, to, you))
	case "drop":
		return said(a.DropCapsule(slug, req.Capsule, text, you))
	case "open-pr":
		// Through ws, holding the process's rules about pull requests.
		return said(a.OpenPR(slug, req.Capsule, ops.NewPullRequest{Title: req.Title, Body: req.Text, Draft: req.Draft, Quiet: true}, you))
	case "bundle-approve":
		// Yours, as approving a review is: it passes a Review step that
		// looks at the bundle.
		return said(a.ApproveBundle(slug, req.Bundle, you))
	case "bundle-send-back":
		notes := make([]mission.BundleNote, len(req.Notes))
		for i, n := range req.Notes {
			notes[i] = mission.BundleNote{Check: n.Check, Text: n.Text}
		}
		return said(a.SendBackBundle(slug, req.Bundle, notes, you))
	case "merge":
		// One of its pull requests (PR: repo#N), at the commit its page
		// shows; a stack whole, from its top.
		return said(a.MergeMissionPR(slug, req.PR, ops.MergeNow, you))
	case "merge-ready":
		return said(a.MergeMissionPR(slug, req.PR, ops.MergeReady, you))
	case "merge-off":
		return said(a.MergeMissionPR(slug, req.PR, ops.MergeOff, you))
	case "bundle-looked":
		if err := a.MarkBundleLooked(slug, req.Bundle, req.Check, req.Looked, you); err != nil {
			return "", err
		}
		return "Marked.", nil
	}
	return "", fmt.Errorf("unknown mission action %q", req.Verb)
}

// said is what an operation did, said, for the page.
func said[T fmt.Stringer](r T, err error) (string, error) {
	if err != nil {
		return "", err
	}
	return r.String(), nil
}
