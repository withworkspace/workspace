import { describe, expect, it, vi } from "vitest";

// The tests run without a page, so the accent's colours are given: each
// token stands for itself, to show which stop it lands on.
vi.mock("../ui/accent", () => ({
  shownAs: (token: string) =>
    ({
      "--brand-1": "#111111",
      "--brand-2": "#222222",
      "--brand-3": "#333333",
      "--brand-on-dark-1": "#aaaaaa",
      "--brand-on-dark-2": "#bbbbbb",
      "--brand-on-dark-3": "#cccccc",
      "--brand-on-dark-4": "#dddddd",
    })[token],
  useAccent: () => "",
}));

const { markIcon } = await import("./notify");

const svg = (lit: boolean) => {
  const uri = markIcon(lit);
  expect(uri.startsWith("data:image/svg+xml,")).toBe(true);
  return decodeURIComponent(uri.slice("data:image/svg+xml,".length));
};

// brand:svg/mark.svg's two shapes: what shows of the disc behind, and the disc in front.
const BEHIND = "M155.56 86.23A192 192 0 1 1 425.77 356.44A224 224 0 0 0 155.56 86.23Z";
const FRONT = "M16 304a192 192 0 1 0 384 0a192 192 0 1 0 -384 0Z";

describe("markIcon", () => {
  it("is brand's eclipse, on brand's 512 square with no tile", () => {
    const s = svg(false);
    expect(s).toContain('viewBox="0 0 512 512"');
    expect(s).not.toContain("<rect");
    expect(s).toContain(`<path d="${BEHIND}" class="g"/>`);
    expect(s).toContain(`<path d="${FRONT}" class="i"/>`);
    expect(s.indexOf(BEHIND)).toBeLessThan(s.indexOf(FRONT)); // the front disc is drawn over
  });

  it("runs the accent's light run behind on a light tab bar, and its dark run on a dark one", () => {
    const s = svg(false);
    expect(s).toContain(
      '<linearGradient id="l" x1="112" y1="0" x2="496" y2="0" gradientUnits="userSpaceOnUse"><stop offset="0" stop-color="#111111"/><stop offset="0.62" stop-color="#222222"/><stop offset="1" stop-color="#333333"/></linearGradient>',
    );
    expect(s).toContain(
      '<linearGradient id="d" x1="112" y1="0" x2="496" y2="0" gradientUnits="userSpaceOnUse"><stop offset="0" stop-color="#aaaaaa"/><stop offset="0.55" stop-color="#bbbbbb"/><stop offset="0.78" stop-color="#cccccc"/><stop offset="1" stop-color="#dddddd"/></linearGradient>',
    );
    expect(s).toContain(".g{fill:url(#l)}");
    expect(s).toMatch(/@media \(prefers-color-scheme:dark\)\{\.g\{fill:url\(#d\)\}/);
  });

  it("keeps the disc in front darker than the light behind: the ink on light, the dark run's first step dimmed on dark", () => {
    const s = svg(false);
    expect(s).toContain(".i{fill:#0d0b1e;fill-opacity:0.8}");
    expect(s).toContain(".i{fill:#aaaaaa;fill-opacity:0.45}");
  });

  it("is lit while something needs you: the light behind turns amber, in either theme", () => {
    expect(svg(false)).not.toContain("#f0a020");
    const s = svg(true);
    expect(s).toContain(`<path d="${BEHIND}" fill="#f0a020"/>`);
    expect(s).not.toContain('class="g"');
    expect(s).toContain(`<path d="${FRONT}" class="i"/>`);
  });
});
