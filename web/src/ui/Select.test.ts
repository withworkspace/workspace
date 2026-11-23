import { createElement as h } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { Option, Select } from "./Select";

const html = (props: Partial<Parameters<typeof Select>[0]> = {}) =>
  renderToStaticMarkup(h(Select, { value: "claude", onChange: () => {}, "aria-label": "Agent", ...props, children: [h(Option, { key: "a", value: "claude", children: "claude" }), h(Option, { key: "b", value: "codex", children: "codex" })] }));

describe("Select", () => {
  it("is a button that says what's chosen, and that it opens a list", () => {
    const out = html();
    expect(out).toMatch(/<button [^>]*aria-haspopup="listbox"/);
    expect(out).toMatch(/aria-expanded="false"/);
    expect(out).toContain(">claude<");
  });

  it("marks its button: the library's, keys typed there its own, and not where a dialog's focus starts", () => {
    const out = html();
    expect(out).toMatch(/<button [^>]*data-ui=""/);
    expect(out).toMatch(/<button [^>]*data-field="true"/);
    expect(out).toMatch(/<button [^>]*data-select=""/);
  });

  it("has its size, and the page's class last", () => {
    expect(html({ size: "small", fill: true, className: "setting-select" })).toMatch(/class="[^"]*select[^"]*small[^"]*fill[^" ]* setting-select"/);
  });

  it("is disabled as the browser's is", () => {
    expect(html({ disabled: true })).toMatch(/<button [^>]*disabled=""/);
  });
});
