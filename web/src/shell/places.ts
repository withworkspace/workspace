import type { Plugin, Workspace } from "../api";
import { type IconName, icons } from "../ui/Icon";
import type { Waiting } from "./notify";

// Places are where the sidebar goes: ws's own (Missions, Conversations…)
// and each view a plugin adds. You arrange them: their order in the
// sidebar, and which are moved out of it into the workspace's menu (from
// its name), kept as [ui] sidebar and [ui] menu in ws.local.toml, each a
// list of ids. A place in neither list goes at the end of its home (the
// sidebar, unless it says the menu), in the order it comes; an id that
// isn't a place now (a plugin's view that's gone) is passed by, and kept
// where it was when you arrange again.

export interface Place {
  // id is what [ui] sidebar and menu name it by: ws's own by its name
  // ("missions"), a plugin's view by its address ("x/jira/board").
  id: string;
  to: string;
  icon: IconName;
  label: string;
  count?: number;
  countTitle?: string;
  // keys is the keys that go there, as Keys reads them ("g m"): one of
  // ws's own has them, from goKeys.
  keys?: string;
  match: (path: string) => boolean;
  // under is the group a plugin's view names: its title shows above it
  // in the sidebar.
  under?: string;
  // home is where it goes until you arrange it: the menu, for a place
  // most needn't see every day (Runs). The sidebar, unsaid.
  home?: "menu";
}

// Arrangement is where the places go, by id.
export interface Arrangement {
  sidebar: string[];
  menu: string[];
}

// Arranged is the places as arranged.
export interface Arranged {
  sidebar: Place[];
  menu: Place[];
}

export function arrange(places: Place[], saved?: Arrangement): Arranged {
  saved ??= { sidebar: [], menu: [] };
  const byId = new Map(places.map((p) => [p.id, p]));
  const taken = new Set<string>();
  const pick = (ids: string[]) =>
    ids.flatMap((id) => {
      const p = byId.get(id);
      if (!p || taken.has(id)) return [];
      taken.add(id);
      return [p];
    });
  const menu = pick(saved.menu);
  const sidebar = pick(saved.sidebar);
  const rest = places.filter((p) => !taken.has(p.id));
  return { sidebar: [...sidebar, ...rest.filter((p) => p.home !== "menu")], menu: [...menu, ...rest.filter((p) => p.home === "menu")] };
}

// saving is what's written for an arrangement: its ids, and after each
// list's the ids it had that aren't places now, so a plugin that's away
// a while finds its views where they were.
export function saving(arranged: Arranged, places: Place[], saved?: Arrangement): Arrangement {
  const here = new Set(places.map((p) => p.id));
  const away = (ids: string[] = []) => ids.filter((id) => !here.has(id));
  return {
    sidebar: [...arranged.sidebar.map((p) => p.id), ...away(saved?.sidebar)],
    menu: [...arranged.menu.map((p) => p.id), ...away(saved?.menu)],
  };
}

export const sameArrangement = (a: Arrangement, b: Arrangement) =>
  a.sidebar.join("\n") === b.sidebar.join("\n") && a.menu.join("\n") === b.menu.join("\n");

// move takes ids out of wherever they are and puts them into a list:
// before or after a place in it, or at its end.
export function move(arranged: Arranged, ids: string[], into: keyof Arranged, at?: { id: string; after: boolean }): Arranged {
  const all = [...arranged.sidebar, ...arranged.menu];
  const moving = ids.flatMap((id) => all.find((p) => p.id === id) ?? []);
  if (!moving.length) return arranged;
  const gone = new Set(moving.map((p) => p.id));
  const out: Arranged = { sidebar: arranged.sidebar.filter((p) => !gone.has(p.id)), menu: arranged.menu.filter((p) => !gone.has(p.id)) };
  const list = out[into];
  let i = at ? list.findIndex((p) => p.id === at.id) : -1;
  if (i < 0) i = list.length;
  else if (at?.after) i++;
  list.splice(i, 0, ...moving);
  return out;
}

// headed is the group title to show above each place in a list: where a
// plugin's group starts.
export function headed(list: Place[]): (string | undefined)[] {
  return list.map((p, i) => (p.under && p.under !== list[i - 1]?.under ? p.under : undefined));
}

