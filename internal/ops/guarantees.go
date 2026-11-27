package ops

import (
	"fmt"
	"path/filepath"

	"github.com/withworkspace/workspace/internal/review"
	"github.com/withworkspace/workspace/internal/workspace"
)

// Guarantees are rules ws holds for every mission, where it acts, whatever
// an agent reads: ws.toml's to say ([missions]), not the process's. The
// process's preferences are applied by the planner and the orchestrator; a
// guarantee is checked where ws does the thing, and refuses it.

// Guarantee is one of them: its key in ws.toml, and what ws does, in words.
type Guarantee struct {
	Key  string // missions.review_before_pr
	What string
}

// The guarantees ws can hold.
var (
	GuaranteeReviewBeforePR    = Guarantee{"missions.review_before_pr", "Workspace opens a pull request from a mission's capsule only once the user's latest review approves its current commit"}
	GuaranteeReviewBeforeLand  = Guarantee{"missions.review_before_land", "Workspace lands a mission's capsule in a repo with no pull requests only once the user's latest review approves it as it is"}
	GuaranteeReviewBeforeReady = Guarantee{"missions.review_before_ready", "Workspace marks a mission's draft pull request ready only once the user's latest review of its capsule approves its current commit"}
	GuaranteeRepliesNeedMe     = Guarantee{"missions.replies_need_me", "ws posts a reply drafted to a person on a mission's pull request only once the user has approved it; a reply to a bot needn't wait"}
	GuaranteeEndWhenMerged     = Guarantee{"missions.end_when_merged", "ws ends a mission landed itself once the orchestrator suggests it landed (every part of Z met, every pull request merged) and nothing else on it waits for the user or crew, and so it does one whose plan is done and pull requests merged after a turn of the orchestrator's that suggested no ending; any other ending waits for the user, who isn't asked to check Z"}
)

// guaranteeMaxRounds is how many rounds of a step's checks ws lets come
// round unmet before the step's the user's: always held, at n.
func guaranteeMaxRounds(n int) Guarantee {
	return Guarantee{"missions.max_rounds", fmt.Sprintf("a Work step's checks come round unmet %d %s at most before the step is the user's: after that, ws asks them, whatever the process says", n, plural(n, "time"))}
}

// Guarantees are the guarantees on in this workspace.
func (w *Workspace) Guarantees() []Guarantee {
	out := []Guarantee{guaranteeCrewAllowed(w), guaranteeMaxRounds(w.Config().MaxRounds())}
	if w.Config().ReviewBeforePR() {
		out = append(out, GuaranteeReviewBeforePR)
	}
	if w.Config().ReviewBeforeLand() {
		out = append(out, GuaranteeReviewBeforeLand)
	}
	if w.Config().ReviewBeforeReady() {
		out = append(out, GuaranteeReviewBeforeReady)
	}
	if w.Config().RepliesNeedMe() {
		out = append(out, GuaranteeRepliesNeedMe)
	}
	if w.Config().EndWhenMerged() {
		out = append(out, GuaranteeEndWhenMerged)
	}
	return out
}

// holding reports whether a guarantee is on.
func (w *Workspace) holding(g Guarantee) bool {
	for _, on := range w.Guarantees() {
		if on.Key == g.Key {
			return true
		}
	}
	return false
}

// reviewThen is what whoever was refused for want of the user's review
// does next.
func reviewThen(by Actor, then string) string {
	if by.IsYou() {
		return "Review and approve first (Review on the capsule), then " + then + "."
	}
	return "Ask the user to review it (ws mission ask \"…\") and wait for them: once their review approves it as it is, " + then + "."
}

// holdReview holds a guarantee that wants the user's review before work
// leaves a capsule of a mission: refused, when it's on, unless their latest
// review of it approves it as it is. what is what's being done, and then
// what to do about it.
func (w *Workspace) holdReview(g Guarantee, slug, repo, name, what, then string) error {
	if !w.holding(g) {
		return nil
	}
	capsule := repo + "/" + name
	head := workspace.GitRevParse(filepath.Join(w.WS().RepoDir(repo), name), "HEAD")
	r, _ := review.For(w.Root(), slug).Latest(capsule)
	var why string
	switch {
	case r == nil:
		why = "no review of " + capsule
	case r.Verdict != review.Approve:
		why = fmt.Sprintf("latest review of %s (%s) does not approve", capsule, r.ID)
	case r.Head != head:
		why = fmt.Sprintf("latest review of %s (%s) approved %s, now at %s", capsule, r.ID, short(r.Head), short(head))
	default:
		return nil
	}
	return refused("guarantee", "%s (%s) holds back %s: %s. %s", g.Key, g.What, what, why, then)
}
