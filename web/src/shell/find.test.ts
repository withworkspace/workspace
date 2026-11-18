import { describe, expect, it } from "vitest";
import type { Workspace } from "../api";
import { asks, build, type Found, type FoundGroup, foundItem, type Item, localItems, parseQuery, rank, rematch, withScope } from "./find";
import { recentItems, visitAt, visitOf } from "./recent";

const headline = (label: string, verb?: string) => ({
  group: "needs-you",
  kind: "asks",
  label,
  text: "",
  action: { verb, label: verb ? "accept it" : undefined },
});
const workspace = {
  name: "test",
  root: "/work/ws",
  missions: [
    {
      slug: "fix-token-refresh",
      title: "Fix token refresh",
      state: "active",
      created: "",
      updated: "",
      headline: headline("ready", "accept"),
      capsules: [{ repo: "demo", name: "token-cache" }],
      crew: [{ id: "c1", harness: "claude" }],
    },
    {
      slug: "cache",
      title: "Cache the sessions",
      state: "active",
      created: "",
      updated: "",
      headline: headline("working"),
      capsules: [],
      crew: [],
    },
  ],
  conversations: [
    {
      id: "w4",
      title: "Why is billing slow?",
      opening: "why",
      status: "open",
      at: "",
      updated: "",
    },
  ],
  pages: [{ id: "p3", title: "Token cache notes", updated: "" }],
} as unknown as Workspace;

const local = localItems(workspace);
const file = (title: string, more: Partial<Found> = {}): Found => ({
  kind: "file",
  ref: title,
  title,
  score: 40,
  repo: "demo",
  worktree: ".ground",
  path: title.slice(5),
  ...more,
});
const pull = (number: number, title: string, more: Partial<Found> = {}): Found => ({
  kind: "pr",
  ref: `demo#${number}`,
  title,
  score: 30,
  repo: "demo",
  number,
  author: "sam",
  state: "needs-review",
  words: "asked of you",
  ...more,
});
const titles = (items: Item[]) => items.map((it) => it.title);
const section = (b: ReturnType<typeof build>, group: string) => b.sections.find((s) => s.group === group);

describe("what's typed", () => {
  it("says what it's after by its first character", () => {
    expect(parseQuery("token")).toEqual({ scope: "all", q: "token" });
    expect(parseQuery("> accept")).toEqual({ scope: "do", q: "accept" });
    expect(parseQuery("#12")).toEqual({ scope: "pulls", q: "12" });
    expect(parseQuery("/main.go:12")).toEqual({
      scope: "files",
      q: "main.go:12",
    });
    expect(parseQuery("demo#12")).toEqual({ scope: "all", q: "demo#12" });
    expect(parseQuery("  ")).toEqual({ scope: "all", q: "" });
  });

  it("reads a path on disk whole, not as files only", () => {
    expect(parseQuery("/work/ws/repos/demo/.ground/main.go", "/work/ws")).toEqual({ scope: "all", q: "/work/ws/repos/demo/.ground/main.go" });
    expect(parseQuery("/work/other/main.go", "/work/ws")).toEqual({
      scope: "files",
      q: "work/other/main.go",
    });
  });

  it("changes what it's after and keeps the rest", () => {
    expect(withScope("token", "files")).toBe("/token");
    expect(withScope("/token", "pulls")).toBe("#token");
    expect(withScope("#token", "all")).toBe("token");
  });

  it("asks the server only for what the page doesn't hold", () => {
    expect(asks("all", undefined, "")).toBeUndefined();
    expect(asks("all", undefined, "tok")).toEqual({ limit: 5 });
    expect(asks("pulls", undefined, "")).toEqual({
      kinds: ["pulls"],
      limit: 60,
    });
    expect(asks("files", undefined, "tok")).toEqual({
      kinds: ["ref", "files"],
      limit: 60,
    });
    expect(asks("do", undefined, "acc")).toBeUndefined();
    expect(asks("all", "missions", "tok")).toBeUndefined();
  });
});

