import { describe, expect, it } from "vitest";
import type { Space, SpaceDoc } from "../api";
import { fileShelves, filterItems, holds, plural, siblings, spaceShelves, trail } from "./shelves";

const space = (o: Partial<Space> = {}): Space => ({ repo: "records", label: "Records", color: "", count: 0, cloned: true, folders: [], kinds: [], ...o });

// A records space: ADRs (numbered), notes (a kind with no number) and,
// under Other, a README and guides in nested folders.
const records = space({
  kinds: [
    { name: "adr", title: "ADR", states: [] },
    { name: "note", title: "Note" },
    { name: "plan", title: "Plan" }, // none yet
  ],
});
const docs: SpaceDoc[] = [
  { path: "adr/0010-tens.md", title: "Tens", kind: "adr", number: "0010" },
  { path: "adr/0002-two.md", title: "Two", kind: "adr", number: "0002" },
  { path: "adr/0001-one.md", title: "One", kind: "adr", number: "0001" },
  { path: "adr/README.md", title: "About ADRs" }, // in adr/, of no kind
  { path: "notes/greeting.md", title: "Greeting", kind: "note" },
  { path: "README.md", title: "records" },
  { path: "guides/setup/install.md", title: "Install" },
  { path: "guides/b-using.md", title: "Using it" },
  { path: "guides/a-start.md", title: "Starting" },
  { path: "guides/10-later.md", title: "Later" },
  { path: "guides/2-sooner.md", title: "Sooner" },
];

describe("spaceShelves", () => {
  const s = spaceShelves(records, docs);

  it("puts its kinds on top, then its folders, then its own documents", () => {
    expect(s.shelves.get("")!.items.map((i) => [i.shelf ?? i.path, i.title, i.note])).toEqual([
      ["kind:adr", "ADRs", "3"],
      ["kind:note", "Notes", "1"],
      ["adr/", "adr", "1"],
      ["guides/", "guides", "5"],
      ["README.md", "records", "README.md"],
    ]);
  });

  it("lists a kind in the library's order, each by its number", () => {
    const adr = s.shelves.get("kind:adr")!;
    expect(adr).toMatchObject({ label: "ADRs", name: "ADRs", up: "", kind: "adr" });
    expect(adr.items.map((i) => [i.path, i.note])).toEqual([
      ["adr/0001-one.md", "0001"],
      ["adr/0002-two.md", "0002"],
      ["adr/0010-tens.md", "0010"],
    ]);
  });

  it("lists a folder's folders, then its documents in the order of their paths", () => {
    const g = s.shelves.get("guides/")!;
    expect(g).toMatchObject({ label: "guides", name: "guides/", up: "" });
    expect(g.items.map((i) => i.shelf ?? i.path)).toEqual(["guides/setup/", "guides/2-sooner.md", "guides/10-later.md", "guides/a-start.md", "guides/b-using.md"]);
    expect(s.shelves.get("guides/setup/")).toMatchObject({ label: "setup", name: "guides/setup/", up: "guides/" });
  });

  it("keeps a kind's document on its kind, and any other in its folder", () => {
    expect(trail(s, "adr/0002-two.md").map((x) => x.label)).toEqual(["Records", "ADRs"]);
    expect(trail(s, "adr/README.md").map((x) => x.label)).toEqual(["Records", "adr"]);
    expect(trail(s, "guides/setup/install.md").map((x) => x.label)).toEqual(["Records", "guides", "setup"]);
    expect(trail(s, "README.md").map((x) => x.label)).toEqual(["Records"]);
    expect(trail(s, "nowhere.md")).toEqual([]);
  });

  it("has the document being read, when the list it's from doesn't yet", () => {
    const t = spaceShelves(records, docs, { path: "adr/0011-new.md", title: "New", kind: "adr" });
    expect(t.shelves.get("kind:adr")!.items.map((i) => i.path)).toContain("adr/0011-new.md");
    const u = spaceShelves(records, docs, { path: "drafts/x.md", title: "X" });
    expect(trail(u, "drafts/x.md").map((x) => x.id)).toEqual(["", "drafts/"]);
  });

  it("reads a folder space from below its folder", () => {
    const f = spaceShelves(space({ repo: "site", label: "Site docs", folder: "docs" }), [
      { path: "docs/index.md", title: "Index" },
      { path: "docs/how/one.md", title: "One" },
    ]);
    expect(f.shelves.get("")!.items.map((i) => i.shelf ?? i.path)).toEqual(["docs/how/", "docs/index.md"]);
    expect(f.shelves.get("docs/how/")).toMatchObject({ label: "how", name: "how/", up: "" });
  });
});

