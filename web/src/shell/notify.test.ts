import { describe, expect, it, vi } from "vitest";

// The tests run without a page, so the accent's colours are given: each
// token stands for itself, to show which stop it lands on.
vi.mock("../ui/accent", () => ({
  shownAs: (token: string) => ({ "--brand-1": "#111111", "--brand-2": "#222222", "--brand-3": "#333333" })[token],
  useAccent: () => "",
}));

const { markIcon } = await import("./notify");

const svg = (dot: boolean) => {
  const uri = markIcon(dot);
  expect(uri.startsWith("data:image/svg+xml,")).toBe(true);
  return decodeURIComponent(uri.slice("data:image/svg+xml,".length));
};

// The numbers in a path, as [x, y] points: H gives an x on the last y.
function points(d: string): [number, number][] {
  const out: [number, number][] = [];
  let y = 0;
  for (const m of d.matchAll(/([MLHZ])?\s*([\d.]+)?(?:\s+([\d.]+))?/g)) {
    if (m[1] === "H") out.push([Number(m[2]), y]);
    else if (m[2] && m[3]) {
      y = Number(m[3]);
      out.push([Number(m[2]), y]);
    }
  }
  return out;
}

describe("markIcon", () => {
  it("is brand's square-w: a 12% radius tile and a solid white w in its bottom-left", () => {
    const s = svg(false);
    expect(s).toContain('viewBox="0 0 32 32"');
    expect(s).toMatch(/<rect width="32" height="32" rx="3.81" fill="url\(#g\)"\/>/); // 61/512 of 32
    const d = s.match(/<path d="([^"]+)" fill="#fff"\/>/)![1];
    expect(s).not.toContain("stroke");
    // brand's "w", scaled from 512 to 32
    expect(d).toBe("M6.15 27.52 4.48 17.75H6.52L7.76 26.15H8.07L9.77 17.75H13.08L14.77 26.15H15.09L16.32 17.75H18.36L16.7 27.52H13.27L11.58 19.12H11.26L9.57 27.52Z");
    const ps = points(d);
    expect(Math.min(...ps.map((p) => p[0]))).toBeCloseTo(4.48, 2); // 14% in from the left
    expect(Math.max(...ps.map((p) => p[1]))).toBeCloseTo(27.52, 2); // and from the bottom
  });

  it("runs the accent's three stops on brand's diagonal", () => {
    expect(svg(false)).toContain(
      '<linearGradient id="g" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="#111111"/><stop offset="0.62" stop-color="#222222"/><stop offset="1" stop-color="#333333"/></linearGradient>',
    );
  });

  it("draws the amber dot only when asked, clear of the w", () => {
    expect(svg(false)).not.toContain("<circle");
    const s = svg(true);
    const [, cx, cy, r, sw] = s.match(/<circle cx="([\d.]+)" cy="([\d.]+)" r="([\d.]+)" fill="#f0a020" stroke="#fff" stroke-width="([\d.]+)"\/>/)!.map(Number);
    const d = s.match(/<path d="([^"]+)"/)![1];
    const ps = points(d);
    const outer = r + sw / 2;
    // the dot's outer edge is above and right of the "w"'s bounds
    expect(cy + outer).toBeLessThan(Math.min(...ps.map((p) => p[1])));
    expect(cx - outer).toBeGreaterThan(Math.min(...ps.map((p) => p[0])));
  });
});
