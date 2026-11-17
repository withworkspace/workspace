import { describe, expect, it } from "vitest";
import { docAddress, type DocKind, type PageInfo, pageAddress, type Space, type SpaceDoc, spaceAt } from "../api";
import { sorted } from "./Library";
import { placePages, refPlace } from "./place";

const spaces: Space[] = [
  { repo: "records", label: "Records", color: "#000", count: 3, cloned: true, folders: [], kinds: [] },
  { repo: "api", label: "API", color: "#000", folder: "docs/", count: 1, cloned: true, folders: [], kinds: [] },
];
const kinds: DocKind[] = [
  { name: "bp", title: "Build proposal", where: "records:bp/proposed/{date}-{slug}.md" },
  { name: "rfc", title: "RFC", where: "web:rfcs/{number}-{slug}.md" },
];
const page = (id: string, more: Partial<PageInfo> = {}): PageInfo => ({ id, title: id, updated: "2026-11-12T00:00:00Z", ...more });
const source = (ref: string) => ({ ref, version: "v", at: "" });

describe("placePages", () => {
  it("lists each page where it'll live", () => {
    const placed = placePages(
      [
        page("p1"), // a scratch page
        page("p2", { source: source("records:adr/0001.md"), unsaved: true }),
        page("p3", { source: source("records:adr/0002.md") }), // nothing to save
        page("p4", { source: source("demo:README.md"), unsaved: true }), // not a space
        page("p5", { kind: "bp" }),
        page("p6", { kind: "rfc" }), // its kind's path isn't in a space
        page("p7", { kind: "bp", filed: { ref: "records/bp-x:bp/proposed/2026-11-12-x.md", kind: "bp", at: "", version: "" } }),
        page("p8", { kind: "bp", filed: { ref: "records:bp/proposed/2026-11-11-y.md", kind: "bp", at: "", version: "", landed: true } }),
        page("p9", { source: source("api:docs/guide.md"), unsaved: true }),
        page("p10", { source: source("api:README.md"), unsaved: true }), // api, but not its docs folder
        page("p11", { to: "records:guides/" }),
        page("p12", { to: "demo:notes/" }), // not a space
      ],
      spaces,
      kinds,
    );
    const where = Object.fromEntries(placed.map((p) => [p.page.id, p.repo ? `${p.repo} ${p.why}` : "local"]));
    expect(where).toEqual({
      p1: "local",
      p2: "records unsaved",
      p4: "local",
      p5: "records kind",
      p6: "local",
      p7: "records filed",
      p9: "api unsaved",
      p10: "local",
      p11: "records to",
      p12: "local",
    });
  });
  it("reads a ref as its repo, capsule and path", () => {
    expect(refPlace("records/bp-x:bp/a.md")).toEqual({ repo: "records", capsule: "bp-x", path: "bp/a.md" });
    expect(refPlace("records:a.md")).toEqual({ repo: "records", capsule: undefined, path: "a.md" });
    expect(refPlace("p3")).toBeUndefined();
  });
});

describe("a document's one address", () => {
  it("is its repo and path, a capsule's version and editing as asked", () => {
    expect(docAddress("cr-records:adr/proposed/0001 uom.md")).toBe("/documents/cr-records/adr/proposed/0001%20uom.md");
    expect(docAddress("records/edit-risk:methodology/risk.md")).toBe("/documents/records/methodology/risk.md?in=edit-risk");
    expect(docAddress("records:a.md", true)).toBe("/documents/records/a.md?edit");
    expect(docAddress("records/x:a.md", "p8")).toBe("/documents/records/a.md?in=x&edit=p8");
  });
  it("is the document a page is tied to, edited, else Local's", () => {
    expect(pageAddress({ id: "p3" })).toBe("/documents/local/p3");
    expect(pageAddress({ id: "p8", source: { ref: "cr-docs:technical/deploy.md" } })).toBe("/documents/cr-docs/technical/deploy.md?edit=p8");
  });
  it("is said by a space only for its markdown under its folder", () => {
    const spaces = [{ repo: "api", folder: "docs/", label: "API", color: "" }];
    expect(spaceAt(spaces, "api", "docs/guide.md")?.repo).toBe("api");
    expect(spaceAt(spaces, "api", "README.md")).toBeUndefined();
    expect(spaceAt(spaces, "api", "docs/logo.png")).toBeUndefined();
  });
});

describe("a kind's list", () => {
  const d = (path: string, more: Partial<SpaceDoc>): SpaceDoc => ({ path, title: path, ...more });
  it("is the oldest number first, or the newest date", () => {
    const adrs = [d("0010.md", { number: "0010" }), d("0002.md", { number: "0002" }), d("0100.md", { number: "0100" })];
    expect(sorted(adrs, "made").map((x) => x.number)).toEqual(["0002", "0010", "0100"]);
    const bps = [d("a.md", { date: "2026-10-01" }), d("b.md", { date: "2026-11-12" })];
    expect(sorted(bps, "made").map((x) => x.date)).toEqual(["2026-11-12", "2026-10-01"]);
  });
  it("sorts by where each stands, in the order of its life, and by when it changed", () => {
    const bps = [d("a.md", { state: "accepted", changed: "2026-11-01" }), d("b.md", { state: "proposed", changed: "2026-11-10" })];
    expect(sorted(bps, "stands", ["proposed", "accepted"]).map((x) => x.state)).toEqual(["proposed", "accepted"]);
    expect(sorted(bps, "changed").map((x) => x.path)).toEqual(["b.md", "a.md"]);
  });
});
