import { describe, expect, it } from "vitest";
import type { Mission, RunDetail } from "../api";
import { isNew, jumpTo, ofPlace, placeAt, placeSaid, services } from "./runsBelow";

const run = (more: Partial<RunDetail> = {}): RunDetail => ({
  ref: "web.cache.frontend",
  name: "web",
  type: "service",
  link: "http://web.sorrel.localhost",
  local: "http://127.0.0.1:41000",
  port: 41000,
  state: "running",
  owner: "none",
  command: "npm run dev",
  started: "2026-11-19T10:00:00Z",
  repo: "frontend",
  worktree: "cache",
  ...more,
});

const missions = [{ slug: "cache-it", title: "Cache it", capsules: [{ repo: "frontend", name: "cache", path: "", exists: true }] }] as unknown as Mission[];

describe("placeAt", () => {
  it("is a mission on any of its pages, and a repo's worktree on Files", () => {
    expect(placeAt("/missions/cache-it")).toEqual({ mission: "cache-it" });
    expect(placeAt("/missions/cache-it/crew/c2")).toEqual({ mission: "cache-it" });
    expect(placeAt("/files/frontend")).toEqual({ repo: "frontend", worktree: "ground" });
    expect(placeAt("/files/frontend/cache")).toEqual({ repo: "frontend", worktree: "cache" });
  });
  it("is nothing on a page of no place", () => {
    expect(placeAt("/")).toBeUndefined();
    expect(placeAt("/files")).toBeUndefined();
    expect(placeAt("/missions")).toBeUndefined();
    expect(placeAt("/runs")).toBeUndefined();
  });
});

describe("ofPlace", () => {
  const refs = (rs: RunDetail[]) => rs.map((r) => r.ref);
  it("has a mission's own runs and its capsules'", () => {
    const here = { mission: "cache-it" };
    const runs = [run({ ref: "own", repo: "api", worktree: "x", mission: "cache-it" }), run({ ref: "capsule" }), run({ ref: "other", worktree: "other" })];
    expect(refs(ofPlace(runs, here, missions))).toEqual(["own", "capsule"]);
  });
  it("has a worktree's runs only", () => {
    const runs = [run({ ref: "capsule" }), run({ ref: "ground", worktree: "ground" })];
    expect(refs(ofPlace(runs, { repo: "frontend", worktree: "cache" }, missions))).toEqual(["capsule"]);
  });
  it("has what the place's runs need, wherever it runs", () => {
    const runs = [run({ ref: "web.cache.frontend" }), run({ ref: "api.ground.records", repo: "records", worktree: "ground", neededBy: ["web.cache.frontend"] }), run({ ref: "x.ground.records", repo: "records", worktree: "ground", neededBy: ["y.z.w"] })];
    expect(refs(ofPlace(runs, { repo: "frontend", worktree: "cache" }, missions))).toEqual(["web.cache.frontend", "api.ground.records"]);
  });
});

describe("services", () => {
  it("is what's running that isn't a task, the newest first", () => {
    const got = services([
      run({ ref: "a", started: "2026-11-19T10:00:00Z" }),
      run({ ref: "b", started: "2026-11-19T11:00:00Z" }),
      run({ ref: "t", type: "task" }),
      run({ ref: "x", state: "exited" }),
    ]);
    expect(got.map((r) => r.ref)).toEqual(["b", "a"]);
  });
});

describe("isNew", () => {
  it("is started after Below was last seen, and nothing before it's been seen", () => {
    const at = Date.parse("2026-11-19T10:30:00Z");
    expect(isNew(run({ started: "2026-11-19T11:00:00+01:00" }), at)).toBe(false);
    expect(isNew(run({ started: "2026-11-19T10:31:00Z" }), at)).toBe(true);
    expect(isNew(run(), undefined)).toBe(false);
  });
});

describe("jumpTo", () => {
  it("goes to its worktree's Runs tab with it picked", () => {
    expect(jumpTo(run({ recipe: "dev" }))).toBe("/files/frontend/cache?view=runs&service=dev");
    expect(jumpTo(run({ worktree: "ground" }))).toBe("/files/frontend?view=runs&service=web");
  });
  it("goes to the Runs page for the workspace's own and a silo's", () => {
    expect(jumpTo(run({ repo: undefined, ref: "docs.ws" }))).toBe("/runs?show=docs.ws");
    expect(jumpTo(run({ worktree: "silo" }))).toBe("/runs?show=web.cache.frontend");
  });
});

describe("placeSaid", () => {
  it("is a mission's title, or the repo and worktree", () => {
    expect(placeSaid({ mission: "cache-it" }, missions)).toBe("Cache it");
    expect(placeSaid({ mission: "gone" }, missions)).toBe("gone");
    expect(placeSaid({ repo: "frontend", worktree: "ground" }, missions)).toBe("frontend / ground");
  });
});
