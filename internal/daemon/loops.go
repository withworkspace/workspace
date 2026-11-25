package daemon

import (
	"context"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/crew"
	"github.com/withworkspace/workspace/internal/execlog"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/pulls"
	"github.com/withworkspace/workspace/internal/release"
	"github.com/withworkspace/workspace/internal/runs"
	"github.com/withworkspace/workspace/internal/web"
)

// loops starts what the daemon keeps doing while it runs, each until stop
// closes: watching the missions (their pull requests, held turns, crew
// who reported done, steps out of time, Wait steps), sweeping (asks
// nobody kept, expired artifacts, runs whose owner's gone), its names
// under .localhost, and looking for updates, the grounds' remotes and who's
// asking for your review. w is its web UI, nil for none.
func (d *daemon) loops(w *web.Server, stop <-chan struct{}) {
	logf := d.s.Log.Printf
	var reviews *pulls.Service
	if w != nil {
		reviews = w.Reviews
	}
	go d.watchPRsLoop(stop)
	go d.watchHeldLoop(stop)                                                // turns held on crew's own work
	go d.closeDoneLoop(logf, stop)                                          // crew that reported done, their turn over
	go sweepAsksLoop(d.cfg.Root, logf, stop)                                // asks nobody kept
	go d.watchLimitsLoop(logf, stop)                                        // steps out of time
	go d.watchWaitsLoop(logf, stop)                                         // Wait steps ws watches
	go d.sweepArtifactsLoop(logf, stop)                                     // artifacts on this machine that expired
	go d.sweepRunsLoop(logf, stop)                                          // runs whose owner has gone, or whose time is up
	go routerLoop(d.cfg.Root, d.s.WebPort, logf, stop)                      // names under .localhost, with [router] enabled
	go d.checkUpdatesLoop(logf, stop)                                       // later releases of ws and plugins, once a day
	go d.followGroundsLoop(logf, stop)                                      // each repo's ground kept up with its remote
	go followReviewsLoop(reviews, logf, stop)                               // who's asking for your review by name
	go d.readCrewSpendLoop(logf, stop)                                      // what crew have spent, from their session files
	go d.w.NoteRunning(func() string { return previousVersion(d.cfg.Exe) }) // which ws this is, for what's new
	go d.w.NoteStarted()                                                    // and for the feed: an upgrade it restarted onto
}

// Crew that end a turn to wait on work they left running have that turn
// held (mission.Crew.Hold), and nobody else looks at it while the work
// runs. The workspace's daemon watches it: it records the work ending, and
// hands the turn to the orchestrator once it's no longer held: the work
// ended and nothing woke the crew, or it has run too long.

// watchHeldEvery is how often the daemon looks at held turns: often enough
// for the minute crew have to be woken (mission.WakeGrace).
const watchHeldEvery = 15 * time.Second

// workSeen is how the daemon sees work end: alive says whether a process
// is, and callDone whether a tool call in the session of crew over ACP the
// daemon holds has finished (nil: it holds none).
type workSeen struct {
	alive    func(pid int) bool
	callDone func(slug, crewID, call string) bool
}

// ended is whether a piece of crew's work has ended: its tool call
// finished, or its process gone. Work with neither isn't seen to end.
func (s workSeen) ended(slug, crewID string, w mission.Work) bool {
	return w.Call != "" && s.callDone != nil && s.callDone(slug, crewID, w.Call) ||
		w.PID > 0 && !s.alive(w.PID)
}

// watchHeld looks at the held turns of crew in play, at now. Work seen to
// end (seen.ended) is recorded as ended. A turn no longer held is one the
// orchestrator takes (mission.OrchestratorTakes): notify hands it on, and
// it's taken once.
func watchHeld(store mission.Store, now time.Time, seen workSeen, notify func(slug string, e mission.OrchestratorEvent)) {
	missions, err := store.InPlay()
	if err != nil {
		return
	}
	for _, m := range missions {
		if !ops.Watched(m) {
			continue
		}
		for _, c := range m.Crew {
			if c.Pending == nil || c.Ended != nil {
				continue
			}
			var gone []mission.Work
			for _, w := range c.Pending.Running() {
				if seen.ended(m.Slug, c.ID, w) {
					gone = append(gone, w)
				}
			}
			if len(gone) > 0 {
				ended, err := store.EndWork(m.Slug, c.ID, c.Pending.At, now, func(w mission.Work) bool { return slices.Contains(gone, w) })
				if err != nil || !ended {
					continue
				}
				if m, err = store.Load(m.Slug); err != nil {
					break
				}
			}
			e := mission.OrchestratorEvent{Crew: c.ID, Trigger: "quiet", At: now}
			if takes, _ := mission.OrchestratorTakes(m, e); takes {
				notify(m.Slug, e)
			}
		}
	}
}

