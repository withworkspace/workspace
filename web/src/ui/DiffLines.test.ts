import { createElement as h } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { DiffLines } from "./DiffLines";
import type { DiffCell, Keyed } from "./diffLayout";

const cell = (num: number, kind: "c" | "a" | "d" = "c", more: Partial<DiffCell> = {}): DiffCell => ({ num, text: `line ${num}`, kind, ...more });

const rows: Keyed[] = [
  { type: "gap", key: "g0", count: 8, section: "func main()" } as Keyed,
  { type: "line", key: "n9", left: cell(9), right: cell(9) } as Keyed,
  { type: "line", key: "o10", left: cell(10, "d") } as Keyed,
  { type: "line", key: "n10", right: cell(10, "a", { noeol: true }) } as Keyed,
  { type: "said", key: "said" },
  { type: "gap", key: "g1" } as Keyed,
];

type Props = Parameters<typeof DiffLines>[0];
const html = (props: Partial<Props> = {}, of: Keyed[] = rows) =>
  renderToStaticMarkup(h(DiffLines, { rows: of, "aria-label": "Changes to main.go", html: (c: DiffCell, side: "old" | "new") => `${side}:${c.text}`, ...props }));

describe("DiffLines", () => {
  it("is a file's diff, named, with the page's class last and its numbers' width set", () => {
    const out = html({ className: "diff" });
    expect(out).toMatch(/role="group"/);
    expect(out).toContain('aria-label="Changes to main.go"');
    expect(out).toMatch(/class="[^"]*lines[^" ]* diff"/);
    expect(out).toContain("--diff-digits:2");
  });

  it("draws a line once in one column, by the side it's on, with a sign", () => {
    const out = html();
    expect(out.match(/<code/g)).toHaveLength(3);
    expect(out).toContain("old:line 10");
    expect(out).toContain("new:line 10");
    expect(out).not.toContain("old:line 9");
    expect(out).toContain(">+<");
    expect(out).toContain(">−<");
  });

  it("draws each side side by side, with nothing where a side has no line", () => {
    const out = html({ split: true });
    expect(out).toContain("old:line 9");
    expect(out).toContain("new:line 9");
    expect(out.match(/<code/g)).toHaveLength(4);
    expect(out).not.toContain(">+<");
  });

  it("says where each side's lines are read, for what follows a name in them", () => {
    const out = html({ at: { old: { rev: "aaa", path: "was.go" }, new: { rev: "bbb", path: "main.go" } } });
    expect(out).toMatch(/<code[^>]*data-rev="aaa"[^>]*data-path="was.go"[^>]*data-line="10"[^>]*data-side="old"/);
    expect(out).toMatch(/<code[^>]*data-rev="bbb"[^>]*data-path="main.go"[^>]*data-line="10"[^>]*data-side="new"/);
  });

  it("gives each line number its row to pick, and side by side its column", () => {
    expect(html()).toMatch(/data-pick="o10"[^>]*aria-label="Old line 10"|aria-label="Old line 10"[^>]*data-pick="o10"/);
    expect(html()).not.toContain("data-side=\"old\" tabindex");
    expect(html({ split: true })).toMatch(/data-pick="n10" data-side="new"/);
  });

  it("tints the picked lines, and side by side only their column", () => {
    const count = (out: string) => (out.match(/picked/g) ?? []).length;
    expect(count(html())).toBe(0);
    // One column: both numbers, the sign and the code of each of two lines.
    expect(count(html({ picked: { from: "o10", to: "n10" } }))).toBe(8);
    // Side by side: the new side's number and code of one line.
    expect(count(html({ split: true, picked: { from: "n10", to: "n10", side: "new" } }))).toBe(2);
  });

  it("says how many lines a gap hides and what they're in, or only that there's more", () => {
    const out = html();
    expect(out).toContain("Show 8 unchanged lines");
    expect(out).toContain("func main()");
    expect(out).toContain("Show more");
  });

  it("marks a last line with no newline after it", () => {
    expect(html()).toContain("No newline at the end of the file");
  });

  it("gives a row it doesn't know to the page, and leaves it out with nothing to draw it", () => {
    expect(html({ draw: (r) => (r.type === "said" ? "SAID" : undefined) })).toMatch(/data-row="said"[^>]*>SAID</);
    expect(html()).not.toContain('data-row="said"');
  });

  it("marks the row the page's keys are on", () => {
    expect(html({ cursor: "n9" })).toMatch(/data-row="n9" class="[^"]*cursor/);
    expect(html({ cursor: "g0" })).toMatch(/data-row="g0"[^>]*class="[^"]*cursor/);
  });

  it("says a note's words, and makes one that stands for lines pressable", () => {
    const notes: Keyed[] = [{ type: "note", key: "a", text: "Binary file." } as Keyed, { type: "note", key: "b", text: "Large diff.", large: true } as Keyed];
    const out = html({}, notes);
    expect(out).toMatch(/<div data-row="a"[^>]*>Binary file\.</);
    expect(out).toMatch(/<button[^>]*data-row="b"[^>]*>Large diff\.</);
  });
});
