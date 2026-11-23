import { describe, expect, it } from "vitest";
import { keepSame } from "./keepSame";

describe("keepSame", () => {
  it("is what it was when nothing's changed", () => {
    const was = { name: "sorrel", missions: [{ slug: "a", crew: [{ id: "c1" }] }], seen: 3 };
    const now = JSON.parse(JSON.stringify(was));
    expect(keepSame(was, now)).toBe(was);
  });

  it("keeps the parts that haven't changed, and only those", () => {
    const was = { missions: [{ slug: "a", crew: [{ id: "c1", activity: "working" }] }, { slug: "b", crew: [] }], needs: [{ to: "/x" }] };
    const now = JSON.parse(JSON.stringify(was));
    now.missions[0].crew[0].activity = "idle";
    const kept = keepSame(was, now);
    expect(kept).not.toBe(was);
    expect(kept).toEqual(now);
    expect(kept.missions).not.toBe(was.missions);
    expect(kept.missions[0]).not.toBe(was.missions[0]);
    expect(kept.missions[1]).toBe(was.missions[1]);
    expect(kept.needs).toBe(was.needs);
  });

  it("is new where something's added, taken away or left out", () => {
    expect(keepSame({ a: 1 }, { a: 1, b: 2 })).toEqual({ a: 1, b: 2 });
    expect(keepSame({ a: 1, b: 2 }, { a: 1 })).toEqual({ a: 1 });
    expect(keepSame([1, 2], [1, 2, 3])).toEqual([1, 2, 3]);
    expect(keepSame([1, 2, 3], [1, 2])).toEqual([1, 2]);
    // A key that's there with nothing isn't one that's not there.
    const was = { a: undefined };
    expect(keepSame(was, {})).not.toBe(was);
  });

  it("doesn't take a list for an object, or nothing for something", () => {
    expect(keepSame({ 0: "a" }, ["a"])).toEqual(["a"]);
    expect(keepSame(undefined, { a: 1 })).toEqual({ a: 1 });
    expect(keepSame({ a: 1 }, null)).toBe(null);
    const list = [{ a: 1 }];
    expect(keepSame(list, [{ a: 1 }])).toBe(list);
  });
});