describe("what the page holds", () => {
  it("is found by its title, and by its hint for less", () => {
    expect(titles(rank(local, "token refresh")).slice(0, 2)).toEqual(["Fix token refresh", "Accept it: Fix token refresh"]);
    const bySlug = rank(local, "fix-token");
    expect(bySlug[0].title).toBe("Fix token refresh");
    expect(bySlug[0].at).toBeUndefined(); // its slug matched, so nothing in the title's marked
  });

  it("offers what a headline asks, and starting things", () => {
    expect(titles(local.filter((it) => it.group === "do"))).toEqual([
      "Start a conversation",
      "Start a page",
      "Accept it: Fix token refresh",
      "Light theme",
      "Dark theme",
      "Theme from the system",
      "Show the keys",
    ]);
  });
});

describe("the window", () => {
  it("shows what needs you, where you've been and the places before anything's typed", () => {
    const needs = [
      {
        ...local.find((it) => it.id === "mission:fix-token-refresh")!,
        group: "needs" as const,
      },
    ];
    const recent = recentItems(
      [
        {
          id: "mission:fix-token-refresh",
          group: "missions",
          kind: "Mission",
          title: "Fix token refresh",
          to: "/missions/fix-token-refresh",
          at: 2,
        },
        {
          id: "file:demo/main.go",
          group: "files",
          kind: "File",
          title: "demo/main.go",
          to: "/files/demo?path=main.go",
          at: 1,
        },
        {
          id: "mission:gone",
          group: "missions",
          kind: "Mission",
          title: "Gone",
          to: "/missions/gone",
          at: 0,
        },
      ],
      local,
    );
    const b = build({ q: "", scope: "all", local, found: [], needs, recent });
    expect(b.sections.map((s) => s.group)).toEqual(["needs", "recent", "places"]);
    // What needs you isn't said again as recent, and what's gone isn't said.
    expect(titles(section(b, "recent")!.items)).toEqual(["demo/main.go"]);
    expect(b.rows.length).toBe(1 + 1 + 8);
  });

  it("groups what's found, a few of each, with the best of any kind first", () => {
    const found: FoundGroup[] = [
      { kind: "ref", results: [] },
      {
        kind: "pulls",
        results: [pull(12, "Refresh the token before it expires"), pull(89, "Token refresh: retry on 401")],
        at: "2026-11-10T12:00:00Z",
      },
      {
        kind: "files",
        results: ["a", "b", "c", "d", "e", "f"].map((n) => file(`demo/internal/token/${n}.go`)),
        more: 14,
      },
    ];
    const b = build({ q: "token", scope: "all", local, found });
    expect(b.sections.map((s) => s.group)).toEqual(["best", "pulls", "files", "pages", "crew", "capsules", "do"]);
    // A mission outweighs a file that matches as well.
    expect(section(b, "best")!.items[0].id).toBe("mission:fix-token-refresh");
    // The best line isn't said again in its group: that group's gone.
    expect(section(b, "missions")).toBeUndefined();
    // Five files of the six that came, and the rest counted.
    expect(section(b, "files")!.items.length).toBe(5);
    expect(section(b, "files")!.more).toBe(15);
    expect(b.rows.length).toBe(b.sections.reduce((n, s) => n + s.items.length, 0));
  });

  it("puts a ref typed exactly first, whatever else matches", () => {
    const exact = file("demo/main.go", {
      ref: "demo/main.go:12",
      line: 12,
      exact: true,
      score: 1000,
    });
    const b = build({
      q: "demo/main.go:12",
      scope: "all",
      local,
      found: [
        { kind: "ref", results: [exact] },
        {
          kind: "files",
          results: [file("demo/main.go", { line: 12 }), file("demo/main_test.go", { line: 12 })],
        },
      ],
    });
    expect(section(b, "best")!.items[0]).toMatchObject({
      id: "file:demo/main.go",
      ref: "demo/main.go:12",
      to: "/files/demo?path=main.go#L12",
    });
    expect(titles(section(b, "files")!.items)).toEqual(["demo/main_test.go"]);
  });

  it("offers a pull request that isn't in hand after those that are", () => {
    const asking = pull(1, "Open demo#1", { ref: "demo#1", asks: true, exact: true, score: 1000, words: undefined, state: undefined });
    const b = build({ q: "demo#1", scope: "all", local, found: [{ kind: "ref", results: [asking] }, { kind: "pulls", results: [pull(12, "Refresh the token", { score: 390 })] }] });
    expect(section(b, "best")!.items[0].id).toBe("pr:demo#12");
    expect(titles(section(b, "pulls")!.items)).toEqual(["Open demo#1"]);
    // With nothing else it could be, it's what was meant.
    const alone = build({ q: "demo#812", scope: "all", local, found: [{ kind: "ref", results: [{ ...asking, ref: "demo#812", number: 812 }] }, { kind: "pulls", results: [] }] });
    expect(section(alone, "best")!.items[0].id).toBe("pr:demo#812");
  });

  it("names a capsule once, a mission's from what the page holds", () => {
    const cap = (name: string): Found => ({ kind: "capsule", ref: `demo/${name}`, title: `demo/${name}`, score: 50, repo: "demo", worktree: name, hint: "2 files changed" });
    const b = build({ q: "demo/", scope: "all", local, found: [{ kind: "capsules", results: [cap("token-cache"), cap("loose")] }] });
    const caps = b.rows.filter((it) => it.kind === "Capsule");
    expect(caps.map((it) => [it.title, it.to]).sort()).toEqual([
      ["demo/loose", "/files/demo/loose"],
      ["demo/token-cache", "/missions/fix-token-refresh/files/demo/token-cache"],
    ]);
  });

  it("takes a mission's slug, a page's id or a conversation's typed whole as naming it", () => {
    expect(build({ q: "cache", scope: "all", local, found: [] }).sections[0].items[0].id).toBe("mission:cache");
    expect(build({ q: "p3", scope: "all", local, found: [] }).sections[0].items[0].id).toBe("page:p3");
    expect(build({ q: "W4", scope: "all", local, found: [] }).sections[0].items[0].id).toBe("talk:w4");
  });

  it("narrows to one group, and shows all of it", () => {
    const files = Array.from({ length: 20 }, (_, i) => file(`demo/pkg/f${i}.go`));
    const b = build({
      q: "f",
      scope: "files",
      local,
      found: [{ kind: "files", results: files, more: 3, reading: 2 }],
    });
    expect(b.sections.map((s) => s.group)).toEqual(["files"]);
    expect(b.sections[0].items.length).toBe(20);
    expect(b.sections[0].more).toBe(3);
    expect(b.sections[0].note).toBe("Reading 2 capsules: their files may be missing.");
    // Narrowed with ⇥ to a kind the page holds.
    const m = build({
      q: "",
      scope: "all",
      only: "missions",
      local,
      found: [],
    });
    expect(m.sections.map((s) => s.group)).toEqual(["missions"]);
    expect(m.sections[0].items.length).toBe(2);
    // Things to do, by >.
    const d = build({ q: "theme", scope: "do", local, found: [] });
    expect(titles(d.rows)).toEqual(["Dark theme", "Light theme", "Theme from the system"]);
  });

  it("names what capsules have changed, with nothing typed for files", () => {
    const b = build({
      q: "",
      scope: "files",
      local,
      found: [
        {
          kind: "files",
          results: [
            file("demo/token-cache:main.go", {
              worktree: "token-cache",
              path: "main.go",
              status: "modified",
            }),
          ],
        },
      ],
    });
    expect(b.sections[0].title).toBe("Changed in capsules");
    expect(b.sections[0].items[0]).toMatchObject({
      to: "/files/demo/token-cache?path=main.go",
      notes: ["modified in token-cache"],
    });
  });

  it("puts somewhere you go above somewhere you don't", () => {
    const found: FoundGroup[] = [
      {
        kind: "files",
        results: [file("demo/a/cache.go"), file("demo/b/cache.go")],
      },
    ];
    const plain = build({ q: "cache.go", scope: "files", local, found });
    expect(titles(plain.rows)).toEqual(["demo/a/cache.go", "demo/b/cache.go"]);
    const been = build({
      q: "cache.go",
      scope: "files",
      local,
      found,
      memory: { visits: [{ id: "file:demo/b/cache.go" }], chosen: {} },
    });
    expect(titles(been.rows)).toEqual(["demo/b/cache.go", "demo/a/cache.go"]);
    const chose = build({
      q: "cache.go",
      scope: "files",
      local,
      found,
      memory: {
        visits: [{ id: "file:demo/b/cache.go" }],
        chosen: { "cache.go": "file:demo/a/cache.go" },
      },
    });
    expect(titles(chose.rows)).toEqual(["demo/a/cache.go", "demo/b/cache.go"]);
  });
});

