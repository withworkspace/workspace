package artifact

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

// A conversation's agent proposes an artifact, and you add it or not. One
// that's more than its page is kept, until then, as a commit in a repo
// beside the artifacts (.ws/artifacts/.proposed.git): the proposal names
// the commit, its card's frame runs from it, and adding it reads the
// files back out. A page alone is kept in the proposal itself, as it
// always was.

// Proposed is the repo that keeps what conversations proposed.
func (s Store) Proposed() Repo { return Repo(filepath.Join(s.Dir(), ".proposed.git")) }

var commitShape = regexp.MustCompile(`^[0-9a-f]{40,64}$`)

// IsCommit reports whether s names a commit, as a proposal does.
func IsCommit(s string) bool { return commitShape.MatchString(s) }

// Propose keeps files a conversation proposed, and names them: the
// commit. The same files proposed again are the same commit.
func (s Store) Propose(ctx context.Context, files Files, what string, at time.Time) (string, error) {
	files, err := checkFiles(files)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	for _, f := range files {
		fmt.Fprintf(h, "%d %s\x00%d\x00", len(f.Path), f.Path, len(f.Data))
		h.Write(f.Data)
	}
	ref := "refs/proposed/" + hex.EncodeToString(h.Sum(nil))[:40]
	repo := s.Proposed()
	commit, err := repo.Commit(ctx, ref, "", files, "Proposed: "+what+"\n", at.UTC().Truncate(time.Second))
	if errors.Is(err, os.ErrExist) {
		out, err := repo.out(ctx, nil, "rev-parse", "--verify", ref+"^{commit}")
		return string(bytesTrim(out)), err
	}
	return commit, err
}

func bytesTrim(b []byte) []byte {
	for len(b) > 0 && (b[len(b)-1] == '\n' || b[len(b)-1] == ' ') {
		b = b[:len(b)-1]
	}
	return b
}

// ProposedFiles is what a proposal's commit holds, whole.
func (s Store) ProposedFiles(ctx context.Context, commit string) (Files, error) {
	if !IsCommit(commit) {
		return nil, fmt.Errorf("%q does not match a proposal", commit)
	}
	files, err := s.Proposed().AllAt(ctx, commit)
	if err != nil || len(files) == 0 {
		return nil, fmt.Errorf("proposed files are missing (%s in %s)", commit[:12], string(s.Proposed()))
	}
	return files, nil
}

// ProposedFile is one file of what a proposal's commit holds, and what
// names its bytes. A file it hasn't is ErrNoFile.
func (s Store) ProposedFile(ctx context.Context, commit, p string) ([]byte, string, error) {
	if !IsCommit(commit) {
		return nil, "", fmt.Errorf("%w: %q does not match a proposal", ErrNoFile, commit)
	}
	return s.Proposed().ReadAt(ctx, commit, p)
}