// placesOf is the sidebar's places as they come: ws's own, then each
// view a plugin adds, those naming a group (under) together beneath its
// name, the groups by name. Where each goes is yours to arrange
// (shell/places.ts).
// goKeys is the letter after g that goes to each of ws's own places, by
// where it goes: g then m is the missions (useGlobalKeys binds them, a
// place's link says them). Documents has two, as it was Pages once.
export const goKeys: Record<string, string> = { m: "/", c: "/conversations", d: "/documents", p: "/documents", a: "/artifacts", r: "/reviews", t: "/team", f: "/files", o: "/orchestrator" };

// goKeysTo is the keys that go to an address, if it has them: "g m".
export function goKeysTo(to: string): string | undefined {
  const letter = Object.keys(goKeys).find((k) => goKeys[k] === to);
  return letter && `g ${letter}`;
}

export function placesOf(workspace: Workspace | undefined, waiting: (Waiting & { kind: string })[], plugins?: Plugin[]): Place[] {
  const places: Place[] = [
    { id: "missions", to: "/", icon: "flag", label: "Missions", count: waiting.filter((w) => w.kind === "mission").length, match: (p) => p === "/" || p.startsWith("/missions/") },
    { id: "conversations", to: "/conversations", icon: "message", label: "Conversations", count: waiting.filter((w) => w.kind === "talk").length, match: (p) => p.startsWith("/conversations") },
    { id: "documents", to: "/documents", icon: "file", label: "Documents", match: (p) => p === "/documents" || p.startsWith("/documents/") || p.startsWith("/pages/") || p === "/docs" },
    { id: "artifacts", to: "/artifacts", icon: "artifact", label: "Artifacts", match: (p) => p === "/artifacts" || p.startsWith("/artifacts/") },
    {
      id: "reviews",
      to: "/reviews",
      icon: "pr",
      label: "Reviews",
      count: workspace?.reviews?.count,
      countTitle: "pull requests need you",
      match: (p) => p === "/reviews" || p.startsWith("/reviews/"),
    },
    { id: "team", to: "/team", icon: "user", label: "Team", match: (p) => p === "/team" },
    { id: "files", to: "/files", icon: "layers", label: "Repos", match: (p) => p === "/files" || p.startsWith("/files/") },
    { id: "orchestrator", to: "/orchestrator", icon: "sparkle", label: "Orchestrator", match: (p) => p.startsWith("/orchestrator") },
    { id: "runs", to: "/runs", icon: "play", label: "Runs", match: (p) => p === "/runs", home: "menu" },
    { id: "automations", to: "/automations", icon: "bolt", label: "Automations", match: (p) => p === "/automations" || p.startsWith("/automations/"), home: "menu" },
  ];
  const own = places.map((p) => ({ ...p, keys: goKeysTo(p.to) }));

  // The workspace's apps: its own recipes with a page, each its run framed.
  const apps: Place[] = (workspace?.apps ?? []).map((a) => ({
    id: `apps/${a.name}`,
    to: `/apps/${a.name}`,
    icon: a.icon && a.icon in icons ? (a.icon as IconName) : "box",
    label: a.title,
    match: (p: string) => p === `/apps/${a.name}`,
  }));
  const views: Place[] = (plugins ?? []).flatMap((plugin) =>
    plugin.views.map((view) => {
      const to = `/x/${plugin.name}${view.path === "/" ? "" : view.path}`;
      return {
        id: to.slice(1),
        to,
        icon: plugin.state === "failed" ? "alert" : (view.icon && view.icon in icons ? (view.icon as IconName) : "box"),
        label: view.title,
        count: view.badge,
        countTitle: "to see",
        match: (p: string) => p === `/x/${plugin.name}` || p.startsWith(`/x/${plugin.name}/`),
        under: view.under || undefined,
      };
    }),
  );
  // Those in no group first, then the groups by name; within each, as they came.
  const group = (p: Place) => p.under ?? "";
  views.sort((a, b) => (group(a) === group(b) ? 0 : group(a) === "" ? -1 : group(b) === "" ? 1 : group(a).localeCompare(group(b))));
  return [...own, ...apps, ...views];
}