describe("a plugin's things", () => {
  const known = {
    repos: new Map(),
    missions: [],
    pages: [],
    talks: [],
    things: [{ plugin: "jira", what: "issue", title: "Jira issue", prefixes: ["RAM-"], address: "https://acme.atlassian.net/browse/{ref}", path: "/issue/{ref}" }],
  };
  const card = { kind: "thing", plugin: "jira", what: "issue", id: "RAM-12", url: "https://acme.atlassian.net/browse/RAM-12" } as const;
  const told: Found = {
    kind: "said",
    ref: "RAM-12",
    title: "Token refresh is flaky",
    thing: "thing",
    place: "thing",
    what: "Jira issue",
    where: "the jira plugin",
    state: "In Progress",
    hint: "The clock should be injected.",
    score: 300,
    card,
    to: "/x/jira/issue/RAM-12",
  };

  it("are found by what they say, as their plugin told ws of them", () => {
    const it = foundItem(told);
    expect(it).toMatchObject({ id: "thing:jira/RAM-12", group: "said", kind: "Jira issue", title: "Token refresh is flaky", to: "/x/jira/issue/RAM-12", ref: "RAM-12", notes: ["RAM-12", "In Progress"] });
    // One with nowhere to open in ws goes to where it is outside.
    expect(foundItem({ ...told, to: card.url }).to).toBe(card.url);
    // One whose plugin has gone is a name, and goes nowhere.
    expect(foundItem({ ...told, card: undefined, to: undefined, what: "something a plugin had" })).toMatchObject({ kind: "Thing", to: undefined });
  });

  it("are named outright by a name typed whole, told of or not", () => {
    // Told of: the best line is the thing as ws has it.
    const b = build({ q: "RAM-12", scope: "all", local: [], found: [{ kind: "said", results: [told] }], known });
    expect(b.sections[0]).toMatchObject({ group: "best", items: [{ id: "thing:jira/RAM-12", title: "Token refresh is flaky" }] });
    expect(b.rows.filter((r) => r.id === "thing:jira/RAM-12")).toHaveLength(1);
    // Not told of (it's not on the plugin's board): its name, to go to,
    // with a card that asks the plugin.
    const off = build({ q: "RAM-99", scope: "all", local: [], found: [], known });
    expect(off.sections[0]).toMatchObject({ group: "best", items: [{ id: "thing:jira/RAM-99", kind: "Jira issue", title: "RAM-99", to: "/x/jira/issue/RAM-99", card: { kind: "thing", id: "RAM-99" } }] });
    // By where it is, too.
    expect(build({ q: "https://acme.atlassian.net/browse/RAM-7?x=1", scope: "all", local: [], found: [], known }).sections[0].items[0].id).toBe("thing:jira/RAM-7");
    // Words that only look like a name name nothing.
    expect(build({ q: "UTF-8", scope: "all", local: [], found: [], known }).sections).toHaveLength(0);
  });
});

