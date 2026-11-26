import { describe, expect, it } from "vitest";
import { countsAt, countWords, failuresOn } from "./flowWords";

describe("an automation's outline", () => {
  it("adds the builtins called at one place together, and says how they went", () => {
    const counts = countsAt([
      { at: "9:14", builtin: "notify", done: 2 },
      { at: "9:14", builtin: "notify", started: 1 },
      { at: "12:20", builtin: "wait", met: 3, timedOut: 1, waiting: 1 },
    ]);
    expect(countWords("notify", counts.get("9:14"))).toEqual(["2 sent", "1 unsure"]);
    expect(countWords("notify", counts.get("9:14"), true)).toEqual(["would notify 2×", "1 unsure"]);
    expect(countWords("wait", counts.get("12:20"))).toEqual(["3 met", "1 timed out", "1 waiting"]);
    expect(countWords("read", undefined)).toEqual([]);
  });

  it("puts a failure on the step where it was, or on the step on its line", () => {
    const steps = [
      { kind: "read", at: "9:14", line: 9 },
      { kind: "if", at: "10:5", line: 10, branches: [{ label: "else", steps: [{ kind: "code", at: "11:9", line: 11 }] }] },
    ];
    const on = failuresOn(steps, { "9:14": 2, "11:20": 1, "40:1": 5 });
    expect(on.get("9:14")).toBe(2);
    expect(on.get("11:9")).toBe(1);
    expect(on.size).toBe(2);
  });
});
