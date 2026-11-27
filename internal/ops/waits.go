package ops

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/crew"
	"github.com/withworkspace/workspace/internal/execlog"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/workspace"
)

// Wait's expansion: what ws does at a Wait step once the mission reaches
// it, by who it's on and whether it has checks (mission/wait.go).
//
//   - On you: ws asks you, with what to do, and you mark it done.
//   - On anyone else: it waits, in their court.
//   - With checks (commands), it's watched: ws runs them every so often
//     (the daemon, CheckWaits), and when you ask it to look, and passes
//     the step once every one is met. You saying it's done has ws look
//     too: unmet, the step stays, with what the look found.
//   - Without, nobody's watching: it's passed when someone says it's
//     happened. Nothing wakes the orchestrator for it, and nothing says it
//     will.
//
// You can always pass a Wait as it is.

// waitCheckTimeout is how long a Wait's check may run: it's a look at
// whether something's happened, run every few minutes, not a build.
const waitCheckTimeout = time.Minute

// reachWait is the mission reaching a Wait step. It says what it did.
func (w *Workspace) reachWait(m *mission.Mission, done string, st mission.StageState) string {
	w.Missions.ReachWait(m.Slug, st.N) //nolint:errcheck // since when: the step waits without it
	on := strings.TrimPrefix(waitsOnWords(st), "waits")
	watched := ""
	if st.Step.Watched() {
		watched = fmt.Sprintf("Workspace watches (%s) and passes the step once it happens", waitCommands(st))
	}
	if st.Step.WaitOnYou() {
		text := fmt.Sprintf("%sStep %d is yours to do: %s", done, st.N, sentence(st.Plain()))
		if left := w.leftBefore(m); left != "" {
			text += " " + left
		}
		if watched != "" {
			text += " " + watched + "; or mark done to check now."
		} else {
			text += " Mark done once it has happened, with anything the next crew should know (Done: tagged v1.4.0)."
		}
		if _, err := w.Missions.AskAtStep(m.Slug, mission.TaskDo, text, st.N, mission.WaitDone); err != nil {
			return "could not ask the user: " + err.Error()
		}
		if watched != "" {
			return fmt.Sprintf("step %d waits for the user; %s", st.N, watched)
		}
		return fmt.Sprintf("step %d waits for the user to mark done", st.N)
	}
	if watched != "" {
		return fmt.Sprintf("step %d waits%s: %s", st.N, on, watched)
	}
	return fmt.Sprintf("step %d waits%s, unwatched: passes when marked done (or ws mission accept, once it has happened)", st.N, on)
}

// waitsOnWords is who a Wait step waits on, as a sentence says it: "waits
// on npm", or "waits" for one that doesn't say.
func waitsOnWords(st mission.StageState) string {
	switch s := st.Step; {
	case s.Who == mission.WhoOutside && s.Whom != "":
		return "waits on " + s.Whom
	case s.Who == mission.WhoCI:
		return "waits on " + s.WhoSays()
	case s.Who == mission.WhoYou:
		return "waits on the user"
	}
	return "waits"
}

// waitCommands is a Wait step's checks, as a sentence names them.
func waitCommands(st mission.StageState) string {
	var out []string
	for _, c := range st.Step.Checks {
		if c.Heard() {
			out = append(out, "`"+c.Until+"`")
			continue
		}
		out = append(out, "`"+c.Command+"`")
	}
	return strings.Join(out, ", ")
}

// leftBefore is what the crew who last handed off said is left, for a
// Wait on you: what to do is often there, in more detail than the plan
// had when it was written (the version to tag, the setting to flip).
func (w *Workspace) leftBefore(m *mission.Mission) string {
	latest := m.LatestHandoffs()
	if len(latest) == 0 {
		return ""
	}
	c := latest[0]
	data, err := os.ReadFile(w.Missions.HandoffPath(m.Slug, c.ID))
	if err != nil {
		return ""
	}
	h, err := mission.ParseHandoff(string(data))
	if err != nil || h.LeftLine() == "" {
		return ""
	}
	return fmt.Sprintf("Left to do, from %s: %s", c.ID, sentence(clipLine(h.Left, 400)))
}

// waitDir is where a Wait step's checks run: the mission's capsule in the
// repo it names, or with none there, that repo's ground; the workspace's
// root when it names no repo.
func (w *Workspace) waitDir(m *mission.Mission, st mission.StageState) string {
	if st.Step.Capsule == "" {
		return w.Root()
	}
	if capsule, ok := w.stepCapsule(m, st); ok {
		repo, name, _ := strings.Cut(capsule, "/")
		return filepath.Join(w.WS().RepoDir(repo), name)
	}
	repo, _, _ := strings.Cut(st.Step.Capsule, "/")
	if r, ok := w.WS().ResolveAlias(repo); ok {
		repo = r
	}
	if _, ok := isDir(filepath.Join(w.WS().RepoDir(repo), workspace.GroundDir)); ok {
		return filepath.Join(w.WS().RepoDir(repo), workspace.GroundDir)
	}
	return w.Root()
}