describe("what the server found", () => {
  it("is a file to open at its lines, saying who's changed it", () => {
    const it = foundItem(
      file("demo/main.go", {
        ref: "demo/main.go:12-20",
        line: 12,
        endLine: 20,
        in: [
          { capsule: "token-cache", status: "modified" },
          { capsule: "old", status: "removed" },
        ],
      }),
    );
    expect(it).toMatchObject({
      id: "file:demo/main.go",
      group: "files",
      to: "/files/demo?path=main.go#L12-L20",
      ref: "demo/main.go:12-20",
      path: true,
    });
    expect(it.notes).toEqual(["lines 12 to 20", "changed in token-cache", "removed in old"]);
    expect(it.card).toEqual({
      kind: "file",
      repo: "demo",
      worktree: ".ground",
      path: "main.go",
      line: 12,
      endLine: 20,
    });
  });

  it("is a capsule's own file, in its capsule", () => {
    const it = foundItem(
      file("demo/token-cache:store.go", {
        worktree: "token-cache",
        path: "store.go",
        status: "added",
      }),
    );
    expect(it).toMatchObject({
      to: "/files/demo/token-cache?path=store.go",
      notes: ["added in token-cache"],
    });
  });

  it("is a pull request to review, with where it stands", () => {
    expect(foundItem(pull(18, "WIP", { draft: true, words: "draft" })).notes).toEqual(["draft"]);
    const it = foundItem(
      pull(12, "Refresh the token", {
        checks: "failure",
        stack: "2 of 3: Cache",
      }),
    );
    expect(it).toMatchObject({
      id: "pr:demo#12",
      group: "pulls",
      to: "/reviews/demo/12",
      ref: "demo#12",
      hint: "demo#12 · sam · 2 of 3: Cache",
      tone: "attention",
    });
    expect(it.notes).toEqual(["asked of you", "checks fail"]);
    const asked = foundItem(
      pull(812, "Open demo#812", {
        asks: true,
        hint: "not among the open ones here: GitHub's asked",
        words: undefined,
        state: undefined,
      }),
    );
    expect(asked).toMatchObject({
      to: "/reviews/demo/812",
      hint: "not among the open ones here: GitHub's asked",
    });
  });

  it("is kept between letters, to what still matches", () => {
    const was: FoundGroup[] = [
      { kind: "ref", results: [file("demo/main.go", { exact: true })] },
      {
        kind: "files",
        results: [file("demo/main.go"), file("demo/README.md")],
        more: 9,
      },
    ];
    const now = rematch(was, "main");
    expect(now.map((g) => g.kind)).toEqual(["files"]);
    expect(now[0].results.map((f) => f.title)).toEqual(["demo/main.go"]);
    expect(now[0].results[0].at).toEqual([5, 6, 7, 8]);
    expect(now[0].more).toBe(0);
    // Lines asked for aren't letters to match.
    expect(rematch(was, "main:12")[0].results.length).toBe(1);
  });
});

