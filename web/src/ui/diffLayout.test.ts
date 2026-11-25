import { describe, expect, it } from "vitest";
import { type DiffRow, digitsOf, pickedRows, pickFrom, unifiedKind } from "./diffLayout";

const cell = (num: number, kind: "c" | "a" | "d" = "c") => ({ num, text: `line ${num}`, kind });

const rows: DiffRow[] = [
  { type: "gap", key: "g0", count: 8 },
  { type: "line", key: "n9", left: cell(9), right: cell(9) },
  { type: "line", key: "o10", left: cell(10, "d") },
  { type: "line", key: "n10", right: cell(10, "a") },
  { type: "line", key: "n11", left: cell(11), right: cell(1204) },
  { type: "note", key: "note", text: "Binary file." },
];

describe("pickFrom", () => {
  it("is the rows between where a pick began and where it is, top first, either way", () => {
    expect(pickFrom(rows, "n9", "n10")).toEqual({ from: "n9", to: "n10", side: undefined });
    expect(pickFrom(rows, "n10", "n9", "new")).toEqual({ from: "n9", to: "n10", side: "new" });
    expect(pickFrom(rows, "o10", "o10")).toEqual({ from: "o10", to: "o10", side: undefined });
  });

  it("is nothing when a row isn't there (the diff's been laid out again)", () => {
    expect(pickFrom(rows, "n9", "gone")).toBeUndefined();
  });
});

describe("pickedRows", () => {
  it("says where the picked rows are", () => {
    expect(pickedRows(rows, { from: "o10", to: "n11" })).toEqual({ from: 2, to: 4 });
  });

  it("is nothing with no pick, or one whose rows aren't there", () => {
    expect(pickedRows(rows, undefined)).toBeUndefined();
    expect(pickedRows(rows, { from: "gone", to: "n11" })).toBeUndefined();
    expect(pickedRows(rows, { from: "n9", to: "gone" })).toBeUndefined();
  });
});

describe("unifiedKind", () => {
  it("is removed with only an old side, added with only a new, unchanged with both", () => {
    expect(unifiedKind({ type: "line", key: "a", left: cell(1, "d") })).toBe("d");
    expect(unifiedKind({ type: "line", key: "b", right: cell(1, "a") })).toBe("a");
    expect(unifiedKind({ type: "line", key: "c", left: cell(1), right: cell(1) })).toBe("c");
  });
});

describe("digitsOf", () => {
  it("is the widest line number's digits, two at least", () => {
    expect(digitsOf(rows)).toBe(4);
    expect(digitsOf(rows.slice(0, 3))).toBe(2);
    expect(digitsOf([])).toBe(2);
  });
});
