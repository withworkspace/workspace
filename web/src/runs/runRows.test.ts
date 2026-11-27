import { describe, expect, it } from "vitest";
import type { Mission, RecipeInfo, RunDetail } from "../api";
import { bare, came, commitSaid, forWhom, pickedIn, placeOf, placeRuns, plain, runId, runStatus, split, startedBy, took } from "./runRows";

const run = (more: Partial<RunDetail> = {}): RunDetail => ({
  ref: "web.cache.frontend",
  name: "web",
  type: "service",
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
    expect(forWhom(run({ owner: "looking", mission: "cache-it" }), missions).text).toBe("anyone with Cache it open");
    expect(forWhom(run({ owner: "looking" }), missions)).toEqual({ text: "anyone with the files open", to: "/files/frontend/cache" });
    expect(forWhom(run({ owner: "looking", worktree: "ground" }), missions).to).toBe("/files/frontend");
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
    expect(forWhom(run({ owner: "run" }), missions).text).toBe("an ended run");
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
  it("keeps tasks that have ended apart, as results, and those running with what's running", () => {
    const { running, results, ended } = split([
      run({ ref: "web" }),
      run({ ref: "test", type: "task", result: "running" }),
      run({ ref: "lint", type: "task", state: "exited", result: "passed", ended: "2026-11-19T10:00:00Z" }),
      run({ ref: "build", type: "task", state: "gone", result: "stopped", ended: "2026-11-19T12:00:00Z" }),
      run({ ref: "old", state: "exited", ended: "2026-11-19T10:00:00Z" }),
    ]);
    expect(running.map((r) => r.ref).sort()).toEqual(["test", "web"]);
    expect(results.map((r) => r.ref)).toEqual(["build", "lint"]);
    expect(ended.map((r) => r.ref)).toEqual(["old"]);
  });
});