describe("where you've been", () => {
  it("is read from where the address bar is", () => {
    expect(visitAt("/missions/fix-token-refresh", "", "", workspace, 1)).toMatchObject({
      id: "mission:fix-token-refresh",
      title: "Fix token refresh",
    });
    expect(visitAt("/pages/p3", "", "", workspace, 1)).toMatchObject({
      id: "page:p3",
      title: "Token cache notes",
    });
    expect(visitAt("/conversations/w4", "", "", workspace, 1)).toMatchObject({
      id: "talk:w4",
    });
    expect(visitAt("/reviews/demo/12", "", "", workspace, 1)).toMatchObject({
      id: "pr:demo#12",
      title: "demo#12",
    });
    expect(visitAt("/files/demo", "?path=internal%2Fa.go", "#L3", workspace, 1)).toMatchObject({
      id: "file:demo/internal/a.go",
      to: "/files/demo?path=internal%2Fa.go#L3",
    });
    expect(visitAt("/files/demo/token-cache", "?path=a.go", "", workspace, 1)).toMatchObject({ id: "file:demo/token-cache:a.go" });
    // A list, a mission's crew, a mission that isn't there: not kept.
    for (const p of ["/", "/reviews", "/files/demo", "/missions/fix-token-refresh/crew/c1", "/missions/nope"]) expect(visitAt(p, "", "", workspace, 1)).toBeUndefined();
  });

  it("is kept as the kind of thing it is, wherever it was shown", () => {
    const best = {
      ...foundItem(pull(12, "Refresh the token")),
      group: "best" as const,
    };
    expect(visitOf(best, 1)).toMatchObject({
      id: "pr:demo#12",
      group: "pulls",
      title: "Refresh the token",
    });
    expect(
      visitOf(
        local.find((it) => it.id === "place:/reviews")!,
        1,
      ),
    ).toBeUndefined();
    expect(
      visitOf(
        local.find((it) => it.id === "do:keys")!,
        1,
      ),
    ).toBeUndefined();
  });
});

describe("a document found by what it says", () => {
  it("is in Documents, led to its section", () => {
    const it = foundItem({ kind: "document", ref: "cr-records:adr/0013-hooks.md", title: "ADR-0013: Hooks › Decision", hint: "most hooks are…", score: 9, anchor: "decision" });
    expect(it.group).toBe("documents");
    expect(it.to).toBe("/documents/cr-records/adr/0013-hooks.md#decision");
  });
});

