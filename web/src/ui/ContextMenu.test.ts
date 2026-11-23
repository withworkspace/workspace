import { createElement as h } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { ContextMenuProvider, useContextMenu } from "./ContextMenu";
import { MenuItem } from "./Menu";

type Handler = ReturnType<ReturnType<typeof useContextMenu>>;

// What a press of the other button is, as far as a handler's concerned.
const press = () => {
  let prevented = false;
  const e = { clientX: 10, clientY: 20, preventDefault: () => (prevented = true) } as unknown as Parameters<Handler>[0];
  return { e, prevented: () => prevented };
};

// grab draws a part inside (or outside) the provider and hands back the
// handler it was given for its menu.
function grab(items: Parameters<ReturnType<typeof useContextMenu>>[1], provided: boolean): { out: string; on: Handler } {
  let on: Handler = () => {};
  function Part() {
    on = useContextMenu()("A row", items);
    return h("p", null, "A row");
  }
  const out = renderToStaticMarkup(provided ? h(ContextMenuProvider, null, h(Part)) : h(Part));
  return { out, on };
}

const items = () => h(MenuItem, { id: "park", textValue: "Park it", children: "Park it" });

describe("ContextMenu", () => {
  // Shut, there's nothing of it on the page but where it'll open by.
  it("draws no menu until one's asked for", () => {
    const { out } = grab(items, true);
    expect(out).toContain("<p>A row</p>");
    expect(out).not.toContain('role="menu"');
  });

  it("leaves the browser's menu alone where a part has nothing for its own", () => {
    const { on } = grab(null, true);
    const p = press();
    on(p.e);
    expect(p.prevented()).toBe(false);
  });

  it("leaves the browser's menu alone where nothing around could draw one", () => {
    const { on } = grab(items, false);
    const p = press();
    on(p.e);
    expect(p.prevented()).toBe(false);
  });
});
