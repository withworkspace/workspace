import { createElement as h } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { MemoryRouter } from "react-router";
import { describe, expect, it } from "vitest";
import { Crumbs, CrumbSep } from "./Crumbs";

const html = (props: Parameters<typeof Crumbs>[0]) => renderToStaticMarkup(h(MemoryRouter, null, h(Crumbs, props)));

describe("Crumbs", () => {
  it("is a breadcrumb, for a screen reader and for what a print leaves out", () => {
    const out = html({ steps: [{ to: "/missions/x", label: "X" }] });
    expect(out).toMatch(/^<nav [^>]*aria-label="Breadcrumb"/);
    expect(out).toContain('data-crumbs=""');
  });

  it("has an arrow on the way back, and only there", () => {
    const out = html({
      steps: [
        { to: "/missions/x", label: "X" },
        { to: "/orchestrator", label: "All runs" },
      ],
    });
    expect(out.match(/<svg/g)).toHaveLength(1);
    expect(out).toMatch(/<a [^>]*href="\/missions\/x"[^>]*><svg/);
    expect(out).toMatch(/<span[^>]*>·<\/span><a [^>]*href="\/orchestrator"[^>]*> All runs<\/a>/);
  });

  it("draws a path of the page's own pieces", () => {
    const out = html({ children: [h("b", { key: 1 }, "Documents"), h(CrumbSep, { key: 2 }, "/"), h("i", { key: 3 }, "records")] });
    expect(out).toMatch(/<b>Documents<\/b><span[^>]*>\/<\/span><i>records<\/i><\/nav>$/);
  });
});