// lookAtWait runs a watched step's checks now, and keeps what they came
// to. It reports whether every one is met.
func (w *Workspace) lookAtWait(m *mission.Mission, st mission.StageState) ([]mission.CheckResult, bool) {
	dir, env := w.waitDir(m, st), crew.Env(w.Root(), m.Slug, "")
	results := make([]mission.CheckResult, len(st.Step.Checks))
	met := len(results) > 0
	for i, c := range st.Step.Checks {
		if c.Heard() {
			results[i] = w.untilResult(st, c)
			met = met && results[i].Result == mission.CheckMet
			continue
		}
		who := fmt.Sprintf("watch %s step %d, %d of %d", m.Slug, st.N, i+1, len(results))
		since, pid := time.Now(), 0
		results[i] = runCheckWithin(dir, c, env, checkHooks{Started: func(p int) {
			pid = p
			execlog.Start(w.Root(), who, []string{"sh", "-c", c.Command}, dir, env, p)
		}}, waitCheckTimeout)
		if pid == 0 {
			execlog.Start(w.Root(), who, []string{"sh", "-c", c.Command}, dir, env, 0)
		}
		execlog.End(w.Root(), who, pid, time.Since(since), checkEnded(results[i]))
		met = met && results[i].Result == mission.CheckMet
	}
	w.Missions.RecordLook(m.Slug, st.N, results) //nolint:errcheck // the look stands: it's acted on below
	return results, met
}

// waitAt is the Wait step the mission is at, if it's at one that's
// reached (not waiting for a call to start).
func (w *Workspace) waitAt(slug string) (*mission.Mission, mission.StageState, bool) {
	m, err := w.Missions.Load(slug)
	if err != nil {
		return nil, mission.StageState{}, false
	}
	cur, ok := currentStep(w.Missions, m)
	if !ok || cur.Step.Type != mission.StepWait || cur.Held() {
		return m, mission.StageState{}, false
	}
	return m, cur, true
}

// passWatched passes a watched step whose checks are all met, for ws, and
// says what it did.
func (w *Workspace) passWatched(slug string, st mission.StageState) string {
	m, err := w.Missions.Load(slug)
	if err != nil {
		return ""
	}
	began := w.stepBegan(m, st.N)
	said := w.passStep(slug, st.N, "ws", "happened: "+waitCommands(st)+" "+metWord(st), mission.StepWait)
	if said != "" {
		w.settleAsked(m, st.N, began, "wait over")
	}
	return said
}

func metWord(st mission.StageState) string {
	if len(st.Step.Checks) == 1 {
		return "is met"
	}
	return "are met"
}

// CheckWait has ws look at the Wait step the mission is at now, rather
// than when it next would: for you. Met, the step's passed. It says what
// the look found.
func (w *Workspace) CheckWait(slug string, by Actor) (string, error) {
	if !by.IsYou() {
		return "", refused("not-yours", "ws looks at a watched step by itself; the user may have it look now")
	}
	m, st, ok := w.waitAt(slug)
	switch {
	case m == nil:
		return "", ErrNotFound
	case !ok:
		return "", refused("no-wait", "the mission is not at a Wait step")
	case !st.Step.Watched():
		return "", refused("not-watched", "step %d has no check to run: mark done once it has happened", st.N)
	}
	results, met := w.lookAtWait(m, st)
	if !met {
		return fmt.Sprintf("Not yet: %s. Still watching.", lookSaid(results)), nil
	}
	if said := w.passWatched(slug, st); said != "" {
		return upperFirst(said) + ".", nil
	}
	return "Happened: step moved on.", nil
}

// lookSaid is what a look found, for a sentence: each check that isn't
// met, with how its command ended.
func lookSaid(results []mission.CheckResult) string {
	var out []string
	for _, r := range results {
		if r.Result == mission.CheckMet {
			continue
		}
		out = append(out, fmt.Sprintf("%s (%s)", clipLine(r.Check, 120), firstLine(r.Evidence)))
	}
	if len(out) == 0 {
		return "all checks met"
	}
	return strings.Join(out, "; ")
}

