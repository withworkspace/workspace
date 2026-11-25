import type { Setting, SettingsGroup, SettingsInfo } from "../api";
import type { IconName } from "../ui/Icon";

// The Settings page's sections: which there are, and which settings each
// holds. The server says the settings in groups as the files have them (a
// role's table, [missions], [ui]); the page shows them by what they're
// about, so a role's agent and model are with the other roles' (Agents),
// and what else its table sets is with what it governs (the orchestrator's
// switches with the guarantees, under Missions). A group this doesn't know
// (a new one in ws) is a section of its own, so nothing set is ever
// missing from the page.

// name is a key as it's written in the file: [section] key.
export const name = (s: { section: string; key: string }) => (s.section ? `[${s.section}] ${s.key}` : s.key);

// shown is a value as the page says it: a list (picks) as its items,
// with commas.
export function shown(v: unknown): string {
  if (v === undefined || v === null || v === "") return "";
  if (typeof v === "boolean") return v ? "on" : "off";
  if (Array.isArray(v)) return v.join(", ");
  return String(v);
}

// said is a value as a person reads it: a choice by its label.
export function said(s: Setting, v: unknown): string {
  return s.choices?.find((c) => c.value === v)?.label ?? shown(v);
}

// valueSaid is a setting's value as a line of the page says it, out of
// its control: a switch as On or Off, a choice by its label.
export function valueSaid(s: Setting, v: unknown = s.value): string {
  if (typeof v === "boolean") return v ? "On" : "Off";
  return said(s, v);
}

// A Row is a line of a section: a setting, or one of the page's own. The
// sidebar's two keys ([ui] sidebar and menu) are one row, as they're one
// arrangement; theme, sendKey, keyHints and notify are kept in the
// browser, not in a file; daemon is the daemon's switch.
export type Row =
  | { kind: "setting"; s: Setting; crumb?: string } // crumb: whose it is, said where the row is out of its block (Find)
  | { kind: "places"; settings: Setting[] }
  | { kind: "theme" }
  | { kind: "sendKey" }
  | { kind: "keyHints" }
  | { kind: "notify" }
  | { kind: "daemon" };

// What the page's own rows say.
export const words = {
  places: {
    label: "Sidebar",
    about: "Which places are in the sidebar and in what order, and which are moved out of it into the workspace's menu. Yours, setting either, replaces the team's arrangement.",
  },
  theme: { label: "Theme", about: "Light, dark, or as your system has it." },
  sendKey: {
    label: "Send with",
    about: "The key that sends what you write in a box: a reply, a message to crew, a comment. A form with more than one box keeps its own key.",
  },
  keyHints: {
    label: "Key hints",
    about: "When you press something with the pointer that has a key, ws says which, in passing. A key is in the tooltip of what it's for either way.",
  },
  notify: {
    label: "Desktop notifications",
    about: "A notification for each new thing that needs you, and for what happened that you've asked to be told of below, while this tab is in the background. With it in front, what happened is told in the page instead. The tab's icon lights either way.",
  },
  daemon: {
    label: "The daemon",
    about: "Shutting it down hangs up every crew it holds, as ws daemon stop does, and this page closes with it. ws web starts it again.",
  },
};

// A Block is a run of rows under a heading (or none). Its id is what an
// address's hash names (/settings/missions#orchestrator).
export interface Block {
  id: string;
  title?: string;
  about?: string;
  // under is what a run of blocks are together (Notifications' blocks,
  // one per plugin, under Plugins): said once, above the first.
  under?: Under;
  rows: Row[];
  // roles: the block is the roles' table, each role's agent and model in
  // one line: its rows are those settings, for Find.
  roles?: SettingsGroup[];
}

export interface Section {
  id: string;
  title: string;
  icon: IconName;
  about: string;
  heading?: string; // the heading it starts, in the list of sections
  blocks: Block[];
}

export interface Under {
  title: string;
  about: string;
}

