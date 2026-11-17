import { describe, expect, it } from "vitest";
import { parseHex, pulled } from "./accent";

describe("parseHex", () => {
  it("takes a hex colour as [ui] accent does, as #rrggbb in lowercase", () => {
    expect(parseHex("#336699")).toBe("#336699");
    expect(parseHex("#3A7BD5")).toBe("#3a7bd5");
    expect(parseHex("#abc")).toBe("#aabbcc");
    expect(parseHex("  #F0f ")).toBe("#ff00ff");
  });

  it("refuses anything else", () => {
    for (const s of ["", "#", "336699", "#33669", "#3366999", "#33669980", "#ggg", "red", "rgb(51 102 153)", "#fff;}"]) {
      expect(parseHex(s), s).toBeNull();
    }
  });
});

describe("pulled", () => {
  it("says how the theme had to show a pick", () => {
    expect(pulled("#336699", "#336699")).toBeNull();
    expect(pulled("#336699", "#33669b")).toBeNull();
    expect(pulled("#336699", "#a2b2e4")).toBe("lighter");
    expect(pulled("#fff59d", "#69653e")).toBe("darker");
  });

  it("says nothing of what it can't read", () => {
    expect(pulled("", "#336699")).toBeNull();
    expect(pulled("#336699", "")).toBeNull();
  });
});
