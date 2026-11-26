import type { DiffFile } from "../api";

// The list of a diff's files, in groups: a folder's files under the
// folder, named once, so a row is the file's name and is never cut short
// for its path. Files to skim (lockfiles, generated code, renames) are a
// group of their own, last. The files come in the order they're read in
// (review.order, in Go): groups are runs of it.

export interface FileGroup {
  key: string;
  // label is the folder, without what every folder starts with; "" for
  // the files at the top of the repo, or of what they all share.
  label: string;
  title: string; // the folder in full
  skim?: boolean;
  // named: it's a group by name (a guide's), not a folder: its rows say
  // their folders.
  named?: boolean;
  why?: string; // a guide's group: why its files are read there
  files: number[]; // places in the diff's files
}

export interface FileGroups {
  // common is the folder every file to read is under, said once.
  common: string;
  groups: FileGroup[];
}

export const dirOf = (p: string) => (p.includes("/") ? p.slice(0, p.lastIndexOf("/") + 1) : "");
export const baseOf = (p: string) => p.slice(p.lastIndexOf("/") + 1);

// commonDir is the folder every one of dirs is in or under.
function commonDir(dirs: string[]): string {
  if (!dirs.length) return "";
  let parts = dirs[0].split("/").filter(Boolean);
  for (const d of dirs.slice(1)) {
    const p = d.split("/").filter(Boolean);
    let i = 0;
    while (i < parts.length && i < p.length && parts[i] === p[i]) i++;
    parts = parts.slice(0, i);
    if (!parts.length) return "";
  }
  return parts.length ? `${parts.join("/")}/` : "";
}

// Order is an order to read the files in that isn't their folders': a
// guide's groups, each with why it's read there. stale says the change
// has moved on since it was made: files it doesn't place are then new to
// it, and come first.
export interface Order {
  key: string; // changes when the order does
  stale?: boolean;
  groups: { name: string; why?: string; skim?: boolean; files: string[] }[];
  notes: Record<string, string>; // what the change does to a file, in a line, by path
}

const unplaced = (o: Order) => (o.stale ? "Not in the guide" : "Not placed");

// orderFiles is the files in an order's order: its groups, with the files
// it doesn't place after them (or, the order being stale, before: they
// may be what's new). Nothing's left out. A file in a group to skim is
// one to skim.
export function orderFiles(files: DiffFile[], o: Order): DiffFile[] {
  const by = new Map(files.map((f) => [f.path, f]));
  const placed = new Set<string>();
  const out: DiffFile[] = [];
  for (const g of o.groups) {
    for (const path of g.files) {
      const f = by.get(path);
      if (!f || placed.has(path)) continue;
      placed.add(path);
      out.push(g.skim && !f.skim ? { ...f, skim: "guide" } : f);
    }
  }
  const rest = files.filter((f) => !placed.has(f.path));
  return o.stale ? [...rest, ...out] : [...out, ...rest];
}

// groupByOrder is the files (in the order's order) in the order's groups.
export function groupByOrder(files: DiffFile[], o: Order): FileGroups {
  const at = new Map(files.map((f, i) => [f.path, i]));
  const placed = new Set<string>();
  const groups: FileGroup[] = [];
  o.groups.forEach((g, n) => {
    const idx = g.files.flatMap((p) => (at.has(p) && !placed.has(p) ? [at.get(p)!] : []));
    g.files.forEach((p) => placed.add(p));
    if (idx.length) groups.push({ key: `guide:${n}`, label: g.name, title: g.why || g.name, skim: g.skim, named: true, why: g.why, files: idx });
  });
  const rest = files.flatMap((f, i) => (placed.has(f.path) ? [] : [i]));
  if (rest.length) {
    const why = o.stale ? "Added since the guide was written" : "Not placed by the guide";
    const g: FileGroup = { key: "rest", label: unplaced(o), title: why, named: true, why, files: rest };
    if (o.stale) groups.unshift(g);
    else groups.push(g);
  }
  return { common: "", groups };
}

export function groupFiles(files: DiffFile[]): FileGroups {
  const reading = files.filter((f) => !f.skim);
  const common = commonDir(reading.map((f) => dirOf(f.path)));
  const groups: FileGroup[] = [];
  files.forEach((f, i) => {
    const dir = dirOf(f.path);
    const key = f.skim ? "skim" : `dir:${dir}`;
    const last = groups[groups.length - 1];
    if (last?.key === key) return void last.files.push(i);
    groups.push(f.skim ? { key, label: "To skim", title: "Lockfiles, generated and renamed-only files", skim: true, files: [i] } : { key, label: dir.slice(common.length), title: dir || "Repo root", files: [i] });
  });
  return { common, groups };
}

// skimWords says why a file is one to skim.
export const skimWords: Record<NonNullable<DiffFile["skim"]>, string> = {
  lockfile: "Lockfile",
  generated: "Generated",
  snapshot: "Snapshot",
  renamed: "Renamed only",
  guide: "Guide says skim",
};
