import { describe, expect, it } from "vitest";
import { foldsOf, indentFolds, unfolded } from "./folds";

const go = ["package main", "", "import (", '\t"fmt"', '\t"os"', ")", "", "func main() {", "\tif len(os.Args) > 1 {", "\t\tfmt.Println(os.Args[1])", "\t}", "", "}"];

describe("folds by indentation", () => {
  it("are a line and those below it further in, less the blank ones at the end", () => {
    expect(indentFolds(go)).toEqual([
      { at: 3, to: 5 },
      { at: 8, to: 11 },
      { at: 9, to: 10 },
    ]);
  });

  it("end with the file", () => {
    expect(indentFolds(["def f():", "    if x:", "        return 1", ""])).toEqual([
      { at: 1, to: 3 },
      { at: 2, to: 3 },
    ]);
  });

  it("are none in a file that's flat", () => {
    expect(indentFolds(["a", "b", "", "c"])).toEqual([]);
  });
});

describe("a file's folds", () => {
  it("are the server's where it found one, its closing bracket's line left out, and indentation's elsewhere", () => {
    // The server: main, to its closing brace. Not the imports, nor the if.
    expect(foldsOf(go, [[8, 13]])).toEqual([
      { at: 3, to: 5 },
      { at: 8, to: 12 },
      { at: 9, to: 10 },
    ]);
  });

  it("keep a last line that closes nothing: Python's", () => {
    const py = ["def f(", "    a,", "):", "    return a", "", "x = 1"];
    expect(foldsOf(py, [[1, 4]])).toEqual([
      { at: 1, to: 4 },
      { at: 3, to: 4 },
    ]);
  });

  it("leave out what the server found in a longer reading of the file, and what hides nothing", () => {
    expect(foldsOf(["a {", "}"], [[1, 2], [1, 40]])).toEqual([]);
  });
});

describe("going to a line", () => {
  const folds = foldsOf(go, [[8, 13]]);
  it("unfolds what hides it, and nothing else", () => {
    const folded = new Set([3, 8, 9]);
    expect([...unfolded(folds, folded, [10])]).toEqual([3]);
    expect([...unfolded(folds, folded, [4])]).toEqual([8, 9]);
  });

  it("leaves a fold it's the first line of, and what's folded when nothing hides it", () => {
    const folded = new Set([8]);
    expect(unfolded(folds, folded, [8])).toBe(folded);
    expect(unfolded(folds, folded, [13, 2])).toBe(folded);
  });
});
