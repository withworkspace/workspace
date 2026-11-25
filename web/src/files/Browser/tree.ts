import type { TreeFile } from "../../api";

// The explorer's tree, worked out of a worktree's files: what FileBrowser
// lists, and the order [ and ] step through its changed files.

// A folder or a file in the tree. Folders with one folder in them show as
// one ("internal/web"), as editors do.
export interface Node {
  name: string;
  path: string;
  dir: boolean;
  children: Node[];
  file?: TreeFile;
  changed: number; // changed files in it (a folder), for its mark
}

export function buildTree(files: TreeFile[]): Node[] {
  const root: Node = { name: "", path: "", dir: true, children: [], changed: 0 };
  const dirs = new Map<string, Node>([["", root]]);
  const dirOf = (path: string): Node => {
    const have = dirs.get(path);
    if (have) return have;
    const cut = path.lastIndexOf("/");
    const parent = dirOf(cut < 0 ? "" : path.slice(0, cut));
    const node: Node = { name: path.slice(cut + 1), path, dir: true, children: [], changed: 0 };
    parent.children.push(node);
    dirs.set(path, node);
    return node;
  };
  for (const f of files) {
    const cut = f.path.lastIndexOf("/");
    const parent = dirOf(cut < 0 ? "" : f.path.slice(0, cut));
    parent.children.push({ name: f.path.slice(cut + 1), path: f.path, dir: false, children: [], file: f, changed: f.status ? 1 : 0 });
  }
  const finish = (n: Node): number => {
    n.children.sort((a, b) => (a.dir !== b.dir ? (a.dir ? -1 : 1) : a.name.localeCompare(b.name, undefined, { numeric: true, sensitivity: "base" })));
    n.changed = n.dir ? n.children.reduce((s, c) => s + finish(c), 0) : n.changed;
    // A folder holding only a folder: one row.
    if (n.dir && n !== root && n.children.length === 1 && n.children[0].dir) {
      const only = n.children[0];
      n.name = `${n.name}/${only.name}`;
      n.path = only.path;
      n.children = only.children;
    }
    return n.changed;
  };
  finish(root);
  return root.children;
}

export interface Row {
  node: Node;
  depth: number;
}

export function flatten(nodes: Node[], isOpen: (n: Node) => boolean, depth = 0, out: Row[] = []): Row[] {
  for (const n of nodes) {
    out.push({ node: n, depth });
    if (n.dir && isOpen(n)) flatten(n.children, isOpen, depth + 1, out);
  }
  return out;
}

// The folders a path is in: "a", "a/b" for "a/b/c.go".
export const ancestors = (path: string) => {
  const parts = path.split("/");
  return parts.slice(0, -1).map((_, i) => parts.slice(0, i + 1).join("/"));
};

export const baseName = (p: string) => p.slice(p.lastIndexOf("/") + 1);
