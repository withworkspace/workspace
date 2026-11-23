import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { describe, expect, it } from "vitest";

// A modal is drawn in a layer of its own, which a server's render doesn't
// have: what's checked here is what the rest of the app counts on it for.
const src = join(dirname(new URL(import.meta.url).pathname), "..");
const read = (f: string) => readFileSync(join(src, f), "utf8");

describe("Modal", () => {
  it("is the only thing that dims the page: nothing draws an overlay by hand", () => {
    const byHand = ["App.tsx", "shell/Palette/Palette.tsx", "shell/Palette/FindWindow.tsx", "files/CodeFinder.tsx", "files/Explorer.tsx", "review/Review.tsx", "ui/Dialog.tsx"].filter((f) => /className="overlay|aria-modal="true"[^>]*className="(palette|dialog)/.test(read(f)));
    expect(byHand).toEqual([]);
  });

  it("leaves Escape to Find, which steps back before it closes", () => {
    expect(read("shell/Palette/FindWindow.tsx")).toMatch(/<Modal [^>]*escape=\{false\}/);
  });

  it("doesn't count a press on a hover card or a toast as outside it", () => {
    expect(read("ui/Modal.tsx")).toContain('closest("[data-react-aria-top-layer]")');
  });
});
