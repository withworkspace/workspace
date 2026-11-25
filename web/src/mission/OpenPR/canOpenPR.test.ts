import { describe, expect, it } from "vitest";
import { canOpenPR } from "./canOpenPR";

describe("whether a pull request can be opened from a capsule", () => {
  const c = { exists: true, ahead: 2 };
  it("can, from commits in a repo that takes them", () => {
    expect(canOpenPR({ github: true }, c)).toBe(true);
  });
  it("can't where the repo lands straight on its branch, or isn't on GitHub", () => {
    expect(canOpenPR({ github: true }, { ...c, direct: true })).toBe(false);
    expect(canOpenPR({ github: false }, c)).toBe(false);
  });
  it("can't with nothing to open, once burned, or with one open already", () => {
    expect(canOpenPR({ github: true }, { ...c, ahead: 0 })).toBe(false);
    expect(canOpenPR({ github: true }, { exists: true })).toBe(false);
    expect(canOpenPR({ github: true }, { ...c, exists: false })).toBe(false);
    expect(canOpenPR({ github: true }, { ...c, pr: { number: 7 } as never })).toBe(false);
  });
});
