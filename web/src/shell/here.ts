import type { Workspace } from "../api";
import type { SpaceRef } from "../api";
import { GROUND, type Known, namedOf, type PageRef, thingOpenAt } from "../refs/refs";
import { actIcon, actsOn, type Found, foundItem, type Item, missionDo } from "./find";
import { visitAt } from "./recent";

// What Find shows of where you are before anything's typed: the thing the
// page is of (Here), what can be done to it, and what's one link from it
// in the graph (GET /api/around): the mission's brief and handoffs, the
// talk it came from, the issue it's about, whatever names it.

type Thing = Extract<PageRef, { kind: "thing" }>;

// Here is the thing a page is of: what the graph's asked about (ref, and
// scope for what's named in a mission), what it is among Find's rows
// (id), and what it's called.
export interface Here {
  ref: string;
  scope?: string;
  id: string;
  title?: string;
  thing?: Thing; // a plugin's thing, open in its app
}

// hereAt is the thing the page at pathname is of, if it's of one: a
// mission (any page of it), a conversation, a page, an artifact, a pull
// request, a file, a space's document, or a plugin's thing open in its
// app. A list is of nothing.
export function hereAt(pathname: string, search: string, w: Workspace, known: Known): Here | undefined {
  const parts = pathname.split("/").filter(Boolean).map(decodeURIComponent);
  const [a, b, c] = parts;
  if (a === "missions" && b) {
    const m = w.missions.find((x) => x.slug === b);
    return m && { ref: b, id: `mission:${b}`, title: m.title };
  }
  if (a === "documents" && b === "local" && c) {
    const p = w.pages?.find((x) => x.id === c);
    return p && { ref: c, id: `page:${c}`, title: p.title };
  }
  if (a === "documents" && b && parts.length > 2) {
    const ref = `${b}:${parts.slice(2).join("/")}`;
    return { ref, id: `document:${ref}`, title: parts[parts.length - 1] };
  }
  if (a === "x" && b) {
    const thing = thingOpenAt(b, "/" + pathname.split("/").filter(Boolean).slice(2).join("/"), known);
    return thing && { ref: thing.id, id: `thing:${thing.plugin}/${thing.id}`, title: thing.id, thing };
  }
  const v = visitAt(pathname, search, "", w, 0);
  if (!v) return undefined;
  const ref = v.id.slice(v.id.indexOf(":") + 1);
  return { ref, id: v.id, title: v.title };
}

// Near is a thing one link from where you are (web.NearFound): how it's
// linked, and which way (out: where you are points at it).
export type Near = Found & { how: string; out?: boolean };

// What's been answered for each thing, kept while the page is open: Find
// opened again on the same thing shows it at once, and asks again.
const kept = new Map<string, Near[]>();

const keyOf = (h: Here) => `${h.scope ?? ""}\n${h.ref}`;

// keptAround is what was last answered of what's around here.
export function keptAround(h: Here): Near[] {
  return kept.get(keyOf(h)) ?? [];
}

// around asks the server what's one link from here: as the records stand,
// or fresh, once what's edited outside ws is read again (a brief just
// written isn't linked until it's read).
export async function around(h: Here, fresh: boolean, signal?: AbortSignal): Promise<Near[]> {
  const params = new URLSearchParams({ ref: h.ref, limit: "12" });
  if (h.scope) params.set("scope", h.scope);
  if (fresh) params.set("fresh", "1");
  const res = await fetch(`/api/around?${params}`, { signal });
  if (!res.ok) throw new Error(`${res.status} ${res.statusText}`);
  const got = ((await res.json()) as { results?: Near[] }).results ?? [];
  kept.set(keyOf(h), got);
  return got;
}

// hereItems is what can be done to the thing the page is of: a mission's
// headline, where Find can do it outright; a plugin's thing's actions, and
// ws's own (start a mission about it).
export function hereItems(h: Here | undefined, w: Workspace, known: Known): Item[] {
  if (h?.thing) {
    const thing = h.thing;
    return actsOn(namedOf(thing, known)).map((a) => ({
      id: `here:${a.ws ? "ws" : "its"}:${a.id}`,
      group: "here" as const,
      kind: "Do",
      title: a.title,
      hint: a.hint,
      icon: actIcon(a.icon),
      score: 0,
      doing: { thing, does: a },
    }));
  }
  if (h?.id.startsWith("mission:")) {
    const m = w.missions.find((x) => x.slug === h.ref);
    const does = m && missionDo(m);
    return does ? [{ ...does, group: "here" }] : [];
  }
  return [];
}

// How each kind of link is said of what's at its other end, from where
// you are: out, where you are points at it; in, it points at where you
// are.
const said: Record<string, [out: string, into: string]> = {
  about: ["what this is about", "about this"],
  from: ["where this came from", "came from this"],
  in: ["this is in it", "in this"],
  mentions: ["named here", "names this"],
  lands: ["where this lands", "lands this"],
  of: ["what this is of", "of this"],
  on: ["where this is", "on this"],
};

// howSaid is how a thing around where you are is linked to it, in words.
export function howSaid(n: Near): string {
  const s = said[n.how];
  return s ? s[n.out ? 0 : 1] : n.how;
}

// nearItems are what's around where you are as Find's rows: each as the
// page holds it where it does (a mission with its status, a conversation),
// and otherwise as what ws records is drawn; a mission's brief as its
// brief; each saying how it's linked.
export function nearItems(near: Near[], h: Here, local: Item[], spaces?: SpaceRef[]): Item[] {
  const byId = new Map(local.map((it) => [it.id, it]));
  return near.map((n) => {
    const found = foundItem(asFound(n), spaces);
    const held = localOf(n, byId);
    let it = held ?? found;
    if (!held && n.place === "brief" && n.scope) it = { ...found, kind: "Brief", icon: "flag", to: `/missions/${encodeURIComponent(n.scope)}?tab=brief` };
    // Where it's read is beside the point when it's where you are.
    const notes = held ? [] : (found.notes ?? []).filter((x) => x !== h.ref);
    return { ...it, group: "linked" as const, score: 0, at: undefined, notes: [howSaid(n), ...notes] };
  });
}

// asFound is a thing around here as Find finds one: a space's document
// (keyed as its repo's file) as a document found by what it says is, to
// open in Documents; the rest as what ws records.
function asFound(n: Near): Found {
  if (n.thing !== "document" || n.scope) return n;
  const [repo, ...rest] = n.ref.split("/");
  const path = rest.join("/");
  return { kind: "document", ref: `${repo}:${path}`, title: n.title, repo, worktree: GROUND, path, score: 0 };
}

// localOf is the row the page holds for a thing ws records, if it holds
// one: a mission, a conversation, a page, an artifact.
function localOf(n: Near, byId: Map<string, Item>): Item | undefined {
  if (n.thing === "mission") return byId.get(`mission:${n.scope}`);
  if (n.scope) return undefined;
  for (const kind of ["talk", "page", "artifact"]) {
    const it = byId.get(`${kind}:${n.ref}`);
    if (it) return it;
  }
  return undefined;
}

// nearIds are the rows around where you are, by every id they may be
// found by as something's typed: as the page holds them, and as what ws
// records.
export function nearIds(items: Item[], near: Near[]): Set<string> {
  const ids = new Set(items.map((it) => it.id));
  for (const n of near) ids.add(`said:${n.scope ?? ""}/${n.ref}`);
  return ids;
}
