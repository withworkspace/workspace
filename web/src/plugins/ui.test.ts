import { V1 } from "@withworkspace/ui-lib/protocol";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { HIDE_AFTER, SHOW_AFTER } from "./hover";
import { anchorIn, type Asked, frameUis, kinds, pictureRefused, type UiKind } from "./ui";

const box = { x: 212, y: 96.5, width: 54, height: 18 };
const card = (id: number, ref = "THING-2", at = box): Asked => ({ id, ui: "ref-card", box: at, with: { ref } });

describe("the UIs ws has for a plugin's page", () => {
  it("are the ones the frame protocol names, each with how it's had", () => {
    expect(Object.keys(kinds).sort()).toEqual([...V1.UI_NAMES].sort());
  });

  it("a ref's card is asked for with a ref, and the box of its words", () => {
    const refuses = (asked: Asked) => kinds["ref-card"].refuses?.(asked, "fake");
    expect(refuses(card(1))).toBeUndefined();
    expect(refuses({ id: 1, ui: "ref-card", with: { ref: "THING-2" } })).toMatch(/box/);
    expect(refuses({ id: 1, ui: "ref-card", box, with: {} })).toMatch(/ref/);
    expect(refuses({ id: 1, ui: "ref-card", box, with: { ref: 2 } })).toMatch(/ref/);
  });

  it("a picture is asked for by an address ws shows a picture from, and one on ws's is the plugin's own", () => {
    const refused = (w: Record<string, unknown>) => pictureRefused({ id: 1, ui: "picture", with: w }, "jira", "http://ws.test");
    expect(refused({ src: "http://ws.test/plugins/jira/api/issues/RAM-3/attachments/1", title: "now.png" })).toBeUndefined();
    expect(refused({ src: "http://ws.test/plugin-frame/jira/logo.svg" })).toBeUndefined();
    expect(refused({ src: "https://example.com/a.png" })).toBeUndefined();
    expect(refused({ src: "data:image/png;base64,iVBORw0KGgo=" })).toBeUndefined();
    // Another plugin's, ws's own, or what isn't a picture's address.
    expect(refused({ src: "http://ws.test/plugins/sketch/a.png" })).toMatch(/under \/plugins\/jira\//);
    expect(refused({ src: "http://ws.test/api/workspace" })).toBeDefined();
    expect(refused({ src: "data:text/html,<script>" })).toBeDefined();
    expect(refused({ src: "/plugins/jira/a.png" })).toMatch(/full address/);
    expect(refused({})).toMatch(/src/);
    expect(refused({ src: "https://example.com/a.png", title: 3 })).toMatch(/title/);
  });

  it("a picture is open until the person closes it, and the page is told", () => {
    const closed = vi.fn();
    const uis = frameUis(closed, kinds, "jira");
    expect(uis.open({ id: 1, ui: "picture", with: { src: "http://localhost:3000/plugins/jira/a.png" } })).toMatch(/plugin's own/);
    expect(uis.open({ id: 2, ui: "picture", with: { src: "https://example.com/a.png" } })).toBeUndefined();
    expect(uis.opened().map((o) => o.ui)).toEqual(["picture"]);
    uis.done(2);
    expect(closed.mock.calls).toEqual([[2, undefined]]);
  });
});

describe("what a plugin's page has asked ws to show", () => {
  beforeEach(() => void vi.useFakeTimers());
  afterEach(() => void vi.useRealTimers());

  // ws's own, and one that's open until it's done: as a dialog that ws
  // saves from will be.
  const has: Record<string, UiKind> = {
    ...kinds,
    "thing-remote-save": { as: "open", refuses: (asked) => (typeof asked.with.thing === "string" ? undefined : "thing-remote-save is asked for with a thing.") },
  };
  const save = (id: number, thing = "sketch-12"): Asked => ({ id, ui: "thing-remote-save", box, with: { thing } });
  function frame() {
    const closed: [number, unknown][] = [];
    const uis = frameUis((id, result) => closed.push([id, result]), has);
    let changes = 0;
    uis.hear(() => changes++);
    return { uis, closed, changes: () => changes };
  }

  it("says why, for one ws hasn't, and for one that isn't given what it takes", () => {
    const { uis } = frame();
    expect(uis.open({ id: 1, ui: "date-picker", with: {} })).toBe("Workspace has no UI called date-picker. Update Workspace.");
    // A name that's something of every object's is still no UI.
    expect(uis.open({ id: 2, ui: "toString", with: {} })).toBe("Workspace has no UI called toString. Update Workspace.");
    expect(uis.open({ id: 3, ui: "thing-remote-save", with: {} })).toBe("thing-remote-save is asked for with a thing.");
    expect(uis.opened()).toEqual([]);
    expect(uis.hovered()).toBeUndefined();
  });

  it("ws's own UIs are all it has, unless it's told otherwise, and it names them", () => {
    const uis = frameUis(() => {});
    expect(uis.names).toEqual([...V1.UI_NAMES]);
    expect(frame().uis.names).toEqual([...V1.UI_NAMES, "thing-remote-save"]);
    expect(uis.open(card(1))).toBeUndefined();
    expect(uis.open({ id: 2, ui: "thing-remote-save", with: { thing: "sketch-12" } })).toMatch(/no UI called thing-remote-save/);
    uis.forget();
  });

  it("a thing is put in a repo, and saved there, from a UI that's open until ws is done with it", () => {
    const closed = vi.fn();
    const uis = frameUis(closed);
    const about = { kind: "sketch", ref: "sketch-12", title: "Release flow" };
    expect(uis.open({ id: 1, ui: "thing-promote", box: { x: 10, y: 20, width: 80, height: 24 }, with: about })).toBeUndefined();
    // No box is somewhere: ws puts it where it sees fit.
    expect(uis.open({ id: 2, ui: "thing-save", with: about })).toBeUndefined();
    expect(uis.opened().map((o) => o.ui)).toEqual(["thing-promote", "thing-save"]);
    expect(uis.hovered()).toBeUndefined();
    // A thing is its kind and the plugin's name for it: without them, why not is said.
    expect(uis.open({ id: 3, ui: "thing-save", with: { kind: "sketch" } })).toMatch(/thing-save needs the thing's kind and the plugin's ref for it/);
    expect(uis.open({ id: 4, ui: "thing-promote", with: { ref: "sketch-12", kind: " " } })).toMatch(/thing-promote needs the thing's kind/);
    // ws is done with one: the page is told what came of it, and nothing for one dismissed.
    const there = { thing: { name: "records/sketches/release-flow/", repo: "records", path: "sketches/release-flow/", title: "Release flow", version: "3f9c2a" }, changed: true };
    uis.done(1, there);
    uis.done(2);
    expect(closed.mock.calls).toEqual([
      [1, there],
      [2, undefined],
    ]);
    expect(uis.opened()).toEqual([]);
  });

  it("one had on hover comes a moment after it's asked for, and goes a moment after it's closed", () => {
    const { uis, closed } = frame();
    expect(uis.open(card(1))).toBeUndefined();
    vi.advanceTimersByTime(SHOW_AFTER - 1);
    expect(uis.hovered()).toBeUndefined();
    vi.advanceTimersByTime(1);
    expect(uis.hovered()).toEqual(card(1));

    uis.close(1);
    vi.advanceTimersByTime(HIDE_AFTER - 1);
    expect(uis.hovered()).toEqual(card(1));
    vi.advanceTimersByTime(1);
    expect(uis.hovered()).toBeUndefined();
    // The page closed it: there's nothing to tell it.
    expect(closed).toEqual([]);
  });

  it("one asked for and closed before its moment never shows", () => {
    const { uis, changes } = frame();
    uis.open(card(1));
    uis.close(1);
    vi.advanceTimersByTime(SHOW_AFTER * 2);
    expect(uis.hovered()).toBeUndefined();
    expect(changes()).toBe(0);
  });

  it("stays under the pointer, though the page has closed it", () => {
    const { uis } = frame();
    uis.open(card(1));
    vi.advanceTimersByTime(SHOW_AFTER);
    uis.stay();
    uis.close(1);
    vi.advanceTimersByTime(HIDE_AFTER * 10);
    expect(uis.hovered()).toEqual(card(1));
    uis.leave();
    vi.advanceTimersByTime(HIDE_AFTER);
    expect(uis.hovered()).toBeUndefined();
  });

  it("asked for again from the same place while it's going, it stays, and the new asking is the one to close", () => {
    const { uis } = frame();
    uis.open(card(1));
    vi.advanceTimersByTime(SHOW_AFTER);
    uis.close(1);
    uis.open(card(2));
    vi.advanceTimersByTime(SHOW_AFTER + HIDE_AFTER);
    expect(uis.hovered()).toEqual(card(1));

    // The one before is closed already: closing it again does nothing.
    uis.close(1);
    vi.advanceTimersByTime(HIDE_AFTER);
    expect(uis.hovered()).toEqual(card(1));
    uis.close(2);
    vi.advanceTimersByTime(HIDE_AFTER);
    expect(uis.hovered()).toBeUndefined();
  });

  it("goes at once when ws moves the page", () => {
    const { uis } = frame();
    uis.open(card(1));
    vi.advanceTimersByTime(SHOW_AFTER);
    uis.stay();
    uis.unhover();
    expect(uis.hovered()).toBeUndefined();
  });

  it("one that's open is there from when it's asked for, with any others, in the order they were asked for", () => {
    const { uis, changes } = frame();
    expect(uis.open(save(1))).toBeUndefined();
    expect(uis.opened()).toEqual([save(1)]);
    uis.open(save(2, "sketch-13"));
    expect(uis.opened().map((o) => o.id)).toEqual([1, 2]);
    expect(changes()).toBe(2);
    // What's read between changes is the same list: nothing's drawn again for it.
    expect(uis.opened()).toBe(uis.opened());
  });

  it("the page is told what came of one ws is done with, once, and it's gone", () => {
    const { uis, closed } = frame();
    uis.open(save(1));
    uis.open(save(2));

    uis.done(1, { saved: "records:sketches/release-flow.excalidraw" });
    uis.done(1, "twice");
    uis.done(2);

    expect(closed).toEqual([
      [1, { saved: "records:sketches/release-flow.excalidraw" }],
      [2, undefined],
    ]);
    expect(uis.opened()).toEqual([]);
  });

  it("the page isn't told of one it closed itself, nor of one ws finishes after that", () => {
    const { uis, closed } = frame();
    uis.open(save(1));
    uis.close(1);
    uis.done(1, "late");
    expect(uis.opened()).toEqual([]);
    expect(closed).toEqual([]);
  });

  it("forgets all of it when the page goes, and tells nobody", () => {
    const { uis, closed } = frame();
    uis.open(card(1));
    vi.advanceTimersByTime(SHOW_AFTER);
    uis.open(save(2));
    uis.open(card(3, "THING-1", { ...box, y: 300 }));

    uis.forget();

    expect(uis.hovered()).toBeUndefined();
    expect(uis.opened()).toEqual([]);
    vi.advanceTimersByTime(SHOW_AFTER * 2);
    expect(uis.hovered()).toBeUndefined();
    expect(closed).toEqual([]);
  });
});

describe("where a box in the frame's page is in ws's window", () => {
  const frame = (left: number, top: number, border = 0) => ({ isConnected: true, clientLeft: border, clientTop: border, getBoundingClientRect: () => ({ left, top }) as DOMRect });

  it("is the frame's corner, and the box from it", () => {
    const at = anchorIn(() => frame(260, 48), box);
    expect(at.getBoundingClientRect()).toEqual({ left: 472, top: 144.5, right: 526, bottom: 162.5 });
    expect(at.isConnected).toBe(true);
  });

  it("is read when it's asked, so it follows the frame", () => {
    let left = 260;
    const at = anchorIn(() => frame(left, 48), { x: 10, y: 10, width: 20, height: 10 });
    left = 56; // the sidebar folded
    expect(at.getBoundingClientRect().left).toBe(66);
  });

  it("starts inside the frame's border", () => {
    expect(anchorIn(() => frame(100, 100, 2), { x: 0, y: 0, width: 1, height: 1 }).getBoundingClientRect()).toMatchObject({ left: 102, top: 102 });
  });

  it("is gone when the frame is", () => {
    expect(anchorIn(() => null, { x: 0, y: 0, width: 1, height: 1 }).isConnected).toBe(false);
  });
});
