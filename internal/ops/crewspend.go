package ops

import (
	"errors"
	"io/fs"

	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/spend"
)

// What crew spend is read from their session files, which their harness
// writes and ws only reads (an ACP crew's from ws's own record of it, which
// has what the agent said each turn spent): every few minutes for the missions in play
// (the daemon's readCrewSpendLoop), and once more as a mission's crew are
// closed for its ending, so what they spent last isn't missed. A session
// file is read only where it's grown since (spend.ReadSession), and the
// harness clears them after a while: what isn't read before then is lost.

// ReadCrewSpend reads what the crew on missions in play have spent since
// it was last read. It says how many crew had spent more, and the first
// error it met, if any.
func (w *Workspace) ReadCrewSpend() (int, error) {
	missions, err := w.Missions.InPlay()
	if err != nil {
		return 0, err
	}
	n := 0
	var first error
	for _, m := range missions {
		got, err := w.readCrewSpend(m)
		n += got
		if first == nil {
			first = err
		}
	}
	return n, first
}

// readCrewSpend reads what mission m's crew have spent since it was last
// read. Crew whose harness keeps no session file ws reads (or says nothing
// of what they spend there) aren't read: what they spent isn't known.
func (w *Workspace) readCrewSpend(m *mission.Mission) (int, error) {
	n := 0
	var first error
	for _, c := range m.Crew {
		caps := harness.CapsOf(c.Harness)
		if !caps.Spends.Crew {
			continue
		}
		var path, source string
		switch {
		case caps.Transcript == harness.TranscriptACP:
			// ws's own record of the session, kept beside the mission.
			path, source = w.Missions.ACPRecordPath(m.Slug, c.ID), spend.RecordSource(m.Slug, c.ID)
		case c.Conversation != "":
			found, ok := harness.SessionFile(c.Harness, c.Conversation)
			if !ok {
				continue
			}
			path, source = found, spend.SessionSource(c.Harness, c.Conversation)
		default:
			continue
		}
		got, err := spend.ReadSession(w.Root(), path, source, spend.For{Mission: m.Slug, Kind: spend.Crew, Subject: c.ID}, c.Harness, c.Model)
		if got {
			n++
		}
		if err != nil && !errors.Is(err, fs.ErrNotExist) && first == nil {
			first = err
		}
	}
	return n, first
}
