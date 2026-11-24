// What a Tree draws (Tree.tsx), and how one's made and asked about: from
// paths, with a folder that holds only a folder shown as one row, cut down
// to what's looked for, and which folders are open over a row.

// TreeNode is a row of a tree: its id (a path, say), the name it shows and
// is found by, and what's in it. One with children, even none, is a
// folder: it folds. value is whatever the page keeps of it (a file's
// status), for its look.
export interface TreeNode<T = unknown> {
  id: string;
  name: string;
  children?: TreeNode<T>[];
  value?: T;
}

const byName = new Intl.Collator(undefined, { numeric: true, sensitivity: "base" });

// treeOf is a tree of things that have paths ("internal/web/api.go"): a
// folder for each folder, named for its own piece of the path and with the
// path so far as its id, and a row for each thing, its value the thing.
// Folders come first, then files, each by name ("file2" before "file10").
export function treeOf<T>(items: T[], pathOf: (item: T) => string): TreeNode<T>[] {
  const root: TreeNode<T> = { id: "", name: "", children: [] };
  const folders = new Map<string, TreeNode<T>>([["", root]]);
  const folder = (path: string): TreeNode<T> => {
    const have = folders.get(path);
    if (have) return have;
    const cut = path.lastIndexOf("/");
    const made: TreeNode<T> = { id: path, name: path.slice(cut + 1), children: [] };
    folder(cut < 0 ? "" : path.slice(0, cut)).children!.push(made);
    folders.set(path, made);
    return made;
  };
  for (const item of items) {
    const path = pathOf(item);
    const cut = path.lastIndexOf("/");
    folder(cut < 0 ? "" : path.slice(0, cut)).children!.push({ id: path, name: path.slice(cut + 1), value: item });
  }
  for (const f of folders.values()) f.children!.sort((a, b) => (!a.children !== !b.children ? (a.children ? -1 : 1) : byName.compare(a.name, b.name)));
  return root.children!;
}

// compact shows a folder that holds only a folder as one row
// ("internal/web"), as editors do: its name both, its id the inner one's.
export function compact<T>(nodes: TreeNode<T>[]): TreeNode<T>[] {
  return nodes.map((n) => {
    if (!n.children) return n;
    let at = n;
    let name = n.name;
    while (at.children!.length === 1 && at.children![0]!.children) {
      at = at.children![0]!;
      name = `${name}/${at.name}`;
    }
    return { ...at, name, children: compact(at.children!) };
  });
}

// filterTree is the tree cut down to the rows kept and the folders they're
// in. A folder that's kept itself keeps all that's in it.
export function filterTree<T>(nodes: TreeNode<T>[], keep: (node: TreeNode<T>) => boolean): TreeNode<T>[] {
  return nodes.flatMap((n) => {
    if (keep(n)) return [n];
    if (!n.children) return [];
    const children = filterTree(n.children, keep);
    return children.length ? [{ ...n, children }] : [];
  });
}

// foldersOf is every folder's id: what's open when all of it is.
export function foldersOf(nodes: TreeNode[], out: string[] = []): string[] {
  for (const n of nodes) {
    if (!n.children) continue;
    out.push(n.id);
    foldersOf(n.children, out);
  }
  return out;
}

// leavesOf is the rows under these that aren't folders: a folder's files,
// to count what's among them.
export function leavesOf<T>(nodes: TreeNode<T>[], out: TreeNode<T>[] = []): TreeNode<T>[] {
  for (const n of nodes) {
    if (n.children) leavesOf(n.children, out);
    else out.push(n);
  }
  return out;
}

// pathTo is the folders a row is in, outermost first: what has to be open
// for it to show. Undefined when it's not in the tree.
export function pathTo(nodes: TreeNode[], id: string): string[] | undefined {
  for (const n of nodes) {
    if (n.id === id) return [];
    if (!n.children) continue;
    const rest = pathTo(n.children, id);
    if (rest) return [n.id, ...rest];
  }
  return undefined;
}

// shownIds is the rows that show with these folders open, top to bottom.
export function shownIds(nodes: TreeNode[], open: ReadonlySet<string>, out: string[] = []): string[] {
  for (const n of nodes) {
    out.push(n.id);
    if (n.children && open.has(n.id)) shownIds(n.children, open, out);
  }
  return out;
}
