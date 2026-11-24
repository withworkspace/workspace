package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/withworkspace/workspace/internal/acp"
	"github.com/withworkspace/workspace/internal/crew"
	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/mission"
)

// Crew over ACP (harness.CrewACP): an agent the server holds, not a
// terminal. The server records its session (the crew's transcript, which
// the web UI reads as it would a harness's session file), tells the
// mission what the crew is doing as a terminal crew's hooks do (working,
// waiting with its reply, permission, a question), leaves what ws can't
// answer for it to you (the web UI's permission and question cards), and
// records its end as any crew's.

// acpCrew is crew over ACP the server holds.
type acpCrew struct {
	spec          Spec
	agent         *acp.Agent
	record        string // its record's path
	stopRequested atomic.Bool
	// dropped is how many turns in a row an error of the agent's cut off
	// (harness.TurnDropped), each of which ws had it go on from.
	dropped atomic.Int32
}

// A turn an error of the agent's cut off is gone on with by ws, as you
// would by sending it "continue": after droppedWait (as long again for
// each in a row, the error being one that may pass), and no more than
// maxDropped times in a row, after which it's left as any turn that ended
// without the crew saying where they are.
const maxDropped = 3

var droppedWait = 5 * time.Second

// droppedText is what crew are sent to go on from a turn that was cut
// off.
const droppedText = "Continue. Your last turn was cut off by an error from your own agent (%s), not ended by you, and ws is sending this in its place. The step you were in the middle of may not have finished: check what it left before you go on."

// acpStartTimeout bounds starting an agent and its session.
const acpStartTimeout = 90 * time.Second

func (s *Server) launchACP(spec Spec) (int, error) {
	if err := s.reserve(spec.Key()); err != nil {
		return 0, err
	}
	started := false
	defer func() {
		if !started {
			s.release(spec.Key())
		}
	}()

	run := *spec.ACP
	path := s.store.ACPRecordPath(spec.Slug, spec.CrewID)
	// A crew resumed goes on from what ws kept of it.
	events, _, _ := acp.ReadRecordFile(path, 0)
	kept := acp.Replay(nil, events)
	rec, err := acp.AppendRecord(path)
	if err != nil {
		return 0, err
	}
	key := spec.Key()
	ctx, cancel := context.WithTimeout(context.Background(), acpStartTimeout)
	defer cancel()
	ag, err := acp.Run(ctx, acp.Options{
		Command:   acp.Command{Args: run.Agent, Dir: run.Dir, Env: append(append([]string(nil), spec.Env...), run.Env...), Stderr: s.agentLog(key)},
		Session:   acp.SessionParams{Dir: run.Dir, Dirs: run.Dirs},
		Load:      run.Load,
		Kept:      kept,
		Mode:      run.Mode,
		Model:     run.Model,
		Settings:  run.Settings,
		Auth:      run.Auth,
		Policy:    harness.CrewPolicy(run.Allow),
		Extension: acpExtension(run),
		Record:    rec,
		OnTurn:    func(t acp.Turn) { s.acpTurn(spec, t) },
	})
	if err != nil {
		rec.Write(acp.Event{Error: err.Error()}) //nolint:errcheck // failing anyway
		rec.Close()
		return 0, err
	}
	cr := &acpCrew{spec: spec, agent: ag, record: path}
	started = true
	s.mu.Lock()
	delete(s.starting, key)
	s.agents[key] = cr
	s.checkIdleLocked()
	s.mu.Unlock()
	s.store.CrewHook(spec.Slug, spec.CrewID, mission.Hook{Conversation: ag.State().Session}) //nolint:errcheck // a notice
	s.Log.Printf("launched %s (%s, over ACP) pid %d in %s: session %s", key, spec.Harness, ag.Client().Pid(), run.Dir, ag.State().Session)
	if strings.TrimSpace(run.Prompt) != "" {
		ag.Send(acp.Text(run.Prompt)) //nolint:errcheck // it's just started
	}
	go s.followACP(cr)
	s.watchers.Add(1)
	go func() {
		defer s.watchers.Done()
		s.watchACP(cr, rec)
	}()
	return ag.Client().Pid(), nil
}

// taken is whether crew of key are running, either kind, or starting
// (s.mu held).
func (s *Server) taken(key string) bool {
	_, term := s.crews[key]
	_, agent := s.agents[key]
	return term || agent || s.starting[key]
}

// reserve holds key for crew about to be started, unless crew of that key
// are running or starting already (crew.ErrRunning), or the server's
// going. The launch puts its crew in their map in the reservation's place,
// or releases it.
func (s *Server) reserve(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closing {
		return errors.New("server is shutting down")
	}
	if s.taken(key) {
		return fmt.Errorf("crew %s is %w", key, crew.ErrRunning)
	}
	s.starting[key] = true
	return nil
}

