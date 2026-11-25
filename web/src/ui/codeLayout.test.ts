import { describe, expect, it } from "vitest";
import { marksOf, opened, rangeFrom } from "./codeLayout";

describe("marksOf", () => {
  it("marks each line of a run that's added or modified", () => {
    const m = marksOf([
      { kind: "added", start: 3, end: 5 },
      { kind: "modified", start: 9 },
    ]);
    expect([...m]).toEqual([
      [3, "added"],
      [4, "added"],
      [5, "added"],
      [9, "modified"],
    ]);
  });

  it("marks the line lines were removed after", () => {
    expect(marksOf([{ kind: "deleted", start: 7 }]).get(7)).toBe("removedAfter");
  });

  it("says a line's own change over what was removed after it", () => {
    expect(marksOf([{ kind: "modified", start: 7 }, { kind: "deleted", start: 7 }]).get(7)).toBe("modified");
    expect(marksOf([{ kind: "deleted", start: 7 }, { kind: "added", start: 7 }]).get(7)).toBe("added");
  });
});

describe("opened", () => {
  const folds = [
    { at: 2, to: 20 },
    { at: 5, to: 9 },
    { at: 30, to: 40 },
  ];

  it("opens every fold that hides a line gone to", () => {
    expect([...opened(folds, new Set([2, 5, 30]), [7])]).toEqual([30]);
  });

  it("leaves a fold shut when the line is the one it starts at, or outside it", () => {
    const folded = new Set([2, 30]);
    expect(opened(folds, folded, [2])).toBe(folded);
    expect(opened(folds, folded, [25])).toBe(folded);
  });
});

describe("rangeFrom", () => {
  it("is the run between where a pick began and where it is, either way", () => {
    expect(rangeFrom(4, 9)).toEqual({ from: 4, to: 9 });
    expect(rangeFrom(9, 4)).toEqual({ from: 4, to: 9 });
    expect(rangeFrom(4, 4)).toEqual({ from: 4, to: 4 });
  });
});
