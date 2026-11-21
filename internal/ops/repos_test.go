package ops

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/withworkspace/workspace/internal/store/storetest"
)

func shareGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "commit.gpgsign=false", "-c", "user.name=Ana", "-c", "user.email=ana@example.com"}, args...)...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// sharedWorld is a workspace whose config is a repo with a remote, its git
// put away as ws keeps it between syncs, and a repo of yours, notes, in
// ws.local.toml alone.
func sharedWorld(t *testing.T) (w *Workspace, root, remote string) {
	t.Helper()
	remote = filepath.Join(t.TempDir(), "config.git")
	shareGit(t, filepath.Dir(remote), "init", "-q", "--bare", "--initial-branch=main", remote)
	root = t.TempDir()
	storetest.Own(t, root)
	shareGit(t, root, "init", "-q", "--initial-branch=main")
	shareGit(t, root, "config", "user.name", "Ana")
	shareGit(t, root, "config", "user.email", "ana@example.com")
	shareGit(t, root, "config", "commit.gpgsign", "false")
	os.WriteFile(filepath.Join(root, "ws.toml"), []byte("[workspace]\norg = \"acme\"\n\n[repos.api]\n"), 0o644)
	os.WriteFile(filepath.Join(root, ".gitignore"), []byte("ws.local.toml\nws.local.toml.lock\n.ws/\nrepos/\n"), 0o644)
	shareGit(t, root, "add", "-A")
	shareGit(t, root, "commit", "-q", "-m", "workspace")
	shareGit(t, root, "remote", "add", "origin", remote)
	shareGit(t, root, "push", "-q", "-u", "origin", "main")
	os.WriteFile(filepath.Join(root, "ws.local.toml"), []byte("[repos.notes]\ngithub = \"someone/notes\"\ndisplay_name = \"Notes\"\n"), 0o644)
	os.Rename(filepath.Join(root, ".git"), filepath.Join(root, ".git-disabled"))
	return Open(root, Options{}), root, remote
}

// A repo of yours given to the team is in ws.toml, committed and pushed,
// and out of ws.local.toml; the config's git is put away again after.
func TestShareRepo_PushedToTheTeam(t *testing.T) {
	w, root, remote := sharedWorld(t)
	if !w.WS().Yours["notes"] || w.WS().Yours["api"] {
		t.Fatalf("yours = %v, want notes alone", w.WS().Yours)
	}
	if _, err := w.ShareRepo(context.Background(), AsCrew("c1"), "notes"); err == nil {
		t.Error("crew shouldn't change the team's ws.toml")
	}

	done, err := w.ShareRepo(context.Background(), AsYou, "notes")
	if err != nil {
		t.Fatal(err)
	}
	if done.Branch != "main" || done.Commit == "" {
		t.Errorf("shared %+v", done)
	}
	pushed := shareGit(t, remote, "show", "main:ws.toml")
	if !strings.Contains(pushed, "[repos.notes]\ngithub = \"someone/notes\"\ndisplay_name = \"Notes\"") {
		t.Errorf("the remote's ws.toml:\n%s", pushed)
	}
	if got := shareGit(t, remote, "log", "-1", "--format=%s", "main"); got != "Add repo notes" {
		t.Errorf("the commit: %s", got)
	}
	if local, _ := os.ReadFile(filepath.Join(root, "ws.local.toml")); strings.Contains(string(local), "notes") {
		t.Errorf("ws.local.toml still has it:\n%s", local)
	}
	if _, err := os.Stat(filepath.Join(root, ".git")); err == nil {
		t.Error("the config's git should be put away again")
	}
	if ws := w.WS(); ws.Yours["notes"] || ws.RemoteOf("notes").String() != "someone/notes" {
		t.Errorf("after: yours %v, notes at %s", ws.Yours, ws.RemoteOf("notes"))
	}
	if _, err := w.ShareRepo(context.Background(), AsYou, "notes"); err == nil {
		t.Error("the team's already: nothing to share")
	}
}

// A push the remote refuses takes the commit back: ws sync would lose it.
// The repo stays yours, and ws.toml is as it was.
func TestShareRepo_RefusedPushStaysYours(t *testing.T) {
	w, root, remote := sharedWorld(t)
	hook := filepath.Join(remote, "hooks", "pre-receive")
	os.WriteFile(hook, []byte("#!/bin/sh\necho 'protected branch' >&2\nexit 1\n"), 0o755)

	if _, err := w.ShareRepo(context.Background(), AsYou, "notes"); err == nil || !strings.Contains(err.Error(), "stays yours") {
		t.Fatalf("err = %v, want the push refused", err)
	}
	team, _ := os.ReadFile(filepath.Join(root, "ws.toml"))
	if string(team) != "[workspace]\norg = \"acme\"\n\n[repos.api]\n" {
		t.Errorf("ws.toml after:\n%s", team)
	}
	os.Rename(filepath.Join(root, ".git-disabled"), filepath.Join(root, ".git"))
	if got := shareGit(t, root, "log", "--format=%s"); got != "workspace" {
		t.Errorf("the config repo's commits after: %q", got)
	}
	if local, _ := os.ReadFile(filepath.Join(root, "ws.local.toml")); !strings.Contains(string(local), "[repos.notes]") {
		t.Errorf("ws.local.toml lost it:\n%s", local)
	}
}
