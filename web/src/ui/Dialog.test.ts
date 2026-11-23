import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { describe, expect, it } from "vitest";

// A dialog is drawn in a layer of its own, which a server's render
// doesn't have: what's checked here is what's said about it elsewhere.
const src = dirname(new URL(import.meta.url).pathname);
const read = (f: string) => readFileSync(join(src, f), "utf8");

describe("Dialog", () => {
  // react-aria makes everything outside an open dialog inert. What's
  // drawn over it from elsewhere on the page has to say it's to be left.
  it("leaves hover cards and toasts be", () => {
    expect(read("FloatingCard.tsx")).toContain('data-react-aria-top-layer=""');
    expect(read("Toasts.tsx")).toContain('data-react-aria-top-layer=""');
  });

  it("is what says something's open, for the app's keys", () => {
    expect(read("Modal.tsx")).toContain('data-overlay=""');
    expect(read("../App.tsx")).toContain('document.querySelector("[data-overlay]")');
  });
});
