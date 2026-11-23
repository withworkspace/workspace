import { createElement as h } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { Segment, Segmented } from "./Segmented";

const html = (value: string, props: Partial<Parameters<typeof Segmented>[0]> = {}) =>
  renderToStaticMarkup(
    h(Segmented<string>, { value, onChange: () => {}, "aria-label": "Width", ...props, children: [h(Segment, { key: "a", value: "fit", title: "As wide as the page", children: "Fit" }), h(Segment, { key: "b", value: "390", disabled: true, children: "390" })] }),
  );

describe("Segmented", () => {
  it("is a group of radios, the chosen one checked", () => {
    const out = html("fit");
    expect(out).toMatch(/^<div [^>]*role="radiogroup"/);
    expect(out).toContain('aria-label="Width"');
    expect(out).toMatch(/<button [^>]*role="radio"[^>]*aria-checked="true"[^>]*data-selected="true"[^>]*>Fit</);
    expect(out).toMatch(/<button [^>]*aria-checked="false"[^>]*>390</);
  });

  it("keeps a segment's title, which react-aria's drops, and says it's the library's", () => {
    const out = html("fit");
    expect(out).toContain('title="As wide as the page"');
    expect(out.match(/data-ui=""/g)).toHaveLength(2);
  });

  it("disables a segment, or all of them", () => {
    expect(html("fit").match(/ disabled=""/g)).toHaveLength(1);
    expect(html("fit", { disabled: true }).match(/ disabled=""/g)).toHaveLength(2);
  });

  it("has the id it's given, and the page's class after its own", () => {
    const out = html("fit", { id: "tweak-3", className: "tweak-choice", size: "small" });
    expect(out).toContain('id="tweak-3"');
    expect(out).toMatch(/class="[^"]*segmented[^"]*small[^" ]* tweak-choice"/);
  });
});
