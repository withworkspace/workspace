import { describe, expect, it } from "vitest";
import { GROUND, type Known, refNamed } from "../refs/refs";
import { pageRefAt, pinNameOf } from "./focus";

const known: Known = {
  repos: new Map(),
  missions: [],
  pages: [],
  talks: [],
  things: [{ plugin: "jira", what: "issue", prefixes: ["RAM-"], tail: "number", path: "/issue/{ref}" }],
};

// What a page is, read from its address, and what it's pinned as.
const pinnedAs = (path: string, search = "") => {
  const r = pageRefAt(path, search, known);
  return r && pinNameOf(r);
};

describe("pageRefAt", () => {
  it("reads each page that's something to pin", () => {
    expect(pinnedAs("/missions/cache")).toBe("cache");
    expect(pinnedAs("/missions/cache/brief")).toBe("cache");
    expect(pinnedAs("/missions/cache/conversations/t3")).toBe("cache");
    expect(pinnedAs("/conversations/w4")).toBe("w4");
    expect(pinnedAs("/reviews/demo/12")).toBe("demo#12");
    expect(pinnedAs("/pages/p3")).toBe("p3");
    expect(pinnedAs("/documents/local/p3")).toBe("p3");
    expect(pinnedAs("/artifacts/a2")).toBe("a2");
    expect(pinnedAs("/artifacts/a2/v3")).toBe("a2");
    expect(pinnedAs("/documents/records/docs/verify%20v2.md")).toBe("records/docs/verify v2.md");
    expect(pinnedAs("/x/jira/issue/RAM-123")).toBe("RAM-123");
  });

  it("reads nothing from a list, a capsule's document, or what isn't one", () => {
    for (const path of ["/", "/conversations", "/reviews", "/reviews/demo/new", "/artifacts/compare", "/documents/records", "/pages/open", "/settings", "/x/jira/board", "/x/jira/issue/NOPE-1"]) {
      expect(pinnedAs(path), path).toBeUndefined();
    }
    expect(pinnedAs("/documents/records/docs/plan.md", "?in=fix-plan")).toBeUndefined();
  });
});

describe("pinNameOf", () => {
  it("pins a file rather than its lines, an artifact rather than a version, and no capsule's file", () => {
    expect(pinNameOf({ kind: "file", repo: "demo", worktree: GROUND, path: "src/app.ts", line: 12, endLine: 20 })).toBe("demo/src/app.ts");
    expect(pinNameOf({ kind: "artifact", id: "a2", version: 3 })).toBe("a2");
    expect(pinNameOf({ kind: "file", repo: "demo", worktree: "fix", path: "src/app.ts" })).toBeUndefined();
    expect(pinNameOf({ kind: "repo", repo: "demo" })).toBeUndefined();
  });

  it("reads back from the name it pins as", () => {
    expect(refNamed("RAM-123", known)).toMatchObject({ kind: "thing", plugin: "jira", id: "RAM-123" });
    expect(refNamed("demo#12", known)).toEqual({ kind: "pr", repo: "demo", number: 12 });
    expect(refNamed("demo/src/app.ts", known)).toMatchObject({ kind: "file", repo: "demo", path: "src/app.ts" });
  });
});
