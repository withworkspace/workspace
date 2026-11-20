package workspace

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/git"
)

const (
	siloManifestName = ".silo-manifest"
	siloTargetName   = ".silo-target"
)

// GitSyncableFiles returns all files eligible for silo sync in the given directory.
// This includes both git-tracked files and untracked files that are not gitignored.
// Uses -z for null-separated output to handle filenames with special characters.
func GitSyncableFiles(dir string) ([]string, error) {
	out, err := git.Combined(dir, "ls-files", "-z", "--cached", "--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}
	trimmed := strings.TrimRight(out, "\x00")
	if trimmed == "" {
		return nil, nil
	}
	return strings.Split(trimmed, "\x00"), nil
}

// IsGitIgnored returns true if the given relative path is ignored by git in dir.
// Uses git check-ignore which respects .gitignore, .git/info/exclude, and global excludes.
func IsGitIgnored(dir, relPath string) bool {
	err := git.Run(dir, "check-ignore", "-q", relPath)
	return err == nil
}

// FullSync makes dstDir's syncable files srcDir's (tracked, and untracked
// but not ignored), and returns how many it changed: written, or removed
// because the last sync wrote them and srcDir no longer has them. A file
// dstDir already has the same is left alone, so a sync that finds nothing
// to do touches nothing a dev server would reload for. Files dstDir has
// that no sync wrote are left alone too.
// A manifest file tracks what was synced so cleanup doesn't depend on the
// silo's git index (which is detached and stale).
// Individual file errors are collected and returned but do not abort the sync.
func FullSync(srcDir, dstDir string) (int, error) {
	srcFiles, err := GitSyncableFiles(srcDir)
	if err != nil {
		return 0, fmt.Errorf("listing source files: %w", err)
	}

	srcSet := make(map[string]bool, len(srcFiles))
	for _, f := range srcFiles {
		srcSet[f] = true
	}

	// Continue past individual errors so one missing file doesn't block the rest.
	changed := 0
	var synced []string
	var errs []string
	for _, f := range srcFiles {
		wrote, err := syncFile(srcDir, dstDir, f)
		if err != nil {
			errs = append(errs, f)
			continue
		}
		if wrote {
			changed++
		}
		synced = append(synced, f)
	}

	// Remove files that were in the previous sync but are no longer in the source.
	prevFiles := readManifest(dstDir)
	for _, f := range prevFiles {
		if !srcSet[f] {
			if RemoveSyncedFile(dstDir, f) == nil {
				changed++
			}
		}
	}

	// Write manifest with successfully synced files
	writeManifest(dstDir, synced)

	if len(errs) > 0 {
		return changed, fmt.Errorf("failed to sync %d file(s): %s", len(errs), strings.Join(errs, ", "))
	}
	return changed, nil
}

// readManifest reads the list of previously synced files from the manifest.
func readManifest(siloDir string) []string {
	data, err := os.ReadFile(filepath.Join(siloDir, siloManifestName))
	if err != nil {
		return nil
	}
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "\n")
}

// noteSynced adds files to the manifest of what siloDir was given, and
// takes gone ones from it.
func noteSynced(siloDir string, added, gone []string) {
	if len(added) == 0 && len(gone) == 0 {
		return
	}
	set := map[string]bool{}
	for _, f := range readManifest(siloDir) {
		set[f] = true
	}
	for _, f := range added {
		set[f] = true
	}
	for _, f := range gone {
		delete(set, f)
	}
	files := make([]string, 0, len(set))
	for f := range set {
		files = append(files, f)
	}
	writeManifest(siloDir, files)
}

// writeManifest writes the list of synced files to the manifest.
func writeManifest(siloDir string, files []string) {
	sorted := make([]string, len(files))
	copy(sorted, files)
	sort.Strings(sorted)
	os.WriteFile(filepath.Join(siloDir, siloManifestName), []byte(strings.Join(sorted, "\n")+"\n"), 0644)
}

// SyncFile copies a single file from srcDir to dstDir (relative path),
// unless dstDir has it the same already.
func SyncFile(srcDir, dstDir, relPath string) error {
	_, err := syncFile(srcDir, dstDir, relPath)
	return err
}

// mtimeSettled is how old a file's time must be before the same size
// and time are taken to mean the same file.
const mtimeSettled = 2 * time.Second

// syncFile is SyncFile, saying whether it wrote. The copy is given the
// source's modification time, so a file that's the same size and time
// (one not written in the last mtimeSettled) is taken as unchanged
// without reading it; one that differs in time only is
// compared, and given the time if it's the same.
func syncFile(srcDir, dstDir, relPath string) (bool, error) {
	src := filepath.Join(srcDir, relPath)
	dst := filepath.Join(dstDir, relPath)
	info, err := os.Stat(src)
	if err != nil {
		return false, err
	}
	if have, err := os.Stat(dst); err == nil && have.Mode().IsRegular() && have.Size() == info.Size() && have.Mode().Perm() == info.Mode().Perm() {
		// Trusted only once it's settled: a filesystem keeps times to a
		// tick (a few ms on Linux), so two writes of a size within one
		// share a time, as git's "racily clean" files do.
		if have.ModTime().Equal(info.ModTime()) && time.Since(info.ModTime()) > mtimeSettled {
			return false, nil
		}
		a, errA := os.ReadFile(src)
		b, errB := os.ReadFile(dst)
		if errA == nil && errB == nil && bytes.Equal(a, b) {
			os.Chtimes(dst, info.ModTime(), info.ModTime()) //nolint:errcheck // the next sync compares it again
			return false, nil
		}
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return false, err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return false, err
	}
	if err := os.WriteFile(dst, data, info.Mode()); err != nil {
		return false, err
	}
	os.Chmod(dst, info.Mode().Perm())               //nolint:errcheck // WriteFile keeps an existing file's mode
	os.Chtimes(dst, info.ModTime(), info.ModTime()) //nolint:errcheck // without it, the next sync reads it to compare
	return true, nil
}

// SiloTarget is the capsule whose files a silo was last given in full
// (FullSync, then MarkSiloTarget): "" if none says. The daemon reads it
// to tell a pointing someone else has already synced from one it must.
func SiloTarget(siloDir string) string {
	data, err := os.ReadFile(filepath.Join(siloDir, siloTargetName))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// MarkSiloTarget records that siloDir has capsule's files.
func MarkSiloTarget(siloDir, capsule string) error {
	return os.WriteFile(filepath.Join(siloDir, siloTargetName), []byte(capsule+"\n"), 0644)
}

// RemoveSyncedFile removes a file from dstDir (relative path), and the
// folders it leaves empty.
func RemoveSyncedFile(dstDir, relPath string) error {
	if err := os.Remove(filepath.Join(dstDir, relPath)); err != nil {
		return err
	}
	for d := filepath.Dir(relPath); d != "." && d != "/"; d = filepath.Dir(d) {
		if os.Remove(filepath.Join(dstDir, d)) != nil {
			break // not empty: something else is in it
		}
	}
	return nil
}
