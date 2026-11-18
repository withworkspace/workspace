import { describe, expect, it } from "vitest";
import type { Space, SpaceActivity } from "../api";
import { acrossSpaces, changedSince, lastChanged, saysNoMore, withoutTitle, wordsIn } from "./happening";

describe("saysNoMore", () => {
  it("is so when the subject only names the document", () => {
    expect(saysNoMore("Add doc: Verify v2", { path: "docs/verify-v2.md", title: "Verify v2", how: "added" })).toBe(true);
    expect(saysNoMore("Amend doc verify-v2", { path: "docs/verify-v2.md", title: "Verify v2", how: "changed" })).toBe(true);
  });
  it("reads a dated or numbered file's name without its date or number", () => {
    expect(saysNoMore("Amend plan 2026-11-17 plugin-pages-as-single-page-apps", { path: "plans/2026-11-17-plugin-pages-as-single-page-apps.md", how: "changed" })).toBe(true);
    expect(saysNoMore("Edit retire-the-v1-rest-api", { path: "adr/0014-retire-the-v1-rest-api.md", how: "changed" })).toBe(true);
  });
  it("isn't when the subject says more", () => {
    expect(saysNoMore("Handover: what to write when nothing happened", { path: "guides/on-call/handover.md", title: "On-call handover", how: "changed" })).toBe(false);
  });
});

describe("acrossSpaces", () => {
  const space = (repo: string) => ({ repo, label: repo, color: "", count: 0, cloned: true, folders: [], kinds: [] }) as Space;
  const act = (at: string[], flights: string[] = []): SpaceActivity => ({
    lately: at.map((a, i) => ({ sha: `${a}${i}`, at: a, by: "A B", subject: "s", docs: [] })),
    inFlight: flights.map((c) => ({ capsule: c, docs: [] })),
    fresh: { behind: 0, branch: "main" },
  });
  it("puts every space's changes newest first, each with its space", () => {
    const got = acrossSpaces(
      [space("a"), space("b"), space("c")],
      new Map([
        ["a", act(["2026-11-10T00:00:00Z", "2026-11-01T00:00:00Z"], ["x"])],
        ["b", act(["2026-11-12T00:00:00Z"], ["y"])],
      ]),
    );
    expect(got.lately.map((c) => [c.repo, c.at.slice(0, 10)])).toEqual([
      ["b", "2026-11-12"],
      ["a", "2026-11-10"],
      ["a", "2026-11-01"],
    ]);
    expect(got.inFlight.map((f) => `${f.repo}:${f.capsule}`)).toEqual(["a:x", "b:y"]);
  });
});

describe("changedSince and lastChanged", () => {
  it("count and pick by time", () => {
    const c = (at: string) => ({ sha: at, at, by: "", subject: "", docs: [] });
    expect(changedSince([c("2026-11-18T09:00:00Z"), c("2026-11-01T00:00:00Z")], Date.parse("2026-11-11T00:00:00Z"))).toBe(1);
    expect(lastChanged([{ path: "a", title: "A", changed: "2026-11-01T00:00:00Z" }, { path: "b", title: "B", changed: "2026-11-03T00:00:00Z" }, { path: "c", title: "C" }])?.path).toBe("b");
    expect(lastChanged([{ path: "c", title: "C" }])).toBeUndefined();
  });
});

describe("withoutTitle", () => {
  it("drops the leading title, keeping front matter", () => {
    expect(withoutTitle("# Verify v2\n\nStarted.\n")).toBe("\nStarted.\n");
    expect(withoutTitle("---\nkind: doc\n---\n# T\nBody")).toBe("---\nkind: doc\n---\nBody");
    expect(withoutTitle("Intro\n# Later\n")).toBe("Intro\n# Later\n");
  });
});

describe("wordsIn", () => {
  it("counts words, not code", () => {
    expect(wordsIn("Two words\n```\nlots of code here\n```\n- and three")).toBe(4);
  });
});
