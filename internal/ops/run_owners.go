package ops

import (
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/runs"
)

// SweepRuns ends the workspace's runs whose owner (a mission, its step,
// its crew, a conversation) has ended, or whose time is up, and removes
// what ended by itself an hour ago (runs.Store.Sweep).
func (w *Workspace) SweepRuns(now time.Time) ([]runs.Swept, error) {
	return w.runStore().Sweep(newRunOwners(w.Root()), now)
}

// runOwners says whether owners in the workspace's records have ended, for
// a sweep: each mission read once.
type runOwners struct {
	root     string
	missions map[string]*mission.Mission
	missing  map[string]bool
}

func newRunOwners(root string) *runOwners {
	return &runOwners{root: root, missions: map[string]*mission.Mission{}, missing: map[string]bool{}}
}

// mission is the mission slug, read once: gone is true when it isn't
// there at all, and m is nil (gone false) when it couldn't be read.
func (o *runOwners) mission(slug string) (m *mission.Mission, gone bool) {
	if m, ok := o.missions[slug]; ok {
		return m, false
	}
	if o.missing[slug] {
		return nil, true
	}
	m, err := mission.Store{Root: o.root}.Load(slug)
	if errors.Is(err, mission.ErrNotFound) {
		o.missing[slug] = true
		return nil, true
	}
	if err != nil {
		return nil, false
	}
	o.missions[slug] = m
	return m, false
}

func (o *runOwners) Ended(ow runs.Owner) string {
	if ow.Mission != "" {
		m, gone := o.mission(ow.Mission)
		if gone {
			return fmt.Sprintf("mission %s has gone", ow.Mission)
		}
		if m == nil {
			return ""
		}
		if m.Ending != nil || m.State == mission.Archived {
			return fmt.Sprintf("mission %s has ended", ow.Mission)
		}
		switch ow.Kind {
		case runs.ByStep:
			i := slices.IndexFunc(m.Stages, func(st mission.StageRecord) bool { return st.Text == ow.StepText })
			switch {
			case i < 0:
				return fmt.Sprintf("step %d is no longer in mission %s's plan", ow.Step, ow.Mission)
			case m.Stages[i].Skipped:
				return fmt.Sprintf("step %d of mission %s was skipped", m.Stages[i].N, ow.Mission)
			case m.Stages[i].Accepted != nil:
				return fmt.Sprintf("step %d of mission %s has passed", m.Stages[i].N, ow.Mission)
			}
		case runs.ByCrew:
			i := slices.IndexFunc(m.Crew, func(c mission.Crew) bool { return c.ID == ow.Crew })
			if i < 0 || m.Crew[i].Ended != nil {
				return fmt.Sprintf("crew %s of mission %s have closed", ow.Crew, ow.Mission)
			}
		}
	}
	if ow.Kind == runs.ByConversation {
		c, err := conversation.For(o.root, ow.Mission).Load(ow.Conversation)
		switch {
		case errors.Is(err, conversation.ErrNotFound):
			return fmt.Sprintf("conversation %s has gone", ow.Conversation)
		case err != nil:
			return ""
		case c.Status == conversation.Resolved:
			return fmt.Sprintf("conversation %s was resolved", ow.Conversation)
		}
	}
	return ""
}
