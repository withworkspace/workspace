package nav

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/withworkspace/workspace/internal/git"
)

// Navigation reads files at a commit; a worktree being worked in has
// files no commit holds yet. Now is a commit that holds them as they are:
// the worktree's HEAD when nothing's changed, or else one made of its
// files, tracked or not (and not ignored), with HEAD as its parent. It's
// written with an index of its own, so the worktree, its index and its
// branches are as they were; only git objects nothing points to are added,
// which git's gc clears away in time.

// nowCommit is the snapshot a worktree was last asked for, and what its
// files were then.
type nowCommit struct {
	key string
	rev string
}

type nowCache struct {
	mu    sync.Mutex
	byDir map[string]nowCommit
	makes group[string]
}

// Now is the commit a worktree's files are, as they are now.
func (s *Service) Now(dir string) (string, error) {
	dir = filepath.Clean(dir)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	head, err := git.ReadLine(ctx, dir, "rev-parse", "--verify", "HEAD^{commit}")
	if err != nil {
		return "", fmt.Errorf("no commit in %s yet", filepath.Base(dir))
	}
	status, err := git.Read(ctx, dir, "status", "--porcelain=v1", "-z", "--untracked-files=all", "--ignore-submodules=all")
	if err != nil {
		return "", err
	}
	if len(status) == 0 {
		return head, nil
	}
	key := head + "\x00" + changedStamp(dir, status)
	s.now.mu.Lock()
	if c, ok := s.now.byDir[dir]; ok && c.key == key {
		s.now.mu.Unlock()
		return c.rev, nil
	}
	s.now.mu.Unlock()
	return s.now.makes.do(dir+"\x00"+key, func() (string, error) {
		rev, err := snapshot(ctx, dir, head)
		if err != nil {
			return "", err
		}
		s.now.mu.Lock()
		if s.now.byDir == nil {
			s.now.byDir = map[string]nowCommit{}
		}
		s.now.byDir[dir] = nowCommit{key: key, rev: rev}
		s.now.mu.Unlock()
		return rev, nil
	})
}

// changedStamp is what the changed files are: git's list of them, and
// each one's size and time, since a file changed again lists the same.
func changedStamp(dir string, status []byte) string {
	h := sha256.New()
	h.Write(status)
	recs := strings.Split(string(status), "\x00")
	for i := 0; i < len(recs); i++ {
		rec := recs[i]
		if len(rec) < 4 {
			continue
		}
		xy, p := rec[:2], rec[3:]
		if xy[0] == 'R' || xy[0] == 'C' {
			i++ // the path it was, after it
		}
		if info, err := os.Lstat(filepath.Join(dir, filepath.FromSlash(p))); err == nil {
			fmt.Fprintf(h, "\x00%s\x00%d\x00%d", p, info.Size(), info.ModTime().UnixNano())
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

// snapshot writes the worktree's files as a commit on top of head,
// through a copy of its index: git add -A there, then write-tree and
// commit-tree. Its author and time are fixed, so the same files make the
// same commit.
func snapshot(ctx context.Context, dir, head string) (string, error) {
	tmp, err := os.CreateTemp("", "ws-nav-index-*")
	if err != nil {
		return "", err
	}
	index := tmp.Name()
	defer os.Remove(index)
	defer os.Remove(index + ".lock")
	if p, err := git.ReadLine(ctx, dir, "rev-parse", "--path-format=absolute", "--git-path", "index"); err == nil {
		if src, err := os.Open(p); err == nil {
			_, err = io.Copy(tmp, src)
			src.Close()
			if err != nil {
				tmp.Close()
				return "", err
			}
		}
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	if fi, _ := os.Stat(index); fi != nil && fi.Size() == 0 {
		os.Remove(index) // no index to start from: git makes one
	}
	env := []string{
		"GIT_INDEX_FILE=" + index,
		"GIT_AUTHOR_NAME=ws", "GIT_AUTHOR_EMAIL=ws@localhost", "GIT_AUTHOR_DATE=@0 +0000",
		"GIT_COMMITTER_NAME=ws", "GIT_COMMITTER_EMAIL=ws@localhost", "GIT_COMMITTER_DATE=@0 +0000",
	}
	run := func(args ...string) (string, error) {
		out, err := git.Cmd{Ctx: ctx, Dir: dir, Args: args, Env: env}.Output()
		return strings.TrimSpace(string(out)), err
	}
	if _, err := run("-c", "core.quotePath=false", "add", "-A", "--ignore-errors", "--", "."); err != nil && git.ExitCode(err) != 1 {
		return "", err
	}
	tree, err := run("write-tree")
	if err != nil {
		return "", err
	}
	return run("commit-tree", "--no-gpg-sign", "-p", head, "-m", "ws: the files as they were, for navigation", tree)
}
