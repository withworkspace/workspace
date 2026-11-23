import { createElement as h } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { ToggleButton } from "./ToggleButton";

const html = (props: Partial<Parameters<typeof ToggleButton>[0]> = {}) => renderToStaticMarkup(h(ToggleButton, { pressed: false, onChange: () => {}, children: "Details", ...props }));

describe("ToggleButton", () => {
  it("is a button that says whether it's pressed", () => {
    expect(html()).toMatch(/^<button [^>]*aria-pressed="false"/);
    expect(html({ pressed: true })).toContain('aria-pressed="true"');
    expect(html()).toContain('data-ui=""');
    expect(html()).toContain('type="button"');
  });

  it("is in the accent while it's pressed, plain or ghost", () => {
    expect(html({ pressed: true })).toMatch(/class="[^"]*pressed/);
    expect(html({ pressed: true, variant: "ghost" })).toMatch(/class="[^"]*pressed/);
    expect(html()).not.toMatch(/class="[^"]*pressed/);
  });

  it("leaves a primary or a danger one as it is", () => {
    expect(html({ pressed: true, variant: "danger" })).not.toMatch(/class="[^"]*pressed/);
    expect(html({ pressed: true, variant: "primary" })).not.toMatch(/class="[^"]*pressed/);
  });

  it("keeps a title and aria-busy, which react-aria's drops", () => {
    const out = html({ title: "Every layer's changes (w)", "aria-busy": true });
    expect(out).toContain("title=\"Every layer&#x27;s changes (w)\"");
    expect(out).toContain('aria-busy="true"');
  });

  it("is disabled as the browser's is, and has the page's class last", () => {
    expect(html({ disabled: true })).toContain('disabled=""');
    expect(html({ pressed: true, className: "on-met" })).toMatch(/class="[^"]* on-met"/);
  });
});
