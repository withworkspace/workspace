import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";
import { findAll, findRefs, GROUND, type Known, type PageRef, parseAddress, parseName, refAddress, refAt, refHref, refName, reposNamed, type Where } from "./refs";

// The cases internal/ref is tested against: one grammar, in Go and here.
interface KnownJSON {
  repos: Record<string, string[] | null>;
  aliases?: Record<string, string>;
  capsules?: Record<string, string[]>;
  root?: string;
  missions: string[];
  pages: string[];
  talks: string[];
  artifacts?: string[];
  org?: string;
  defaultBranch?: string;
  branches?: Record<string, Record<string, string>>;
}
const cases = JSON.parse(readFileSync(new URL("../../../internal/ref/testdata/cases.json", import.meta.url), "utf8")) as {
  finds: { about: string; known: KnownJSON; cases: { text: string; found: [string, PageRef][] }[] }[];
  names: { ref: PageRef; name: string }[];
  addresses: { ref: PageRef; where: Where; address: string }[];
  read: { known: KnownJSON; cases: { address: string; ref: PageRef | null }[] };
  parse: { about: string; name: string; ref: PageRef | null; written?: string }[];
};

const knownOf = (j: KnownJSON): Known => ({
  repos: new Map(Object.entries(j.repos).map(([r, files]) => [r, files ? new Set(files) : undefined])),
  aliases: j.aliases && new Map(Object.entries(j.aliases)),
  capsules: j.capsules && new Map(Object.entries(j.capsules)),
  root: j.root,
  missions: j.missions,
  pages: j.pages,
  talks: j.talks,
  artifacts: j.artifacts,
  org: j.org,
  defaultBranch: j.defaultBranch,
  branches: j.branches && new Map(Object.entries(j.branches).map(([r, b]) => [r, new Map(Object.entries(b))])),
});

describe("refs, as internal/ref has them", () => {
  for (const g of cases.finds) {
    it(`finds ${g.about}`, () => {
      const k = knownOf(g.known);
      for (const c of g.cases) expect(findRefs(c.text, k).map((f) => [c.text.slice(f.from, f.to), f.ref]), c.text).toEqual(c.found);
    });
  }

  it("writes each kind's name, and reads it back", () => {
    for (const c of cases.names) {
      expect(refName(c.ref)).toBe(c.name);
      if (c.ref.kind !== "repo") expect(parseName(c.name)).toEqual(c.ref);
    }
  });

  it("reads names by their form alone, and writes them back", () => {
    for (const c of cases.parse) {
      const got = parseName(c.name);
      expect(got, `${c.about}: ${c.name}`).toEqual(c.ref ?? undefined);
      if (got) expect(refName(got), `${c.about}: ${c.name}`).toBe(c.written);
    }
  });

  it("writes addresses, and reads them back", () => {
    for (const c of cases.addresses) expect(refAddress(c.ref, c.where), JSON.stringify(c.ref)).toBe(c.address);
    const k = knownOf(cases.read.known);
    for (const c of cases.read.cases) expect(parseAddress(c.address, k), c.address).toEqual(c.ref ?? undefined);
  });
});

const known: Known = knownOf(cases.finds[0].known);

describe("refs in words", () => {
  it("says when a whole text is one ref", () => {
    expect(refAt("demo/main.go:5", known)).toEqual({ kind: "file", repo: "demo", worktree: ".ground", path: "main.go", line: 5 });
    expect(refAt("see demo/main.go", known)).toBeUndefined();
    expect(refAt("https://example.com/demo/main.go", known)).toBeUndefined();
  });

  it("says where each goes, and which repos a text may name", () => {
    expect(refHref({ kind: "file", repo: "demo", worktree: ".ground", path: "internal/cache/cache.go", line: 12, endLine: 20 })).toBe("/files/demo?path=internal%2Fcache%2Fcache.go#L12-L20");
    expect(refHref({ kind: "file", repo: "api", worktree: "token-cache", path: "a.go" })).toBe("/files/api/token-cache?path=a.go");
    expect(refHref({ kind: "pr", repo: "demo", number: 12 })).toBe("/reviews/demo/12");
    expect(refHref({ kind: "mission", slug: "cache" })).toBe("/missions/cache");
    expect(reposNamed("see demo/main.go and the api", ["demo", "api"])).toEqual(["demo"]);
    expect(reposNamed("in the api `src/x.ts`", ["demo", "api"])).toEqual(["api"]);
  });
});

describe("names as a turn pinned them", () => {
  it("draws what was meant, over what the words find now", () => {
    const gone: PageRef = { kind: "file", repo: "api", worktree: "token-cache", path: "a.go", line: 3, commit: "abc1234" };
    const text = "See api/token-cache:a.go:3, demo/main.go and demo/main.go:5.";
    const found = findAll(text, known, [
      { said: "api/token-cache:a.go:3", ref: gone },
      { said: "demo/main.go", ref: { kind: "file", repo: "demo", worktree: ".ground", path: "main.go", commit: "def5678" } },
    ]);
    expect(found.map((f) => [text.slice(f.from, f.to), f.ref])).toEqual([
      ["api/token-cache:a.go:3", gone],
      ["demo/main.go", { kind: "file", repo: "demo", worktree: ".ground", path: "main.go", commit: "def5678" }],
      // Not pinned whole: found as it is now.
      ["demo/main.go:5", { kind: "file", repo: "demo", worktree: ".ground", path: "main.go", line: 5 }],
    ]);
  });
});

describe("a ref to a space's document", () => {
  const known: Known = { repos: new Map(), missions: [], pages: [], talks: [], spaces: [{ repo: "records" }, { repo: "api", folder: "docs/" }] };
  it("goes to its address in Documents, as a whole or at a heading", () => {
    expect(refHref({ kind: "file", repo: "records", worktree: GROUND, path: "adr/0001-cache.md" }, known)).toBe("/documents/records/adr/0001-cache.md");
    expect(refHref({ kind: "file", repo: "records", worktree: "edit", path: "adr/0001-cache.md", anchor: "decision" }, known)).toBe("/documents/records/adr/0001-cache.md?in=edit#decision");
    expect(refHref({ kind: "file", repo: "api", worktree: GROUND, path: "docs/guide.md" }, known)).toBe("/documents/api/docs/guide.md");
  });
  it("stays in Files at its lines, or for what isn't one of a space's documents", () => {
    expect(refHref({ kind: "file", repo: "records", worktree: GROUND, path: "adr/0001-cache.md", line: 4 }, known)).toMatch(/^\/files\/records\?/);
    expect(refHref({ kind: "file", repo: "api", worktree: GROUND, path: "README.md" }, known)).toMatch(/^\/files\/api\?/);
    expect(refHref({ kind: "file", repo: "records", worktree: GROUND, path: "adr/0001-cache.md" })).toMatch(/^\/files\//);
  });
});
