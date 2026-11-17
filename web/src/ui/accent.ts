// The accent: one colour, --accent-pick on <html>, from which styles.css
// works out every accent colour for the theme. The server puts [ui] accent
// there as it serves the page, so it's there before the page first paints;
// the Settings page changes it there as you pick, so the page follows
// without a reload. With none there, styles.css's own default applies. See
// workspace-records:docs/customisable-ui-accent.md.

import { useSyncExternalStore } from "react";

const pick = "--accent-pick";
const listeners = new Set<() => void>();

// showAccent puts hex on the page as the accent or, given none, takes it
// away, so the default applies.
export function showAccent(hex: string | null) {
  const style = document.documentElement.style;
  if (hex) style.setProperty(pick, hex);
  else style.removeProperty(pick);
  listeners.forEach((l) => l());
}

function subscribe(l: () => void) {
  listeners.add(l);
  return () => listeners.delete(l);
}

// useAccent is the accent on the page now ("" for the default), for what
// draws it where the page's styles don't reach (the tab's icon).
export function useAccent(): string {
  return useSyncExternalStore(subscribe, () => document.documentElement.style.getPropertyValue(pick));
}

// defaultAccent is styles.css's own accent, the one the page has when none
// is set, as #rrggbb. The one set is taken off <html> while it's read, and
// put back, before anything paints.
export function defaultAccent(): string {
  const style = document.documentElement.style;
  const set = style.getPropertyValue(pick);
  if (set) style.removeProperty(pick);
  const own = getComputedStyle(document.documentElement).getPropertyValue(pick).trim();
  if (set) style.setProperty(pick, set);
  return parseHex(own) ?? hexOf(own);
}

// shownAs is a colour token (--accent) as the page draws it now, in this
// theme, as #rrggbb.
export function shownAs(token: string): string {
  const probe = document.createElement("span");
  probe.style.color = `var(${token})`;
  probe.hidden = true;
  document.body.append(probe);
  const color = getComputedStyle(probe).color;
  probe.remove();
  return hexOf(color);
}

let canvas: CanvasRenderingContext2D | null = null;

// hexOf is any CSS colour (oklch(…), color(srgb …)) as #rrggbb, as the page
// draws it in sRGB: "" for one it can't read.
export function hexOf(color: string): string {
  canvas ??= document.createElement("canvas").getContext("2d", { willReadFrequently: true });
  if (!canvas || !color) return "";
  canvas.clearRect(0, 0, 1, 1);
  canvas.fillStyle = "#000";
  canvas.fillStyle = color;
  canvas.fillRect(0, 0, 1, 1);
  const [r, g, b] = canvas.getImageData(0, 0, 1, 1).data;
  return "#" + [r, g, b].map((v) => v.toString(16).padStart(2, "0")).join("");
}

// parseHex is s as #rrggbb in lowercase, if it's a hex colour (#rgb or
// #rrggbb), as [ui] accent takes: null if not.
export function parseHex(s: string): string | null {
  const m = /^#([0-9a-f]{3}|[0-9a-f]{6})$/i.exec(s.trim());
  if (!m) return null;
  const h = m[1].toLowerCase();
  return "#" + (h.length === 3 ? [...h].map((c) => c + c).join("") : h);
}

// luminance is a #rrggbb colour's relative luminance, as contrast is
// worked out from.
function luminance(hex: string): number {
  const [r, g, b] = [1, 3, 5].map((i) => {
    const c = parseInt(hex.slice(i, i + 2), 16) / 255;
    return c <= 0.04045 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4;
  });
  return 0.2126 * r + 0.7152 * g + 0.0722 * b;
}

// pulled is how the theme shows a pick, where it's had to lighten or
// darken it so it reads on the page: "lighter", "darker", or null where
// it shows as picked (within a step no one would see).
export function pulled(picked: string, shown: string): "lighter" | "darker" | null {
  const a = parseHex(picked);
  const b = parseHex(shown);
  if (!a || !b) return null;
  const close = [1, 3, 5].every((i) => Math.abs(parseInt(a.slice(i, i + 2), 16) - parseInt(b.slice(i, i + 2), 16)) <= 4);
  if (close) return null;
  return luminance(b) > luminance(a) ? "lighter" : "darker";
}
