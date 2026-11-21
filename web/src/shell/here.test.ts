import { describe, expect, it } from "vitest";
import type { Workspace } from "../api";
import type { Known } from "../refs/refs";
import { build, type Item, localItems } from "./find";
import { hereAt, hereItems, type Near, nearIds, nearItems } from "./here";

const workspace = {
  name: "test",
  root: "/work/ws",
  missions: [
    {
      slug: "fix-token-refresh",
      title: "Fix token refresh",
      state: "active",
      created: "",
      updated: "",
      headline: { group: "needs-you", kind: "asks", label: "ready", text: "Its work is ready", action: { verb: "accept", label: "accept it" } },
      capsules: [],
      crew: [],
    },
    {
      slug: "cache",
      title: "Cache the sessions",
      state: "active",
      created: "",
      updated: "",
      headline: { group: "working", kind: "working", label: "working", text: "", action: {} },
      capsules: [],
      crew: [],
    },
  ],
  conversations: [{ id: "w4", title: "Why is billing slow?", opening: "why", status: "open", at: "", updated: "" }],
  pages: [{ id: "p3", title: "Token cache notes", updated: "" }],
} as unknown as Workspace;

const known: Known = {
  repos: new Map(),
  missions: ["fix-token-refresh", "cache"],
  pages: ["p3"],
  talks: ["w4"],
  things: [
    {
      plugin: "jira",
      what: "issue",
      title: "Jira issue",
      prefixes: ["RAM-"],
      path: "/issue/{ref}",
      actions: [{ id: "move", title: "Move it" }],
    },
  ],
};

const local = localItems(workspace);
const titles = (items: Item[]) => items.map((it) => it.title);

describe("where you are", () => {
  it("is the thing the page is of, any page of a mission being the mission's", () => {
    expect(hereAt("/missions/fix-token-refresh", "", workspace, known)).toEqual({ ref: "fix-token-refresh", id: "mission:fix-token-refresh", title: "Fix token refresh" });
    expect(hereAt("/missions/cache/crew/c1", "", workspace, known)?.id).toBe("mission:cache");
    expect(hereAt("/conversations/w4", "", workspace, known)).toMatchObject({ ref: "w4", id: "talk:w4" });
    expect(hereAt("/documents/local/p3", "", workspace, known)).toMatchObject({ ref: "p3", id: "page:p3", title: "Token cache notes" });
    expect(hereAt("/documents/records/adr/0001.md", "", workspace, known)).toMatchObject({ ref: "records:adr/0001.md", id: "document:records:adr/0001.md" });
    expect(hereAt("/reviews/demo/12", "", workspace, known)).toMatchObject({ ref: "demo#12", id: "pr:demo#12" });
    expect(hereAt("/files/demo", "?path=main.go", workspace, known)).toMatchObject({ ref: "demo/main.go" });
  });

  it("is a plugin's thing where its app has one open, by where its kind opens", () => {
    expect(hereAt("/x/jira/issue/RAM-12", "", workspace, known)).toMatchObject({ ref: "RAM-12", id: "thing:jira/RAM-12", thing: { kind: "thing", plugin: "jira", id: "RAM-12" } });
    expect(hereAt("/x/jira/board", "", workspace, known)).toBeUndefined();
    expect(hereAt("/x/other/issue/RAM-12", "", workspace, known)).toBeUndefined();
  });

  it("is what a plugin's page said it shows beyond its address, over the address, if ws can read it", () => {
    // An issue in a panel over the board: the address says nothing of it.
    expect(hereAt("/x/jira/board", "", workspace, known, { plugin: "jira", name: "RAM-77" })).toMatchObject({ ref: "RAM-77", thing: { id: "RAM-77" } });
    // Over the issue's own page, too: what's open in front is what's here.
    expect(hereAt("/x/jira/issue/RAM-12", "", workspace, known, { plugin: "jira", name: "RAM-77" })?.ref).toBe("RAM-77");
    // Any of ws's names: a mission.
    expect(hereAt("/x/jira/board", "", workspace, known, { plugin: "jira", name: "cache" })).toMatchObject({ ref: "cache", id: "mission:cache", title: "Cache the sessions" });
    // Said by another plugin's page, or not a name ws can read: the address.
    expect(hereAt("/x/jira/issue/RAM-12", "", workspace, known, { plugin: "other", name: "RAM-77" })?.ref).toBe("RAM-12");
    expect(hereAt("/x/jira/issue/RAM-12", "", workspace, known, { plugin: "jira", name: "a word" })?.ref).toBe("RAM-12");
    // Only on that plugin's pages.
    expect(hereAt("/missions/cache", "", workspace, known, { plugin: "jira", name: "RAM-77" })?.ref).toBe("cache");
  });

  it("is nothing on a list, or a mission that's gone", () => {
    for (const p of ["/", "/documents", "/reviews", "/conversations", "/missions/gone"]) expect(hereAt(p, "", workspace, known)).toBeUndefined();
  });
});

