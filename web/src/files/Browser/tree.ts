import type { TreeFile } from "../../api";
import { compact, leavesOf, type TreeNode, treeOf } from "../../ui/treeNodes";

// The explorer's tree, made of a worktree's files (ui/Tree draws it, and
// ./filesTreeLook says how its rows look): what FileBrowser lists, and the
// order [ and ] step through its changed files.

// filesTree is the files as a tree: a folder that holds only a folder is
// one row ("internal/web"), as editors do.
export const filesTree = (files: TreeFile[]): TreeNode<TreeFile>[] => compact(treeOf(files, (f) => f.path));

// changedIn is how many changed files are in each folder, by its id.
export function changedIn(nodes: TreeNode<TreeFile>[], out = new Map<string, number>()): Map<string, number> {
  for (const n of nodes) {
    if (!n.children) continue;
    out.set(n.id, leavesOf(n.children).filter((l) => l.value?.status).length);
    changedIn(n.children, out);
  }
  return out;
}

// The folders a path is in: "a", "a/b" for "a/b/c.go".
export const ancestors = (path: string) => {
  const parts = path.split("/");
  return parts.slice(0, -1).map((_, i) => parts.slice(0, i + 1).join("/"));
};

export const baseName = (p: string) => p.slice(p.lastIndexOf("/") + 1);
