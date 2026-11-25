import type { Space, SpaceDoc } from "../api";
import { sorted } from "../documents/sorted";

// Where a document is kept, as the reader's crumbs name it (#123, "Getting
// around"): its path, each step of it a menu of what's there, and its
// siblings, the documents beside it that [ and ] go to. What the reader
// works out from a space's documents, or a worktree's files, kept apart
// from drawing it so it can be tested.
//
// In a space, a document of one of its kinds is kept in that kind's list,
// as the space's library lists it (records / ADRs / One binary), and its
// siblings are that list in the library's order; any other document is
// kept in its folder (ws / designs / Documents), as the space lists it
// under Other. Outside a space, a document is kept in its folder of the
// repo, among the markdown there.

// Shelf is somewhere documents are kept: a space's top (or a worktree's),
// one of its kinds, or a folder.
export interface Shelf {
  id: string; // "" for the top; "kind:<name>" for a kind; a folder's path in the repo, with its slash
  kind?: string; // the kind's name, for a kind
  label: string; // its crumb: the space's label, the kind's title, the folder's name
  name: string; // what it's called in its menu and the pager: the space's label, the kind's title, the folder's path below the top ("designs/")
  up?: string; // the shelf it's on; none for the top
  items: ShelfItem[]; // what's on it, in order: shelves, then documents
}

// ShelfItem is a shelf on a shelf, or a document.
export interface ShelfItem {
  shelf?: string; // a shelf's id
  path?: string; // a document's path in the repo
  title: string;
  note: string; // a shelf's count; a document's number or date (of a kind) or its file's name
}

export interface Shelves {
  shelves: Map<string, Shelf>;
  // home is where each document is kept: its shelf's id, by its path.
  home: Map<string, string>;
}

// Here is the document being read, so it's there even when the list it's
// read from doesn't have it yet (it's new, or the list is behind).
export interface Here {
  path: string;
  title: string;
  kind?: string;
}

const kindShelf = (name: string) => `kind:${name}`;
const fileName = (path: string) => path.slice(path.lastIndexOf("/") + 1);
const folderOf = (path: string) => path.slice(0, path.lastIndexOf("/") + 1); // with its slash; "" at the top
const byPath = (a: ShelfItem, b: ShelfItem) => (a.path ?? "").localeCompare(b.path ?? "", undefined, { numeric: true });

// spaceShelves is a space's shelves: its top, a shelf for each of its kinds
// that has documents, then its folders, from its documents (as getSpace
// gives them).
export function spaceShelves(space: Space, docs: SpaceDoc[], here?: Here): Shelves {
  const root = withSlash(space.folder ?? "");
  const library = new Set(space.kinds.map((k) => k.name));
  const all = here && !docs.some((d) => d.path === here.path) ? [...docs, { path: here.path, title: here.title, kind: here.kind }] : docs;
  const shelves = new Map<string, Shelf>();
  const home = new Map<string, string>();
  const top: Shelf = { id: "", label: space.label, name: space.label, items: [] };
  shelves.set("", top);
  // Its kinds first, as its library lists them.
  for (const k of space.kinds) {
    const of = all.filter((d) => d.kind === k.name);
    if (of.length === 0) continue;
    const id = kindShelf(k.name);
    shelves.set(id, {
      id,
      kind: k.name,
      label: plural(k.title),
      name: plural(k.title),
      up: "",
      items: sorted(of, "made", k.states).map((d) => ({ path: d.path, title: d.title || fileName(d.path), note: d.number ?? d.date ?? fileName(d.path) })),
    });
    top.items.push({ shelf: id, title: plural(k.title), note: String(of.length) });
    for (const d of of) home.set(d.path, id);
  }
  // Then the rest, by folder.
  const rest = all.filter((d) => !(d.kind && library.has(d.kind)));
  folders(shelves, home, root, rest.map((d) => ({ path: d.path, title: d.title || fileName(d.path), note: fileName(d.path) })));
  return { shelves, home };
}

// fileShelves is a worktree's markdown as shelves: its top and its
// folders. Its files have no titles here: each is its file's name (here
// is only to be sure it's there).
export function fileShelves(label: string, paths: string[], here?: Here): Shelves {
  const md = paths.filter((p) => /\.(md|markdown|mdx)$/i.test(p));
  if (here && !md.includes(here.path)) md.push(here.path);
  const shelves = new Map<string, Shelf>();
  const home = new Map<string, string>();
  const top: Shelf = { id: "", label, name: label, items: [] };
  shelves.set("", top);
  folders(
    shelves,
    home,
    "",
    md.map((p) => ({ path: p, title: fileName(p), note: "" })),
  );
  return { shelves, home };
}

