import { createElement as h } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { Tab, Tabs } from "./Tabs";

const html = (value: string, controls?: string) =>
  renderToStaticMarkup(
    h(Tabs<string>, { value, onChange: () => {}, controls, "aria-label": "Mission", children: [h(Tab, { key: "a", value: "summary", title: "Summary (1)", children: "Summary" }), h(Tab, { key: "b", value: "work", children: "Work" })] }),
  );

describe("Tabs", () => {
  it("is a list of tabs, the one shown selected", () => {
    const out = html("work");
    expect(out).toMatch(/role="tablist"[^>]*aria-label="Mission"|aria-label="Mission"[^>]*role="tablist"/);
    expect(out).toMatch(/<div [^>]*aria-selected="true"[^>]*role="tab"[^>]*>Work</);
    expect(out).toMatch(/<div [^>]*aria-selected="false"[^>]*>Summary</);
  });

  it("keeps a tab's title, which react-aria's drops", () => {
    expect(html("work")).toContain('title="Summary (1)"');
  });

  it("says what the page says it changes, on the one shown, and nothing of react-aria's own panel", () => {
    expect(html("work", "mission-panel").match(/aria-controls="[^"]*"/g)).toEqual(['aria-controls="mission-panel"']);
    expect(html("work")).not.toContain("aria-controls");
  });

  it("says it's the tabs, for a print to leave out", () => {
    expect(html("work")).toContain('data-tabs=""');
  });
});