// fromPlugins heads where each plugin's kinds of notification are sent.
export const fromPlugins: Under = {
  title: "Plugins",
  about: "What each plugin tells you of, kind by kind, and where each kind is sent. A plugin's kinds are listed once it has started.",
};

// pluginNotifyGroup is the group of where a plugin's kinds of
// notification are sent (notifications.plugin.jira), or the plugin's name.
const pluginNotifyGroup = (id: string): string | undefined => (id.startsWith("notifications.plugin.") ? id.slice("notifications.plugin.".length) : undefined);

const agent = (s: Setting) => s.kind === "harness" || s.kind === "model";

// sectionsOf is the page's sections, holding info's settings (none, until
// it's loaded: the list of sections is there from the first).
export function sectionsOf(info?: SettingsInfo): Section[] {
  const groups = info?.groups ?? [];
  const section = (id: string, title: string, icon: IconName, about: string, heading?: string): Section => ({ id, title, icon, about, heading, blocks: [] });
  const appearance = section("appearance", "Appearance", "sun", "How the web UI looks.", "Interface");
  const keyboard = section("keyboard", "Keyboard", "keyboard", "The key you send with, and being told of the keys you could have pressed.");
  const notifications = section("notifications", "Notifications", "bell", "What ws tells you of outside the page, and where what happened is sent.");
  let feed: Block | undefined;
  const fromEach: Block[] = [];
  const agents = section("agents", "Agents", "terminal", "The agent CLI each role runs in, and the model it's given.", "How work goes");
  const missions = section("missions", "Missions", "flag", "How a mission is briefed and moved on, and what ws holds on every one.");
  const team = section("team", "Team page", "user", "What the Team page says of each change, and how far back it looks.");
  const repos = section("repos", "Repos", "branch", "How repos are cloned and fetched, and the repos the team has here.");
  const runs = section("runs", "Runs", "play", "Where what ws runs for you and your agents is reached.");
  const process = section("process", "Process", "book", "How work goes here, in prose.");
  const plugins = section(
    "plugins",
    "Plugins",
    "box",
    "Programs that add to the workspace without being part of ws: a view in the sidebar, commands after ws, names for their things, a guide for agents. The daemon runs each as one process, started when it's first needed.",
    "This workspace",
  );
  const yours = section("yours", "Set by you", "pencil", "What's yours alone: your file's settings, each with what it would be without yours, and what this browser keeps.");
  const about = section("about", "About", "info", "Which ws this is and whether a later one's out, and where the workspace keeps its files.");
  const others: Section[] = [];

  const rows = (settings: Setting[], crumb?: string): Row[] => settings.map((s) => ({ kind: "setting", s, crumb }));
  const ui: Row[] = [{ kind: "theme" }];
  const roles: Block = { id: "roles", rows: [], roles: groups.filter((g) => g.runs) };
  agents.blocks.push(roles);

  for (const g of groups) {
    const plugin = pluginNotifyGroup(g.id);
    if (plugin) {
      fromEach.push({ id: `notify-${plugin}`, title: g.title, about: g.about, under: fromPlugins, rows: rows(g.settings, `Notifications from ${g.title}`) });
      continue;
    }
    if (g.runs) {
      roles.rows.push(...rows(g.settings.filter(agent), g.title));
      const rest = g.settings.filter((s) => !agent(s));
      if (rest.length === 0) continue;
      if (g.id === "crew") agents.blocks.push({ id: g.id, title: g.title, rows: rows(rest, g.title) });
      else if (g.id === "team") team.blocks.push({ id: g.id, rows: rows(rest) });
      else missions.blocks.push({ id: g.id, title: g.title, about: g.about, rows: rows(rest, g.title) });
      continue;
    }
    switch (g.id) {
      case "ui": {
        const places = g.settings.filter((s) => s.kind === "places");
        ui.push(...rows(g.settings.filter((s) => s.kind !== "places")));
        if (places.length > 0) ui.push({ kind: "places", settings: places });
        break;
      }
      case "missions":
        missions.blocks.push({ id: g.id, title: g.title, about: g.about, rows: rows(g.settings, g.title) });
        break;
      case "workspace":
        repos.blocks.push({ id: g.id, rows: rows(g.settings) });
        break;
      case "router":
        runs.blocks.push({ id: g.id, title: g.title, about: g.about, rows: rows(g.settings, g.title) });
        break;
      case "notifications":
        feed = { id: "feed", title: "What happened", about: g.about, rows: rows(g.settings, g.title) };
        break;
      case "updates":
        about.blocks.push({ id: g.id, title: g.title, rows: rows(g.settings, g.title) });
        break;
      default:
        others.push({ ...section(g.id, g.title, "sliders", g.about ?? ""), blocks: [{ id: g.id, rows: rows(g.settings) }] });
    }
  }
  appearance.blocks.push({ id: "ui", rows: ui });
  keyboard.blocks.push({ id: "send", rows: [{ kind: "sendKey" }, { kind: "keyHints" }] });
  notifications.blocks.push({ id: "notify", rows: [{ kind: "notify" }] });
  if (feed) notifications.blocks.push(feed);
  notifications.blocks.push(...fromEach);
  about.blocks.push({ id: "daemon", title: "Daemon", rows: [{ kind: "daemon" }] });
  return [appearance, keyboard, notifications, agents, missions, team, repos, runs, ...others, process, plugins, yours, about];
}