describe("what ws records, found by what it says", () => {
  const said = (f: Partial<Found>): Found => ({ kind: "said", ref: "d1", title: "Key the cache by tenant", score: 9, scope: "cache", thing: "decision", place: "decision", what: "a decision", where: "mission cache", hint: "Tenants share nothing.", ...f });

  it("is in Said, a line with where it's read, and goes to the thing's page", () => {
    const d = foundItem(said({}));
    expect([d.group, d.kind, d.icon, d.to, d.notes]).toEqual(["said", "Decision", "gavel", "/missions/cache", ["cache"]]);
    expect(d.said?.ref).toBe("d1");
    const cases: [Partial<Found>, string, string][] = [
      [{ ref: "cache", thing: "mission", place: "brief", anchor: "constraints" }, "Brief", "/missions/cache?tab=brief"],
      [{ ref: "cache", thing: "mission", place: "brief" }, "Mission", "/missions/cache"],
      [{ ref: "t3", thing: "thread-ask", place: "turn", anchor: "2", of: "records/cache:adr/1.md" }, "Thread", "/missions/cache/artifact?ref=records%2Fcache%3Aadr%2F1.md"],
      [{ ref: "t1", thing: "talk", place: "turn", anchor: "2" }, "Talk", "/missions/cache/conversations/t1"],
      [{ ref: "r1.c2", thing: "comment", place: "comment", of: "demo/token-cache" }, "Comment", "/missions/cache/review/demo/token-cache"],
      [{ ref: "mission:handoffs/c1.md", thing: "handoff", place: "handoff" }, "Handoff", "/missions/cache/artifact?ref=mission%3Ahandoffs%2Fc1.md"],
      [{ ref: "r1.c1", scope: "api#89", thing: "comment", place: "comment" }, "Comment", "/reviews/api/89"],
      [{ ref: "api#89", scope: "api#89", thing: "pull-request", place: "pull-request" }, "Pull request", "/reviews/api/89"],
      [{ ref: "w4", scope: "", thing: "talk", place: "turn" }, "Talk", "/conversations/w4"],
      [{ ref: "w7", scope: "", thing: "pass", place: "finding", of: "p3" }, "Finding", "/pages/p3"],
      [{ ref: "p3", scope: "", thing: "page", place: "page", anchor: "why", of: "p3" }, "Page", "/pages/p3"],
    ];
    for (const [f, kind, to] of cases) {
      const x = foundItem(said(f));
      expect([x.kind, x.to]).toEqual([kind, to]);
    }
    expect(foundItem(said({ ended: true })).notes).toEqual(["cache", "archived"]);
    expect(foundItem(said({ ref: "w4", scope: "", thing: "talk" })).notes).toEqual([]);
  });

  it("sits after Documents, and narrowing to it asks the server for it", () => {
    const found: FoundGroup[] = [
      { kind: "said", results: [said({}), said({ ref: "d2", title: "Tenants are keyed by id", score: 4 })] },
      { kind: "documents", results: [{ kind: "document", ref: "records:adr/1.md", title: "ADR 1", score: 5 }] },
    ];
    const titles = build({ q: "tenant", scope: "all", local: [], found }).sections.map((s) => s.title);
    expect(titles.slice(1)).toEqual(["Documents", "Said"]); // the first is the best line
    expect(asks("all", "said", "tenant")).toEqual({ kinds: ["said"], limit: 60 });
    expect(asks("all", "documents", "tenant")).toEqual({ kinds: ["documents"], limit: 60 });
  });

  it("doesn't say again a pull request that's listed", () => {
    const kept = (number: number, title: string) => said({ ref: `demo#${number}`, scope: `demo#${number}`, title, thing: "pull-request", place: "pull-request" });
    const found: FoundGroup[] = [
      { kind: "pulls", results: [pull(20, "Move the greeting into a package")] },
      { kind: "said", results: [kept(20, "Move the greeting into a package"), kept(11, "Greet by name")] },
    ];
    const b = build({ q: "greeting", scope: "all", local: [], found });
    const ids = (g: string) => b.sections.find((s) => s.group === g)?.items.map((it) => it.id);
    expect(ids("said")).toEqual(["said:demo#11/demo#11"]);
  });

  it("doesn't say again a page that's listed", () => {
    const pg = (id: string, title: string) => said({ ref: id, scope: "", title, thing: "page", place: "page", of: id });
    const found: FoundGroup[] = [{ kind: "said", results: [pg("p3", "Token cache notes"), pg("p8", "Revocation")] }];
    const b = build({ q: "token", scope: "all", local, found });
    const ids = (g: string) => b.sections.find((s) => s.group === g)?.items.map((it) => it.id);
    expect(ids("pages")).toEqual(["page:p3"]);
    expect(ids("said")).toEqual(["said:/p8"]);
  });
});
