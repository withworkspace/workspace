package crew

import (
	"errors"
	"strings"
	"time"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/withworkspace/workspace/internal/execlog"
	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/mission"
)

// Claude Code's trust dialog (harness.ReadTrustDialog) is the first thing
// crew show in a repo Claude hasn't been trusted in, and until it's
// answered no hook runs: crew on it said nothing, so they looked as if
// they were working. The host watches a crew's screen as it starts
// (WatchTrust): on the dialog, crew are marked as waiting on you for a
// permission, or, where ws.local.toml's [crew] says trust_folders, it's
// answered yes.

// TrustWait is how long after a launch the screen's watched for the dialog.
// It's drawn first if at all, so a screen that's settled without it is
// one that won't show it.
var (
	TrustWait   = 30 * time.Second
	TrustSettle = 3 * time.Second
	trustPoll   = 150 * time.Millisecond
)

// AwaitTrust watches a starting crew's screen for the trust dialog: until
// it's drawn, the screen has settled without it, or the wait's over.
func AwaitTrust(t Term, wait time.Duration) (harness.TrustDialog, bool) {
	deadline := time.Now().Add(wait)
	last, changed, drawn := "", time.Now(), false
	for time.Now().Before(deadline) && !t.Exited() {
		if d, ok := harness.ReadTrustDialog(t.RenderScrolled(0)); ok {
			return d, true
		}
		if text := t.Text(); text != last {
			last, changed = text, time.Now()
			drawn = drawn || strings.TrimSpace(text) != ""
		}
		if drawn && time.Since(changed) >= TrustSettle {
			break
		}
		time.Sleep(trustPoll)
	}
	return harness.TrustDialog{}, false
}

// Trust answers the dialog yes: down to "Yes", checked on screen, then
// Enter. Nothing's confirmed unless the cursor's seen on "Yes".
func Trust(t Term, d harness.TrustDialog) error {
	keys := d.TrustKeys()
	for _, k := range keys[:len(keys)-1] {
		t.SendKey(k)
		time.Sleep(100 * time.Millisecond)
	}
	for end := time.Now().Add(2 * time.Second); ; time.Sleep(trustPoll) {
		now, ok := harness.ReadTrustDialog(t.RenderScrolled(0))
		if !ok {
			return errors.New("the trust dialog went before it was answered")
		}
		if now.ToYes == 0 {
			break
		}
		if time.Now().After(end) {
			return errors.New("the cursor didn't reach \"Yes, I trust this folder\"")
		}
	}
	t.SendKey(uv.KeyPressEvent{Code: uv.KeyEnter})
	return nil
}

// WatchTrust is the host's watch on crew it's just launched, for the trust
// dialog: answered yes where the workspace says so, else the crew's marked
// as asking you for a permission, and marked working again once it's
// answered and they're still running. trust says whether to answer it
// (ws.local.toml's [crew] trust_folders, read as the dialog's seen); nil
// is never.
func WatchTrust(store mission.Store, root string, spec Spec, t Term, trust func() bool) {
	d, ok := AwaitTrust(t, TrustWait)
	if !ok {
		return
	}
	key := "crew " + spec.Key()
	if trust != nil && trust() {
		err := Trust(t, d)
		if err == nil {
			execlog.Note(root, key, "trusted %s in Claude Code (ws.local.toml's [crew] trust_folders)", d.Folder)
			return
		}
		execlog.Note(root, key, "couldn't answer Claude Code's trust dialog: %v", err)
	}
	folder := d.Folder
	if folder == "" {
		folder = "its folder"
	}
	notice := "Claude Code asks whether to trust " + folder + ": answer it in its terminal"
	if err := store.CrewHook(spec.Slug, spec.CrewID, mission.Hook{Activity: mission.ActivityPermission, Notice: notice}); err != nil {
		execlog.Note(root, key, "marking it on Claude Code's trust dialog: %v", err)
		return
	}
	execlog.Note(root, key, "on Claude Code's trust dialog for %s: waiting on you", d.Folder)
	for !t.Exited() {
		if _, ok := harness.ReadTrustDialog(t.RenderScrolled(0)); !ok {
			// Answered yes (no would have ended it). A prompt hook may
			// have said so already: this says no more than it would.
			store.CrewHook(spec.Slug, spec.CrewID, mission.Hook{Prompted: true, WakeUp: true})
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
}
