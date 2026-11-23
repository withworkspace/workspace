import { describe, expect, it } from "vitest";
import { capital, clip, humanError } from "./text";

describe("clip", () => {
  it("leaves text that fits", () => {
    expect(clip("short", 5)).toBe("short");
    expect(clip("", 3)).toBe("");
  });
  it("cuts to n with the ellipsis", () => {
    expect(clip("abcdefgh", 5)).toBe("abcd…");
    expect(clip("abcdefgh", 5)).toHaveLength(5);
  });
  it("leaves no space before the ellipsis", () => {
    expect(clip("one two three", 5)).toBe("one…");
  });
});

describe("capital", () => {
  it("capitals the first letter only", () => {
    expect(capital("needs you")).toBe("Needs you");
    expect(capital("")).toBe("");
  });
});

describe("humanError", () => {
  it("makes an error a sentence", () => {
    expect(humanError("error: mission not found")).toBe("Mission not found.");
    expect(humanError("  the daemon didn't answer ")).toBe("The daemon didn't answer.");
  });
  it("leaves one that ends already", () => {
    expect(humanError("Try again?")).toBe("Try again?");
    expect(humanError("refused (it's parked)")).toBe("Refused (it's parked)");
  });
  it("says something of nothing", () => {
    expect(humanError("")).toBe("Something went wrong.");
    expect(humanError("error: ")).toBe("Something went wrong.");
  });
});
