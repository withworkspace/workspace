import { describe, expect, it } from "vitest";
import { linesAsked, rendersAt } from "./lines";

describe("lines asked for in an address", () => {
  it("are a line or a run of them", () => {
    expect(linesAsked("#L1480")).toEqual({ from: 1480, to: 1480 });
    expect(linesAsked("#L12-L20")).toEqual({ from: 12, to: 20 });
    expect(linesAsked("")).toBeUndefined();
    expect(linesAsked("#problem")).toBeUndefined();
    expect(linesAsked("#L")).toBeUndefined();
  });

  it("open a markdown file as text, at the line: a rendered one has no lines to go to", () => {
    expect(rendersAt("#L1480")).toBe(false);
    expect(rendersAt("#L5-L9")).toBe(false);
    // A heading's anchor, or nothing: rendered, as before.
    expect(rendersAt("#problem")).toBe(true);
    expect(rendersAt("")).toBe(true);
  });
});
