import { describe, expect, it } from "vitest";
import { type Observe, watchBackdrop } from "./backdrop";

// A page with some backdrops in it, and a watch a test fires by hand: the
// browser's own (a MutationObserver on the frame's document) is tried in
// the fixture (testdata/plugins/frame-fixture: "A dialog").
function page(backdrops = 0) {
  const asked: string[] = [];
  let changed = () => {};
  let stopped = false;
  const observe: Observe = (_root, fn) => {
    changed = fn;
    return () => void (stopped = true);
  };
  const doc = {
    documentElement: {} as Node,
    querySelector(selectors: string) {
      asked.push(selectors);
      return backdrops > 0 ? {} : null;
    },
  };
  const said: boolean[] = [];
  const stop = watchBackdrop(doc, (on) => void said.push(on), observe);
  // The app changed its page, to have this many backdrops.
  const now = (n: number) => {
    backdrops = n;
    changed();
  };
  return { said, now, stop, asked, stopped: () => stopped };
}

describe("watching a page for a backdrop", () => {
  it("says so as the first comes, and as the last goes, and nothing in between", () => {
    const { said, now } = page();
    expect(said).toEqual([]);
    now(0); // something else in the page changed
    now(1);
    expect(said).toEqual([true]);
    now(2); // another over it
    now(1);
    expect(said).toEqual([true]);
    now(0);
    expect(said).toEqual([true, false]);
  });

  it("says at once of one that's there as it starts", () => {
    expect(page(1).said).toEqual([true]);
  });

  it("looks for the class ws's shell styles", () => {
    expect(page().asked).toEqual([".ws-backdrop"]);
  });

  it("stops the watch when it's stopped", () => {
    const { stop, stopped } = page(1);
    expect(stopped()).toBe(false);
    stop();
    expect(stopped()).toBe(true);
  });
});
