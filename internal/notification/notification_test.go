package notification

import (
	"testing"

	"github.com/withworkspace/workspace/internal/events"
	"github.com/withworkspace/workspace/internal/store/storetest"
)

// What this ws did for you is a notification as it's kept; what you did
// yourself isn't told back to you, and most events are nobody's news.
func TestMadeFromEvents(t *testing.T) {
	root := t.TempDir()
	storetest.Own(t, root)
	for _, e := range []events.Event{
		{Kind: events.MissionEnded, By: events.ByYou, Mission: "cache", Text: "landed"},
		{Kind: events.MissionEnded, By: events.ByWS, Mission: "cache", Text: "landed, with all of Z met"},
		{Kind: events.MissionStarted, By: events.ByCrew("c2"), Mission: "flaky-test", Text: "The export test is flaky"},
		{Kind: events.PRMerged, By: events.ByWS, Mission: "cache", PR: "demo#12", Text: "demo#12 merged"},
		{Kind: events.CapsuleBurned, By: events.ByWS, Repo: "demo", Capsule: "x"},
	} {
		if _, err := events.Emit(root, e); err != nil {
			t.Fatal(err)
		}
	}
	got, err := List(root, Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("the feed: %+v", got)
	}
	// Newest first.
	if n := got[0]; n.Source != SourcePulls || n.Text != "demo#12 merged" || len(n.Refs) != 2 || n.Refs[0] != "demo#12" || n.Event != 4 {
		t.Errorf("a merged pull request: %+v", n)
	}
	if n := got[1]; n.Source != SourceMissions || n.Text != "Crew c2 started a mission: The export test is flaky" {
		t.Errorf("a mission started for you: %+v", n)
	}
	if n := got[2]; n.Text != "Mission cache ended: landed, with all of Z met" || n.Ref() != "n1" || n.Read != nil {
		t.Errorf("a mission that ended by itself: %+v", n)
	}

	// What it only heard of is nobody's to be told of: a teammate's pull
	// request merging isn't one this ws opened.
	if _, err := events.Emit(root, events.Event{Kind: events.PRMerged, Origin: events.Heard, By: events.ByPerson("brendan"), PR: "demo#9"}); err != nil {
		t.Fatal(err)
	}
	if after, _ := List(root, Filter{}); len(after) != len(got) {
		t.Errorf("a heard merge made a notification: %d, was %d", len(after), len(got))
	}
}

