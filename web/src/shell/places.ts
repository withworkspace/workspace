import type { IconName } from "../ui/Icon";

// Places are where the sidebar goes: ws's own (Missions, Conversations…)
// and each view a plugin adds. You arrange them: their order in the
// sidebar, and which are moved out of it into the workspace's menu (from
// its name), kept as [ui] sidebar and [ui] menu in ws.local.toml, each a
// list of ids. A place in neither list goes at the sidebar's end, in the
// order it comes; an id that isn't a place now (a plugin's view that's
// gone) is passed by, and kept where it was when you arrange again.

export interface Place {
  // id is what [ui] sidebar and menu name it by: ws's own by its name
  // ("missions"), a plugin's view by its address ("x/jira/board").
  id: string;
  to: string;
  icon: IconName;
  label: string;
  count?: number;
  countTitle?: string;
  match: (path: string) => boolean;
  // under is the group a plugin's view names: its title shows above it
  // in the sidebar.
  under?: string;
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
  if (!saved) return { sidebar: places, menu: [] };
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
  return { sidebar: [...sidebar, ...places.filter((p) => !taken.has(p.id))], menu };
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
