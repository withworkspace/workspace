import { describe, expect, it } from "vitest";
import type { Plugin, Workspace } from "../api";
import { actsFor, subjectOf } from "./acts";
import { build, localItems } from "./find";

const headline = (verb?: string) => ({ group: "needs-you", kind: "asks", label: "", text: "", action: { verb, label: verb ? "accept it" : undefined } });
const mission = (slug: string, state: string, more: object = {}) => ({ slug, title: slug, state, created: "", updated: "", headline: headline(), capsules: [], crew: [], ...more });
const workspace = {
  name: "test",
  root: "/work/ws",
  org: "acme",
  missions: [
    mission("ready", "review", { headline: headline("accept"), crew: [{ id: "c1", harness: "claude", ended: false, activity: "working" }, { id: "c2", harness: "claude", ended: true }, { id: "c3", harness: "claude", ended: false, status: "done" }] }),
    mission("aside", "parked"),
    mission("gone", "archived"),
  ],
  conversations: [
    { id: "w1", title: "Open one", opening: "", status: "open", at: "", updated: "", became: "ready" },
    { id: "w2", title: "Done one", opening: "", status: "resolved", at: "", updated: "", pr: "demo#12" },
  ],
  pages: [],
  artifacts: [
    { id: "a1", title: "Many", status: "live", versions: 3 },
    { id: "a2", title: "Thrown", status: "discarded", versions: 1 },
  ],
} as unknown as Workspace;

const titles = (s: Parameters<typeof actsFor>[0]) => actsFor(s, workspace).map((it) => it.title);

describe("what a row is", () => {
  it("is ws's own things by their rows' ids, and nothing else", () => {
    expect(subjectOf("mission:ready")).toEqual({ kind: "mission", slug: "ready" });
    expect(subjectOf("talk:w1")).toEqual({ kind: "talk", id: "w1" });
    expect(subjectOf("artifact:a1")).toEqual({ kind: "artifact", id: "a1" });
    expect(subjectOf("pr:demo#12")).toEqual({ kind: "pr", repo: "demo", number: 12 });
    for (const id of ["file:demo/x.go", "place:/runs", "do:keys", "mission:", undefined]) expect(subjectOf(id)).toBeUndefined();
  });
});

describe("what can be done to it", () => {
  it("is a mission's headline first, then what changes it as it stands, then its brief", () => {
    expect(titles({ kind: "mission", slug: "ready" })).toEqual(["Accept it", "Park it", "Stop c1", "Archive it", "Its brief"]);
    expect(titles({ kind: "mission", slug: "aside" })).toEqual(["Unpark it", "Archive it", "Its brief"]);
    expect(titles({ kind: "mission", slug: "gone" })).toEqual(["Unarchive it", "Its brief"]);
    expect(actsFor({ kind: "mission", slug: "ready" }, workspace)[2].doing).toEqual({ act: "stop", slug: "ready", crew: "c1" });
  });

  it("is a conversation resolved or reopened, and where it went", () => {
    expect(titles({ kind: "talk", id: "w1" })).toEqual(["Resolve it", "The mission it became"]);
    expect(titles({ kind: "talk", id: "w2" })).toEqual(["Reopen it", "Its pull request, demo#12"]);
    expect(actsFor({ kind: "talk", id: "w2" }, workspace)[1].to).toBe("/reviews/demo/12");
  });

  it("is an artifact compared, or a discarded one restored", () => {
    expect(titles({ kind: "artifact", id: "a1" })).toEqual(["Compare its versions"]);
    expect(titles({ kind: "artifact", id: "a2" })).toEqual(["Restore it"]);
  });

  it("is a pull request docked, or opened on GitHub", () => {
    expect(titles({ kind: "pr", repo: "demo", number: 12 })).toEqual(["Dock it here", "Open it on GitHub"]);
    expect(actsFor({ kind: "pr", repo: "demo", number: 12 }, workspace)[1].to).toBe("https://github.com/acme/demo/pull/12");
  });

  it("is nothing for what the workspace doesn't hold", () => {
    expect(actsFor({ kind: "mission", slug: "nope" }, workspace)).toEqual([]);
    expect(actsFor({ kind: "talk", id: "nope" }, workspace)).toEqual([]);
  });

  it("isn't found among everything: Resolve is under its conversation, not a row of its own", () => {
    const local = localItems(workspace);
    const b = build({ q: "resolve", scope: "all", local, found: [] });
    expect(b.rows.some((it) => it.title === "Resolve it")).toBe(false);
  });
});