// watchHeldLoop watches held turns from the workspace's daemon until stop
// closes. callDone is the daemon's: whether a tool call of crew over ACP
// it holds has finished.
func (d *daemon) watchHeldLoop(stop <-chan struct{}) {
	t := time.NewTicker(watchHeldEvery)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
		}
		WatchHeld(d.w, time.Now(), crew.ProcessAlive, d.s.ACPCallDone)
	}
}

// WatchHeld looks at held turns once, at now, as the daemon does each
// tick: alive says whether a process is, and callDone whether a tool call
// of crew over ACP the daemon holds has finished (nil: it holds none).
func WatchHeld(ctx *ops.Workspace, now time.Time, alive func(pid int) bool, callDone func(slug, crewID, call string) bool) {
	root := ctx.Root()
	watchHeld(ctx.Missions, now, workSeen{alive: alive, callDone: callDone}, func(slug string, e mission.OrchestratorEvent) {
		if ctx.Config().OrchestratorEnabled() { // off, a turn no longer held is yours as it is
			ctx.Notify(slug, e)
		}
	})
	// Crew nobody's looking at, their turn long over with nothing said:
	// asked where they are (ops.NudgeIdle).
	for _, said := range ctx.NudgeIdle(now) {
		execlog.Note(root, "idle", "%s", said)
	}
}

// closeDoneEvery is how often the daemon looks for crew to close for having
// reported done (ops.CloseDone): a fraction of how long they're left open.
const closeDoneEvery = 30 * time.Second

// closeDoneLoop closes crew who've reported done, once their turn's over
// and nothing's looking at it, until stop closes.
func (d *daemon) closeDoneLoop(logf func(string, ...any), stop <-chan struct{}) {
	t := time.NewTicker(closeDoneEvery)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case now := <-t.C:
			for _, said := range d.w.CloseDone(now) {
				logf("done crew: %s", said)
			}
		}
	}
}

// readCrewSpendEvery is how often the daemon reads what crew have spent
// (ops.ReadCrewSpend): their session files are cleared after a while, and
// each read is of what's been appended since the last.
const readCrewSpendEvery = 5 * time.Minute

// readCrewSpendLoop reads what crew on missions in play have spent, a
// minute after the daemon starts and then every readCrewSpendEvery, until
// stop closes.
func (d *daemon) readCrewSpendLoop(logf func(string, ...any), stop <-chan struct{}) {
	read := func() {
		if _, err := d.w.ReadCrewSpend(); err != nil {
			logf("spend: couldn't read what crew spent: %v", err)
		}
	}
	select {
	case <-stop:
		return
	case <-time.After(time.Minute):
		read()
	}
	t := time.NewTicker(readCrewSpendEvery)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
			read()
		}
	}
}

// Asks nobody kept (conversation/ask.go) are swept here, in the daemon: it
// owns the records and is up whenever a page that could ask is open (a
// page's socket holds it up). It sweeps as it starts, which takes
// what was due while it was down, and then every SweepEvery.

// sweepAsksLoop sweeps until stop closes.
func sweepAsksLoop(root string, logf func(string, ...any), stop <-chan struct{}) {
	sweepAsks(root, time.Now(), logf)
	t := time.NewTicker(conversation.SweepEvery)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
			sweepAsks(root, time.Now(), logf)
		}
	}
}

// sweepAsks sweeps once, saying in the daemon's log what it swept.
func sweepAsks(root string, now time.Time, logf func(string, ...any)) {
	swept, err := conversation.SweepAsks(context.Background(), root, now)
	switch {
	case err != nil:
		logf("asks: couldn't sweep: %v", err)
	case len(swept) > 0:
		logf("asks: swept %d nobody kept (%s)", len(swept), strings.Join(swept, ", "))
	}
}

// watchLimitsEvery is how often the daemon looks for a step whose time
// has run out.
const watchLimitsEvery = time.Minute

// watchLimitsLoop says of each step whose time has run out while crew are
// still at it, once, until stop closes.
func (d *daemon) watchLimitsLoop(logf func(string, ...any), stop <-chan struct{}) {
	t := time.NewTicker(watchLimitsEvery)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
			for _, said := range d.w.CheckLimits() {
				logf("limits: %s", said)
			}
		}
	}
}

// watchWaitsEvery is how often the daemon looks for a watched Wait step
// whose next look is due (the step says how often that is: mission.Watch).
const watchWaitsEvery = time.Minute

// watchWaitsLoop runs the checks of each watched Wait step a mission is
// at, as its looks come due, and passes those that are met, until stop
// closes. A look runs commands, so it's a loop of its own: a slow one
// holds up nothing but the next look.
func (d *daemon) watchWaitsLoop(logf func(string, ...any), stop <-chan struct{}) {
	t := time.NewTicker(watchWaitsEvery)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
			for _, said := range d.w.CheckWaits() {
				logf("waits: %s", said)
			}
		}
	}
}

