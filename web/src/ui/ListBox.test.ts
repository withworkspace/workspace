import { createElement as h } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { ListBox, ListBoxOption } from "./ListBox";

const html = (value: string | undefined, options = true) =>
  renderToStaticMarkup(
    h(ListBox<string>, {
      value,
      onChange: () => {},
      "aria-label": "Repos",
      className: "repos",
      empty: "Nothing to add here.",
      children: options ? [h(ListBoxOption, { key: "a", value: "api", textValue: "api", children: "api" }), h(ListBoxOption, { key: "b", value: "records", textValue: "records", disabled: true, title: "Here already", className: "repo", children: "records" })] : [],
    }),
  );

describe("ListBox", () => {
  it("is a list of options, named, with the page's class last", () => {
    const out = html(undefined);
    expect(out).toMatch(/role="listbox"/);
    expect(out).toContain('aria-label="Repos"');
    expect(out).toMatch(/class="[^"]*list[^" ]* repos"/);
    expect(out.match(/role="option"/g)).toHaveLength(2);
  });

  it("marks the one picked, and none when none is", () => {
    expect(html(undefined)).not.toContain('aria-selected="true"');
    expect(html("api")).toMatch(/aria-selected="true"[^>]*>api</);
  });

  it("says one can't be picked, and keeps its title, which react-aria's drops", () => {
    const out = html(undefined);
    expect(out).toMatch(/aria-disabled="true"[^>]*title="Here already"|title="Here already"[^>]*aria-disabled="true"/);
    expect(out).toMatch(/class="[^"]*option[^" ]* repo"/);
  });

  it("says what it's given when it's empty", () => {
    expect(html(undefined, false)).toContain("Nothing to add here.");
  });
});