// release gives up key's reservation: the crew didn't start.
func (s *Server) release(key string) {
	s.mu.Lock()
	delete(s.starting, key)
	s.mu.Unlock()
}

// agentLog is where an agent's own logging goes: the server's log.
func (s *Server) agentLog(key string) writerFunc {
	return func(p []byte) (int, error) {
		for _, line := range strings.Split(strings.TrimRight(string(p), "\n"), "\n") {
			if strings.TrimSpace(line) != "" {
				s.Log.Printf("%s: %s", key, line)
			}
		}
		return len(p), nil
	}
}

// acpExtension answers the agent's own requests: some as the run says
// (a plan rejected), some by a person (Cursor's question, which waits as a
// permission does), the rest not offered.
func acpExtension(run harness.ACPRun) func(ctx context.Context, asks *acp.Asks, method string, params json.RawMessage) (any, error) {
	return func(ctx context.Context, asks *acp.Asks, method string, params json.RawMessage) (any, error) {
		if ans, ok := run.Answers[method]; ok {
			return ans, nil
		}
		for _, m := range run.Ask {
			if m != method {
				continue
			}
			ans := asks.Wait(ctx, acp.Ask{Method: method, Params: params})
			if ans.Cancelled || len(ans.Result) == 0 {
				return json.RawMessage(`{"outcome":{"outcome":"cancelled"}}`), nil
			}
			return ans.Result, nil
		}
		return nil, acp.ErrNotOffered
	}
}

// acpTurn tells the mission a turn started (working) or ended (waiting,
// with its reply and what it left running, as its harness reads the turn
// and the process check finds), as a terminal crew's prompt and stop hooks
// do.
func (s *Server) acpTurn(spec Spec, t acp.Turn) {
	if !t.Ended() {
		s.store.CrewHook(spec.Slug, spec.CrewID, mission.Hook{Prompted: true}) //nolint:errcheck // a notice
		return
	}
	if t.Err != nil {
		return // the agent's gone: its end says so
	}
	work := crew.LeftRunning(harness.AtTurnEnd(spec.Harness, t, func() *harness.Found { return s.processCheck(spec) }))
	s.store.CrewHook(spec.Slug, spec.CrewID, mission.Hook{Activity: mission.ActivityWaiting, Reply: t.Reply, Work: work}) //nolint:errcheck // a notice
	if why, dropped := harness.TurnDropped(spec.Harness, t); dropped && s.continueDropped(spec, why) {
		return // not a turn's end to look at: it goes on
	} else if !dropped {
		if cr, ok := s.acpCrew(spec.Key()); ok {
			cr.dropped.Store(0)
		}
	}
	if s.OnACPTurnEnded != nil {
		s.OnACPTurnEnded(spec.Slug, spec.CrewID)
	}
}

// continueDropped has crew go on from a turn an error of their agent's
// cut off (why), and reports whether it will: not crew who'd said where
// they are before it (done, or asking: there's nothing to go on with),
// nor past maxDropped in a row. It's sent after a wait, unless something
// else has been sent them by then, or they've gone, and it's logged on
// the mission as what ws typed in. Whose the turn is doesn't change with
// it: crew you were talking to are still answering you.
func (s *Server) continueDropped(spec Spec, why string) bool {
	cr, ok := s.acpCrew(spec.Key())
	if !ok || cr.stopRequested.Load() {
		return false
	}
	if m, err := s.store.Load(spec.Slug); err == nil {
		if c, ok := m.CrewByID(spec.CrewID); ok && c.Status != "" {
			return false
		}
	}
	n := cr.dropped.Add(1)
	if n > maxDropped {
		s.Log.Printf("%s: its turn was cut off again (%s), %d times in a row: left as it is", spec.Key(), why, n)
		return false
	}
	text := fmt.Sprintf(droppedText, why)
	wait := droppedWait * time.Duration(n)
	s.Log.Printf("%s: its turn was cut off (%s): going on with it in %s", spec.Key(), why, wait)
	go func() {
		select {
		case <-time.After(wait):
		case <-cr.agent.Done():
			return
		}
		if st := cr.agent.State(); cr.stopRequested.Load() || st.Ended || st.Working || st.Queued > 0 {
			return // stopped, or sent something meanwhile: that goes on with it
		}
		if _, err := (crew.AgentRun{S: spec, Agent: cr.agent, Root: s.Root}).Say(text); err != nil {
			s.Log.Printf("%s: couldn't go on with its cut-off turn: %v", spec.Key(), err)
			if s.OnACPTurnEnded != nil {
				s.OnACPTurnEnded(spec.Slug, spec.CrewID) // a turn's end after all
			}
			return
		}
		s.store.AppendLog(spec.Slug, mission.Event{Kind: mission.EventResumeWith, Crew: spec.CrewID, By: "ws", Text: "typed in: " + text}) //nolint:errcheck // a log entry
	}()
	return true
}