// sweepArtifactsEvery is how often a daemon that keeps running sweeps
// again.
const sweepArtifactsEvery = 24 * time.Hour

func (d *daemon) sweepArtifactsLoop(logf func(string, ...any), stop <-chan struct{}) {
	SweepArtifacts(d.w, logf)
	t := time.NewTicker(sweepArtifactsEvery)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
			SweepArtifacts(d.w, logf)
		}
	}
}

// SweepArtifacts sweeps once, saying in the daemon's log what it did.
func SweepArtifacts(w *ops.Workspace, logf func(string, ...any)) {
	swept, err := w.ExpireArtifacts()
	ids := func(n int, id func(int) string) string {
		var out []string
		for i := range n {
			out = append(out, id(i))
		}
		return strings.Join(out, ", ")
	}
	if n := len(swept.Brought); n > 0 {
		logf("artifacts: brought %d onto this machine, from the repo they were kept in (%s)", n, ids(n, func(i int) string { return swept.Brought[i].ID }))
	}
	if n := len(swept.Converted); n > 0 {
		logf("artifacts: put the versions of %d in a repo of their own (%s)", n, ids(n, func(i int) string { return swept.Converted[i].ID }))
	}
	if n := len(swept.Discarded); n > 0 {
		logf("artifacts: discarded %d expired (%s)", n, ids(n, func(i int) string { return swept.Discarded[i].ID }))
	}
	if n := len(swept.Erased); n > 0 {
		logf("artifacts: deleted %d discarded 30 days ago for good (%s)", n, ids(n, func(i int) string { return swept.Erased[i].ID }))
	}
	if err != nil {
		logf("artifacts: couldn't sweep: %v", err)
	}
}

// The daemon ends runs (internal/runs) whose owner has gone or whose time
// is up: as it starts, which takes what was due while it was down, then
// every runs.SweepEvery, and at once when it's poked (crew have ended). A
// run that ended by itself is removed an hour later, its log with it.
func (d *daemon) sweepRunsLoop(logf func(string, ...any), stop <-chan struct{}) {
	d.sweepRuns(logf)
	t := time.NewTicker(runs.SweepEvery)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
			d.sweepRuns(logf)
		case <-d.runsPoke:
			d.sweepRuns(logf)
		}
	}
}

// sweepRuns sweeps once, saying in the daemon's log what it ended.
func (d *daemon) sweepRuns(logf func(string, ...any)) {
	swept, err := d.w.SweepRuns(time.Now())
	for _, s := range swept {
		if s.Run.State == runs.Running {
			logf("runs: ended %s: %s", s.Run.Ref(), s.Why)
		}
	}
	if err != nil {
		logf("runs: couldn't end every one due: %v", err)
	}
}

// The missions' PRs change on GitHub while nobody's looking: checks go red,
// a reviewer asks for changes, someone comments, one merges. The
// workspace's daemon watches them, and a change is noticed once, whenever
// it's next looked at (mission.SeePR), and acted on (ops.WatchPRs).

// watchPRsLoop watches the missions' PRs from the workspace's daemon until stop
// closes, sharing what GitHub said with the web UI. It looks every
// ops.WatchPRsEvery, and as soon as a page's fetch brings news (a merge,
// say: PRCache.News), so ws never sits on what a page already shows.
func (d *daemon) watchPRsLoop(stop <-chan struct{}) {
	t := time.NewTicker(ops.WatchPRsEvery)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
		case <-d.prs.News():
		}
		WatchPRs(d.w, d.prs)
	}
}

// WatchPRs looks at the missions' PRs once, and acts on what's changed.
func WatchPRs(ctx *ops.Workspace, prs *ops.PRCache) {
	if ctx.GitHub() == nil {
		return
	}
	root := ctx.Root()
	ctx.TidyDocked() // capsules ws docked for a question, whose PR is done
	missions, err := (mission.Store{Root: root}).List()
	if err != nil {
		return
	}
	var live []*mission.Mission
	for _, m := range missions {
		if ops.Watched(m) {
			live = append(live, m)
		}
	}
	idx := prs.Get(ctx.WS(), ctx.GitHub(), ops.MissionRepos(live))
	ctx.WatchPRs(ops.BuildMissionViewsFrom(ctx.WS(), idx, live))
}

// The daemon looks for later releases once a day: ws's own, and each
// plugin's that was installed from a repo (ops.CheckUpdates). What it
// finds is kept for the web UI to show (Settings' About, Plugins), and
// nothing's installed. [updates] check = false in ws.local.toml turns it
// off.

