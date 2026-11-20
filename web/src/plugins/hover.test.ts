import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { HIDE_AFTER, hovering, SHOW_AFTER } from "./hover";

// Something shown on hover, about a ref at a place.
interface Card {
  ref: string;
  y: number;
}
const thing: Card = { ref: "THING-2", y: 96 };
const mission: Card = { ref: "shelve-the-lantern", y: 96 };

describe("what's shown on hover over a plugin's frame", () => {
  beforeEach(() => void vi.useFakeTimers());
  afterEach(() => void vi.useRealTimers());

  // One frame's, and each thing it's shown since.
  function card() {
    const c = hovering<Card>((a, b) => a.ref === b.ref && a.y === b.y);
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

  it("goes a moment after the pointer leaves", () => {
    const { c } = card();
    c.over(thing);
    vi.advanceTimersByTime(SHOW_AFTER);
    c.over(undefined);
    vi.advanceTimersByTime(HIDE_AFTER - 1);
    expect(c.shown()).toBe(thing);
    vi.advanceTimersByTime(1);
    expect(c.shown()).toBeUndefined();
  });

  it("stays while the pointer's on it, and goes a moment after the pointer leaves it", () => {
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

  it("stays when the page says the pointer left only after it reached what's shown", () => {
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

  it("stays when the pointer comes back to what it's about, from it or from beside it", () => {
    const { c, seen } = card();
    c.over(thing);
    vi.advanceTimersByTime(SHOW_AFTER);
    c.over(undefined);
    vi.advanceTimersByTime(HIDE_AFTER - 1);
    // Asked for again: the same thing in the same place is what's the same.
    c.over({ ...thing });
    vi.advanceTimersByTime(SHOW_AFTER + HIDE_AFTER);
    expect(c.shown()).toBe(thing);

    c.stay();
    c.leave();
    c.over(thing);
    vi.advanceTimersByTime(SHOW_AFTER + HIDE_AFTER);
    expect(seen).toEqual([thing.ref]);
  });

  it("from one to another: the first's goes as it would have, and the second's comes as it would have", () => {
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

  it("the same thing somewhere else is another place for it", () => {
    const { c } = card();
    const again: Card = { ...thing, y: 400 };
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
