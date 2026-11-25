import { createElement as h } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { CodeLines } from "./CodeLines";

const lines = ["func main() {", "\tif ok {", "\t\trun()", "\t}", "}", ""];
const folds = [
  { at: 1, to: 4 },
  { at: 2, to: 3 },
];

const html = (props: Partial<Parameters<typeof CodeLines>[0]> = {}) => renderToStaticMarkup(h(CodeLines, { html: lines, path: "cmd/main.go", ...props }));

describe("CodeLines", () => {
  it("is the file's lines, numbered, named for the file, with the page's class last", () => {
    const out = html({ className: "file" });
    expect(out).toMatch(/role="group"/);
    expect(out).toContain('aria-label="cmd/main.go"');
    expect(out).toMatch(/class="[^"]*lines[^" ]* file"/);
    expect(out.match(/aria-label="Line \d+"/g)).toHaveLength(6);
    expect(out).toContain("--code-digits:2");
  });

  it("says where each line's code is, for what follows a name in it", () => {
    const out = html({ rev: "abc123", side: "old" });
    expect(out).toMatch(/<code[^>]*data-rev="abc123"[^>]*data-path="cmd\/main.go"[^>]*data-line="3"[^>]*data-side="old"/);
  });

  it("keeps an empty line's height", () => {
    expect(html()).toMatch(/data-line="6"[^>]*> <\/code>/);
  });

  it("marks changed lines and tints picked ones", () => {
    const out = html({ marks: new Map([[2, "added"]]), picked: { from: 3, to: 4 } });
    expect(out).toMatch(/<div data-line="2" class="[^"]*added/);
    expect(out.match(/<div data-line="\d" class="[^"]*picked/g)).toHaveLength(2);
  });

  it("gives a line that folds a twist that says what it folds", () => {
    const out = html({ folds });
    expect(out).toContain('aria-label="Fold lines 2 to 4"');
    expect(out).toContain('aria-label="Fold line 3"');
    expect(out.match(/aria-expanded="true"/g)).toHaveLength(2);
  });

  it("hides what's folded, and what folds inside it, and says how much", () => {
    const out = html({ folds, folded: new Set([1, 2]) });
    expect(out.match(/aria-label="Line \d+"/g)).toHaveLength(3);
    expect(out).toContain('aria-label="Unfold lines 2 to 4"');
    expect(out).toContain("⋯ 3 lines");
    expect(html({ folds, folded: new Set([2]) })).toContain("⋯ 1 line<");
  });

  it("draws what the page puts after a line", () => {
    const out = html({ after: (n) => (n === 3 ? "SAID" : undefined) });
    expect(out).toMatch(/data-after="3"[^>]*>SAID</);
    expect(out.match(/data-after=/g)).toHaveLength(1);
  });
});