// folders shelves documents by folder below root, the top's own on top:
// each folder's folders by name, then its documents in the order of their
// paths (a numbered or dated name keeps them in order), as a space lists
// them.
function folders(shelves: Map<string, Shelf>, home: Map<string, string>, root: string, docs: ShelfItem[]) {
  const count = new Map<string, number>();
  const ensure = (id: string): Shelf => {
    const had = shelves.get(id);
    if (had) return had;
    const parent = folderOf(id.slice(0, -1));
    const up = parent.length > root.length ? parent : "";
    const s: Shelf = { id, label: fileName(id.slice(0, -1)), name: id.slice(root.length), up, items: [] };
    shelves.set(id, s);
    ensure(up).items.push({ shelf: id, title: s.label, note: "" });
    return s;
  };
  for (const d of [...docs].sort(byPath)) {
    const at = folderOf(d.path!);
    const id = at.length > root.length && at.startsWith(root) ? at : "";
    ensure(id).items.push(d);
    home.set(d.path!, id);
    for (let f = id; f !== "" && f !== undefined; f = shelves.get(f)!.up!) count.set(f, (count.get(f) ?? 0) + 1);
  }
  // Each shelf's shelves by name, before its documents; a folder's count
  // is every document in it, however deep.
  for (const s of [...shelves.values()]) {
    if (s.kind) continue;
    const kinds = s.items.filter((i) => i.shelf?.startsWith("kind:"));
    const subs = s.items.filter((i) => i.shelf && !i.shelf.startsWith("kind:")).sort((a, b) => a.title.localeCompare(b.title));
    for (const i of subs) i.note = String(count.get(i.shelf!) ?? 0);
    s.items = [...kinds, ...subs, ...s.items.filter((i) => i.path)];
  }
}

const withSlash = (f: string) => (f && !f.endsWith("/") ? `${f}/` : f);

// plural is a kind's title as its list is called (ADR, ADRs): a kind's
// title names one.
export function plural(title: string): string {
  if (/(s|x|z|ch|sh)$/i.test(title)) return `${title}es`;
  if (/[^aeiou]y$/i.test(title)) return `${title.slice(0, -1)}ies`;
  return `${title}s`;
}

// trail is the shelves from the top down to the one a document is kept
// on: its crumbs. Empty for a document that isn't on any.
export function trail(s: Shelves, path: string): Shelf[] {
  const out: Shelf[] = [];
  for (let id = s.home.get(path); id !== undefined; id = s.shelves.get(id)?.up) {
    const shelf = s.shelves.get(id);
    if (!shelf) break;
    out.unshift(shelf);
  }
  return out;
}

// Siblings is where a document is among those kept with it: those before
// and after it ([ and ]), and which it is of how many.
export interface Siblings {
  shelf: Shelf;
  prev?: ShelfItem;
  next?: ShelfItem;
  at: number; // from 1
  of: number;
}

export function siblings(s: Shelves, path: string): Siblings | undefined {
  const shelf = s.shelves.get(s.home.get(path) ?? "\0");
  if (!shelf) return undefined;
  const docs = shelf.items.filter((i) => i.path);
  const i = docs.findIndex((d) => d.path === path);
  if (i < 0) return undefined;
  return { shelf, prev: docs[i - 1], next: docs[i + 1], at: i + 1, of: docs.length };
}

// filterItems is a shelf's items that match what's typed: in their title
// or their note, any case.
export function filterItems(items: ShelfItem[], typed: string): ShelfItem[] {
  const q = typed.trim().toLowerCase();
  if (!q) return items;
  return items.filter((i) => `${i.title} ${i.note} ${i.path ? fileName(i.path) : ""}`.toLowerCase().includes(q));
}

// holds is whether an item is the document being read, or a shelf it's
// kept on, however far up (its path's trail): the item a menu starts on.
export function holds(item: ShelfItem, path: string, on: Shelf[]): boolean {
  return item.path === path || (!!item.shelf && on.some((s) => s.id === item.shelf));
}
