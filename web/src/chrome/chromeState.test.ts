import { describe, expect, it } from "vitest";
import { besideAbout, besideFront, besideShown, besideTabs, besideWidths, belowHeights, belowWhere, closeBesideTab, floatBeside, heightenBelow, hideSidebar, openBeside, pickBeside, pickBelow, read, resting, showBelow, sidebarPlace, toggleBeside, toggleBelow, widenBeside, write } from "./chromeState";

describe("where the sidebar is", () => {
  it("is the page's to say between open and the rail", () => {
    expect(sidebarPlace(resting, false)).toBe("open");
    expect(sidebarPlace(resting, true)).toBe("rail");
  });

  it("stays hidden once you hide it, whatever the page wants", () => {
    const hidden = hideSidebar(resting, true);
    expect(sidebarPlace(hidden, false)).toBe("hidden");
    expect(sidebarPlace(hidden, true)).toBe("hidden");
    expect(sidebarPlace(hideSidebar(hidden, false), true)).toBe("rail");
  });

  it("is a drawer on a narrow window, whatever the page wants or you chose", () => {
    expect(sidebarPlace(resting, false, true)).toBe("drawer");
    expect(sidebarPlace(resting, true, true)).toBe("drawer");
    expect(sidebarPlace(hideSidebar(resting, true), true, true)).toBe("drawer");
  });
});

describe("the arrangement, kept in the browser", () => {
  it("comes back as it was written", () => {
    const s = hideSidebar(resting, true);
    expect(read(write(s))).toEqual(s);
  });

  it("rests when nothing was kept, or what was kept isn't it", () => {
    expect(read(null)).toEqual(resting);
    expect(read("")).toEqual(resting);
    expect(read("not json")).toEqual(resting);
    expect(read("[1, 2]")).toEqual(resting);
    expect(read('"open"')).toEqual(resting);
    expect(read('{"hidden":"yes"}')).toEqual(resting);
  });

  it("takes what's sound of what's beside and rests the rest", () => {
    const got = read(JSON.stringify({ hidden: "yes", beside: { open: true, width: 99999, tabs: ["a", 3, "a", "", "b", "/ask/THING-2"], about: { a: "x", gone: "y", b: 3 }, front: 7, fronts: { x: "a", y: 2 } } }));
    expect(got).toEqual({ hidden: false, beside: { open: true, over: false, width: besideWidths.max, tabs: ["a", "b"], about: { a: "x" }, front: undefined, fronts: { x: "a" } }, below: resting.below });
  });
});

