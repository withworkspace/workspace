import { describe, expect, it } from "vitest";
import type { Workspace } from "../api";
import { asks, build, type CodeHit, codeItem, codeNote, codeRematch, codeRuns, localItems, parseQuery, repoOfPage, withScope } from "./find";

const workspace = { name: "test", root: "/work/ws", missions: [], conversations: [], pages: [], spaces: [{ repo: "records" }] } as unknown as Workspace;
const local = localItems(workspace);

const hit = (more: Partial<CodeHit> = {}): CodeHit => ({
  repo: "demo",
  worktree: ".ground",
  path: "internal/cache.go",
  line: 12,
  column: 6,
  text: "func Load(key string) {",
  ranges: [[5, 9]],
  ...more,
});

describe("the code scope", () => {
  it("is % typed first", () => {
    expect(parseQuery("%useAuth")).toEqual({ scope: "code", q: "useAuth" });
    expect(parseQuery("  % func Load ")).toEqual({ scope: "code", q: "func Load" });
    expect(withScope("useAuth", "code")).toBe("%useAuth");
    expect(withScope("%useAuth", "all")).toBe("useAuth");
    expect(withScope("%useAuth", "files")).toBe("/useAuth");
  });

  it("asks Find's server nothing: code search asks for itself", () => {
    expect(asks("code", undefined, "Load")).toBeUndefined();
  });

  it("shows its hits in the order found, each to its file at its line", () => {
    const b = build({ q: "Load", scope: "code", local, found: [], code: { hits: [hit(), hit({ worktree: "fix", path: "a.go", line: 3 })] } });
    expect(b.sections.map((s) => s.group)).toEqual(["code"]);
    expect(b.rows.map((r) => r.to)).toEqual(["/files/demo?path=internal%2Fcache.go#L12", "/files/demo/fix?path=a.go#L3"]);
    expect(b.rows.map((r) => r.ref)).toEqual(["demo/internal/cache.go:12", "demo/fix:a.go:3"]);
    expect(b.rows[1].notes).toEqual(["in fix"]);
  });

  it("says when it was capped, and why", () => {
    const capped = build({ q: "e", scope: "code", local, found: [], code: { hits: [hit()], end: { matches: 200, bytes: 9000, capped: "matches" } } });
    expect(capped.sections[0].note).toMatch(/Stopped at the first 200 matches/);
    expect(codeNote({ matches: 31, bytes: 262000, capped: "bytes" })).toMatch(/Stopped at 31 matches, as many as fit/);
    expect(codeNote({ matches: 3, bytes: 90 })).toBeUndefined();
    expect(codeNote({ matches: 0, bytes: 0, problem: "regex parse error" })).toBe("regex parse error");
  });

  it("isn't mixed into Everything", () => {
    // Even handed hits, Everything shows none of them, and nothing's
    // asked of code search there.
    const b = build({ q: "Load", scope: "all", local, found: [], code: { hits: [hit()] } });
    expect(b.sections.some((s) => s.group === "code")).toBe(false);
    expect(b.rows.some((r) => r.code)).toBe(false);
    expect(asks("all", undefined, "Load")?.kinds ?? []).not.toContain("code");
    const files = build({ q: "Load", scope: "files", local, found: [], code: { hits: [hit()] } });
    expect(files.rows.some((r) => r.code)).toBe(false);
  });

  it("shows nothing before something's typed", () => {
    expect(build({ q: "", scope: "code", local, found: [], code: { hits: [] } }).rows).toEqual([]);
  });
});

describe("a hit's text", () => {
  it("is in runs, the matches marked, its indent dropped", () => {
    expect(codeRuns("\t  if x == needle && needle {", [[21, 27], [11, 17]])).toEqual([
      { text: "if x == ", hit: false },
      { text: "needle", hit: true },
      { text: " && ", hit: false },
      { text: "needle", hit: true },
      { text: " {", hit: false },
    ]);
    expect(codeRuns("needle", [[0, 6]])).toEqual([{ text: "needle", hit: true }]);
    expect(codeRuns("x", [])).toEqual([{ text: "x", hit: false }]);
  });

  it("is matched again as more's typed, until the answer comes", () => {
    const hits = [hit({ text: "loadAll(); LOAD" }), hit({ text: "nothing here" })];
    expect(codeRematch(hits, "load").map((h) => h.ranges)).toEqual([
      [
        [0, 4],
        [11, 15],
      ],
    ]);
    // A capital makes case matter, as smart case does.
    expect(codeRematch(hits, "LOAD").map((h) => h.ranges)).toEqual([[[11, 15]]]);
    expect(codeRematch(hits, "")).toEqual([]);
  });

  it("is an item that copies as its ref", () => {
    const it = codeItem(hit(), 0);
    expect(it.group).toBe("code");
    expect(it.code?.line).toBe(12);
  });
});

describe("the repo a page is of", () => {
  it("is a repo's files, a file in them, or a capsule's", () => {
    expect(repoOfPage("/files/demo")).toBe("demo");
    expect(repoOfPage("/files/demo/token-cache")).toBe("demo");
    expect(repoOfPage("/missions/fix-token/files/demo/token-cache")).toBe("demo");
    expect(repoOfPage("/missions/fix-token/review/demo/token-cache")).toBe("demo");
    expect(repoOfPage("/files/my%20repo")).toBe("my repo");
  });

  it("is a space's repo, for a document in it", () => {
    expect(repoOfPage("/documents/records/adr/0001.md", workspace.spaces)).toBe("records");
    expect(repoOfPage("/documents/local/p3", workspace.spaces)).toBeUndefined();
    expect(repoOfPage("/documents/records", workspace.spaces)).toBeUndefined();
  });

  it("is none elsewhere", () => {
    for (const p of ["/", "/files", "/missions/fix-token", "/missions/fix-token/crew/c1", "/reviews/demo/12", "/conversations/w3"]) expect(repoOfPage(p)).toBeUndefined();
  });
});
