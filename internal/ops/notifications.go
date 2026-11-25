package ops

import (
	"fmt"

	"github.com/withworkspace/workspace/internal/config"
	"github.com/withworkspace/workspace/internal/execlog"
	"github.com/withworkspace/workspace/internal/notification"
	"github.com/withworkspace/workspace/internal/release"
)

// Notifications: what happened that you'd want to know, kept as a feed
// (internal/notification), beside what needs you (Needs), which is worked
// out and never kept. Where each source is sent is yours to say
// ([notifications] in ws.local.toml), and applied here, as the feed is
// read: a source sent nowhere is left out of it and of its count, and one
// sent to the desktop too is marked for the web UI to tell.

// Notified is a notification as the feed has it for you.
type Notified struct {
	notification.Notification
	// Desktop is whether its source is sent to the desktop as well.
	Desktop bool
}

// NotificationFeed is the feed: its notifications, newest first, and how
// many of them you haven't read.
type NotificationFeed struct {
	Notifications []Notified
	Unread        int
}

// notifyRoutes is where each source is sent, as ws.local.toml says now. A
// file that doesn't read says nothing: every source to the feed.
func notifyRoutes(root string) config.NotificationsConfig {
	l, err := config.ReadLocal(root)
	if err != nil {
		return config.NotificationsConfig{}
	}
	return l.Notifications
}

// mutedSources is the sources sent nowhere.
func mutedSources(routes config.NotificationsConfig) []string {
	var out []string
	for _, s := range notification.Sources {
		if routes.Route(s) == config.NotifyOff {
			out = append(out, s)
		}
	}
	return out
}

// Notifications is the feed: the latest limit (0: every one), only the
// unread where unread says.
func (w *Workspace) Notifications(unread bool, limit int) (NotificationFeed, error) {
	routes := notifyRoutes(w.root)
	muted := mutedSources(routes)
	list, err := notification.List(w.root, notification.Filter{Unread: unread, Muted: muted, Limit: limit})
	if err != nil {
		return NotificationFeed{}, err
	}
	out := NotificationFeed{Notifications: make([]Notified, 0, len(list))}
	for _, n := range list {
		out.Notifications = append(out.Notifications, Notified{Notification: n, Desktop: routes.Route(n.Source) == config.NotifyDesktop})
	}
	out.Unread, err = notification.Unread(w.root, muted)
	return out, err
}

// UnreadNotifications is how many you haven't read: 0 where that can't
// be said.
func (w *Workspace) UnreadNotifications() int {
	n, _ := notification.Unread(w.root, mutedSources(notifyRoutes(w.root)))
	return n
}

// yoursToRead refuses anyone but you: the feed is yours, and what you've
// read of it is yours to say.
func yoursToRead(by Actor) error {
	if by.Kind == You {
		return nil
	}
	return refused("not-yours", "notifications are the user's: only they say what they've read")
}

// NotificationsRead is how many notifications a reading, or a putting
// away, was news of.
type NotificationsRead struct {
	Count     int
	Dismissed bool
}

func (r NotificationsRead) String() string {
	what := "Read"
	if r.Dismissed {
		what = "Dismissed"
	}
	switch r.Count {
	case 0:
		return "Nothing to mark: they were " + map[bool]string{false: "read", true: "dismissed"}[r.Dismissed] + " already."
	case 1:
		return what + " 1 notification."
	}
	return fmt.Sprintf("%s %d notifications.", what, r.Count)
}

// ReadNotifications says you've read the notifications numbered ns: every
// one in the feed, where all says.
func (w *Workspace) ReadNotifications(ns []int64, all bool, by Actor) (NotificationsRead, error) {
	if err := yoursToRead(by); err != nil {
		return NotificationsRead{}, err
	}
	var count int
	var err error
	if all {
		count, err = notification.ReadAll(w.root, mutedSources(notifyRoutes(w.root)))
	} else {
		if len(ns) == 0 {
			return NotificationsRead{}, invalid("say which: a notification's number (n3), or all of them")
		}
		count, err = notification.MarkRead(w.root, ns)
	}
	return NotificationsRead{Count: count}, err
}

// DismissNotifications puts the notifications numbered ns away: read, and
// out of the feed.
func (w *Workspace) DismissNotifications(ns []int64, by Actor) (NotificationsRead, error) {
	if err := yoursToRead(by); err != nil {
		return NotificationsRead{}, err
	}
	if len(ns) == 0 {
		return NotificationsRead{}, invalid("say which: a notification's number (n3)")
	}
	count, err := notification.Dismiss(w.root, ns)
	return NotificationsRead{Count: count, Dismissed: true}, err
}

// AnnounceNotifications answers which of the notifications numbered ns a
// page is to tell the desktop of: those no page has told of yet, as
// Announce does for needs.
func (w *Workspace) AnnounceNotifications(ns []int64) ([]int64, error) {
	return notification.Claim(w.root, ns)
}

// AddNotification adds a notification by hand, as you: for a seed, to
// put in a try's feed what a try can't make happen (a merge, a release).
func (w *Workspace) AddNotification(n notification.Notification, by Actor) (notification.Notification, error) {
	if err := yoursToRead(by); err != nil {
		return n, err
	}
	out, err := notification.Add(w.root, n)
	if err != nil {
		return n, invalid("%v", err)
	}
	return out, nil
}

// notify adds a notification of ws's own, best said and never in the way.
func (w *Workspace) notify(n notification.Notification) {
	if _, err := notification.Add(w.root, n); err != nil {
		execlog.Note(w.root, "notifications", "%q isn't kept: %v", n.Text, err)
	}
}

// notifyPluginUpdates says which plugins have an update out that the look
// before didn't find: once for each release.
func (w *Workspace) notifyPluginUpdates(was, now keptUpdates, installed func(name string) string) {
	for name, kept := range now.Plugins {
		if kept.Latest == "" || kept.Latest == was.Plugins[name].Latest || !later(kept.Latest, installed(name)) {
			continue
		}
		w.notify(notification.Notification{
			Source: notification.SourcePlugins, Kind: notification.PluginUpdate,
			Text: fmt.Sprintf("%s %s is out: update it from Plugins", name, kept.Latest),
			Refs: []string{"plugin:" + name},
		})
	}
}

// NoteStarted is the daemon saying which ws it's started as, for the
// feed: a release later than the one this workspace's daemon last ran is
// an upgrade it restarted onto, and is said once. A build of a clone says
// nothing.
func (w *Workspace) NoteStarted() {
	ws := w.opts.WS
	if !isRelease(ws) {
		return
	}
	was, err := notification.Was(w.root, "ws", ws.Version)
	if err != nil || !release.IsTag(was) || !release.Newer(ws.Version, was) {
		return
	}
	w.notify(notification.Notification{
		Source: notification.SourceWS, Kind: notification.WSUpgraded,
		Text: fmt.Sprintf("ws was upgraded to %s, from %s: the daemon restarted onto it", ws.Version, was),
	})
}
