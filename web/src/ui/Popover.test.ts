import { createElement as h } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { Button } from "./Button";
import { Popover } from "./Popover";

describe("Popover", () => {
  // Shut, there's only its button on the page: what it opens isn't drawn
  // until it's open, and then in a layer of its own.
  it("is its button until it's opened, which says it opens something", () => {
    const out = renderToStaticMarkup(h(Popover, { trigger: h(Button, { variant: "ghost" }, "Tweaks"), "aria-label": "Tweaks", children: "What's in it" }));
    expect(out).toMatch(/^<button [^>]*aria-expanded="false"[^>]*>Tweaks<\/button>$/);
    expect(out).not.toContain("What's in it");
  });
});
