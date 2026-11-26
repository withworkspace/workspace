import { createElement as h } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { MemoryRouter } from "react-router";
import { describe, expect, it } from "vitest";
import { PageBar } from "./PageBar";

const back = { to: "/missions", title: "Missions (esc)", label: "Back to missions" };
const html = (props: Partial<Parameters<typeof PageBar>[0]> = {}) => renderToStaticMarkup(h(MemoryRouter, null, h(PageBar, { back, ...props })));

describe("PageBar", () => {
  it("has the way back first, named for a screen reader", () => {
    const out = html({ title: "Export invoices" });
    expect(out).toMatch(/<header[^>]*><a [^>]*href="\/missions"/);
    expect(out).toContain('aria-label="Back to missions"');
    expect(out).toContain('title="Missions (esc)"');
  });

  it("names the way back by what it says, with no name of its own", () => {
    expect(html({ back: { to: "/", title: "Back to conversations" }, title: "x" })).toContain('aria-label="Back to conversations"');
  });

  it("has the page's name as its h1, said whole on hover", () => {
    expect(html({ title: "Export invoices" })).toMatch(/<h1[^>]*><span[^>]*title="Export invoices">Export invoices<\/span><\/h1>/);
  });

  it("is a blank where the name will be, while there's none", () => {
    const out = html();
    expect(out).toMatch(/<h1[^>]*> <\/h1>/);
    expect(out).not.toContain("<span");
  });

  it("has a pencil only where the name can be changed", () => {
    expect(html({ title: "x", onRename: () => {}, renameLabel: "Rename mission" })).toContain('aria-label="Rename mission"');
    expect(html({ title: "x" })).not.toContain("Rename");
  });

  it("has what's done to it, then its views, after its head, and what hangs from it last", () => {
    const out = html({ title: "x", actions: "ACTS", tabs: "TABS", under: "UNDER", children: "BESIDE" });
    expect(out).toMatch(/BESIDE<\/header><div[^>]*>ACTS<\/div>TABS<\/div>UNDER<\/div>$/);
  });
});
