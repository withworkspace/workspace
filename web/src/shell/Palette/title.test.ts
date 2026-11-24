import { describe, expect, it } from "vitest";
import { titleParts } from "./title";

const said = (title: string, at: number[], path?: boolean) =>
  titleParts(title, at, path)
    .map((p) => `${p.hit ? "[" : ""}${p.name ? p.text.toUpperCase() : p.text}${p.hit ? "]" : ""}`)
    .join("|");

describe("titleParts", () => {
  it("is the title whole where nothing matched", () => {
    expect(titleParts("Rate-limit the API", [])).toEqual([{ text: "Rate-limit the API", hit: false }]);
  });

  it("sets the letters that matched apart, in the order they come", () => {
    expect(said("Export invoices", [0, 1, 7, 8])).toBe("[Ex]|port |[in]|voices");
  });

  it("has nothing for no title", () => {
    expect(titleParts("", [])).toEqual([]);
  });

  it("marks a path's name, after its last slash", () => {
    expect(said("web/src/money.ts", [], true)).toBe("web/src/|MONEY.TS");
  });

  it("splits a run that crosses from the folders into the name", () => {
    // "c/m" matched: the slash is the folders', the m the name's.
    expect(said("web/src/money.ts", [6, 7, 8], true)).toBe("web/sr|[c/]|[M]|ONEY.TS");
  });

  it("takes a line after the name as the name: what's after the last colon", () => {
    expect(said("web/src/money.ts:12", [], true)).toBe("web/src/money.ts:|12");
  });

  it("is all name where a path has no folders", () => {
    expect(said("justfile", [0], true)).toBe("[J]|USTFILE");
  });

  it("marks no name where the title isn't a path", () => {
    expect(titleParts("a/b", []).some((p) => p.name)).toBe(false);
  });
});
