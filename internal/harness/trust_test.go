package harness

import (
	"os"
	"testing"

	uv "github.com/charmbracelet/ultraviolet"
)

// A real screen (claude 2.1.326, as the daemon keeps it): the trust dialog
// in a folder it hadn't been trusted in, "No, exit" selected.
func TestReadRealTrustScreen(t *testing.T) {
	data, err := os.ReadFile("testdata/claude-trust-screen.txt")
	if err != nil {
		t.Fatal(err)
	}
	d, ok := ReadTrustDialog(string(data))
	if !ok || d.ToYes != 1 || d.Folder != "/tmp/ws-try/t1a2b3c4d/repos/demo/greet-by-name" {
		t.Fatalf("%+v %v", d, ok)
	}
	keys := d.TrustKeys()
	if len(keys) != 2 || keys[0].Code != uv.KeyDown || keys[1].Code != uv.KeyEnter {
		t.Errorf("keys %+v", keys)
	}
	// Not a permission menu, so answering one can't land on it.
	if _, ok := ReadPrompt(string(data)); ok {
		t.Error("ReadPrompt read the trust dialog as a numbered menu")
	}
}

func TestReadTrustDialog(t *testing.T) {
	for _, c := range []struct {
		name, screen string
		ok           bool
		toYes        int
	}{
		{"yes first, selected", "Accessing workspace:\n/w\nIs this a project you created or one you trust?\n❯ Yes, I trust this folder\n  No, exit\n", true, 0},
		{"yes below", "Is this a project you created or one you trust?\n❯ No, exit\n  Yes, I trust this folder\n", true, 1},
		{"cursor below yes", "Is this a project you created or one you trust?\n  Yes, I trust this folder\n❯ No, exit\n", true, -1},
		{"no question", "❯ No, exit\n  Yes, I trust this folder\n", false, 0},
		{"no cursor", "Is this a project you created or one you trust?\n  No, exit\n  Yes, I trust this folder\n", false, 0},
		{"a session", "> fix the tests\n  I trust this folder is fine\n", false, 0},
	} {
		d, ok := ReadTrustDialog(c.screen)
		if ok != c.ok || (ok && d.ToYes != c.toYes) {
			t.Errorf("%s: %+v %v", c.name, d, ok)
		}
	}
	if k := (TrustDialog{ToYes: -1}).TrustKeys(); len(k) != 2 || k[0].Code != uv.KeyUp {
		t.Errorf("up: %+v", k)
	}
}