// CheckWaits looks at each mission in play that's at a watched Wait step
// whose next look is due, and passes those whose checks are all met. The
// daemon calls it every minute. It returns what it did, a line each.
func (w *Workspace) CheckWaits() []string {
	missions, err := w.Missions.InPlay()
	if err != nil {
		return nil
	}
	// What's heard of a pull request is heard as the Reviews list is read:
	// a mission waiting on one keeps it read.
	w.freshenForWaits(missions)
	var out []string
	for _, m := range missions {
		if !Watched(m) || m.ToBrief() {
			continue
		}
		cur, ok := currentStep(w.Missions, m)
		if !ok || !cur.Step.Watched() || cur.Held() || w.now().Before(cur.Watch().NextLook()) {
			continue
		}
		if _, met := w.lookAtWait(m, cur); met {
			if said := w.passWatched(m.Slug, cur); said != "" {
				out = append(out, m.Slug+": "+said)
			}
		}
	}
	return out
}

// answerWait answers a Wait on you: Done (alone, or with what you did
// after it) marks it done, and Pass it passes it whatever its checks say.
// Marked done, a watched step is looked at first: met, it's passed; unmet,
// it stays yours, with what the look found. Anything else is answered as
// at any step of yours (ok false).
func (w *Workspace) answerWait(m *mission.Mission, t mission.Task, by Actor, text string) (Answered, bool, error) {
	out := Answered{Slug: m.Slug}
	note, done := mission.SaysDone(text)
	pass := strings.EqualFold(text, mission.ChecksPass)
	if !done && !pass {
		return out, false, nil
	}
	cur, ok := currentStep(w.Missions, m)
	if !ok || cur.N != t.Step || cur.Step.Type != mission.StepWait {
		// The step's moved on since it was asked: the task is moot.
		w.Missions.CloseTask(m.Slug, t.N, by.By(), "") //nolint:errcheck // moot either way
		out.Next = fmt.Sprintf("step %d already moved on", t.Step)
		return out, true, nil
	}
	if done && cur.Step.Watched() {
		if results, met := w.lookAtWait(m, cur); !met {
			if note != "" {
				w.waitNote(m.Slug, cur, by, note)
			}
			text := fmt.Sprintf("Step %d marked done, but checks not met yet: %s. Still watching; mark done again to check now, or pass as is.", cur.N, lookSaid(results))
			if _, err := w.Missions.AskAtStep(m.Slug, mission.TaskDo, text, cur.N, mission.WaitDone, mission.ChecksPass); err != nil {
				return out, true, wrap(err)
			}
			w.youActed(m.Slug, by)
			out.Next = fmt.Sprintf("checks not met yet (%s): step %d stays yours, still watching", lookSaid(results), cur.N)
			return out, true, nil
		}
	}
	if _, _, err := w.Missions.AnswerTask(m.Slug, t.N, text, by.By(), false); err != nil {
		return out, true, wrap(err)
	}
	why := "marked done by the user"
	switch {
	case pass:
		why = "passed by the user, checks as they were"
	case cur.Step.Watched():
		why = "marked done by the user, and " + waitCommands(cur) + " " + metWord(cur)
	}
	if note != "" {
		why += ": " + oneLine(note)
		w.waitNote(m.Slug, cur, by, note)
	}
	began := w.stepBegan(m, cur.N)
	acc, err := w.acceptStage(m.Slug, by, why)
	if err != nil {
		return out, true, err
	}
	w.settleAsked(m, cur.N, began, "marked done")
	out.Next = acc.Next
	return out, true, nil
}

// SayWaiting records that a mission waits on something outside it that ws
// can't see happen, for whoever says so (ws mission wait); "" takes it
// back.
//
// Not the orchestrator's to say at a Wait step. The step says what it
// waits on and how it's passed, with marking it done on the page; the
// same wait said again as the mission's, with nothing for the user to do,
// is what they'd read in its place.
func (w *Workspace) SayWaiting(slug, text string, by Actor) error {
	text = strings.TrimSpace(text)
	if text != "" && by.Kind == Orchestrator {
		if _, st, ok := w.waitAt(slug); ok {
			how := "the user marks it done once it's happened, or you accept it once you're shown it has"
			switch {
			case st.Step.Watched():
				how = "ws watches its checks, and passes it"
			case st.Step.WaitOnYou():
				how = "it's the user's to do and mark done"
			}
			return refused("wait-step", "step %d is a Wait, and says what the mission waits on already: %s. The user is shown that, so there's nothing to add: note why it waits, and leave it there", st.N, how)
		}
	}
	return wrap(w.Missions.SetWaiting(slug, text, by.By()))
}

// waitNote keeps what you said of a Wait you've done, or that's happened
// (the version tagged, where it was deployed), as a decision: the crew
// after are told decisions, and not why a step was passed.
func (w *Workspace) waitNote(slug string, st mission.StageState, by Actor, note string) {
	w.Missions.Decide(slug, mission.Decision{ //nolint:errcheck // the step's why says it too
		What: oneLine(note), By: by.By(),
		Why: fmt.Sprintf("noted at step %d as it passed: %s", st.N, st.Short()),
	})
}