func TestReadDismissAndMuted(t *testing.T) {
	root := t.TempDir()
	storetest.Own(t, root)
	for _, n := range []Notification{
		{Source: SourceMissions, Text: "one"},
		{Source: SourcePlugins, Kind: PluginUpdate, Text: "two"},
		{Source: SourceWS, Kind: WSUpgraded, Text: "three"},
	} {
		if _, err := Add(root, n); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := Add(root, Notification{Source: "nowhere", Text: "x"}); err == nil {
		t.Error("an unknown source is refused")
	}
	if n, _ := Unread(root, nil); n != 3 {
		t.Fatalf("unread: %d", n)
	}
	if n, _ := Unread(root, []string{SourcePlugins}); n != 2 {
		t.Errorf("unread, but for the muted: %d", n)
	}
	if done, err := MarkRead(root, []int64{1, 1, 9}); err != nil || done != 1 {
		t.Errorf("read one: %d, %v", done, err)
	}
	if done, _ := MarkRead(root, []int64{1}); done != 0 {
		t.Errorf("read twice is no news: %d", done)
	}
	got, _ := List(root, Filter{Unread: true})
	if len(got) != 2 || got[0].N != 3 {
		t.Errorf("the unread: %+v", got)
	}
	// Reading all leaves a muted source's as they were.
	if done, _ := ReadAll(root, []string{SourcePlugins}); done != 1 {
		t.Errorf("read all but the muted: %d", done)
	}
	if n, _ := Unread(root, nil); n != 1 {
		t.Errorf("the muted one is still unread: %d", n)
	}
	if done, _ := Dismiss(root, []int64{2}); done != 1 {
		t.Errorf("dismissed: %d", done)
	}
	got, _ = List(root, Filter{})
	if len(got) != 2 {
		t.Errorf("a dismissed one is out of the feed: %+v", got)
	}
	got, _ = List(root, Filter{Dismissed: true})
	if len(got) != 3 || got[1].Dismissed == nil || got[1].Read == nil {
		t.Errorf("and kept, read: %+v", got)
	}
}

// The desktop is told of a notification once, however many pages ask, and
// never of one that's been read.
func TestClaimOnce(t *testing.T) {
	root := t.TempDir()
	storetest.Own(t, root)
	Add(root, Notification{Source: SourceMissions, Text: "one"})
	Add(root, Notification{Source: SourceMissions, Text: "two"})
	MarkRead(root, []int64{2})
	fresh, err := Claim(root, []int64{1, 2})
	if err != nil || len(fresh) != 1 || fresh[0] != 1 {
		t.Fatalf("first asked: %v, %v", fresh, err)
	}
	if fresh, _ := Claim(root, []int64{1, 2}); len(fresh) != 0 {
		t.Errorf("asked again: %v", fresh)
	}
}

func TestWas(t *testing.T) {
	root := t.TempDir()
	storetest.Own(t, root)
	if was, err := Was(root, "ws", "v1.0.0"); err != nil || was != "" {
		t.Fatalf("nothing remembered yet: %q, %v", was, err)
	}
	if was, _ := Was(root, "ws", "v1.1.0"); was != "v1.0.0" {
		t.Errorf("what it was: %q", was)
	}
	if was, _ := Was(root, "ws", "v1.1.0"); was != "v1.1.0" {
		t.Errorf("and is now: %q", was)
	}
}

// A plugin's notification is its own source's, kept once for its key,
// and muted by its kind as well as by its source.
func TestAPluginsNotifications(t *testing.T) {
	root := t.TempDir()
	storetest.Own(t, root)
	jira := PluginSource("jira")
	add := func(kind, key, text string) (Notification, bool) {
		t.Helper()
		n, added, err := AddOnce(root, Notification{Source: jira, Kind: PluginKind("jira", kind), Key: key, Text: text, Refs: []string{"RAM-1"}})
		if err != nil {
			t.Fatal(err)
		}
		return n, added
	}
	first, added := add("assigned", "k1", "Ana gave you RAM-1")
	if !added || first.N != 1 || first.Kind != "jira:assigned" {
		t.Errorf("the first: %+v, added %v", first, added)
	}
	if again, added := add("assigned", "k1", "Ana gave you RAM-1 again"); added || again.N != 1 || again.Text != "Ana gave you RAM-1" {
		t.Errorf("the same key again is the one kept: %+v, added %v", again, added)
	}
	// Another source's key is its own.
	if _, added, _ := AddOnce(root, Notification{Source: PluginSource("sketch"), Kind: "sketch:shared", Key: "k1", Text: "x"}); !added {
		t.Error("another plugin's k1 wasn't kept")
	}
	add("new", "", "RAM-2 is new")
	add("new", "", "RAM-3 is new") // no key: kept every time
	if n, _ := Unread(root, nil); n != 4 {
		t.Errorf("unread: %d", n)
	}
	if n, _ := Unread(root, []string{"jira:new"}); n != 2 {
		t.Errorf("unread with jira:new muted: %d", n)
	}
	if n, _ := Unread(root, []string{jira}); n != 1 {
		t.Errorf("unread with jira muted: %d", n)
	}
	if got, _ := List(root, Filter{Muted: []string{"jira:new", "jira:assigned"}}); len(got) != 1 || got[0].Source != "plugin:sketch" {
		t.Errorf("listed with jira's kinds muted: %+v", got)
	}
	if n, _ := ReadAll(root, []string{"jira:new"}); n != 2 {
		t.Errorf("reading all but jira:new read %d", n)
	}
	if name, ok := PluginOf(jira); !ok || name != "jira" {
		t.Errorf("PluginOf(%q) = %q, %v", jira, name, ok)
	}
	if _, ok := PluginOf(SourcePlugins); ok {
		t.Error("plugins (updates) is ws's own source")
	}
}
