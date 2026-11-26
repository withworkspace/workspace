import { describe, expect, it } from "vitest";
import type { Setting, SettingsGroup, SettingsInfo } from "../api";
import { find, keyOf, minesOf, type Row, sectionAt, sectionsOf } from "./sections";

const setting = (section: string, key: string, kind: Setting["kind"], o: Partial<Setting> = {}): Setting => ({
  section,
  key,
  kind,
  label: key,
  value: null,
  source: "default",
  ...o,
});

const role = (id: string, title: string, rest: Setting[] = []): SettingsGroup => ({
  id,
  title,
  about: `The ${id}.`,
  runs: { harness: "claude" },
  settings: [setting(id, "harness", "harness", { label: "Agent", value: "claude" }), setting(id, "model", "model", { label: "Model" }), ...rest],
});

// The groups as the server sends them: roles first, then the rest.
const info = (extra: SettingsGroup[] = []): SettingsInfo => ({
  file: "/w/ws.local.toml",
  team: "/w/ws.toml",
  groups: [
    role("crew", "Crew", [setting("crew", "allowed", "picks", { label: "Also allowed" })]),
    role("briefing", "Briefing", [setting("briefing", "in", "choice", { label: "Where" })]),
    role("orchestrator", "Orchestrator", [
      setting("orchestrator", "enabled", "bool", { label: "Runs by itself", value: true }),
      setting("orchestrator", "dispatch", "choice", {
        label: "Starting crew",
        value: "ask",
        yours: "ask",
        source: "yours",
        choices: [
          { value: "auto", label: "Start at once" },
          { value: "ask", label: "Wait for you to launch" },
        ],
      }),
    ]),
    role("conversation", "Conversations"),
    role("team", "Team", [setting("team", "narrative", "bool", { label: "Gists" })]),
    {
      id: "missions",
      title: "Guarantees",
      about: "What ws holds.",
      settings: [setting("missions", "review_before_pr", "bool", { label: "Your review before a pull request", about: "ws opens a pull request only once you approve." })],
    },
    { id: "workspace", title: "Repos", settings: [setting("", "git", "choice", { label: "Git protocol" })] },
    { id: "router", title: "Local names", settings: [setting("router", "enabled", "bool", { label: "Names" }), setting("router", "name", "text", { label: "Name", default: "w" })] },
    { id: "updates", title: "Updates", settings: [setting("updates", "check", "bool", { label: "Look for updates", default: true })] },
    {
      id: "ui",
      title: "Appearance",
      settings: [setting("ui", "accent", "color", { label: "Accent" }), setting("ui", "sidebar", "places", { label: "Sidebar" }), setting("ui", "menu", "places", { label: "In the menu" })],
    },
    ...extra,
  ],
});

const rowsOf = (id: string, sections = sectionsOf(info())) => sections.find((s) => s.id === id)!.blocks.map((b) => [b.id, b.rows.map(keyOf)]);
const settingsIn = (rows: Row[]) => rows.flatMap((r) => (r.kind === "setting" ? [r.s] : r.kind === "places" ? r.settings : []));