describe("what's beside", () => {
  const doc = "document:records:adr/3.md";
  const ask = "/conversations/x";

  it("opens a thing in front, once however often it's opened", () => {
    const s = openBeside(openBeside(openBeside(resting, "a"), "b"), "a");
    expect(s.beside).toMatchObject({ open: true, tabs: ["a", "b"], front: "a" });
  });

  it("has what's about this page's thing first, only on that thing's page", () => {
    const s = openBeside(openBeside(resting, "anywhere"), ask, { about: doc, here: doc });
    expect(besideTabs(s, doc)).toEqual({ here: [ask], anywhere: ["anywhere"], all: [ask, "anywhere"] });
    expect(besideTabs(s, "mission:dark-mode").all).toEqual(["anywhere"]);
    expect(besideTabs(s).all).toEqual(["anywhere"]);
    expect(besideAbout(s, ask)).toBe(doc);
    expect(besideAbout(s, "anywhere")).toBeUndefined();
  });

  it("isn't drawn where it has nothing, and is back with its page", () => {
    const s = openBeside(resting, ask, { about: doc, here: doc });
    expect(besideShown(s, "mission:dark-mode")).toBe(false);
    expect(besideShown(s, doc)).toBe(true);
    expect(besideShown(toggleBeside(s), doc)).toBe(false);
  });

  it("has in front again what was in front on this thing's page", () => {
    let s = openBeside(openBeside(resting, "anywhere"), ask, { about: doc, here: doc });
    s = pickBeside(s, "anywhere", "mission:dark-mode");
    expect(besideFront(s, "mission:dark-mode")).toBe("anywhere");
    expect(besideFront(s, doc)).toBe(ask);
  });

  it("puts a conversation that's begun where the one about to begin was", () => {
    let s = openBeside(openBeside(openBeside(resting, "a"), "/ask/THING-2", { about: "thing:fixture/THING-2", here: "thing:fixture/THING-2" }), "c");
    s = openBeside(s, ask, { inPlaceOf: "/ask/THING-2", about: "thing:fixture/THING-2", here: "thing:fixture/THING-2" });
    expect(s.beside.tabs).toEqual(["a", ask, "c"]);
    expect(s.beside.about).toEqual({ [ask]: "thing:fixture/THING-2" });
    expect(s.beside.front).toBe(ask);
  });

  it("is a new tab at the end when the one to take the place of isn't there, and isn't itself", () => {
    expect(openBeside(openBeside(resting, "a"), ask, { inPlaceOf: "/ask/THING-2" }).beside.tabs).toEqual(["a", ask]);
    expect(openBeside(openBeside(resting, "a"), "a", { inPlaceOf: "a" }).beside.tabs).toEqual(["a"]);
  });

  it("brings the next forward when the one in front is closed, or the one before at the end", () => {
    const s = openBeside(openBeside(openBeside(resting, "a"), "b"), "c");
    expect(besideFront(closeBesideTab(pickBeside(s, "b"), "b"))).toBe("c");
    expect(besideFront(closeBesideTab(s, "c"))).toBe("b");
    expect(besideFront(closeBesideTab(pickBeside(s, "a"), "c"))).toBe("a");
  });

  it("closed, takes what was known of it with it, and stays open for what's beside another page", () => {
    const s = closeBesideTab(openBeside(resting, ask, { about: doc, here: doc }), ask, doc);
    expect(s.beside).toMatchObject({ open: true, tabs: [], about: {} });
    expect(Object.values(s.beside.fronts)).not.toContain(ask);
    expect(besideShown(s, doc)).toBe(false);
  });

  it("floats, and is as wide as it may be", () => {
    expect(floatBeside(resting, true).beside.over).toBe(true);
    expect(widenBeside(resting, 10).beside.width).toBe(besideWidths.min);
    expect(widenBeside(resting, 512.4).beside.width).toBe(512);
  });

  it("is kept as it was", () => {
    const s = widenBeside(floatBeside(openBeside(resting, ask, { about: doc, here: doc }), true), 500);
    expect(read(write(s))).toEqual(s);
  });
});

describe("what's below", () => {
  it("opens and closes, keeping the view that was showing", () => {
    const s = toggleBelow(pickBelow(resting, "results"));
    expect(s.below).toMatchObject({ open: true, tab: "results" });
    expect(toggleBelow(s).below).toMatchObject({ open: false, tab: "results" });
  });

  it("is opened on a view by what points at it, and closed by the same if that's the one showing", () => {
    const s = showBelow(resting, "results");
    expect(s.below).toMatchObject({ open: true, tab: "results" });
    expect(showBelow(s, "running").below).toMatchObject({ open: true, tab: "running" });
    expect(showBelow(s, "results").below).toMatchObject({ open: false, tab: "results" });
  });

  it("shows here's or everywhere's, and is as tall as it may be", () => {
    expect(belowWhere(resting, "everywhere").below.where).toBe("everywhere");
    expect(heightenBelow(resting, 10).below.height).toBe(belowHeights.min);
    expect(heightenBelow(resting, 5000).below.height).toBe(belowHeights.max);
    expect(heightenBelow(resting, 301.4).below.height).toBe(301);
  });

  it("is the same arrangement when nothing of it changes", () => {
    const s = pickBelow(resting, "results");
    expect(pickBelow(s, "results")).toBe(s);
    expect(belowWhere(s, "here")).toBe(s);
  });

  it("is kept as it was, with what isn't sound of it as it rests", () => {
    const s = belowWhere(heightenBelow(showBelow(resting, "results"), 320), "everywhere");
    expect(read(write(s))).toEqual(s);
    const got = read(JSON.stringify({ below: { open: "yes", height: "tall", tab: 3, where: "nowhere" } }));
    expect(got.below).toEqual({ open: false, height: belowHeights.rest, tab: undefined, where: "here" });
    // Kept by a ws from before there was a Below.
    expect(read(JSON.stringify({ hidden: true })).below).toEqual(resting.below);
  });

  it("leaves what's beside as it was", () => {
    const s = openBeside(resting, "a");
    expect(toggleBelow(s).beside).toBe(s.beside);
    expect(openBeside(toggleBelow(resting), "a").below.open).toBe(true);
  });
});
