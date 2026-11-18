package crew

import (
	"github.com/withworkspace/workspace/internal/store/storetest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnv_PutsThisBinaryFirstOnPath(t *testing.T) {
	root := t.TempDir()
	storetest.Own(t, root)
	t.Setenv("PATH", "/usr/bin:/bin")
	env := Env(root, "m", "c1")

	shim := filepath.Join(root, ".ws", "bin")
	var path string
	for _, kv := range env {
		if v, ok := strings.CutPrefix(kv, "PATH="); ok {
			if path != "" {
				t.Fatal("PATH set twice")
			}
			path = v
		}
	}
	if path != shim+":/usr/bin:/bin" {
		t.Errorf("PATH = %q", path)
	}
	exe, _ := os.Executable()
	exe, _ = filepath.EvalSymlinks(exe)
	for _, name := range []string{"ws", "workspace"} {
		if got, _ := os.Readlink(filepath.Join(shim, name)); got != exe {
			t.Errorf("%s -> %q, want %q", name, got, exe)
		}
	}

	// A stale link (an earlier build) is replaced; repeating is harmless.
	link := filepath.Join(shim, "ws")
	os.Remove(link)
	os.Symlink("/old/workspace", link)
	t.Setenv("PATH", shim+":/usr/bin")
	env = Env(root, "m", "c2")
	if got, _ := os.Readlink(link); got != exe {
		t.Errorf("stale link not replaced: %q", got)
	}
	for _, kv := range env {
		if v, ok := strings.CutPrefix(kv, "PATH="); ok && v != shim+":/usr/bin" {
			t.Errorf("shim dir duplicated on PATH: %q", v)
		}
	}
}

// A deck started from inside a Claude Code, pi or OpenCode session mustn't hand that
// session's identity to crew, but the agents' settings pass through.
func TestEnv_DropsAgentSessionIdentity(t *testing.T) {
	t.Setenv("CLAUDECODE", "1")
	t.Setenv("CLAUDE_CODE_SESSION_ID", "parent-session")
	t.Setenv("CLAUDE_CODE_MESSAGING_SOCKET", "/tmp/parent.sock")
	t.Setenv("CLAUDE_CONFIG_DIR", "/home/me/.claude-work")
	t.Setenv("CLAUDE_CODE_USE_BEDROCK", "1")
	t.Setenv("PI_CODING_AGENT", "true")
	t.Setenv("PI_SESSION_ID", "parent-pi")
	t.Setenv("PI_MODEL", "claude-opus-5-5")
	t.Setenv("PI_CODING_AGENT_DIR", "/home/me/.pi-work")
	t.Setenv("OPENCODE", "1")
	t.Setenv("OPENCODE_PID", "4242")
	t.Setenv("AGENT", "1")
	env := strings.Join(Env(t.TempDir(), "m", "c1"), "\n") + "\n"
	for _, gone := range []string{"CLAUDECODE=", "CLAUDE_CODE_SESSION_ID=", "CLAUDE_CODE_MESSAGING_SOCKET=", "PI_CODING_AGENT=", "PI_SESSION_ID=", "PI_MODEL=", "OPENCODE=", "OPENCODE_PID=", "AGENT="} {
		if strings.Contains(env, "\n"+gone) || strings.HasPrefix(env, gone) {
			t.Errorf("%s passed to crew", gone)
		}
	}
	for _, kept := range []string{"CLAUDE_CONFIG_DIR=/home/me/.claude-work\n", "CLAUDE_CODE_USE_BEDROCK=1\n", "PI_CODING_AGENT_DIR=/home/me/.pi-work\n"} {
		if !strings.Contains(env, kept) {
			t.Errorf("%q dropped", kept)
		}
	}
}

// git run in a repo's folder (repos/<repo>, which holds its clone and
// capsules but isn't a repository) fails, rather than answering for the
// workspace's config repo above it; in a capsule it works as ever.
func TestEnv_GitStopsAtTheReposFolder(t *testing.T) {
	root := t.TempDir()
	storetest.Own(t, root)
	git := func(dir string, env []string, args ...string) (string, error) {
		c := exec.Command("git", args...)
		c.Dir, c.Env = dir, env
		out, err := c.CombinedOutput()
		return strings.TrimSpace(string(out)), err
	}
	if out, err := git(root, nil, "init", "-q"); err != nil {
		t.Fatal(out)
	}
	repo := filepath.Join(root, "repos", "demo")
	capsule := filepath.Join(repo, "fix")
	os.MkdirAll(capsule, 0o755)
	if out, err := git(capsule, nil, "init", "-q"); err != nil {
		t.Fatal(out)
	}
	env := Env(root, "m", "c1")
	if out, err := git(repo, env, "rev-parse", "--show-toplevel"); err == nil {
		t.Errorf("git in repos/demo answered for %s", out)
	}
	if out, err := git(capsule, env, "rev-parse", "--show-toplevel"); err != nil || !strings.HasSuffix(out, filepath.Join("repos", "demo", "fix")) {
		t.Errorf("git in a capsule: %q, %v", out, err)
	}
}

// What turns ws eval on isn't handed to an agent: one that finds the
// command can't run it.
func TestEnv_DropsEval(t *testing.T) {
	t.Setenv(EnvEval, "1")
	for _, kv := range Env(t.TempDir(), "m", "c1") {
		if strings.HasPrefix(kv, EnvEval+"=") {
			t.Errorf("crew are given %s", kv)
		}
	}
}