describe("sectionsOf", () => {
  it("lists the sections before anything's loaded", () => {
    expect(sectionsOf().map((s) => s.id)).toEqual(["appearance", "keyboard", "notifications", "agents", "missions", "team", "repos", "runs", "process", "plugins", "yours", "about"]);
    expect(sectionsOf().filter((s) => s.heading).map((s) => [s.heading, s.id])).toEqual([
      ["Interface", "appearance"],
      ["How work goes", "agents"],
      ["This workspace", "plugins"],
    ]);
  });

  it("puts every role's agent and model in the one table, and what else crew's table sets beside it", () => {
    expect(rowsOf("agents")).toEqual([
      ["roles", ["[crew] harness", "[crew] model", "[briefing] harness", "[briefing] model", "[orchestrator] harness", "[orchestrator] model", "[conversation] harness", "[conversation] model", "[team] harness", "[team] model"]],
      ["crew", ["[crew] allowed"]],
    ]);
    const roles = sectionsOf(info()).find((s) => s.id === "agents")!.blocks[0].roles;
    expect(roles?.map((g) => g.id)).toEqual(["crew", "briefing", "orchestrator", "conversation", "team"]);
  });

  it("puts what else a role's table sets with the guarantees, each under the role's name", () => {
    const missions = sectionsOf(info()).find((s) => s.id === "missions")!;
    expect(missions.blocks.map((b) => [b.id, b.title, b.rows.map(keyOf)])).toEqual([
      ["briefing", "Briefing", ["[briefing] in"]],
      ["orchestrator", "Orchestrator", ["[orchestrator] enabled", "[orchestrator] dispatch"]],
      ["missions", "Guarantees", ["[missions] review_before_pr"]],
    ]);
    expect(missions.blocks[1].about).toBe("The orchestrator.");
  });

  it("gives the Team page its own, the repos theirs, Runs the local names, and appearance the theme first and the sidebar's two keys as one row", () => {
    expect(rowsOf("team")).toEqual([["team", ["[team] narrative"]]]);
    expect(rowsOf("repos")).toEqual([["workspace", ["git"]]]);
    expect(rowsOf("runs")).toEqual([["router", ["[router] enabled", "[router] name"]]]);
    // Whether the daemon looks for updates is with which ws this is, before the daemon's switch.
    expect(rowsOf("about")).toEqual([
      ["updates", ["[updates] check"]],
      ["daemon", ["daemon"]],
    ]);
    expect(rowsOf("appearance")).toEqual([["ui", ["theme", "[ui] accent", "places"]]]);
  });

  it("holds every setting once", () => {
    const sent = info().groups.flatMap((g) => g.settings);
    const held = sectionsOf(info()).flatMap((s) => s.blocks.flatMap((b) => settingsIn(b.rows)));
    expect(held).toHaveLength(sent.length);
    expect(new Set(held)).toEqual(new Set(sent));
  });

  it("makes a group it doesn't know a section of its own, before the process", () => {
    const sections = sectionsOf(info([{ id: "github", title: "GitHub", about: "How ws asks GitHub.", settings: [setting("github", "host", "text")] }]));
    expect(sections.map((s) => s.id)).toEqual(["appearance", "keyboard", "notifications", "agents", "missions", "team", "repos", "runs", "github", "process", "plugins", "yours", "about"]);
    expect(rowsOf("github", sections)).toEqual([["github", ["[github] host"]]]);
  });

  it("puts a role it doesn't know in the table, and what else it sets under Missions", () => {
    const sections = sectionsOf(info([role("planner", "Planner", [setting("planner", "depth", "number")])]));
    expect(rowsOf("agents", sections)[0][1]).toContain("[planner] model");
    expect(rowsOf("missions", sections).at(-1)).toEqual(["planner", ["[planner] depth"]]);
  });
});

describe("sectionAt", () => {
  const sections = sectionsOf(info());
  const at = (id: string) => sectionAt(sections, id)?.id;

  it("finds where the one page's anchors went", () => {
    expect(at("orchestrator")).toBe("missions");
    expect(at("briefing")).toBe("missions");
    expect(at("missions")).toBe("missions");
    expect(at("crew")).toBe("agents");
    expect(at("conversation")).toBe("agents");
    expect(at("team")).toBe("team");
    expect(at("workspace")).toBe("repos");
    expect(at("ui")).toBe("appearance");
    expect(at("process")).toBe("process");
  });

  it("has nowhere for what was never there", () => {
    expect(at("nope")).toBeUndefined();
  });
});

