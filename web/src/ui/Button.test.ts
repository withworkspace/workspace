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

  it("keeps aria-busy, which react-aria's drops, and the browser's title while it's disabled", () => {
    const out = html({ title: "Save it (s)", "aria-busy": true, disabled: true });
    expect(out).toContain('title="Save it (s)"');
    expect(out).toContain('aria-busy="true"');
  });

  it("has its title as a tooltip, not the browser's, when it can be pointed at", () => {
    expect(html({ title: "Save it (s)" })).not.toContain("title=");
  });

  it("is named by its title when it's an icon alone, unless it's given a name", () => {
    expect(html({ title: "Close (esc)", square: true })).toContain('aria-label="Close (esc)"');
    expect(html({ title: "Close (esc)", square: true, "aria-label": "Close" })).toContain('aria-label="Close"');
    expect(html({ title: "Save it (s)" })).not.toContain("aria-label");
  });

  it("says its key after its title, from keys or from what it's given to press, and is named without it", () => {
    expect(html({ title: "Save it", keys: "s", disabled: true })).toContain('title="Save it (s)"');
    expect(html({ title: "Close", keys: "Escape", disabled: true })).toContain('title="Close (esc)"');
    const save = Object.assign(() => undefined, { keys: "s" });
    expect(html({ title: "Save it", onClick: save, disabled: true })).toContain('title="Save it (s)"');
    expect(html({ title: "Save it", onClick: () => save(), disabled: true })).toContain('title="Save it"');
    expect(html({ title: "Close", keys: "esc", square: true })).toContain('aria-label="Close"');
  });

  it("looks disabled and says so when it's unavailable, but can still be pointed at and focused", () => {
    const out = html({ unavailable: "Pick a verdict first", type: "submit" });
    expect(out).toContain('aria-disabled="true"');
    expect(out).toMatch(/class="[^"]*unavailable/);
    expect(out).not.toContain('disabled=""');
    expect(out).toContain('type="button"');
  });

  it("is disabled, not unavailable, when it's both", () => {
    const out = html({ unavailable: "Pick a verdict first", disabled: true });
    expect(out).toContain('disabled=""');
    expect(out).not.toContain("aria-disabled");
    expect(out).not.toMatch(/class="[^"]*unavailable/);
  });

  it("says pressed, expanded and its label", () => {
    const out = html({ "aria-pressed": true, "aria-expanded": false, "aria-label": "Pin" });
    expect(out).toContain('aria-pressed="true"');
    expect(out).toContain('aria-expanded="false"');
    expect(out).toContain('aria-label="Pin"');
  });

  it("has no look of the library's when it's bare, only the class it's given", () => {
    expect(html({ variant: "bare", className: "arrange-grip" })).toContain('class="arrange-grip"');
    expect(html({ variant: "bare", className: "arrange-grip" })).toContain('data-ui=""');
  });

  it("has the page's class after its own", () => {
    expect(html({ variant: "ghost", className: "find-chip" })).toMatch(/class="[^"]*ghost[^" ]* find-chip"/);
  });
});