describe("a task's result", () => {
  it("says how long it took", () => {
    expect([took(undefined), took(40), took(420), took(42_400), took(185_000), took(3_725_000)]).toEqual(["", "under 0.1s", "0.4s", "42s", "3m 05s", "1h 02m"]);
  });
  it("says what it came to", () => {
    const task = (more: Partial<RunDetail>) => run({ type: "task", state: "exited", ...more });
    expect(came(task({ result: "passed", took: 42_000 }))).toBe("passed in 42s");
    expect(came(task({ result: "failed", status: 2, took: 185_000 }))).toBe("failed (exit 2) in 3m 05s");
    expect(came(task({ result: "stopped", why: "timed out" }))).toBe("stopped: timed out");
    expect(came(task({ result: "stopped" }))).toBe("stopped early");
    expect(came(task({ state: "running", result: "running" }))).toBe("running");
  });
  it("says the commit it ran at", () => {
    expect(commitSaid(run({ commit: "0123456789abcdef" }))).toBe("012345678");
    expect(commitSaid(run({ commit: "0123456789abcdef", dirty: true }))).toBe("012345678, with uncommitted changes");
    expect(commitSaid(run())).toBe("");
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

describe("runStatus", () => {
  it("says a service up, coming up, or how it ended", () => {
    expect(runStatus(run({ up: true }))).toMatchObject({ tone: "done", words: "Up", live: true });
    expect(runStatus(run())).toMatchObject({ tone: "progress", words: "Starting" });
    expect(runStatus(run({ state: "exited", status: 2 }))).toMatchObject({ tone: "danger", words: "Exited 2" });
    expect(runStatus(run({ state: "exited", status: 0 }))).toMatchObject({ tone: "quiet", words: "Exited" });
    expect(runStatus(run({ state: "gone" }))).toMatchObject({ tone: "quiet", words: "Gone" });
  });
  it("says what a task came to", () => {
    const task = (more: Partial<RunDetail>) => run({ type: "task", state: "exited", ...more });
    expect(runStatus(task({ state: "running", result: "running" }))).toMatchObject({ tone: "progress", live: true });
    expect(runStatus(task({ result: "passed" })).tone).toBe("done");
    expect(runStatus(task({ result: "failed" })).tone).toBe("danger");
    expect(runStatus(task({ result: "stopped" })).tone).toBe("quiet");
  });
});

describe("startedBy", () => {
  it("says who started it, and from where", () => {
    expect(startedBy(run({ owner: "pid", by: "you", ownerCommand: "zsh" }))).toBe("you, from a terminal");
    expect(startedBy(run({ owner: "looking", by: "you" }))).toBe("you, from Files");
    expect(startedBy(run({ owner: "looking", by: "you", mission: "cache-it" }))).toBe("you, from the mission's page");
    expect(startedBy(run({ owner: "looking", by: "you", app: "docs" }))).toBe("you, by opening the docs tab");
    expect(startedBy(run({ owner: "crew", by: "c2", mission: "cache-it", crew: "c2" }))).toBe("crew c2");
    expect(startedBy(run({ owner: "none" }))).toBe("you");
  });
  it("says ws ran it for a step's check, or for what needs it", () => {
    expect(startedBy(run({ type: "task", owner: "mission", by: "ws", check: { mission: "cache-it", step: 2, n: 1 } }))).toBe("Workspace, for check 1 of step 2");
    expect(startedBy(run({ owner: "run", by: "ws", neededBy: ["web.cache.frontend"] }))).toBe("Workspace, as web needs it");
    expect(startedBy(run({ owner: "run", by: "ws", neededBy: ["web.cache.frontend", "admin.cache.frontend"] }))).toBe("Workspace, as web and admin need it");
    expect(startedBy(run({ owner: "run", by: "ws" }))).toBe("Workspace, for a run that needed it");
  });
});

describe("a place's runs", () => {
  const recipe = (name: string, more: Partial<RecipeInfo> = {}): RecipeInfo => ({ name, type: "service", kind: "app", render: "inline", command: "", ...more });
  const recipes = [recipe("admin"), recipe("docs", { kind: "tool", render: "tab" }), recipe("test", { type: "task", kind: undefined, render: undefined }), recipe("web", { title: "Storefront" })];
  const test = (n: number, ended: string) => run({ ref: "test.cache.frontend", nref: `test.cache.frontend#${n}`, n, recipe: "test", type: "task", state: "exited", result: "passed", ended });
  const web = run({ ref: "web.cache.frontend", recipe: "web", up: true, started: "2026-11-19T10:00:00Z" });
  const docs = run({ ref: "docs.cache.frontend", recipe: "docs", state: "exited", status: 1, ended: "2026-11-19T09:30:00Z" });
  const adhoc = run({ ref: "npx-serve.cache.frontend", name: "npx-serve", started: "2026-11-19T08:00:00Z" });
  const lint = run({ ref: "lint.cache.frontend", nref: "lint.cache.frontend#1", n: 1, type: "task", state: "running", result: "running", started: "2026-11-19T11:00:00Z" });
  const p = placeRuns([web, docs, adhoc, lint, test(3, "2026-11-19T09:45:00Z")], [test(2, "2026-11-19T09:00:00Z"), test(1, "2026-11-18T09:00:00Z")], recipes);
  it("lists each service recipe, running or not, with its run, apps apart from tools, and commands run as apps", () => {
    expect(p.apps.map((s) => [s.name, s.run?.ref])).toEqual([
      ["admin", undefined],
      ["web", "web.cache.frontend"],
      ["npx-serve", "npx-serve.cache.frontend"],
    ]);
    expect(p.tools.map((s) => [s.name, s.run?.state])).toEqual([["docs", "exited"]]);
  });
  it("lists tasks' runs, what's running first, then the last to end, the runs of a task among them", () => {
    expect(p.tasks.map(runId)).toEqual(["lint.cache.frontend#1", "test.cache.frontend#3", "test.cache.frontend#2", "test.cache.frontend#1"]);
  });
  it("picks what's asked for, a task's ref being its latest, else a service that's up", () => {
    expect(pickedIn(p, { service: "docs" })?.service?.name).toBe("docs");
    expect(pickedIn(p, { run: "test.cache.frontend#2" })?.task?.n).toBe(2);
    expect(pickedIn(p, { run: "test.cache.frontend" })?.task?.n).toBe(3);
    expect(pickedIn(p, {})?.service?.name).toBe("web");
    expect(pickedIn(p, { service: "gone" })?.service?.name).toBe("web");
  });
  it("picks a task's run with no service running, else the first app", () => {
    const quiet = placeRuns([test(3, "2026-11-19T09:45:00Z")], [], recipes);
    expect(pickedIn(quiet, {})?.task?.n).toBe(3);
    expect(pickedIn(placeRuns([], [], recipes), {})?.service?.name).toBe("admin");
    expect(pickedIn(placeRuns([], [], []), {})).toBeUndefined();
  });
});
