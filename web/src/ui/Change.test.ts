import { createElement as h } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { Change } from "./Change";

const html = (props: Parameters<typeof Change>[0]) => renderToStaticMarkup(h(Change, props));
// The words of each line, in order.
const lines = (out: string) => [...out.matchAll(/<div class="[^"]*_line_[^"]*">([^<]*)<\/div>/g)].map((m) => m[1]);

describe("Change", () => {
  it("draws what was there over what would be", () => {
    expect(lines(html({ was: ["a", "b"], now: ["c"] }))).toEqual(["a", "b", "c"]);
  });

  it("signs each line where it's asked to", () => {
    expect(lines(html({ signs: true, was: ["a"], now: ["b"] }))).toEqual(["− a", "+ b"]);
  });

  it("keeps an empty line a line's height", () => {
    expect(lines(html({ was: [""], now: [""] }))).toEqual([" ", " "]);
  });

  it("says what's gone where there's nothing in its place, and only then", () => {
    expect(lines(html({ was: ["a"], now: [], gone: "(delete)" }))).toEqual(["a", "(delete)"]);
    expect(lines(html({ was: ["a"], now: ["b"], gone: "(delete)" }))).toEqual(["a", "b"]);
  });

  it("draws each passage of several", () => {
    const out = html({
      hunks: [
        { was: ["a"], now: ["b"] },
        { was: ["c"], now: ["d"] },
      ],
    });
    expect(lines(out)).toEqual(["a", "b", "c", "d"]);
    expect(out.match(/_hunk_/g)).toHaveLength(2);
  });

  it("has a title only when it's given one", () => {
    expect(html({ title: "Suggested change", was: ["a"] })).toContain(">Suggested change</div>");
    expect(html({ was: ["a"] })).not.toContain("_title_");
  });
});