describe("siblings", () => {
  const s = spaceShelves(records, docs);

  it("is the documents kept with it, and where it is among them", () => {
    const sib = siblings(s, "adr/0002-two.md")!;
    expect(sib.shelf.id).toBe("kind:adr");
    expect([sib.prev?.path, sib.next?.path, sib.at, sib.of]).toEqual(["adr/0001-one.md", "adr/0010-tens.md", 2, 3]);
  });

  it("has nothing before the first or after the last, and leaves out folders", () => {
    const first = siblings(s, "guides/2-sooner.md")!;
    expect([first.prev, first.next?.path, first.at, first.of]).toEqual([undefined, "guides/10-later.md", 1, 4]);
    expect(siblings(s, "guides/b-using.md")!.next).toBeUndefined();
  });

  it("is nothing for a document it doesn't know", () => {
    expect(siblings(s, "nowhere.md")).toBeUndefined();
  });
});

describe("fileShelves", () => {
  it("shelves a worktree's markdown by folder, each by its file's name", () => {
    const f = fileShelves("demo/ground", ["main.go", "README.md", "docs/a.md", "docs/deep/b.md", "docs/logo.png"]);
    expect(f.shelves.get("")!.items.map((i) => [i.shelf ?? i.path, i.title, i.note])).toEqual([
      ["docs/", "docs", "2"],
      ["README.md", "README.md", ""],
    ]);
    expect(trail(f, "docs/deep/b.md").map((x) => x.name)).toEqual(["demo/ground", "docs/", "docs/deep/"]);
    expect(siblings(f, "docs/a.md")).toMatchObject({ at: 1, of: 1 });
  });

  it("has the document being read, though the files don't", () => {
    const f = fileShelves("demo/ground", [], { path: "notes/x.md", title: "X" });
    expect(trail(f, "notes/x.md").map((x) => x.id)).toEqual(["", "notes/"]);
  });
});

describe("filterItems and holds", () => {
  const s = spaceShelves(records, docs);
  const top = s.shelves.get("")!.items;

  it("filters by title, note or file name, any case", () => {
    expect(filterItems(top, "  ").length).toBe(top.length);
    expect(filterItems(top, "GUI").map((i) => i.shelf)).toEqual(["guides/"]);
    expect(filterItems(s.shelves.get("kind:adr")!.items, "0010").map((i) => i.title)).toEqual(["Tens"]);
    expect(filterItems(s.shelves.get("guides/")!.items, "b-using").map((i) => i.title)).toEqual(["Using it"]);
  });

  it("holds the document, and the shelves it's on", () => {
    const on = trail(s, "guides/setup/install.md");
    expect(top.filter((i) => holds(i, "guides/setup/install.md", on)).map((i) => i.shelf)).toEqual(["guides/"]);
    const guides = s.shelves.get("guides/")!.items;
    expect(guides.filter((i) => holds(i, "guides/a-start.md", trail(s, "guides/a-start.md"))).map((i) => i.path)).toEqual(["guides/a-start.md"]);
  });
});

describe("plural", () => {
  it("names a kind's list", () => {
    expect(["ADR", "Plan", "Build proposal", "Policy", "Day", "Box", "Sketch", "Class"].map(plural)).toEqual(["ADRs", "Plans", "Build proposals", "Policies", "Days", "Boxes", "Sketches", "Classes"]);
  });
});
