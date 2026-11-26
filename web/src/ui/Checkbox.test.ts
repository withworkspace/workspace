import { createElement as h } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { Checkbox } from "./Checkbox";

const html = (props: Partial<Parameters<typeof Checkbox>[0]> = {}) => renderToStaticMarkup(h(Checkbox, { checked: false, onChange: () => {}, children: "Open as draft", ...props }));

describe("Checkbox", () => {
  it("is a label over a checkbox, with its words", () => {
    const out = html();
    expect(out).toMatch(/^<label /);
    expect(out).toMatch(/<input [^>]*type="checkbox"/);
    expect(out).toContain("Open as draft");
  });

  it("says whether it's ticked", () => {
    expect(html()).not.toContain("data-selected");
    expect(html({ checked: true })).toContain('data-selected="true"');
    expect(html({ checked: true })).toMatch(/<input [^>]*checked=""/);
  });

  it("keeps a title, which react-aria's drops, and has the page's class last", () => {
    const out = html({ title: "Nothing's hidden", className: "rpass-show" });
    expect(out).toMatch(/^<label [^>]*title="Nothing&#x27;s hidden"/);
    expect(out).toMatch(/class="[^"]*checkbox[^" ]* rpass-show"/);
  });

  it("is disabled as the browser's is, and named when it has no words", () => {
    expect(html({ disabled: true })).toMatch(/<input [^>]*disabled=""/);
    expect(html({ children: undefined, "aria-label": "Accept item 3" })).toContain('aria-label="Accept item 3"');
  });
});
