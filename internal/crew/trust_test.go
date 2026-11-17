package crew

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/mission"
)

// trustScript draws the real trust dialog (as the daemon renders it), then
// waits for Down and Enter, as Claude Code would: the dialog's redrawn with
// the cursor on "Yes", then cleared once it's confirmed.
func trustScript(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("../harness/testdata/claude-trust-screen.txt")
	if err != nil {
		t.Fatal(err)
	}
	plain := harness.PlainScreen(string(data))
	yes := strings.Replace(strings.Replace(plain, "❯ No, exit", "  No, exit", 1), "  Yes, I trust", "❯ Yes, I trust", 1)
	dir := t.TempDir()
	os.WriteFile(dir+"/no.txt", []byte(plain), 0o644)
	os.WriteFile(dir+"/yes.txt", []byte(yes), 0o644)
	// Raw mode: keys arrive as they're pressed. Down is ESC [ B.
	return `stty -icanon -echo min 1; clear; cat '` + dir + `/no.txt'
key=$(dd bs=3 count=1 2>/dev/null)
clear; cat '` + dir + `/yes.txt'
dd bs=1 count=1 >/dev/null 2>&1
clear; echo "trusted, working"; sleep 5`
}

func trustFixture(t *testing.T, script string, trust bool) (*LocalHost, mission.Store, string) {
	t.Helper()
	h, store, slug, _ := localFixture(t)
	h.TrustFolders = func() bool { return trust }
	_, err := Start(Request{Store: store, Host: h, Root: store.Root, Slug: slug, Harness: "shell",
		Place: Place{Dir: store.Root, Capsule: "demo/a"}, Dispatch: "go", By: "you",
		Width: 100, Height: 30, Prepare: func() (harness.Launch, error) {
			return harness.Launch{Args: []string{"/bin/sh", "-c", script}}, nil
		}})
	if err != nil {
		t.Fatal(err)
	}
	return h, store, slug
}

func waitFor(t *testing.T, what string, ok func() bool) {
	t.Helper()
	for end := time.Now().Add(10 * time.Second); time.Now().Before(end); time.Sleep(50 * time.Millisecond) {
		if ok() {
			return
		}
	}
	t.Fatalf("never: %s", what)
}

// Crew on the trust dialog are asking you for a permission, not working;
// once it's answered, they're working.
func TestWatchTrust_MarksCrewWaitingOnYou(t *testing.T) {
	h, store, slug := trustFixture(t, trustScript(t), false)
	crew := func() mission.Crew { m, _ := store.Load(slug); return m.Crew[0] }
	waitFor(t, "c1 marked on the trust dialog", func() bool { return crew().Activity == mission.ActivityPermission })
	c := crew()
	if !strings.Contains(c.Notice, "trust /tmp/ws-try/t1a2b3c4d/repos/demo/greet-by-name") {
		t.Errorf("notice %q", c.Notice)
	}
	m, _ := store.Load(slug)
	if st, _ := (mission.Snapshot{M: m}).Crew(c); st != mission.CrewPermission {
		t.Errorf("state %s", st)
	}
	// Nothing was typed: it's still on "No".
	term, _ := h.Term(slug + "/c1")
	if d, ok := harness.ReadTrustDialog(term.RenderScrolled(0)); !ok || d.ToYes != 1 {
		t.Fatalf("dialog %+v %v", d, ok)
	}
	// You answer it, in its terminal.
	for _, k := range d0(t, term).TrustKeys() {
		term.SendKey(k)
		time.Sleep(100 * time.Millisecond)
	}
	waitFor(t, "c1 working once it's answered", func() bool { return crew().Activity == mission.ActivityWorking })
	if c := crew(); c.Notice != "" {
		t.Errorf("notice left: %q", c.Notice)
	}
}

func d0(t *testing.T, term Term) harness.TrustDialog {
	d, ok := harness.ReadTrustDialog(term.RenderScrolled(0))
	if !ok {
		t.Fatal("no dialog")
	}
	return d
}

// Where the workspace says to trust folders, the host answers it yes itself.
func TestWatchTrust_TrustsWhereTheWorkspaceSays(t *testing.T) {
	h, store, slug := trustFixture(t, trustScript(t), true)
	term, _ := h.Term(slug + "/c1")
	waitFor(t, "the dialog answered", func() bool { return strings.Contains(term.Text(), "trusted, working") })
	m, _ := store.Load(slug)
	if a := m.Crew[0].Activity; a == mission.ActivityPermission {
		t.Errorf("marked as asking, though it was answered")
	}
}

// Crew whose screen settles without it aren't touched.
func TestAwaitTrust_NoDialog(t *testing.T) {
	old := TrustSettle
	TrustSettle = 300 * time.Millisecond
	t.Cleanup(func() { TrustSettle = old })
	h, _, slug := trustFixture(t, `echo "> ready"; sleep 5`, false)
	term, _ := h.Term(slug + "/c1")
	start := time.Now()
	if _, ok := AwaitTrust(term, 5*time.Second); ok {
		t.Error("found a dialog")
	}
	if time.Since(start) > 3*time.Second {
		t.Errorf("waited %s for a screen that had settled", time.Since(start))
	}
}