describe("find", () => {
  const sections = sectionsOf(info());
  const found = (q: string) => find(sections, q).map((f) => [f.section.id, f.rows.map(keyOf)]);

  it("finds nothing for nothing typed", () => {
    expect(found("")).toEqual([]);
    expect(found("   ")).toEqual([]);
  });

  it("reads a setting's name, what it says, its key and whose it is", () => {
    expect(found("gists")).toEqual([["team", ["[team] narrative"]]]);
    expect(found("approve")).toEqual([["missions", ["[missions] review_before_pr"]]]);
    expect(found("review_before")).toEqual([["missions", ["[missions] review_before_pr"]]]);
    expect(found("conversations model")).toEqual([["agents", ["[conversation] model"]]]);
  });

  it("reads what it's set to, as the page says it", () => {
    expect(found("wait for you")).toEqual([["missions", ["[orchestrator] dispatch"]]]);
  });

  it("wants every word", () => {
    expect(found("crew allowed")).toEqual([["agents", ["[crew] allowed"]]]);
    expect(found("crew gists")).toEqual([]);
  });

  it("finds the page's own rows", () => {
    expect(found("theme")).toEqual([["appearance", ["theme"]]]);
    expect(found("sidebar")).toEqual([["appearance", ["places"]]]);
    expect(found("send with")).toEqual([["keyboard", ["sendKey"]]]);
    expect(found("key hints")).toEqual([["keyboard", ["keyHints"]]]);
    expect(found("notification")).toEqual([["notifications", ["notify"]]]);
    expect(found("shutting")).toEqual([["about", ["daemon"]]]);
  });
});

describe("minesOf", () => {
  it("lists what your file sets, with where it is and what it is out of its table", () => {
    const i = info();
    const model = i.groups[0].settings[1];
    model.source = "yours";
    model.yours = "opus";
    const menu = i.groups.at(-1)!.settings[2];
    menu.source = "yours";
    expect(minesOf(sectionsOf(i)).map((m) => [m.section.id, m.what])).toEqual([
      ["appearance", "In the menu"],
      ["agents", "Crew model"],
      ["missions", "Starting crew"],
    ]);
  });

  it("puts where what happened is sent under Notifications, after the desktop's switch", () => {
    const feed: Setting = { section: "notifications", key: "missions", label: "Missions", about: "A mission ended by itself.", kind: "choice", value: "feed", source: "default", choices: [] } as unknown as Setting;
    const sections = sectionsOf({ groups: [{ id: "notifications", title: "Notifications", about: "Where each source is sent.", settings: [feed] }] } as unknown as SettingsInfo);
    const notifications = sections.find((s) => s.id === "notifications")!;
    expect(notifications.blocks.map((b) => b.id)).toEqual(["notify", "feed"]);
    expect(notifications.blocks[1].rows).toEqual([{ kind: "setting", s: feed, crumb: "Notifications" }]);
    // And it's no section of its own.
    expect(sections.filter((s) => s.id === "notifications")).toHaveLength(1);
  });

  it("puts each plugin's kinds of notification under Notifications, a block each, under Plugins", () => {
    const kind = (key: string, label: string) => ({ section: "notifications.plugin.jira", key, label, kind: "choice", value: "feed", source: "default", choices: [] }) as unknown as Setting;
    const assigned = kind("assigned", "Given to you");
    const sections = sectionsOf({
      groups: [
        { id: "notifications", title: "Notifications", settings: [kind("missions", "Missions")] },
        { id: "notifications.plugin.jira", title: "jira", about: "What jira tells you of.", settings: [assigned, kind("new", "New on the board")] },
        { id: "notifications.plugin.sketch", title: "sketch", settings: [kind("shared", "Shared with you")] },
      ],
    } as unknown as SettingsInfo);
    const notifications = sections.find((s) => s.id === "notifications")!;
    expect(notifications.blocks.map((b) => [b.id, b.title, b.under?.title])).toEqual([
      ["notify", undefined, undefined],
      ["feed", "What happened", undefined],
      ["notify-jira", "jira", "Plugins"],
      ["notify-sketch", "sketch", "Plugins"],
    ]);
    expect(notifications.blocks[2].rows[0]).toEqual({ kind: "setting", s: assigned, crumb: "Notifications from jira" });
    // No section of their own.
    expect(sections.some((s) => s.id.startsWith("notifications.plugin"))).toBe(false);
  });
});
