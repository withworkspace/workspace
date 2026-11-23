import { createElement as h } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { Switch } from "./Switch";

const html = (props: Partial<Parameters<typeof Switch>[0]> = {}) => renderToStaticMarkup(h(Switch, { on: false, onChange: () => {}, "aria-label": "Tell the desktop", ...props }));

describe("Switch", () => {
  it("is a switch that says whether it's on, by its name", () => {
    expect(html()).toMatch(/<input [^>]*role="switch"/);
    expect(html()).toContain('aria-label="Tell the desktop"');
    expect(html()).not.toContain("data-selected");
    expect(html({ on: true })).toContain('data-selected="true"');
    expect(html({ on: true })).toMatch(/<input [^>]*checked=""/);
  });

  it("is disabled as the browser's is, and has the page's class last", () => {
    expect(html({ disabled: true })).toMatch(/<input [^>]*disabled=""/);
    expect(html({ className: "tweak-toggle" })).toMatch(/class="[^"]*switch[^" ]* tweak-toggle"/);
  });
});