describe("what can be done here", () => {
  it("is a mission's headline first, by what's done, then what changes it and its brief", () => {
    const mission = hereAt("/missions/fix-token-refresh", "", workspace, known);
    const items = hereItems(mission, workspace, known);
    expect(items[0]).toMatchObject({ group: "here", title: "Accept it", doing: { act: "accept", slug: "fix-token-refresh" } });
    expect(items.map((it) => it.title).slice(1)).toEqual(expect.arrayContaining(["Park it", "Archive it", "Its brief"]));
    expect(items.every((it) => it.group === "here")).toBe(true);
  });

  it("is adding a repo, in Settings' repos, and nothing in Settings' other places", () => {
    const items = hereItems(hereAt("/settings/repos", "", workspace, known), workspace, known);
    expect(items).toEqual([expect.objectContaining({ group: "here", title: "Add a repo from GitHub", to: "/settings/repos?add=1" })]);
    expect(hereAt("/settings/appearance", "", workspace, known)).toBeUndefined();
  });

  it("is a plugin's thing's actions, then ws's own", () => {
    const items = hereItems(hereAt("/x/jira/issue/RAM-12", "", workspace, known), workspace, known);
    expect(titles(items)).toEqual(["Move it", "Start a mission about it"]);
    expect(items[0].doing).toMatchObject({ thing: { id: "RAM-12" }, does: { id: "move" } });
  });
});

describe("what's around here", () => {
  const here = hereAt("/missions/fix-token-refresh", "", workspace, known)!;
  const near: Near[] = [
    { kind: "said", ref: "w4", title: "Why is billing slow?", thing: "talk", score: 0, how: "from", out: true },
    { kind: "said", ref: "brief", scope: "fix-token-refresh", title: "Fix token refresh", thing: "brief", place: "brief", score: 0, how: "in" },
    { kind: "said", ref: "d2", scope: "cache", title: "Wait for fix-token-refresh", thing: "decision", score: 0, how: "mentions" },
    { kind: "said", ref: "records/notes/tokens.md", title: "Tokens", thing: "document", score: 0, how: "mentions" },
  ];

  it("is drawn as the page holds it where it does, saying how it's linked", () => {
    const items = nearItems(near, here, local);
    expect(items[0]).toMatchObject({ id: "talk:w4", group: "linked", title: "Why is billing slow?", notes: ["where this came from"] });
    // The brief is the brief, on its tab, and where it's read isn't said:
    // it's here.
    expect(items[1]).toMatchObject({ kind: "Brief", to: "/missions/fix-token-refresh?tab=brief", notes: ["in this"] });
    // Something in another mission says which.
    expect(items[2]).toMatchObject({ id: "said:cache/d2", kind: "Decision", notes: ["names this", "cache"] });
    // A space's document is a document, as Find finds one.
    expect(items[3]).toMatchObject({ id: "document:records:notes/tokens.md", kind: "Document", title: "Tokens", notes: ["names this"] });
    expect(nearIds(items, near)).toContain("said:/w4");
  });

  it("comes first before anything's typed, after what needs you, in place of the places", () => {
    const linked = nearItems(near, here, local);
    const needs = [{ ...local.find((it) => it.id === "mission:cache")!, group: "needs" as const }];
    const recent = [
      { ...local.find((it) => it.id === "mission:fix-token-refresh")!, group: "recent" as const },
      { ...local.find((it) => it.id === "talk:w4")!, group: "recent" as const },
      { ...local.find((it) => it.id === "page:p3")!, group: "recent" as const },
    ];
    const b = build({ q: "", scope: "all", local, found: [], needs, recent, at: here.id, hereTitle: here.title, here: hereItems(here, workspace, known), linked });
    expect(b.sections.map((s) => [s.group, s.title])).toEqual([
      ["here", "Fix token refresh"],
      ["needs", "Needs you"],
      ["linked", "Linked"],
      ["recent", "Recent"],
    ]);
    // Where you are, and what's linked, aren't said again as recent.
    expect(titles(b.sections[3].items)).toEqual(["Token cache notes"]);
    // Somewhere of nothing: the places, as before.
    expect(build({ q: "", scope: "all", local, found: [], recent }).sections.map((s) => s.group)).toEqual(["recent", "places"]);
  });

  it("counts for more as something's typed", () => {
    const two = [
      { id: "mission:a", group: "missions" as const, kind: "Mission", title: "Alpha one", score: 0 },
      { id: "mission:b", group: "missions" as const, kind: "Mission", title: "Alpha two", score: 0 },
    ];
    expect(titles(build({ q: "alpha", scope: "all", local: two, found: [] }).rows)).toEqual(["Alpha one", "Alpha two"]);
    expect(titles(build({ q: "alpha", scope: "all", local: two, found: [], near: new Set(["mission:b"]) }).rows)).toEqual(["Alpha two", "Alpha one"]);
  });
});
