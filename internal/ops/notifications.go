package ops

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/withworkspace/plugin/protocol"
	"github.com/withworkspace/workspace/internal/config"
	"github.com/withworkspace/workspace/internal/execlog"
	"github.com/withworkspace/workspace/internal/notification"
	"github.com/withworkspace/workspace/internal/plugin"
	"github.com/withworkspace/workspace/internal/ref"
	"github.com/withworkspace/workspace/internal/release"
)

// Notifications: what happened that you'd want to know, kept as a feed
// (internal/notification), beside what needs you (Needs), which is worked
// out and never kept. Where each source is sent is yours to say
// ([notifications] in ws.local.toml), and applied here, as the feed is
// read: a source sent nowhere is left out of it and of its count, and one
// to be told of as it happens (desktop) is marked for the web UI to
// tell: in a toast, or on the desktop while the page is hidden.

// Notified is a notification as the feed has it for you.
type Notified struct {
	notification.Notification
	// Desktop is whether you're told of it as it happens ("Feed, and
	// tell me"): in the page, or on the desktop while it's hidden.
	Desktop bool
	// Opens is the page a plugin's opens: its first ref's, one of the
	// plugin's things in its app. "" for ws's own, whose page is said by
	// its kind.
	Opens string
}

// NotificationFeed is the feed: its notifications, newest first, and how
// many of them you haven't read.
type NotificationFeed struct {
	Notifications []Notified
	Unread        int
}

// routing is where each source, and each plugin's kind, is sent: as
// ws.local.toml says now, and where it doesn't, as each plugin said of its
// kinds when it last started.
type routing struct {
	said config.NotificationsConfig
	// kinds is each installed plugin's kinds of notification, by name.
	kinds map[string][]protocol.Notifies
}

// notifyRoutes is where each source is sent. A file that doesn't read
// says nothing: every source to the feed, each plugin's kinds where the
// plugin said.
func notifyRoutes(root string) routing {
	out := routing{kinds: pluginNotifies(root)}
	if l, err := config.ReadLocal(root); err == nil {
		out.said = l.Notifications
	}
	return out
}

// pluginNotifies is the kinds of notification each installed plugin
// registered, when it last started: none of one whose kinds don't hold.
func pluginNotifies(root string) map[string][]protocol.Notifies {
	out := map[string][]protocol.Notifies{}
	for _, name := range plugin.Installed(root) {
		reg, ok := plugin.ReadRegistration(root, name)
		if !ok || len(reg.Notifies) == 0 || protocol.NotifiesProblem(reg.Notifies) != "" {
			continue
		}
		out[name] = reg.Notifies
	}
	return out
}

// pluginRoute is where the plugin's kind (its id) is sent.
func (r routing) pluginRoute(name, id string) string {
	def := ""
	if k, ok := protocol.NotifiesKind(r.kinds[name], id); ok {
		def = k.Default
	}
	return r.said.PluginRoute(name, id, def)
}

// route is where a notification is sent: by its source, or for a
// plugin's, by its kind.
func (r routing) route(n notification.Notification) string {
	if name, ok := notification.PluginOf(n.Source); ok {
		return r.pluginRoute(name, strings.TrimPrefix(n.Kind, name+":"))
	}
	return r.said.Route(n.Source)
}

// muted is the sources sent nowhere, and the plugins' kinds.
func (r routing) muted() []string {
	var out []string
	for _, s := range notification.Sources {
		if r.said.Route(s) == config.NotifyOff {
			out = append(out, s)
		}
	}
	for name, kinds := range r.kinds {
		for _, k := range kinds {
			if r.pluginRoute(name, k.ID) == config.NotifyOff {
				out = append(out, notification.PluginKind(name, k.ID))
			}
		}
	}
	return out
}

// Notifications is the feed: the latest limit (0: every one), only the
// unread where unread says.
func (w *Workspace) Notifications(unread bool, limit int) (NotificationFeed, error) {
	routes := notifyRoutes(w.root)
	muted := routes.muted()
	list, err := notification.List(w.root, notification.Filter{Unread: unread, Muted: muted, Limit: limit})
	if err != nil {
		return NotificationFeed{}, err
	}
	out := NotificationFeed{Notifications: make([]Notified, 0, len(list))}
	for _, n := range list {
		it := Notified{Notification: n, Desktop: routes.route(n) == config.NotifyDesktop}
		if _, ok := notification.PluginOf(n.Source); ok && len(n.Refs) > 0 {
			it.Opens = w.thingPage(n.Refs[0])
		}
		out.Notifications = append(out.Notifications, it)
	}
	out.Unread, err = notification.Unread(w.root, muted)
	return out, err
}

// UnreadNotifications is how many you haven't read: 0 where that can't
// be said.
func (w *Workspace) UnreadNotifications() int {
	n, _ := notification.Unread(w.root, notifyRoutes(w.root).muted())
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
		return "Nothing to mark: all " + map[bool]string{false: "read", true: "dismissed"}[r.Dismissed] + " already."
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
		count, err = notification.ReadAll(w.root, notifyRoutes(w.root).muted())
	} else {
		if len(ns) == 0 {
			return NotificationsRead{}, invalid("name a notification (n3), or all")
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
		return NotificationsRead{}, invalid("name a notification (n3)")
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
			Text: fmt.Sprintf("%s %s is out: update from Plugins", name, kept.Latest),
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
		Text: fmt.Sprintf("Workspace upgraded to %s, from %s", ws.Version, was),
	})
}

// thingPage is the page one of a plugin's things opens: in its plugin's
// app, or where it is outside ws. "" for a name that isn't one.
func (w *Workspace) thingPage(name string) string {
	r, kept, ok := w.ThingNamed(name)
	if !ok {
		return ""
	}
	if kept.Path != "" {
		return "/x/" + kept.Plugin + strings.Replace(kept.Path, ref.Placeholder, url.PathEscape(r.ID), 1)
	}
	return r.URL
}

// pluginNotifies keeps a notification plugin who sends (notification/add):
// one of the kinds it registered, in the feed under its name, once for
// its key. One of a kind sent nowhere is kept all the same, and said
// muted.
func (w *Workspace) pluginNotifies(who string, in protocol.NotificationAddParams) (protocol.NotificationAdded, error) {
	routes := notifyRoutes(w.root)
	if _, ok := protocol.NotifiesKind(routes.kinds[who], in.Kind); !ok {
		return protocol.NotificationAdded{}, plugin.InvalidParams(fmt.Errorf("%s registered no kind of notification %q", who, in.Kind))
	}
	if strings.TrimSpace(in.Text) == "" {
		return protocol.NotificationAdded{}, plugin.InvalidParams(errors.New("the notification says nothing"))
	}
	n, added, err := notification.AddOnce(w.root, notification.Notification{
		At: in.At, Source: notification.PluginSource(who), Kind: notification.PluginKind(who, in.Kind),
		Text: strings.TrimSpace(in.Text), Refs: in.Refs, Key: in.Key, Attention: in.Attention,
	})
	if err != nil {
		return protocol.NotificationAdded{}, err
	}
	return protocol.NotificationAdded{N: n.N, Added: added, Muted: routes.route(n) == config.NotifyOff}, nil
}
