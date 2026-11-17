package git

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func repo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"config", "user.email", "t@example.com"},
		{"config", "user.name", "T"},
		{"config", "commit.gpgsign", "false"},
	} {
		if err := Run(dir, args...); err != nil {
			t.Fatal(err)
		}
	}
	os.WriteFile(filepath.Join(dir, "a b.txt"), []byte("one\n"), 0o644)
	if err := Run(dir, "add", "."); err != nil {
		t.Fatal(err)
	}
	if err := Run(dir, "commit", "-q", "-m", "first"); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestOutputLineAndLines(t *testing.T) {
	dir := repo(t)
	if b, err := Line(dir, "rev-parse", "--abbrev-ref", "HEAD"); err != nil || b != "main" {
		t.Errorf("branch = %q, %v", b, err)
	}
	if ls := Lines(dir, "log", "--format=%s"); len(ls) != 1 || ls[0] != "first" {
		t.Errorf("lines = %q", ls)
	}
	if ls := Lines(dir, "log", "nope"); ls != nil {
		t.Errorf("a failure has lines: %q", ls)
	}
	if !Ok(dir, "show-ref", "--verify", "--quiet", "refs/heads/main") || Ok(dir, "show-ref", "--verify", "--quiet", "refs/heads/nope") {
		t.Error("Ok doesn't tell a ref there from one that isn't")
	}
}

// Reading quotes no paths, and takes no optional locks.
func TestRead(t *testing.T) {
	dir := repo(t)
	out, err := Read(context.Background(), dir, "ls-files")
	if err != nil || strings.TrimSpace(string(out)) != "a b.txt" {
		t.Errorf("ls-files = %q, %v", out, err)
	}
	if got, err := ReadLine(context.Background(), dir, "rev-parse", "HEAD"); err != nil || len(got) != 40 {
		t.Errorf("head = %q, %v", got, err)
	}
}

// A failure says the subcommand and what git said, and keeps the exit
// code and what it printed.
func TestErrors(t *testing.T) {
	dir := repo(t)
	_, err := Output(dir, "rev-parse", "--verify", "nope")
	if err == nil || !strings.HasPrefix(err.Error(), "git rev-parse: fatal: ") {
		t.Errorf("err = %v", err)
	}
	if ExitCode(err) != 128 {
		t.Errorf("exit code = %d", ExitCode(err))
	}
	if ExitCode(nil) != -1 {
		t.Error("no error has an exit code")
	}
	if err := Run(dir, "checkout", "nope"); err == nil || !strings.Contains(err.Error(), "nope") {
		t.Errorf("run's err = %v", err)
	}
	// merge-file says how many clashes in its exit code, and prints all the same.
	a, b, c := filepath.Join(dir, "x"), filepath.Join(dir, "y"), filepath.Join(dir, "z")
	os.WriteFile(a, []byte("mine\n"), 0o644)
	os.WriteFile(b, []byte("base\n"), 0o644)
	os.WriteFile(c, []byte("theirs\n"), 0o644)
	out, err := Cmd{Args: []string{"merge-file", "-p", a, b, c}}.Output()
	if ExitCode(err) != 1 || !strings.Contains(string(out), "<<<<<<<") {
		t.Errorf("merge-file: %q, %v", out, err)
	}
}

func TestCmd_EnvAndStdin(t *testing.T) {
	dir := repo(t)
	out, err := Cmd{Dir: dir, Args: []string{"hash-object", "--stdin"}, Stdin: strings.NewReader("one\n")}.Output()
	if err != nil || len(strings.TrimSpace(string(out))) != 40 {
		t.Errorf("hash-object = %q, %v", out, err)
	}
	got, err := Cmd{Dir: dir, Args: []string{"var", "GIT_EDITOR"}, Env: []string{"GIT_EDITOR=true"}}.Output()
	if err != nil || strings.TrimSpace(string(got)) != "true" {
		t.Errorf("GIT_EDITOR = %q, %v", got, err)
	}
}
