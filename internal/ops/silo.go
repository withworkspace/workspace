package ops

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/withworkspace/workspace/internal/config"
	"github.com/withworkspace/workspace/internal/workspace"
)

// SiloState is where a silo stands, as the daemon keeping it has it.
type SiloState = workspace.SiloState

// PointSilo points repo's silo at capsule (a capsule's name, or .ground):
// it makes .silo if there's none, mirrors the capsule's tracked files into
// it, records where it points (the daemon keeps it from there), and runs
// its hooks (after_create, then ws.repo.toml's after_switch). What it does, and what its hooks say, is
// written to out as it goes. A hook that fails is said there, and doesn't
// stop it.
func (w *Workspace) PointSilo(repo, capsule string, out io.Writer) error {
	ws := w.WS()
	if _, ok := w.Config().Repos[repo]; !ok {
		return fmt.Errorf("%w: no repo %s", ErrNotFound, repo)
	}
	capsuleDir := filepath.Join(ws.RepoDir(repo), capsule)
	if capsule == "" || capsule == workspace.SiloDir || !dirExists(capsuleDir) {
		return fmt.Errorf("%w: %s has no capsule %s", ErrNotFound, repo, capsule)
	}
	siloDir := ws.SiloWorktree(repo)
	if _, err := os.Stat(siloDir); os.IsNotExist(err) {
		fmt.Fprintf(out, "  Creating silo for %s...\n", ws.FormatRepoName(repo))
		if err := workspace.GitWorktreeAddDetached(ws.BareDir(repo), siloDir, ws.DefaultBranch); err != nil {
			return fmt.Errorf("creating silo worktree: %w", err)
		}
	}
	fmt.Fprintf(out, "  Syncing %s -> .silo...\n", capsule)
	if _, err := workspace.FullSync(capsuleDir, siloDir); err != nil {
		return fmt.Errorf("syncing: %w", err)
	}
	// Marked before it's recorded: the daemon, which keeps the silo from
	// then on, sees it has capsule's files, and leaves the hooks to this.
	if err := workspace.MarkSiloTarget(siloDir, capsule); err != nil {
		return fmt.Errorf("marking the silo: %w", err)
	}
	if ws.Silo == nil {
		ws.Silo = make(map[string]string)
	}
	ws.Silo[repo] = capsule
	if err := config.SaveSilo(ws.Root, ws.Silo); err != nil {
		return fmt.Errorf("saving silo state: %w", err)
	}
	// after_create: ws.local.toml's, then ws.toml's, then ws.repo.toml's.
	hook, hasHook := ws.AfterCreateHooks[repo]
	repoCfg, err := config.ParseRepoConfig(filepath.Join(ws.MainWorktree(repo), config.RepoFileName))
	if err != nil {
		return err
	}
	if !hasHook && repoCfg != nil && repoCfg.Capsule.AfterCreate != "" {
		hook, hasHook = repoCfg.Capsule.AfterCreate, true
	}
	if hasHook {
		fmt.Fprintf(out, "  Running after_create hook...\n")
		if err := workspace.RunHook(siloDir, hook, out, out); err != nil {
			fmt.Fprintf(out, "  ⚠ after_create hook failed: %v\n", err)
		}
	}
	if repoCfg != nil && repoCfg.Silo.AfterSwitch != "" {
		fmt.Fprintf(out, "  Running after_switch hook...\n")
		if err := workspace.RunHook(siloDir, repoCfg.Silo.AfterSwitch, out, out); err != nil {
			fmt.Fprintf(out, "  ⚠ after_switch hook failed: %v\n", err)
		}
	}
	return nil
}