// followACP tells the mission when the agent waits on you, and when it's
// been answered: what a terminal crew's notification and question hooks
// say.
func (s *Server) followACP(cr *acpCrew) {
	ag := cr.agent
	shown := ""
	for {
		select {
		case <-ag.Updates():
		case <-ag.Done():
			return
		}
		st := ag.State()
		if len(st.Asks) == 0 {
			if shown != "" && st.Working {
				s.store.CrewHook(cr.spec.Slug, cr.spec.CrewID, mission.Hook{Activity: mission.ActivityWorking}) //nolint:errcheck // a notice
			}
			shown = ""
			continue
		}
		a := st.Asks[0]
		if a.ID == shown {
			continue
		}
		shown = a.ID
		h := mission.Hook{Activity: mission.ActivityPermission}
		switch {
		case a.Permission != nil:
			h.Notice = cr.spec.CrewID + " wants to " + harness.AskedFor(*a.Permission)
		default:
			if q, options, ok := harness.AskQuestion(a); ok {
				h = mission.Hook{Activity: mission.ActivityQuestion, Notice: q, Options: options}
			} else {
				h.Notice = cr.spec.CrewID + " is waiting for you"
			}
		}
		s.store.CrewHook(cr.spec.Slug, cr.spec.CrewID, h) //nolint:errcheck // a notice
	}
}

// watchACP records the crew's end on its mission once the agent's gone.
func (s *Server) watchACP(cr *acpCrew, rec *acp.Recorder) {
	<-cr.agent.Done()
	exit := cr.agent.State().Exit
	s.mu.Lock()
	closing := s.closing
	s.mu.Unlock()
	if closing {
		exit = "stopped: " + exit
	}
	rec.Close()
	s.Log.Printf("%s ended: %s", cr.spec.Key(), exit)
	text := ""
	if data, err := os.ReadFile(cr.record); err == nil {
		text = harness.ACPTranscriptText(data)
	}
	if err := crew.Finish(s.store, cr.spec, exit, text, crew.HowEnded(cr.stopRequested.Load(), closing), s.OnCrewEnded); err != nil {
		s.Log.Printf("recording end of %s: %v", cr.spec.Key(), err)
	}
	s.mu.Lock()
	if s.agents[cr.spec.Key()] == cr { // not crew of the same key started since
		delete(s.agents, cr.spec.Key())
	}
	s.checkIdleLocked()
	s.mu.Unlock()
}

// stop ends the agent, recorded as stopped.
func (cr *acpCrew) stop(grace time.Duration) {
	cr.stopRequested.Store(true)
	go cr.agent.Close(grace) //nolint:errcheck // its end is recorded by watchACP
}

// Agent is the agent of crew running over ACP, for the web UI to talk to:
// send it a message, answer what it asks, interrupt its turn.
func (s *Server) Agent(key string) (*acp.Agent, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cr, ok := s.agents[key]
	if !ok {
		return nil, false
	}
	return cr.agent, true
}

// processCheck is what the process check finds that crew over ACP left
// running, the agent's process being their harness: nil when it can't be
// made.
func (s *Server) processCheck(spec Spec) *harness.Found {
	ag, ok := s.Agent(spec.Key())
	if !ok {
		return nil
	}
	read := s.ReadProcs
	if read == nil {
		read = harness.ReadProcs
	}
	procs, err := read()
	if err != nil {
		return nil
	}
	return procs.CrewWork(harness.ProcCrew{Mission: spec.Slug, Crew: spec.CrewID, Harness: ag.Client().Pid()})
}

// ACPCallDone is whether a tool call in the session of crew over ACP (of
// slug and crewID) has finished: work it left running at a turn's end has
// ended (mission.Work.Call). A call this server holds no agent for, or
// whose session doesn't have it, has too: closing the session stops what
// it ran.
func (s *Server) ACPCallDone(slug, crewID, call string) bool {
	ag, ok := s.Agent(Spec{Slug: slug, CrewID: crewID}.Key())
	if !ok {
		return true
	}
	c, ok := ag.Call(call)
	return !ok || c.Done()
}

// acpCrew finds crew over ACP by key.
func (s *Server) acpCrew(key string) (*acpCrew, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cr, ok := s.agents[key]
	return cr, ok
}
