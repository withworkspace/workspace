import { describe, expect, it } from "vitest";
import type { Mission, RunDetail } from "../api";
import { bare, forWhom, placeOf, plain, split } from "./runRows";

const run = (more: Partial<RunDetail> = {}): RunDetail => ({
  ref: "web.cache.frontend",
  name: "web",
  link: "http://127.0.0.1:41000",
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

describe("forWhom", () => {
  it("names the mission, its step, crew and conversation, with somewhere to go", () => {
    expect(forWhom(run({ owner: "step", mission: "cache-it", step: 2 }), missions)).toEqual({ text: "step 2 of Cache it", to: "/missions/cache-it" });
    expect(forWhom(run({ owner: "crew", mission: "cache-it", crew: "c3" }), missions)).toEqual({ text: "crew c3 of Cache it", to: "/missions/cache-it/crew/c3" });
    expect(forWhom(run({ owner: "looking", mission: "cache-it" }), missions).text).toBe("whoever has Cache it open");
    expect(forWhom(run({ owner: "conversation", conversation: "t1" }), missions, [{ id: "t1", title: "Try the cache" } as never])).toEqual({
      text: "the conversation “Try the cache”",
      to: "/conversations/t1",
    });
    expect(forWhom(run({ owner: "conversation", conversation: "t1", mission: "cache-it" }), missions).to).toBe("/missions/cache-it/conversations/t1");
  });
  it("says a mission the workspace no longer lists by its slug", () => {
    expect(forWhom(run({ owner: "mission", mission: "gone" }), missions)).toEqual({ text: "gone", to: "/missions/gone" });
  });
  it("says a run started for another's need by what needs it", () => {
    expect(forWhom(run({ owner: "run", neededBy: ["web.cache.frontend"] }), missions).text).toBe("web.cache.frontend, which needs it");
    expect(forWhom(run({ owner: "run", neededBy: ["web.cache.frontend", "admin.cache.frontend", "docs.cache.frontend"] }), missions).text).toBe(
      "web.cache.frontend, admin.cache.frontend and docs.cache.frontend, which need it",
    );
    expect(forWhom(run({ owner: "run" }), missions).text).toBe("what needed it, which has ended");
  });
  it("says how many others asked for a run since it started", () => {
    expect(forWhom(run({ owner: "pid", ownerPid: 812, ownerCommand: "zsh", also: 1 }), missions).text).toBe("zsh (pid 812), and one other who asked for it since");
    expect(forWhom(run({ owner: "step", mission: "cache-it", step: 2, also: 2 }), missions)).toEqual({
      text: "step 2 of Cache it, and 2 others who asked for it since",
      to: "/missions/cache-it",
    });
  });
  it("says a process by its command and pid", () => {
    expect(forWhom(run({ owner: "pid", ownerPid: 812, ownerCommand: "zsh" }), missions).text).toBe("zsh (pid 812)");
    expect(forWhom(run({ owner: "pid", ownerPid: 812 }), missions).text).toBe("pid 812");
  });
});

describe("placeOf", () => {
  it("goes to the mission whose capsule it is, else the capsule's files", () => {
    expect(placeOf(run(), missions)).toEqual({ text: "frontend / cache", to: "/missions/cache-it" });
    expect(placeOf(run({ worktree: "spare" }), missions).to).toBe("/files/frontend/spare");
    expect(placeOf(run({ worktree: "ground" }), missions).to).toBe("/files/frontend");
    expect(placeOf(run({ worktree: "silo" }), missions).to).toBeUndefined();
  });
});

describe("split", () => {
  it("puts running runs first, newest first, and ended ones by when they ended", () => {
    const { running, ended } = split([
      run({ ref: "a", started: "2026-11-19T09:00:00Z" }),
      run({ ref: "b", started: "2026-11-19T11:00:00Z" }),
      run({ ref: "c", state: "exited", ended: "2026-11-19T10:00:00Z" }),
      run({ ref: "d", state: "gone", ended: "2026-11-19T12:00:00Z" }),
    ]);
    expect(running.map((r) => r.ref)).toEqual(["b", "a"]);
    expect(ended.map((r) => r.ref)).toEqual(["d", "c"]);
  });
});

describe("words", () => {
  it("reads a link bare", () => {
    expect(bare("http://web.cache.frontend.ws.localhost/")).toBe("web.cache.frontend.ws.localhost");
  });
  it("takes a terminal's colours out of a log", () => {
    expect(plain("\x1b[32m  VITE\x1b[0m ready\r\n\x1b]8;;http://x\x07link\x1b]8;;\x07\nprogress 1\rprogress 2")).toBe("  VITE ready\r\nlink\nprogress 1\nprogress 2");
  });
});
