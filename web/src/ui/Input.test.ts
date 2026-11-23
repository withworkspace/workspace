import { createElement as h } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { Input } from "./Input";

const html = (props: Parameters<typeof Input>[0] = {}) => renderToStaticMarkup(h(Input, { value: "", onChange: () => {}, ...props }));

describe("Input", () => {
  it("is an input, with the box's class and then the page's", () => {
    expect(html({ className: "mono" })).toMatch(/^<input [^>]*class="[^"]*input[^" ]* mono"/);
  });

  // react-aria's input passes on what it's given; the pages count on these.
  it("takes what the browser's takes", () => {
    const out = html({ type: "text", placeholder: "Find", title: "Find a file", list: "places", spellCheck: false, inputMode: "decimal", autoComplete: "off", id: "as", "aria-label": "Find", "aria-invalid": true, "aria-busy": true, disabled: true });
    for (const said of ['type="text"', 'placeholder="Find"', 'title="Find a file"', 'list="places"', 'spellCheck="false"', 'inputMode="decimal"', 'autoComplete="off"', 'id="as"', 'aria-label="Find"', 'aria-invalid="true"', 'aria-busy="true"', 'disabled=""']) expect(out).toContain(said);
  });

  it("can be a finder's: a combobox that says which of a list it's on", () => {
    const out = html({ role: "combobox", "aria-expanded": true, "aria-controls": "found", "aria-activedescendant": "found-3" });
    for (const said of ['role="combobox"', 'aria-expanded="true"', 'aria-controls="found"', 'aria-activedescendant="found-3"']) expect(out).toContain(said);
  });

  it("is marked for the page's keys as it's given (data-field)", () => {
    expect(html({ "data-field": "" } as Parameters<typeof Input>[0])).toContain('data-field=""');
  });
});
