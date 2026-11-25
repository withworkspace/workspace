import { type ArtifactDetail, type FileTree, type FileView, getArtifactDetail, getFile, getTree, worktreeAPI } from "../api";

// What's been read for refs' cards, kept while the page is open: a file's
// lines, a folder's tree, an artifact's versions (and a pull request,
// prs.ts).

const files = new Map<string, Promise<FileView>>();

export const fileOf = (repo: string, worktree: string, path: string) => {
  const key = `${repo}\n${worktree}\n${path}`;
  let got = files.get(key);
  if (!got) {
    got = getFile(worktreeAPI(repo, worktree), path);
    files.set(key, got);
    got.catch(() => files.delete(key));
  }
  return got;
};

// An artifact's versions, read once while the page is open: a card's
// version is written once, and the workspace says when there's another.
const artifacts = new Map<string, Promise<ArtifactDetail>>();
export const artifactOf = (id: string, versions: number) => {
  const key = `${id}\n${versions}`;
  let got = artifacts.get(key);
  if (!got) {
    got = getArtifactDetail(id);
    artifacts.set(key, got);
    got.catch(() => artifacts.delete(key));
  }
  return got;
};

const trees = new Map<string, Promise<FileTree>>();
export const treeOf = (repo: string, worktree: string) => {
  const key = `${repo}\n${worktree}`;
  let got = trees.get(key);
  if (!got) {
    got = getTree(worktreeAPI(repo, worktree));
    trees.set(key, got);
    // One still being read is asked again next time.
    got.then((t) => t.unread && trees.delete(key)).catch(() => trees.delete(key));
  }
  return got;
};

// excerptRoom is how many lines of a file a card shows.
export const excerptRoom = 12;