describe("places", () => {
  const plugins = [{ name: "jira", state: "running", views: [{ title: "Board", path: "/board", under: "Jira" }], actions: [] }] as unknown as Plugin[];
  const local = localItems(workspace, plugins);

  it("are the sidebar's, with each plugin's views, found by name", () => {
    const b = build({ q: "board", scope: "all", local, found: [] });
    expect(b.rows[0]).toMatchObject({ title: "Board", to: "/x/jira/board", notes: ["jira"] });
  });

  it("have Settings, and each of its sections found by name, not offered before anything's typed", () => {
    expect(build({ q: "settings", scope: "all", local, found: [] }).rows[0]).toMatchObject({ title: "Settings", to: "/settings" });
    expect(build({ q: "appearance", scope: "all", local, found: [] }).rows[0]).toMatchObject({ to: "/settings/appearance", notes: ["Settings"] });
    const empty = build({ q: "", scope: "all", local, found: [] });
    expect(empty.rows.some((it) => it.to === "/settings")).toBe(true);
    expect(empty.rows.some((it) => it.to?.startsWith("/settings/"))).toBe(false);
  });
});

describe("pinning it", () => {
  const pinned = (...names: string[]) => ({ ...workspace, focus: names.map((name) => ({ name })) }) as unknown as Workspace;
  const last = (s: Parameters<typeof actsFor>[0], w: Workspace) => actsFor(s, w).at(-1);

  it("is the last thing offered, by the name ws writes the thing with", () => {
    const w = pinned();
    expect(last({ kind: "mission", slug: "ready" }, w)).toMatchObject({ title: "Pin it", doing: { ws: { pin: "ready" } } });
    expect(last({ kind: "talk", id: "w1" }, w)).toMatchObject({ title: "Pin it", doing: { ws: { pin: "w1" } } });
    expect(last({ kind: "artifact", id: "a1" }, w)).toMatchObject({ title: "Pin it", doing: { ws: { pin: "a1" } } });
    expect(last({ kind: "pr", repo: "demo", number: 12 }, w)).toMatchObject({ title: "Pin it", doing: { ws: { pin: "demo#12" } } });
  });

  it("is unpinning it, for what's pinned", () => {
    const w = pinned("ready", "demo#12");
    expect(last({ kind: "mission", slug: "ready" }, w)).toMatchObject({ title: "Unpin it", doing: { ws: { unpin: "ready" } } });
    expect(last({ kind: "pr", repo: "demo", number: 12 }, w)).toMatchObject({ title: "Unpin it", doing: { ws: { unpin: "demo#12" } } });
    expect(last({ kind: "talk", id: "w1" }, w)?.title).toBe("Pin it");
  });

  // A pin of one would leave your focus at once: it's not offered. One
  // pinned before it ended can still be unpinned.
  it("isn't offered for what's come to an end", () => {
    const titles = (s: Parameters<typeof actsFor>[0], w: Workspace) => actsFor(s, w).map((it) => it.title);
    expect(titles({ kind: "mission", slug: "gone" }, pinned())).not.toContain("Pin it");
    expect(titles({ kind: "talk", id: "w2" }, pinned())).not.toContain("Pin it");
    expect(titles({ kind: "artifact", id: "a2" }, pinned())).not.toContain("Pin it");
    expect(titles({ kind: "mission", slug: "gone" }, pinned("gone"))).toContain("Unpin it");
  });

  it("isn't offered by a daemon that keeps no pins", () => {
    expect(actsFor({ kind: "mission", slug: "ready" }, workspace).map((it) => it.title)).not.toContain("Pin it");
  });
});
