import { describe, expect, it } from "vitest";
import type { TranscriptEntry } from "../../api";
import { noticeWords, screenContext, shortPath, wantsTo, workingOn } from "./words";

const call = (tool: string, more: Partial<TranscriptEntry> = {}): TranscriptEntry => ({ by: "call", tool, input: "", ...more });
const edit = (old: string) => call("Edit", { edit: { path: "/c/greet/greet.go", old, new: "x" } });

describe("wantsTo", () => {
  it("says the step crew want leave for", () => {
    expect(wantsTo(edit("y"), undefined, undefined)).toBe("edit");
    expect(wantsTo(edit(""), undefined, undefined)).toBe("write");
    expect(wantsTo(call("Bash"), undefined, undefined)).toBe("run");
    expect(wantsTo(call("WebFetch"), undefined, undefined)).toBe("use WebFetch");
  });

  it("reads it off crew's screen when the transcript hasn't the step", () => {
    const screen = { title: "Bash command: Do you want to make this edit to greet_test.go?", context: ["greet_test.go"], options: [] };
    expect(wantsTo(undefined, screen, screen.context)).toBe("make this edit to greet_test.go");
    expect(wantsTo(undefined, screen, [])).toBe("continue");
    expect(wantsTo(undefined, undefined, undefined)).toBe("continue");
  });
});

describe("workingOn", () => {
  it("says what crew are at, and on what", () => {
    expect(workingOn(call("Bash", { input: "go test ./..." }))).toEqual({ verb: "Running", what: "go test ./..." });
    expect(workingOn(edit("y"))).toEqual({ verb: "Working on", what: "greet.go" });
  });
});

describe("the rest", () => {
  it("says a notice, a screen's words and a path as the page does", () => {
    expect(noticeWords("Request interrupted by user for tool use")).toBe("Interrupted");
    expect(noticeWords("Caveat: x")).toBe("Caveat: x");
    expect(screenContext({ title: "", context: ["a", "Tip: press shift+tab", "b"], options: [] })).toEqual(["a", "b"]);
    expect(shortPath("/c/repos/demo/greet/greet.go")).toBe("greet/greet.go");
  });
});
