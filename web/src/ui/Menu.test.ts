import { createElement as h } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { Button } from "./Button";
import { Menu, MenuItem } from "./Menu";

describe("Menu", () => {
  // Shut, there's only its button on the page: the list isn't drawn until
  // it's open, and then in a layer of its own.
  it("is its button until it's opened, which says it has a menu", () => {
    const out = renderToStaticMarkup(h(Menu, { trigger: h(Button, { "aria-label": "Panes" }, "Panes"), "aria-label": "Panes", children: h(MenuItem, { id: "2", textValue: "Two", children: "Two" }) }));
    expect(out).toMatch(/^<button [^>]*>Panes<\/button>/);
    expect(out).toContain('aria-haspopup="true"');
    expect(out).toContain('aria-expanded="false"');
    expect(out).not.toContain('role="menu"');
  });
});
