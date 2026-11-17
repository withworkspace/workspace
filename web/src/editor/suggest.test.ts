import { describe, expect, it } from "vitest";
import { suggestionsFor } from "./suggest";

// An edit becomes suggestions: one per passage it changes, whole lines,
// each in the document once, with the words it's on.
describe("an edit as suggestions", () => {
  const base = "# Plan\n\nShip on Friday.\n\n- one\n- two\n\nNotes:\n\n- one\n";

  it("is a suggestion per passage changed", () => {
    const s = suggestionsFor(base, base.replace("Friday", "Tuesday").replace("- two\n", "- two\n- three\n"));
    expect(s).toEqual([
      { from: "Ship on Friday.\n", to: "Ship on Tuesday.\n", line: 3, endLine: 3, quote: "Ship on Friday." },
      { from: "- two\n", to: "- two\n- three\n", line: 6, endLine: 6, quote: "two" },
    ]);
  });

  it("widens a passage that's in the document more than once", () => {
    // "- one" is twice, with a blank line above it each time: the first
    // is said with the lines either side of it.
    const s = suggestionsFor(base, base.replace("- one\n- two", "- uno\n- two"));
    expect(s).toHaveLength(1);
    expect(s[0].from).toBe("\n- one\n- two\n");
    expect(s[0].to).toBe("\n- uno\n- two\n");
    expect(base.split(s[0].from).length).toBe(2);
  });

  it("takes out lines, and puts some in at the top", () => {
    expect(suggestionsFor(base, base.replace("Notes:\n\n- one\n", ""))).toEqual([{ from: "Notes:\n\n- one\n", to: "", line: 8, endLine: 10, quote: "Notes:\none" }]);
    const top = suggestionsFor(base, "Draft.\n\n" + base);
    expect(top).toEqual([{ from: "# Plan\n", to: "Draft.\n\n# Plan\n", line: 1, endLine: 1, quote: "Plan" }]);
  });

  it("is nothing for no change", () => {
    expect(suggestionsFor(base, base)).toEqual([]);
  });
});