// updatesFirst is how long after it starts a daemon first asks whether
// it's time to look, and updatesTick how often after that: a look that's
// due is made within the hour, and one that couldn't ask (no network) is
// tried again then.
var (
	updatesFirst = time.Minute
	updatesTick  = time.Hour
)

// checkUpdatesLoop looks for updates when it's a day since a look last
// answered, until stop closes.
func (d *daemon) checkUpdatesLoop(logf func(string, ...any), stop <-chan struct{}) {
	t := time.NewTimer(updatesFirst)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
		}
		if d.w.UpdatesDue() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			logf("updates: %s", updatesSaid(d.w.CheckUpdates(ctx)))
			cancel()
		}
		t.Reset(updatesTick)
	}
}

// updatesSaid is what a look found, in a line for the daemon's log.
func updatesSaid(up ops.Updates) string {
	said := "ws " + up.WS.Version + " is the latest release"
	switch {
	case up.WS.Error != "":
		said = "couldn't look for ws's latest release: " + up.WS.Error
	case up.WS.Newer:
		said = "ws " + up.WS.Latest + " is out (this is " + up.WS.Version + ")"
	case up.WS.Latest == "":
		said = "ws has no release to compare with"
	case up.WS.Latest != up.WS.Version:
		said = "the latest release of ws is " + up.WS.Latest + " (this is " + up.WS.Version + ")"
	}
	for _, name := range slices.Sorted(maps.Keys(up.Plugins)) {
		if p := up.Plugins[name]; p.Newer {
			said += "; " + name + " " + p.Latest + " is out"
		}
	}
	return said
}

// followGroundsFirst is how long after it starts a daemon first looks,
// and followGroundsEvery how often after that.
var (
	followGroundsFirst = time.Minute
	followGroundsEvery = 5 * time.Minute
)

// followGroundsLoop follows the repos' grounds until stop closes, and
// logs what's changed: a ground moved, or one that couldn't be (said
// once, until what stops it does).
func (d *daemon) followGroundsLoop(logf func(string, ...any), stop <-chan struct{}) {
	t := time.NewTimer(followGroundsFirst)
	defer t.Stop()
	said := map[string]string{} // repo → why it couldn't be followed, as last logged
	for {
		select {
		case <-stop:
			return
		case <-t.C:
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		for _, f := range d.w.FollowGrounds(ctx) {
			if f.Moved > 0 {
				logf("grounds: %s moved forward %d %s, to %s", f.Repo, f.Moved, plural(f.Moved, "commit"), f.At)
			}
			why := groundHeld(f)
			if why != said[f.Repo] && why != "" {
				logf("grounds: %s couldn't be followed: %s", f.Repo, why)
			}
			said[f.Repo] = why
		}
		cancel()
		t.Reset(followGroundsEvery)
	}
}

// groundHeld is why a ground wasn't followed, in a line: "" for nothing
// in its way.
func groundHeld(f ops.GroundFollowed) string {
	switch {
	case f.Err != nil:
		return f.Err.Error()
	case f.Held != nil:
		return f.Held.Reason
	}
	return ""
}

// followReviewsFirst is how long after it starts a daemon first looks,
// followReviewsEvery how often after that, and followReviewsSoon how long
// after something here made the list stale (a review posted, your name
// taken off): long enough for GitHub to say so, and for the reviews of a
// stack posted together to be one look.
var (
	followReviewsFirst = time.Minute
	followReviewsEvery = 5 * time.Minute
	followReviewsSoon  = 5 * time.Second
)

// followReviewsLoop looks at who's asking for your review until stop
// closes. reviews is the service the web UI reads from, so what a look
// finds is what a page shows; nil where there's no GitHub to ask. What
// stops it looking is logged once, until it changes.
func followReviewsLoop(reviews *pulls.Service, logf func(string, ...any), stop <-chan struct{}) {
	if reviews == nil {
		return
	}
	t := time.NewTimer(followReviewsFirst)
	defer t.Stop()
	due := reviews.Due()
	said := ""
	for {
		select {
		case <-stop:
			return
		case <-t.C:
		case <-due:
			select {
			case <-stop:
				return
			case <-time.After(followReviewsSoon):
			}
		}
		why := ""
		if _, err := reviews.Look(); err != nil {
			why = err.Error()
		}
		if why != said && why != "" {
			logf("reviews: who's asking for your review couldn't be looked at: %s", why)
		}
		said = why
		if !t.Stop() {
			select {
			case <-t.C:
			default:
			}
		}
		t.Reset(followReviewsEvery)
	}
}

// previousVersion is the ws an upgrade replaced, where one is kept beside
// exe: "" for none.
func previousVersion(exe string) string {
	if exe == "" {
		return ""
	}
	v, _ := release.VersionOf(release.PrevPath(exe)) // none kept is none
	return v
}

func plural(n int, word string) string {
	if n == 1 {
		return word
	}
	return word + "s"
}
