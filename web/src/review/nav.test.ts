import { expect, test } from "vitest";
import type { DiffFile } from "../api";
import { fileRows } from "./model";
import { answerWords, gapOf, groupUses, nameAt, rowOf, type NavNames } from "./nav";

const names: NavNames = { lang: "go", lines: { "3": [[4, 5, "d"], [8, 13, "r"]] } };

test("a name is found by its line and a column inside it", () => {
  expect(nameAt(names, 3, 4)).toEqual([4, 5, "d"]);
  expect(nameAt(names, 3, 5)).toBeUndefined(); // just after it
  expect(nameAt(names, 3, 12)).toEqual([8, 13, "r"]);
  expect(nameAt(names, 4, 0)).toBeUndefined();
  expect(nameAt(undefined, 3, 4)).toBeUndefined();
});

// A file changed at lines 10 and 30, of 40.
const file: DiffFile = {
  path: "a.go",
  status: "modified",
  additions: 2,
  deletions: 1,
  hunks: [
    { oldStart: 9, oldLines: 3, newStart: 9, newLines: 3, lines: [
      { k: "c", o: 9, n: 9, t: "a" }, { k: "d", o: 10, t: "old" }, { k: "a", n: 10, t: "new" }, { k: "c", o: 11, n: 11, t: "b" },
    ] },
    { oldStart: 29, oldLines: 2, newStart: 29, newLines: 3, lines: [
      { k: "c", o: 29, n: 29, t: "c" }, { k: "a", n: 30, t: "more" }, { k: "c", o: 30, n: 31, t: "d" },
    ] },
  ],
};

test("a line shown in the diff is found on its side", () => {
  const rows = fileRows(0, file, { viewed: false, expanded: {} }, false);
  expect(rowOf(rows, "old", 10)).toBeDefined();
  expect(rowOf(rows, "new", 10)).toBeDefined();
  expect(rowOf(rows, "old", 10)).not.toBe(rowOf(rows, "new", 10));
  expect(rowOf(rows, "new", 20)).toBeUndefined();
});

test("a line folded away is in the gap around it, on either side", () => {
  expect(gapOf(file, undefined, "new", 3)).toEqual({ gap: 0, size: 8 });
  expect(gapOf(file, undefined, "new", 20)).toEqual({ gap: 1, size: 17 });
  // Old line 25 is new line 25 (nothing moved before the second hunk).
  expect(gapOf(file, undefined, "old", 25)).toEqual({ gap: 1, size: 17 });
  // After the last hunk: unknown until the file's read.
  expect(gapOf(file, undefined, "new", 35)).toEqual({ gap: 2, size: undefined });
  expect(gapOf(file, undefined, "new", 10)).toBeUndefined();
});

test("an answer says how it was found in a few words", () => {
  expect(answerWords({ name: "Fetch", answer: "candidates", targets: [{ path: "a", line: 1, col: 0, endLine: 1, kind: "func" }] })).toBe("1 named Fetch");
  expect(answerWords({ name: "x", answer: "resolved", how: "local", targets: [] })).toBe("local");
  expect(answerWords({ name: "Println", answer: "external", external: { module: "fmt" }, targets: [] })).toBe("External: fmt");
});

test("uses in the change come first, those on a changed line before the rest", () => {
  const uses = [
    { path: "z/other.go", line: 3, col: 0, text: "" },
    { path: "a.go", line: 40, col: 0, text: "" },
    { path: "a.go", line: 10, col: 0, text: "" },
    { path: "b.go", line: 5, col: 0, text: "", maybe: true },
    { path: "c/c.go", line: 1, col: 0, text: "" },
  ];
  const g = groupUses(uses, (p) => p === "a.go" || p === "b.go", (p, l) => p === "a.go" && l === 40);
  expect(g.map((x) => [x.title, x.uses.map((u) => `${u.path}:${u.line}`), x.maybes.length])).toEqual([
    ["In this change", ["a.go:40", "a.go:10"], 1],
    ["Elsewhere", ["c/c.go:1", "z/other.go:3"], 0],
  ]);
});
