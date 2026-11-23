import { createElement as h } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { Radio, RadioGroup } from "./RadioGroup";

const html = (value: string, props: Partial<Parameters<typeof RadioGroup>[0]> = {}) =>
  renderToStaticMarkup(h(RadioGroup<string>, { value, onChange: () => {}, "aria-label": "Whose", ...props, children: [h(Radio, { key: "a", value: "me", children: "Just me" }), h(Radio, { key: "b", value: "team", children: "The team" })] }));

describe("RadioGroup", () => {
  it("is a group of radios, named, the chosen one checked", () => {
    const out = html("team");
    expect(out).toMatch(/^<div [^>]*role="radiogroup"/);
    expect(out).toContain('aria-label="Whose"');
    expect(out.match(/<input [^>]*type="radio"/g)).toHaveLength(2);
    expect(out).toMatch(/<label [^>]*data-selected="true"[^>]*>.*The team/);
    expect(out.match(/checked=""/g)).toHaveLength(1);
  });

  it("disables all of them, and has the page's class last", () => {
    expect(html("me", { disabled: true }).match(/<input [^>]*disabled=""/g)).toHaveLength(2);
    expect(html("me", { className: "thing-ui-place-list" })).toMatch(/class="[^"]*group[^" ]* thing-ui-place-list"/);
  });
});
