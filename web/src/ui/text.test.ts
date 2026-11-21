import { describe, expect, it } from "vitest";
import { capital, clip } from "./text";

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
