package ops

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/withworkspace/plugin/protocol"
	"github.com/withworkspace/workspace/internal/notification"
	"github.com/withworkspace/workspace/internal/plugin"
)

// Where a source is sent is applied as the feed is read: one sent nowhere
// is out of it and its count, one sent to the desktop is marked, and
// nothing's lost by changing your mind.
func TestNotificationsAsRouted(t *testing.T) {
	w, _ := fixture(t, "")
	notification.Add(w.Root(), notification.Notification{Source: notification.SourceMissions, Text: "A mission ended"})
	notification.Add(w.Root(), notification.Notification{Source: notification.SourcePlugins, Kind: notification.PluginUpdate, Text: "jira v1.2.0 is out"})

	feed, err := w.Notifications(false, 0)
	if err != nil || len(feed.Notifications) != 2 || feed.Unread != 2 || feed.Notifications[0].Desktop {
		t.Fatalf("every source to the feed, unless said: %+v, %v", feed, err)
	}

	local := filepath.Join(w.Root(), "ws.local.toml")
	os.WriteFile(local, []byte("[notifications]\nplugins = \"off\"\nmissions = \"desktop\"\n"), 0o644)
	feed, _ = w.Notifications(false, 0)
	if len(feed.Notifications) != 1 || feed.Unread != 1 || !feed.Notifications[0].Desktop {
		t.Errorf("plugins off, missions to the desktop: %+v", feed)
	}
	if n := w.UnreadNotifications(); n != 1 {
		t.Errorf("unread: %d", n)
	}
	if done, err := w.ReadNotifications(nil, true, AsYou); err != nil || done.Count != 1 {
		t.Errorf("read all: %+v, %v", done, err)
	}

	os.WriteFile(local, nil, 0o644)
	if feed, _ = w.Notifications(true, 0); len(feed.Notifications) != 1 || feed.Notifications[0].Source != notification.SourcePlugins {
		t.Errorf("unmuted, it's there and unread: %+v", feed)
	}
}

// The feed is yours: crew and the orchestrator don't say what you've read.
func TestNotificationsAreYoursToRead(t *testing.T) {
	w, _ := fixture(t, "")
	notification.Add(w.Root(), notification.Notification{Source: notification.SourceMissions, Text: "A mission ended"})
	_, err := w.ReadNotifications([]int64{1}, false, AsCrew("c1"))
	refusedWith(t, err, "not-yours")
	_, err = w.DismissNotifications([]int64{1}, AsOrchestrator)
	refusedWith(t, err, "not-yours")
	if _, err := w.ReadNotifications(nil, false, AsYou); err == nil {
		t.Error("reading none is refused")
	}
	if done, err := w.DismissNotifications([]int64{1}, AsYou); err != nil || done.Count != 1 || done.String() != "Dismissed 1 notification." {
		t.Errorf("dismissed: %+v, %v", done, err)
	}
}

// A plugin tells of what it saw as one of the kinds it registered: kept
// once for its key, routed kind by kind (where it said until you say),
// opening the thing it's about, and its kinds yours to set in Settings.
func TestAPluginsNotifications(t *testing.T) {
	w, _ := fixture(t, "")
	root := w.Root()
	if err := os.WriteFile(filepath.Join(root, "ws.toml"), []byte("[workspace]\norg = \"acme\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	installed(t, root, "jira", jiraIssue)
	registered(t, root, "jira", plugin.Registration{Refs: []protocol.Ref{jiraIssue}, Description: "A Jira board", Notifies: []protocol.Notifies{
		{ID: "assigned", Title: "Given to you", About: "Someone gives you an issue."},
		{ID: "new", Title: "New on the board", Default: protocol.RouteOff},
	}})
	ask := func(in protocol.NotificationAddParams) (protocol.NotificationAdded, error) {
		params, _ := json.Marshal(in)
		got, err := w.AnswerPlugin(context.Background(), "jira", protocol.MethodNotificationAdd, params)
		if err != nil {
			return protocol.NotificationAdded{}, err
		}
		return got.(protocol.NotificationAdded), nil
	}

	if got, err := ask(protocol.NotificationAddParams{Kind: "assigned", Text: "Ana gave you RAM-1", Refs: []string{"RAM-1"}, Key: "k1"}); err != nil || !got.Added || got.Muted || got.N != 1 {
		t.Fatalf("the first: %+v, %v", got, err)
	}
	if got, _ := ask(protocol.NotificationAddParams{Kind: "assigned", Text: "Ana gave you RAM-1", Key: "k1"}); got.Added || got.N != 1 {
		t.Errorf("the same key again: %+v", got)
	}
	if got, _ := ask(protocol.NotificationAddParams{Kind: "new", Text: "RAM-2 is new", Refs: []string{"RAM-2"}}); !got.Added || !got.Muted {
		t.Errorf("a kind the plugin sends nowhere by default is kept, muted: %+v", got)
	}
	for _, bad := range []protocol.NotificationAddParams{{Kind: "moved", Text: "x"}, {Kind: "assigned", Text: " "}} {
		if _, err := ask(bad); err == nil {
			t.Errorf("%+v was kept", bad)
		}
	}

	feed, _ := w.Notifications(false, 0)
	if len(feed.Notifications) != 1 || feed.Unread != 1 {
		t.Fatalf("only the kind not off: %+v", feed)
	}
	if n := feed.Notifications[0]; n.Source != "plugin:jira" || n.Kind != "jira:assigned" || n.Opens != "/x/jira/issue/RAM-1" || n.Desktop {
		t.Errorf("as listed: %+v", n)
	}

	// Yours: new to the feed, assigned to the desktop as well.
	if _, err := w.SetSettings([]SettingChange{
		{Section: "notifications.plugin.jira", Key: "new", Value: "feed"},
		{Section: "notifications.plugin.jira", Key: "assigned", Value: "desktop"},
	}); err != nil {
		t.Fatal(err)
	}
	feed, _ = w.Notifications(false, 0)
	if len(feed.Notifications) != 2 || !feed.Notifications[1].Desktop || feed.Notifications[0].Desktop {
		t.Errorf("as you route them: %+v", feed)
	}
	data, _ := os.ReadFile(filepath.Join(root, "ws.local.toml"))
	if !strings.Contains(string(data), "[notifications.plugin.jira]") {
		t.Errorf("ws.local.toml:\n%s", data)
	}

	info, err := w.Settings()
	if err != nil {
		t.Fatal(err)
	}
	var group *SettingsGroup
	for i := range info.Groups {
		if info.Groups[i].ID == "notifications.plugin.jira" {
			group = &info.Groups[i]
		}
	}
	if group == nil || group.Title != "jira" || len(group.Settings) != 2 || group.About != "A Jira board." {
		t.Fatalf("jira's group: %+v", group)
	}
	if s := group.Settings[1]; s.Label != "New on the board" || s.Default != "off" || s.Value != "feed" || s.Source != "yours" {
		t.Errorf("new, set by you: %+v", s)
	}
	if _, err := w.SetSettings([]SettingChange{{Section: "notifications.plugin.jira", Key: "new", Clear: true}}); err != nil {
		t.Fatal(err)
	}
	if feed, _ = w.Notifications(false, 0); len(feed.Notifications) != 1 {
		t.Errorf("cleared, new is off again, as the plugin said: %+v", feed)
	}
}
