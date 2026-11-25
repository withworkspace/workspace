package web

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/events"
	"github.com/withworkspace/workspace/internal/notification"
	"github.com/withworkspace/workspace/internal/ops"
)

// Notifying is the feed of what happened (ops.Notifications): read, and
// what you've read of it said.
type Notifying interface {
	Notifications(unread bool, limit int) (ops.NotificationFeed, error)
	ReadNotifications(ns []int64, all bool, by ops.Actor) (ops.NotificationsRead, error)
	DismissNotifications(ns []int64, by ops.Actor) (ops.NotificationsRead, error)
	AnnounceNotifications(ns []int64) ([]int64, error)
}

// Notification is one thing that happened, as the feed lists it.
type Notification struct {
	// N is its number, and Ref how it's named (n12).
	N   int64     `json:"n"`
	Ref string    `json:"ref"`
	At  time.Time `json:"at"`
	// Source is where it comes from, which is what's routed: missions,
	// pulls, plugins or ws.
	Source string `json:"source"`
	// Kind is what happened (mission.ended, pr.merged, plugin.update…).
	Kind string `json:"kind"`
	// Text says it, as a sentence.
	Text string `json:"text"`
	// To is the page it opens: "" where there's none.
	To string `json:"to,omitempty"`
	// Refs is what it's about, as ws names them.
	Refs []string `json:"refs,omitempty"`
	// By is who did it: ws, the orchestrator, crew by id (crew:c1), or ""
	// where nobody here did.
	By string `json:"by,omitempty"`
	// Attention marks one that went wrong.
	Attention bool `json:"attention,omitempty"`
	Read      bool `json:"read"`
	// Desktop is whether its source is sent to the desktop as well.
	Desktop bool `json:"desktop,omitempty"`
}

// Notifications is GET /api/notifications: the feed, newest first, and how
// many of it you haven't read.
type Notifications struct {
	Notifications []Notification `json:"notifications"`
	Unread        int            `json:"unread"`
}

// NotificationsCount is the feed as the workspace's view carries it: how
// many you haven't read, and the newest's number, which moves when one is
// added, so a page showing the feed reads it again.
type NotificationsCount struct {
	Unread int   `json:"unread"`
	Latest int64 `json:"latest"`
}

// NotificationsMark is which notifications a page says you've read, or
// puts away (POST /api/notifications/read, …/dismiss): those numbered Ns,
// or, read only, All of the feed.
type NotificationsMark struct {
	Ns  []int64 `json:"ns,omitempty"`
	All bool    `json:"all,omitempty"`
}

// NotificationsAnnounce is a page asking which notifications it should
// tell the desktop of (POST /api/notifications/announce), and
// NotificationsAnnounced the answer: those no page had told of yet.
type NotificationsAnnounce struct {
	Ns []int64 `json:"ns"`
}

type NotificationsAnnounced struct {
	Fresh []int64 `json:"fresh"`
}

// NotificationsOf is the feed as the API serves it.
func NotificationsOf(f ops.NotificationFeed) Notifications {
	out := Notifications{Notifications: []Notification{}, Unread: f.Unread}
	for _, n := range f.Notifications {
		out.Notifications = append(out.Notifications, Notification{
			N: n.N, Ref: n.Ref(), At: n.At, Source: n.Source, Kind: n.Kind, Text: n.Text,
			To: notificationPlace(n.Notification), Refs: n.Refs, By: n.By,
			Attention: n.Attention, Read: n.Read != nil, Desktop: n.Desktop,
		})
	}
	return out
}

// notificationPlace is the page a notification opens: its mission's, its
// pull request's review, or Plugins.
func notificationPlace(n notification.Notification) string {
	first := ""
	if len(n.Refs) > 0 {
		first = n.Refs[0]
	}
	switch n.Kind {
	case events.MissionEnded, events.MissionStarted:
		if first != "" {
			return "/missions/" + first
		}
	case events.PRMerged:
		// A mission's pull request is the mission's to show.
		if len(n.Refs) > 1 {
			return "/missions/" + n.Refs[1]
		}
		if at := strings.LastIndex(first, "#"); at > 0 {
			return "/reviews/" + first[:at] + "/" + first[at+1:]
		}
	case notification.PluginUpdate:
		return "/settings/plugins"
	case notification.WSUpgraded:
		return "/settings/about"
	}
	return ""
}

// notificationsCount is the feed's count for the workspace's view.
func notificationsCount(a Notifying) NotificationsCount {
	f, err := a.Notifications(false, 1)
	if err != nil {
		return NotificationsCount{}
	}
	out := NotificationsCount{Unread: f.Unread}
	if len(f.Notifications) > 0 {
		out.Latest = f.Notifications[0].N
	}
	return out
}

func (s *Server) notifications(w http.ResponseWriter, r *http.Request) {
	f, err := s.App.Notifications(r.URL.Query().Get("unread") != "", 200)
	if err != nil {
		writeError(w, err, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, NotificationsOf(f))
}

// markNotifications is a page saying you've read some, or putting them
// away: it answers with the feed as it is then.
func (s *Server) markNotifications(dismiss bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req NotificationsMark
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, http.StatusBadRequest, "which notifications didn't read: "+err.Error())
			return
		}
		var err error
		if dismiss {
			_, err = s.App.DismissNotifications(req.Ns, ops.AsYou)
		} else {
			_, err = s.App.ReadNotifications(req.Ns, req.All, ops.AsYou)
		}
		if err != nil {
			writeError(w, err, http.StatusInternalServerError)
			return
		}
		s.notifications(w, r)
	}
}

// announceNotifications answers a page that would tell the desktop of
// notifications new to it: which of them no page has told of yet.
func (s *Server) announceNotifications(w http.ResponseWriter, r *http.Request) {
	var req NotificationsAnnounce
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "what to announce didn't read: "+err.Error())
		return
	}
	fresh, err := s.App.AnnounceNotifications(req.Ns)
	if err != nil {
		writeError(w, err, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, NotificationsAnnounced{Fresh: fresh})
}
