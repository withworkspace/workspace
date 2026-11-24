import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { describe, expect, it } from "vitest";

// A finder is drawn in a layer of its own, which a server's render doesn't
// have: what's checked here is that the finders there are, are it.
const src = join(dirname(new URL(import.meta.url).pathname), "..");
const read = (f: string) => readFileSync(join(src, f), "utf8");

describe("Finder", () => {
  it("is what the files' two finders and the review's are", () => {
    for (const f of ["files/Explorer.tsx", "files/CodeFinder.tsx", "review/Review.tsx"]) expect(read(f), f).toMatch(/<Finder\b/);
  });

  it("is the only place a text box moves a cursor down a list by hand, but for Find and the bars that suggest", () => {
    const byHand = ["files/Explorer.tsx", "files/CodeFinder.tsx", "review/Review.tsx"].filter((f) => /e\.ctrlKey && e\.key === "n"/.test(read(f)));
    expect(byHand).toEqual([]);
  });

  it("draws Find's box and its text box's row", () => {
    expect(read("shell/Palette/FindWindow.tsx")).toContain("finderBox");
    expect(read("shell/Palette/FindBar.tsx")).toContain("finderInput");
  });
});
