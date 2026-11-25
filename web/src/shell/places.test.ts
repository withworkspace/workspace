import { describe, expect, it } from "vitest";
import { arrange, goKeys, goKeysTo, headed, move, placesOf, saving, type Place } from "./places";

const place = (id: string, under?: string): Place => ({ id, to: `/${id}`, icon: "box", label: id, match: () => false, under });
const ids = (ps: Place[]) => ps.map((p) => p.id);

const places = [place("missions"), place("conversations"), place("reviews"), place("team"), place("x/jira", "jira"), place("x/jira/board", "jira")];

describe("arrange", () => {
  it("is the places as they come, none in the menu, when nothing's saved", () => {
    const a = arrange(places);
    expect(ids(a.sidebar)).toEqual(ids(places));
    expect(a.menu).toEqual([]);
  });

  it("puts each where it's saved, those in neither at the sidebar's end, and passes by ids that aren't places", () => {
    const a = arrange(places, { sidebar: ["reviews", "gone", "missions"], menu: ["team", "x/old"] });
    expect(ids(a.sidebar)).toEqual(["reviews", "missions", "conversations", "x/jira", "x/jira/board"]);
    expect(ids(a.menu)).toEqual(["team"]);
  });

  it("puts a place whose home is the menu there, until it's arranged", () => {
    const withRuns = [...places, { ...place("runs"), home: "menu" as const }];
    expect(ids(arrange(withRuns).menu)).toEqual(["runs"]);
    expect(ids(arrange(withRuns).sidebar)).not.toContain("runs");
    // Named in neither list of an arrangement: after the menu's own.
    expect(ids(arrange(withRuns, { sidebar: ["missions"], menu: ["team"] }).menu)).toEqual(["team", "runs"]);
    // Named in the sidebar: there.
    const moved = arrange(withRuns, { sidebar: ["runs", "missions"], menu: [] });
    expect(ids(moved.sidebar).slice(0, 2)).toEqual(["runs", "missions"]);
    expect(moved.menu).toEqual([]);
    // And saved where it is, so it stays.
    expect(saving(moved, withRuns).sidebar).toContain("runs");
  });

  it("takes a place named in both lists once, in the menu", () => {
    const a = arrange(places, { sidebar: ["team", "missions"], menu: ["team"] });
    expect(ids(a.menu)).toEqual(["team"]);
    expect(ids(a.sidebar)).not.toContain("team");
  });
});

describe("move", () => {
  const a = arrange(places);

  it("puts places before or after one, or at a list's end", () => {
    expect(ids(move(a, ["team"], "sidebar", { id: "missions", after: false }).sidebar).slice(0, 2)).toEqual(["team", "missions"]);
    expect(ids(move(a, ["missions"], "sidebar", { id: "reviews", after: true }).sidebar).slice(0, 4)).toEqual(["conversations", "reviews", "missions", "team"]);
    const m = move(a, ["team", "reviews"], "menu");
    expect(ids(m.menu)).toEqual(["team", "reviews"]);
    expect(ids(m.sidebar)).toEqual(["missions", "conversations", "x/jira", "x/jira/board"]);
  });

  it("moves between the lists, either way", () => {
    const m = move(move(a, ["team"], "menu"), ["team"], "sidebar", { id: "missions", after: true });
    expect(m.menu).toEqual([]);
    expect(ids(m.sidebar).slice(0, 2)).toEqual(["missions", "team"]);
  });
});

describe("saving", () => {
  it("keeps the ids of places that are away, after each list's", () => {
    const saved = { sidebar: ["x/gone", "missions"], menu: ["x/old/view"] };
    const a = move(arrange(places, saved), ["reviews"], "menu");
    expect(saving(a, places, saved)).toEqual({
      sidebar: ["missions", "conversations", "team", "x/jira", "x/jira/board", "x/gone"],
      menu: ["reviews", "x/old/view"],
    });
  });
});

describe("headed", () => {
  it("titles where a plugin's group starts, again if it's split", () => {
    const list = [place("missions"), place("x/a", "jira"), place("x/b", "jira"), place("team"), place("x/c", "jira")];
    expect(headed(list)).toEqual([undefined, "jira", undefined, undefined, "jira"]);
  });
});

describe("the keys that go to a place", () => {
  it("are g and its letter, for each of ws's own that has one", () => {
    expect(goKeysTo("/")).toBe("g m");
    expect(goKeysTo("/documents")).toBe("g d");
    expect(goKeysTo("/files")).toBe("g f");
    expect(goKeysTo("/runs")).toBeUndefined();
  });

  it("are what each place's link says, and every letter goes to one of them", () => {
    const own = placesOf(undefined, []);
    expect(own.filter((p) => p.keys).map((p) => [p.id, p.keys])).toEqual([
      ["missions", "g m"],
      ["conversations", "g c"],
      ["documents", "g d"],
      ["artifacts", "g a"],
      ["reviews", "g r"],
      ["team", "g t"],
      ["files", "g f"],
      ["orchestrator", "g o"],
    ]);
    const tos = new Set(own.map((p) => p.to));
    expect(Object.values(goKeys).filter((to) => !tos.has(to))).toEqual([]);
  });
});