// sectionAt is the section an old address's hash named (/settings#process,
// #orchestrator: the page was one once, each group an anchor), or that a
// block's id is in: undefined for one that's nowhere.
export function sectionAt(sections: Section[], id: string): Section | undefined {
  return (
    sections.find((sec) => sec.blocks.some((b) => b.id === id)) ??
    sections.find((sec) => sec.id === id) ??
    sections.find((sec) => sec.blocks.some((b) => b.roles?.some((g) => g.id === id)))
  );
}

// keyOf is a row's key among its section's.
export function keyOf(row: Row): string {
  return row.kind === "setting" ? name(row.s) : row.kind;
}

// textOf is what Find reads of a row.
function textOf(row: Row): string {
  switch (row.kind) {
    case "setting": {
      const s = row.s;
      return [row.crumb, s.label, s.about, name(s), shown(s.value), said(s, s.value)].join(" ");
    }
    case "places":
      return [words.places.label, words.places.about, ...row.settings.map(name)].join(" ");
    default:
      return `${words[row.kind].label} ${words[row.kind].about}`;
  }
}

// wordsOf is what's typed into Find, as the words a row has to have.
export const wordsOf = (query: string) => query.toLowerCase().split(/\s+/).filter(Boolean);

export interface Found {
  section: Section;
  rows: Row[];
}

// find is the rows that have every word of query, by section, in the
// page's order: none for nothing typed.
export function find(sections: Section[], query: string): Found[] {
  const want = wordsOf(query);
  if (want.length === 0) return [];
  const out: Found[] = [];
  for (const section of sections) {
    const rows = section.blocks.flatMap((b) => b.rows).filter((row) => {
      const text = textOf(row).toLowerCase();
      return want.every((w) => text.includes(w));
    });
    if (rows.length > 0) out.push({ section, rows });
  }
  return out;
}

export interface Mine {
  section: Section;
  s: Setting;
  what: string; // what it is, said out of its block: "Crew model"
}

// minesOf is what your file sets, each with the section it's in.
export function minesOf(sections: Section[]): Mine[] {
  const out: Mine[] = [];
  for (const section of sections) {
    for (const b of section.blocks) {
      for (const row of b.rows) {
        if (row.kind === "setting" && row.s.source === "yours") {
          out.push({ section, s: row.s, what: b.roles ? `${row.crumb} ${row.s.label.toLowerCase()}` : row.s.label });
        }
        if (row.kind === "places") for (const s of row.settings) if (s.source === "yours") out.push({ section, s, what: s.label });
      }
    }
  }
  return out;
}
