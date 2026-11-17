import type { Workspace } from "../api";
import type { GroupKey, Item, Memory } from "./find";

// What Find remembers of you, in the browser, for each workspace: where
// you've been (the latest first), and what you chose for what you typed.
// It's what "Recent" shows before anything's typed, and why somewhere you
// go often comes above somewhere you don't.

export interface Visit {
  id: string; // an Item's id
  group: GroupKey; // the kind of thing it is
  kind: string;
  title: string;
  hint?: string;
  to: string;
  at: number;
}

interface Stored extends Memory {
  visits: Visit[];
}

const most = 40;
const key = (root: string) => `ws:find:${root}`;

export function loadMemory(root: string): Stored {
  try {
    const got = JSON.parse(localStorage.getItem(key(root)) ?? "") as Partial<Stored>;
    return {
      visits: Array.isArray(got.visits) ? got.visits : [],
      chosen: got.chosen && typeof got.chosen === "object" ? got.chosen : {},
    };
  } catch {
    return { visits: [], chosen: {} };
  }
}

function save(root: string, m: Stored) {
  try {
    localStorage.setItem(key(root), JSON.stringify(m));
  } catch {
    // Nowhere to keep it: Find works without.
  }
}

// noteVisit keeps somewhere you've gone, at the front. One known only by
// its name (a pull request opened by its address) doesn't replace what it
// was found as, with its title.
export function noteVisit(root: string, v: Visit) {
  const m = loadMemory(root);
  const was = m.visits.find((x) => x.id === v.id);
  const kept = was && v.title === v.hint ? { ...was, to: v.to, at: v.at } : v;
  m.visits = [kept, ...m.visits.filter((x) => x.id !== v.id)].slice(0, most);
  save(root, m);
}

// noteChosen keeps what you chose for what you'd typed.
export function noteChosen(root: string, q: string, id: string) {
  const typed = q.trim().toLowerCase();
  if (!typed) return;
  const m = loadMemory(root);
  const chosen = Object.entries({ ...m.chosen, [typed]: id }).slice(-200);
  m.chosen = Object.fromEntries(chosen);
  save(root, m);
}

// visitOf is an item as somewhere you went, if it's somewhere.
export function visitOf(it: Item, at: number): Visit | undefined {
  if (!it.to || it.group === "places" || it.group === "do") return undefined;
  // The kind of thing it is, whichever group it was shown in.
  const group = (it.id.startsWith("pr:") ? "pulls" : it.id.startsWith("file:") || it.id.startsWith("dir:") ? "files" : it.group) as GroupKey;
  return {
    id: it.id,
    group,
    kind: it.kind,
    title: it.title,
    hint: it.hint,
    to: it.to,
    at,
  };
}

// visitAt is where the address bar is, as somewhere you went: a mission,
// a page, a conversation, an artifact, a pull request, a file. Anything
// else (a list, a mission's crew) isn't kept.
export function visitAt(pathname: string, search: string, hash: string, w: Workspace, at: number): Visit | undefined {
  const parts = pathname.split("/").filter(Boolean).map(decodeURIComponent);
  const [a, b, c] = parts;
  if (a === "missions" && b && parts.length === 2) {
    const m = w.missions.find((x) => x.slug === b);
    return (
      m && {
        id: `mission:${b}`,
        group: "missions",
        kind: "Mission",
        title: m.title,
        hint: b,
        to: pathname,
        at,
      }
    );
  }
  if (a === "pages" && b && parts.length === 2) {
    const p = w.pages?.find((x) => x.id === b);
    return (
      p && {
        id: `page:${b}`,
        group: "pages",
        kind: "Page",
        title: p.title,
        hint: b,
        to: pathname,
        at,
      }
    );
  }
  if (a === "conversations" && b && parts.length === 2) {
    const t = w.conversations.find((x) => x.id === b);
    return (
      t && {
        id: `talk:${b}`,
        group: "talks",
        kind: "Conversation",
        title: t.title || t.opening,
        hint: b,
        to: pathname,
        at,
      }
    );
  }
  if (a === "artifacts" && b && parts.length <= 3) {
    const art = w.artifacts?.find((x) => x.id === b);
    return (
      art && {
        id: `artifact:${b}`,
        group: "artifacts",
        kind: "Artifact",
        title: art.title,
        hint: b,
        to: `/artifacts/${b}`,
        at,
      }
    );
  }
  if (a === "reviews" && b && c && parts.length === 3) {
    const ref = `${b}#${c}`;
    return {
      id: `pr:${ref}`,
      group: "pulls",
      kind: "Pull request",
      title: ref,
      hint: ref,
      to: pathname,
      at,
    };
  }
  if (a === "files" && b) {
    const path = new URLSearchParams(search).get("path");
    if (!path) return undefined;
    const title = c ? `${b}/${c}:${path}` : `${b}/${path}`;
    return {
      id: `file:${title}`,
      group: "files",
      kind: "File",
      title,
      to: pathname + search + hash,
      at,
    };
  }
  return undefined;
}

// recentItems are visits as items of the list: with what the workspace
// says of each now, where it still has it.
export function recentItems(visits: Visit[], local: Item[]): Item[] {
  const byId = new Map(local.map((it) => [it.id, it]));
  const out: Item[] = [];
  for (const v of visits) {
    const now = byId.get(v.id);
    if (now) {
      out.push({ ...now, group: "recent" });
      continue;
    }
    // What the page doesn't hold (a file, a pull request) is as it was.
    if (v.group !== "files" && v.group !== "pulls") continue; // gone: a mission archived, a page removed
    out.push({
      id: v.id,
      group: "recent",
      kind: v.kind,
      title: v.title,
      hint: v.hint === v.title ? undefined : v.hint,
      icon: v.group === "pulls" ? "pr" : "file",
      score: 0,
      to: v.to,
      ref: v.group === "pulls" ? v.hint?.split(" · ")[0] : v.title,
      path: v.group === "files",
    });
  }
  return out;
}
