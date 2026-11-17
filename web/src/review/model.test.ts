import { describe, expect, it } from "vitest";
import type { DiffFile, DiffLine } from "../api";
import { cellHTML, fileRows, type Highlighted } from "./model";

// A file whose first hunk adds 30 lines, so in the second an unchanged
// line is old line 40 and new line 70: and new line 40 is in the first
// hunk, another line altogether.
function file(): DiffFile {
  const first: DiffLine[] = [{ k: "c", o: 1, n: 1, t: "<!doctype html>" }];
  for (let i = 0; i < 30; i++) first.push({ k: "a", n: i + 2, t: `<p>added ${i + 1}</p>` });
  for (let i = 0; i < 10; i++) first.push({ k: "c", o: i + 2, n: i + 32, t: `<p>kept ${i + 1}</p>` });
  const second: DiffLine[] = [
    { k: "c", o: 40, n: 70, t: "<main>" },
    { k: "d", o: 41, t: "<h1>Old</h1>" },
    { k: "a", n: 71, t: "<h1>New</h1>" },
    { k: "c", o: 42, n: 72, t: "</main>" },
  ];
  return {
    path: "a1.v2.html",
    status: "modified",
    additions: 31,
    deletions: 1,
    hunks: [
      { oldStart: 1, oldLines: 11, newStart: 1, newLines: 41, lines: first },
      { oldStart: 40, oldLines: 3, newStart: 70, newLines: 3, lines: second },
    ],
  };
}

// highlighted colours each side's lines by their numbers there, as the
// review's highlight does (marked, to tell them from escaped text).
function highlighted(f: DiffFile): Highlighted {
  const hl: Highlighted = { old: new Map(), new: new Map(), shown: [] };
  for (const h of f.hunks)
    for (const l of h.lines) {
      if (l.k !== "a") hl.old.set(l.o!, `<b>${l.t}</b>`);
      if (l.k !== "d") hl.new.set(l.n!, `<b>${l.t}</b>`);
    }
  return hl;
}

describe("cellHTML", () => {
  const f = file();
  const hl = highlighted(f);

  for (const split of [true, false]) {
    it(`colours each line as itself, ${split ? "side by side" : "unified"}`, () => {
      const rows = fileRows(0, f, { viewed: false, expanded: {} }, split);
      let lines = 0;
      for (const r of rows) {
        if (r.type !== "line") continue;
        if (split) {
          if (r.left) expect(cellHTML(hl, r.left, "old")).toBe(`<b>${r.left.text}</b>`);
          if (r.right) expect(cellHTML(hl, r.right, "new")).toBe(`<b>${r.right.text}</b>`);
        } else {
          const c = r.right ?? r.left!;
          expect(cellHTML(hl, c, r.right ? "new" : "old")).toBe(`<b>${c.text}</b>`);
        }
        lines++;
      }
      expect(lines).toBeGreaterThan(40);
    });
  }

  it("reads a line opened from a gap in the file at head, and escapes what isn't coloured", () => {
    const shown = { ...hl, shown: ["<i>one</i>", "<i>two</i>"] };
    expect(cellHTML(shown, { num: 5, at: 2, text: "two", kind: "c", shown: true }, "old")).toBe("<i>two</i>");
    expect(cellHTML({ old: new Map(), new: new Map(), shown: [] }, { num: 3, text: "<p>", kind: "a" }, "new")).toBe("&lt;p&gt;");
  });
});
