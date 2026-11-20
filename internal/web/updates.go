package web

import (
	"context"
	"net/http"
	"time"

	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/release"
)

// Updates on Settings' About: which ws this is, the latest release as the
// daemon last found it (it looks once a day, and when it's asked here),
// and installing it in place of the one that's running, as ws upgrade
// does. What's out for each plugin is on the Plugins page (plugins.go).

// Updating is looking for later releases of ws and of the plugins
// installed from a repo, and upgrading ws.
type Updating interface {
	Updates() ops.Updates
	CheckUpdates(ctx context.Context) ops.Updates
	UpgradeWS(ctx context.Context, by ops.Actor) (release.Upgraded, error)
	LaterWS() string
}

// UpdatesInfo is GET /api/updates, and what POST /api/updates/check
// answers once it's looked.
type UpdatesInfo struct {
	// Checks is whether the daemon looks by itself, once a day.
	Checks bool `json:"checks"`
	// Checked is when a look last answered, and Tried when one was last
	// made: neither, for never.
	Checked *time.Time `json:"checked,omitempty"`
	Tried   *time.Time `json:"tried,omitempty"`
	// Version is the ws that's running, and Release whether it's a
	// release's build: the kind Upgrade replaces.
	Version string `json:"version"`
	Release bool   `json:"release"`
	// Latest is the latest release, as last found, and Newer whether it's
	// later than Version. Error is why the last look didn't find it.
	Latest string `json:"latest,omitempty"`
	Newer  bool   `json:"newer"`
	Error  string `json:"error,omitempty"`
	// Page is where the latest release is described, to read what's in it.
	Page string `json:"page,omitempty"`
	// Installed is a ws put in the daemon's place that it isn't running
	// yet: it restarts onto it once nothing's at work, and Waits is what
	// is.
	Installed string   `json:"installed,omitempty"`
	Waits     []string `json:"waits,omitempty"`
	// Plugins counts the plugins a later release is out for.
	Plugins int `json:"plugins"`
}

// UpdatesInfoOf is what a look found, as the API serves it.
func UpdatesInfoOf(up ops.Updates) UpdatesInfo {
	out := UpdatesInfo{
		Checks: up.Checks, Checked: timeOrNil(up.Checked), Tried: timeOrNil(up.Tried),
		Version: up.WS.Version, Release: up.WS.Release, Latest: up.WS.Latest, Newer: up.WS.Newer, Error: up.WS.Error,
		Page: up.WS.Page, Installed: up.WS.Installed, Waits: up.WS.Waits,
	}
	for _, p := range up.Plugins {
		if p.Newer {
			out.Plugins++
		}
	}
	return out
}

func timeOrNil(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

// Upgraded is what POST /api/upgrade did: the ws that was running, the
// release looked at, and whether it was installed (and the old one kept,
// to put back with ws upgrade --rollback). Not installed, To is what's
// running already (Current), or older than it (Older).
type Upgraded struct {
	From      string `json:"from"`
	To        string `json:"to"`
	Installed bool   `json:"installed"`
	Kept      bool   `json:"kept"`
	Current   bool   `json:"current"`
	Older     bool   `json:"older"`
}

func (s *Server) updates(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, UpdatesInfoOf(s.App.Updates()))
}

// checkUpdates is POST /api/updates/check: look now, and say what was
// found. GitHub is asked, so it may take a moment.
func (s *Server) checkUpdates(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, UpdatesInfoOf(s.App.CheckUpdates(r.Context())))
}

// upgrade is POST /api/upgrade: the latest release of ws, installed in
// place of the one the daemon runs. The daemon restarts onto it by itself
// once nothing's at work.
func (s *Server) upgrade(w http.ResponseWriter, r *http.Request) {
	got, err := s.App.UpgradeWS(r.Context(), ops.AsYou)
	if err != nil {
		writeError(w, err, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, Upgraded{From: got.From, To: got.To, Installed: got.Installed, Kept: got.Kept, Current: got.Current, Older: got.Older})
}
