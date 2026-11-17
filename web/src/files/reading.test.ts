import { describe, expect, it } from "vitest";
import { progress, sectionAt } from "./reading";

describe("sectionAt", () => {
  it("is none before the first heading reaches the line", () => {
    expect(sectionAt([200, 900, 1600])).toBe(-1);
    expect(sectionAt([])).toBe(-1);
  });
  it("is the last heading at or above the line", () => {
    expect(sectionAt([80, 900, 1600])).toBe(0);
    expect(sectionAt([-400, 40, 600])).toBe(1);
    expect(sectionAt([-1200, -600, -10])).toBe(2);
  });
});

describe("progress", () => {
  it("is how far down, as a whole percentage", () => {
    expect(progress(0, 2000, 1000)).toBe(0);
    expect(progress(333, 2000, 1000)).toBe(33);
    expect(progress(1000, 2000, 1000)).toBe(100);
  });
  it("is all of it when there's nothing to scroll", () => {
    expect(progress(0, 800, 1000)).toBe(100);
  });
  it("stays between none and all as the page bounces", () => {
    expect(progress(-20, 2000, 1000)).toBe(0);
    expect(progress(1040, 2000, 1000)).toBe(100);
  });
});
