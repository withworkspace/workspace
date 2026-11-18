package cli

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

// --stdin-from gives a command its standard input from a file: what a
// conversation's agent wrote for it, set aside by ws's guard, which is
// removed once it's open. Any other file is read and left.
func TestStdinFrom(t *testing.T) {
	was := os.Stdin
	t.Cleanup(func() { os.Stdin, stdinFrom = was, "" })
	dir := t.TempDir()
	aside := filepath.Join(dir, "claude", "stdin", "abc123")
	other := filepath.Join(dir, "notes.md")
	os.MkdirAll(filepath.Dir(aside), 0o755)
	for path, body := range map[string]string{aside: "## Goal\nFaster tests.\n", other: "mine\n"} {
		os.WriteFile(path, []byte(body), 0o600)
	}
	read := func(path string) string {
		t.Helper()
		stdinFrom = path
		useStdinFrom()
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	if got := read(aside); got != "## Goal\nFaster tests.\n" {
		t.Errorf("read %q", got)
	}
	if _, err := os.Stat(aside); err == nil {
		t.Error("what ws set aside is still there, read")
	}
	if got := read(other); got != "mine\n" {
		t.Errorf("read %q", got)
	}
	if _, err := os.Stat(other); err != nil {
		t.Error("a file that isn't ws's was removed")
	}
	// One that isn't there reads as nothing, not as the terminal.
	if got := read(filepath.Join(dir, "gone")); got != "" {
		t.Errorf("a file that's gone read %q", got)
	}
	// The flag's there on every command, and nobody's shown it.
	root := newBuiltinRootCmd("test")
	if f := root.PersistentFlags().Lookup("stdin-from"); f == nil || !f.Hidden {
		t.Errorf("the flag: %+v", f)
	}
}
