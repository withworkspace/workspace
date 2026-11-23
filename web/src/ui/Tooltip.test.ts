import { createElement as h } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { Button } from "./Button";
import { Tooltip } from "./Tooltip";

describe("Tooltip", () => {
  // It's drawn only while it's shown, in a layer of its own: until then
  // there's only what it's about on the page.
  it("is what it's about until it's shown", () => {
    const out = renderToStaticMarkup(h(Tooltip, { text: "Approve the document as it is (⇧A)", children: h(Button, { variant: "primary" }, "Approve") }));
    expect(out).toMatch(/^<button [^>]*>Approve<\/button>$/);
    expect(out).not.toContain("⇧A");
  });
});
