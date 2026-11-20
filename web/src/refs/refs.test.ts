import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";
import { type Files, findAll, findRefs, GROUND, groundPaths, isOutside, type Known, type Named, type PageRef, parseAddress, parseName, refAddress, refAt, refHref, refName, reposNamed, thingAtPath, thingOpenAt, type Where } from "./refs";

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
  things?: Named[];
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
  things: j.things,
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
      // A repo's name alone is read as a mission's, and a plugin's
      // thing's isn't read by its form: which names are things is known.
      if (c.ref.kind !== "repo" && c.ref.kind !== "thing") expect(parseName(c.name)).toEqual(c.ref);
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

describe("refs on a repo whose files are checked as they're named", () => {
  // demo's files as a page has them: main.go and internal/greet checked
  // and there, nope.go checked and not; nothing else asked yet.
  const answered = new Set(["main.go", "nope.go", "internal/greet", "README"]);
  const files: Files = {
    has: (p) => (answered.has(p) ? p === "main.go" || p === "README" : undefined),
    hasDir: (p) => (answered.has(p) ? p === "internal/greet" : undefined),
  };
  const k: Known = { repos: new Map([["demo", files]]), missions: [], pages: [], talks: [] };
  const found = (text: string) => findRefs(text, k).map((f) => [text.slice(f.from, f.to), f.ref.kind]);

  it("finds what's been checked as it is", () => {
    expect(found("see demo/main.go, demo/internal/greet/ and demo/README.")).toEqual([
      ["demo/main.go", "file"],
      ["demo/internal/greet/", "dir"],
      ["demo/README", "file"],
    ]);
    expect(found("demo/nope.go isn't there")).toEqual([]);
  });
  it("takes what looks like a file for one until it's checked, and nothing else", () => {
    expect(found("demo/later.go:3 and demo/later.md. demo/internal/other")).toEqual([
      ["demo/later.go:3", "file"],
      ["demo/later.md", "file"],
    ]);
  });
  it("says what's to be checked of each repo a text names", () => {
    const known: Known = { repos: new Map([["demo", undefined], ["records", undefined]]), aliases: new Map([["dm", "demo"]]), missions: [], pages: [], talks: [], root: "/ws" };
    const got = groundPaths("demo/main.go:3, dm/internal/ and /ws/repos/records/.ground/adr/one.md. In records `notes/a.md:2`; demo/x/y.", known);
    expect(Object.fromEntries([...got].map(([r, ps]) => [r, [...ps].sort()]))).toEqual({
      demo: ["internal", "main.go", "x/y", "x/y."],
      records: ["adr/one.md", "adr/one.md.", "notes/a.md"],
    });
  });
});

describe("a plugin's thing", () => {
  const jira = "https://acme.atlassian.net/browse/";
  const issue: PageRef = { kind: "thing", plugin: "jira", what: "issue", id: "RAM-123", url: `${jira}RAM-123` };
  const sprint: PageRef = { kind: "thing", plugin: "jira", what: "sprint", id: "SPRINT-4" };
  const things: Named[] = [
    { plugin: "jira", what: "issue", prefixes: ["RAM-", "RAP-"], tail: "number", address: `${jira}{ref}`, path: "/issue/{ref}" },
    { plugin: "jira", what: "sprint", prefixes: ["SPRINT-"] },
    { plugin: "jira", what: "epic", prefixes: ["RAM-"], path: "/epic/{ref}" },
  ];
  const known: Known = { repos: new Map(), missions: [], pages: [], talks: [], things };

  it("a kind's page by a folder's name is the thing that lives there, for a kind whose things can live in a repo", () => {
    const sketches: Known = { ...known, things: [...things, { plugin: "sketch", what: "sketch", prefixes: ["sketch-"], path: "/s/{ref}", remote: true }] };
    const inRepo = { kind: "thing", plugin: "sketch", what: "sketch", id: "records/sketches/release-flow/" };
    // As ws writes the address: the name as one part of the path.
    expect(thingAtPath("/x/sketch/s/records%2Fsketches%2Frelease-flow%2F", sketches)).toEqual(inRepo);
    expect(thingOpenAt("sketch", "/s/records%2Fsketches%2Frelease-flow%2F", sketches)).toEqual(inRepo);
    // One of the kind's own names is as ever.
    expect(thingOpenAt("sketch", "/s/sketch-12", sketches)).toEqual({ kind: "thing", plugin: "sketch", what: "sketch", id: "sketch-12" });
    // Not a folder in a repo; and a kind whose things don't live in one.
    expect(thingOpenAt("sketch", "/s/records", sketches)).toBeUndefined();
    expect(thingOpenAt("sketch", "/s/records%2Fsketches%2Frelease-flow", sketches)).toBeUndefined();
    expect(thingAtPath("/x/jira/issue/records%2Fissues%2Fa%2F", sketches)).toBeUndefined();
  });

  it("goes where its kind opens one in the plugin's app", () => {
    expect(refHref(issue, known)).toBe("/x/jira/issue/RAM-123");
    expect(isOutside(refHref(issue, known))).toBe(false);
  });
  it("goes to its address outside ws when its kind opens nowhere, or to the plugin", () => {
    expect(refHref(issue, { ...known, things: [{ ...things[0], path: undefined }] })).toBe(`${jira}RAM-123`);
    expect(isOutside(`${jira}RAM-123`)).toBe(true);
    // Its plugin's gone, or what there is to name isn't given.
    expect(refHref(issue)).toBe(`${jira}RAM-123`);
    expect(refHref(sprint, known)).toBe("/x/jira");
    expect(isOutside("/x/jira")).toBe(false);
  });
  it("is the first kind's to name its words", () => {
    expect(findRefs("RAM-123 in SPRINT-4", known).map((f) => f.ref)).toEqual([issue, sprint]);
    expect(refAt("RAM-123", known)).toEqual(issue);
    expect(refAt("ws:SPRINT-4", known)).toEqual(sprint);
    expect(refAt(`${jira}RAM-123?focusedCommentId=7`, known)).toEqual(issue);
  });
  it("is only words where what there is to name has none", () => {
    const none: Known = { repos: new Map(), missions: [], pages: [], talks: [] };
    expect(findRefs("RAM-123", none)).toEqual([]);
    expect(parseAddress(`${jira}RAM-123`, none)).toBeUndefined();
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
