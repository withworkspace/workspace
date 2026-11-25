package ops

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/withworkspace/workspace/internal/notification"
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
