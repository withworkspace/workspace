import { createElement as h } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { TextArea } from "./TextArea";

const html = (props: Parameters<typeof TextArea>[0] = {}) => renderToStaticMarkup(h(TextArea, { value: "", onChange: () => {}, ...props }));

describe("TextArea", () => {
  it("is a textarea, with the box's class and then the page's", () => {
    expect(html({ className: "doc-source" })).toMatch(/^<textarea [^>]*class="[^"]*textarea[^" ]* doc-source"/);
  });

  // react-aria's passes on what it's given; the pages count on these.
  it("takes what the browser's takes", () => {
    const out = html({ rows: 3, placeholder: "Why", title: "Say why", "aria-label": "Why", disabled: true, value: "Because" });
    for (const said of ['rows="3"', 'placeholder="Why"', 'title="Say why"', 'aria-label="Why"', 'disabled=""', ">Because</textarea>"]) expect(out).toContain(said);
  });
});
