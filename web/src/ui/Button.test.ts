import { createElement as h } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { Button } from "./Button";

// Button is react-aria's, which drops what it doesn't know of: these are
// what a page says to a button as it would to the browser's, and each has
// to reach the element.
const html = (props: Parameters<typeof Button>[0]) => renderToStaticMarkup(h(Button, props, "Save"));

describe("Button", () => {
  it("is the library's, and react-aria's", () => {
    const out = html({});
    expect(out).toMatch(/^<button /);
    expect(out).toContain('data-ui=""');
    expect(out).toContain("data-rac");
  });

  it("is a button, not a form's submit, unless it's said", () => {
    expect(html({})).toContain('type="button"');
    expect(html({ type: "submit" })).toContain('type="submit"');
  });

  it("is disabled as the browser's is", () => {
    expect(html({ disabled: true })).toContain('disabled=""');
    expect(html({})).not.toContain("disabled");
  });

  it("keeps a title and aria-busy, which react-aria's drops", () => {
    const out = html({ title: "Save it (s)", "aria-busy": true, disabled: true });
    expect(out).toContain('title="Save it (s)"');
    expect(out).toContain('aria-busy="true"');
  });

  it("says pressed, expanded and its label", () => {
    const out = html({ "aria-pressed": true, "aria-expanded": false, "aria-label": "Pin" });
    expect(out).toContain('aria-pressed="true"');
    expect(out).toContain('aria-expanded="false"');
    expect(out).toContain('aria-label="Pin"');
  });

  it("has the page's class after its own", () => {
    expect(html({ variant: "ghost", className: "find-chip" })).toMatch(/class="[^"]*ghost[^" ]* find-chip"/);
  });
});
