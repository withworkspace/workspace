import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { anchorIn, frameCard, HIDE_AFTER, type Over, SHOW_AFTER } from "./hover";

const thing: Over = { ref: '{"kind":"thing","id":"THING-2"}', box: { x: 212, y: 96, width: 54, height: 18 } };
const mission: Over = { ref: '{"kind":"mission","slug":"shelve-the-lantern"}', box: { x: 300, y: 96, width: 120, height: 18 } };

describe("the card of a ref in a plugin's page", () => {
  beforeEach(() => void vi.useFakeTimers());
  afterEach(() => void vi.useRealTimers());

  // A card, and each thing it's shown since.
  function card() {
    const c = frameCard();
    const seen: (string | undefined)[] = [];
    c.hear(() => seen.push(c.shown()?.ref));
    return { c, seen };
  }

  it("shows a moment after the pointer comes, and not for a pointer passing over", () => {
    const { c, seen } = card();
    c.over(thing);
    vi.advanceTimersByTime(SHOW_AFTER - 1);
    expect(c.shown()).toBeUndefined();
    vi.advanceTimersByTime(1);
    expect(c.shown()).toBe(thing);

    c.clear();
    c.over(mission);
    vi.advanceTimersByTime(SHOW_AFTER - 1);
    c.over(undefined);
    vi.advanceTimersByTime(SHOW_AFTER * 2);
    expect(seen).toEqual([thing.ref, undefined]);
  });

  it("goes a moment after the pointer leaves the ref", () => {
    const { c } = card();
    c.over(thing);
    vi.advanceTimersByTime(SHOW_AFTER);
    c.over(undefined);
    vi.advanceTimersByTime(HIDE_AFTER - 1);
    expect(c.shown()).toBe(thing);
    vi.advanceTimersByTime(1);
    expect(c.shown()).toBeUndefined();
  });

  it("stays while the pointer's on the card, and goes a moment after it leaves the card", () => {
    const { c } = card();
    c.over(thing);
    vi.advanceTimersByTime(SHOW_AFTER);
    c.over(undefined);
    vi.advanceTimersByTime(HIDE_AFTER - 1);
    c.stay();
    vi.advanceTimersByTime(HIDE_AFTER * 10);
    expect(c.shown()).toBe(thing);

    c.leave();
    vi.advanceTimersByTime(HIDE_AFTER);
    expect(c.shown()).toBeUndefined();
  });

  it("stays when the page says the pointer left the ref only after it reached the card", () => {
    // The page's word comes a turn after the pointer moved: ws has seen
    // the pointer on the card by then.
    const { c } = card();
    c.over(thing);
    vi.advanceTimersByTime(SHOW_AFTER);
    c.stay();
    c.over(undefined);
    vi.advanceTimersByTime(HIDE_AFTER * 10);
    expect(c.shown()).toBe(thing);
  });

  it("stays when the pointer comes back to its ref, from the card or from beside it", () => {
    const { c, seen } = card();
    c.over(thing);
    vi.advanceTimersByTime(SHOW_AFTER);
    c.over(undefined);
    vi.advanceTimersByTime(HIDE_AFTER - 1);
    // The page measured the words again: where they are is what's the same.
    c.over({ ref: thing.ref, box: { ...thing.box } });
    vi.advanceTimersByTime(SHOW_AFTER + HIDE_AFTER);
    expect(c.shown()).toBe(thing);

    c.stay();
    c.leave();
    c.over(thing);
    vi.advanceTimersByTime(SHOW_AFTER + HIDE_AFTER);
    expect(seen).toEqual([thing.ref]);
  });

  it("from one ref to another: the first's goes as it would have, and the second's comes as it would have", () => {
    const { c, seen } = card();
    c.over(thing);
    vi.advanceTimersByTime(SHOW_AFTER);
    c.over(undefined);
    c.over(mission);
    vi.advanceTimersByTime(HIDE_AFTER);
    expect(c.shown()).toBeUndefined();
    vi.advanceTimersByTime(SHOW_AFTER - HIDE_AFTER);
    expect(c.shown()).toBe(mission);
    expect(seen).toEqual([thing.ref, undefined, mission.ref]);
  });

  it("the same ref written somewhere else is another place for the card", () => {
    const { c } = card();
    const again: Over = { ref: thing.ref, box: { ...thing.box, y: 400 } };
    c.over(thing);
    vi.advanceTimersByTime(SHOW_AFTER);
    c.over(again);
    vi.advanceTimersByTime(SHOW_AFTER);
    expect(c.shown()).toBe(again);
  });

  it("clears at once, and what was on its way doesn't come", () => {
    const { c, seen } = card();
    c.over(thing);
    vi.advanceTimersByTime(SHOW_AFTER);
    c.over(mission);
    c.clear();
    expect(c.shown()).toBeUndefined();
    vi.advanceTimersByTime(SHOW_AFTER * 2);
    expect(seen).toEqual([thing.ref, undefined]);
  });
});

describe("where a box in the frame's page is in ws's window", () => {
  const frame = (left: number, top: number, border = 0) => ({ isConnected: true, clientLeft: border, clientTop: border, getBoundingClientRect: () => ({ left, top }) as DOMRect });

  it("is the frame's corner, and the box from it", () => {
    const at = anchorIn(() => frame(260, 48), { x: 212, y: 96.5, width: 54, height: 18 });
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
