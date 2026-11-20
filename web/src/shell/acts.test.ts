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
