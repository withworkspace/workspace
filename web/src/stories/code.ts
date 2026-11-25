import type { DiffFile, DiffLine, Hunk } from "../api";

// Code for the stories of what draws it (CodeLines, DiffLines): a Go file
// as it is now and as it was before a change, and the diff between them,
// made here as git would make it.

const now = `// Package nav is code navigation for reviews and the explorer: which
// words in a file are names, where a name is defined, and where a
// definition is used.
package nav

import (
	"errors"
	"fmt"
	"path/filepath"
)

// CacheMax is the most the facts kept on disk take up.
const CacheMax = 200 << 20

// Service answers questions about names, for any repo at any commit.
type Service struct {
	Runtime Runtime
	disk    disk
	facts   *lru[*Facts]   // by blob and grammar
	trees   *lru[*gitTree] // by git directory and commit
	answers *lru[*Answer]  // by question: a commit's don't change
}

// New is a service that keeps what it parses in cacheDir ("" for memory
// only), and prunes it there now and then.
func New(cacheDir string) *Service {
	s := &Service{
		Runtime: Builtin(),
		disk:    disk{dir: cacheDir},
		facts:   newLRU[*Facts](2000),
		trees:   newLRU[*gitTree](8),
		answers: newLRU[*Answer](5000),
	}
	if cacheDir != "" {
		go s.disk.prune(CacheMax)
	}
	return s
}

// Tree is a git directory's files at a commit.
func (s *Service) Tree(dir, rev string) (Tree, error) {
	if rev == "" {
		return nil, errors.New("nav: no commit to read " + filepath.Base(dir) + " at, which is what a worktree with nothing committed in it has, so there's nothing to say of its names yet")
	}
	t, err := s.tree(dir, rev)
	if err != nil {
		return nil, fmt.Errorf("nav: %w", err)
	}
	return t, nil
}
`;

// goNow is the file's lines as they are.
export const goNow: string[] = now.trimEnd().split("\n");

// goBefore is its lines before the change: no cache size, the answers
// kept in a map, and a shorter error.
export const goBefore: string[] = now
  .replace('// CacheMax is the most the facts kept on disk take up.\nconst CacheMax = 200 << 20\n\n', "")
  .replace("\tanswers *lru[*Answer]  // by question: a commit's don't change\n", "\tanswers map[string]*Answer\n")
  .replace("\t\tanswers: newLRU[*Answer](5000),\n", "\t\tanswers: map[string]*Answer{},\n")
  .replace('\tif cacheDir != "" {\n\t\tgo s.disk.prune(CacheMax)\n\t}\n', "")
  .replace(/errors\.New\("nav: no commit to read "[^\n]*\)/, 'errors.New("nav: no commit")')
  .trimEnd()
  .split("\n");

// diffOf is the diff from one file's lines to another's, as git gives
// it: hunks of what changed with a few unchanged lines around each.
export function diffOf(before: string[], after: string[], path: string, around = 3): DiffFile {
  // The longest run of lines both have, in order: what's left is the change.
  const n = before.length;
  const m = after.length;
  const lcs = Array.from({ length: n + 1 }, () => new Uint16Array(m + 1));
  for (let i = n - 1; i >= 0; i--) for (let j = m - 1; j >= 0; j--) lcs[i]![j] = before[i] === after[j] ? lcs[i + 1]![j + 1]! + 1 : Math.max(lcs[i + 1]![j]!, lcs[i]![j + 1]!);
  const all: DiffLine[] = [];
  let i = 0;
  let j = 0;
  while (i < n || j < m) {
    if (i < n && j < m && before[i] === after[j]) all.push({ k: "c", o: ++i, n: ++j, t: after[j - 1]! });
    else if (i < n && (j >= m || lcs[i + 1]![j]! >= lcs[i]![j + 1]!)) all.push({ k: "d", o: ++i, t: before[i - 1]! });
    else all.push({ k: "a", n: ++j, t: after[j - 1]! });
  }
  // Each changed line with the lines around it, runs that touch as one.
  const keep = new Uint8Array(all.length);
  all.forEach((l, at) => {
    if (l.k !== "c") for (let k = Math.max(0, at - around); k <= Math.min(all.length - 1, at + around); k++) keep[k] = 1;
  });
  const hunks: Hunk[] = [];
  let oldAt = 1;
  let newAt = 1;
  for (let at = 0; at < all.length; ) {
    const l = all[at]!;
    if (!keep[at]) {
      oldAt = l.o! + 1;
      newAt = l.n! + 1;
      at++;
      continue;
    }
    const lines: DiffLine[] = [];
    while (at < all.length && keep[at]) lines.push(all[at++]!);
    const olds = lines.filter((x) => x.k !== "a").length;
    const news = lines.filter((x) => x.k !== "d").length;
    hunks.push({ oldStart: olds ? oldAt : oldAt - 1, oldLines: olds, newStart: news ? newAt : newAt - 1, newLines: news, lines });
    oldAt += olds;
    newAt += news;
  }
  const status = !n ? "added" : !m ? "deleted" : "modified";
  return { path, status, additions: all.filter((l) => l.k === "a").length, deletions: all.filter((l) => l.k === "d").length, hunks };
}
