import { describe, expect, it } from "vitest";
import type { Repo, Workspace } from "../api";
import { type FoundEnv, refsFound } from "./found";
import { GROUND, type Known, refOf } from "./refs";

// What a plugin's app is told of the names in its words: found as in ws's
// own pages, each with where it goes and how it's drawn there.

const known: Known = {
  repos: new Map([
    ["demo", new Set(["main.go", "internal/cache.go"])],
    ["records", undefined],
  ]),
  capsules: new Map([["demo", ["token-cache"]]]),
  missions: ["fix-token-refresh"],
  pages: ["p3"],
  talks: ["w4"],
  artifacts: ["a3"],
  things: [
    { plugin: "jira", what: "issue", prefixes: ["RAM-"], address: "https://jira.example/browse/{ref}", path: "/issue/{ref}" },
    { plugin: "notes", what: "note", prefixes: ["NOTE-"], address: "https://notes.example/{ref}" },
  ],
};
const repo = (name: string, label: string, color: string): Repo => ({ name, label, color, cloned: true, capsules: [], aliases: [] });
const env: FoundEnv = {
  known,
  repos: new Map([["demo", repo("demo", "Demo", "#2f7d5b")]]),
  workspace: { missions: [], conversations: [] } as unknown as Workspace,
};
const found = (text: string, of: FoundEnv = env) => refsFound(text, of).map((f) => ({ ...f, said: text.slice(f.from, f.to) }));

describe("refs found for a plugin's app", () => {
  it("are where they are in the text, with ws's word for each, its kind, and where it goes", () => {
    const got = found("RAM-12 is fixed by demo#7, in fix-token-refresh; see demo/main.go:3 and p3.");
    expect(got.map((f) => [f.said, f.kind, f.href])).toEqual([
      ["RAM-12", "thing", "/x/jira/issue/RAM-12"],
      ["demo#7", "pr", "/reviews/demo/7"],
      ["fix-token-refresh", "mission", "/missions/fix-token-refresh"],
      ["demo/main.go:3", "file", "/files/demo?path=main.go#L3"],
      ["p3", "page", "/pages/p3"],
    ]);
    // ws's word for one is what its own pages know it by.
    expect(refOf(got[0].ref)).toEqual({ kind: "thing", plugin: "jira", what: "issue", id: "RAM-12", url: "https://jira.example/browse/RAM-12" });
    expect(refOf(got[3].ref)).toEqual({ kind: "file", repo: "demo", worktree: GROUND, path: "main.go", line: 3 });
  });

  it("go outside ws, for a thing whose kind opens nowhere in its plugin's app", () => {
    expect(found("NOTE-4").map((f) => f.href)).toEqual(["https://notes.example/NOTE-4"]);
  });

  it("say how a kind is drawn where it isn't as words: a repo's pill, in its colour", () => {
    expect(found("demo and RAM-1")).toEqual([
      { from: 0, to: 4, ref: '{"kind":"repo","repo":"demo"}', kind: "repo", href: "/files/demo", shape: "pill", accent: "#2f7d5b", said: "demo" },
      { from: 9, to: 14, ref: expect.any(String), kind: "thing", href: "/x/jira/issue/RAM-1", said: "RAM-1" },
    ]);
  });

  it("send a file in a capsule that's gone to the default branch, and keep what was named", () => {
    expect(found("demo/token-cache:internal/cache.go").map((f) => f.href)).toEqual(["/files/demo/token-cache?path=internal%2Fcache.go"]);
    // As an agent writes a file, by where it is on disk: its capsule isn't
    // looked for when it's found, so it may have landed since.
    const [gone] = found("repos/demo/landed/internal/cache.go");
    expect(gone.href).toBe("/files/demo?path=internal%2Fcache.go");
    expect(refOf(gone.ref)).toMatchObject({ kind: "file", worktree: "landed", path: "internal/cache.go" });
  });

  it("find nothing in words that only look like names, or where nothing's there to name", () => {
    expect(found("RAM-12-fix, docs/RAM-12.md, records/a.md and p9")).toEqual([]);
    expect(found("RAM-12", { ...env, known: { ...known, things: [] } })).toEqual([]);
  });
});
