// Which side of the page a dock is on: beside it (at its right), or below.
export type DockSide = "beside" | "below";

// How far a key moves a dock's edge.
export const DOCK_STEP = 16;

// sized is a dock's size after its edge is dragged by (dx, dy) from where
// it was at start, kept within what it may be. Beside's edge is its left
// and Below's its top, so dragging left or up makes the dock bigger.
export function sized(
  side: DockSide,
  start: number,
  dx: number,
  dy: number,
  min: number,
  max: number,
): number {
  const to = side === "beside" ? start - dx : start - dy;
  return Math.round(Math.min(max, Math.max(min, to)));
}

// keyed is a dock's size after a key on its edge, or nothing if the key
// isn't one of the edge's: the arrows that point away from the dock grow
// it, the ones that point into it shrink it, Home and End are its least
// and its most.
export function keyed(
  side: DockSide,
  key: string,
  size: number,
  min: number,
  max: number,
): number | undefined {
  const grow = side === "beside" ? "ArrowLeft" : "ArrowUp";
  const shrink = side === "beside" ? "ArrowRight" : "ArrowDown";
  switch (key) {
    case grow:
      return sized(side, size + DOCK_STEP, 0, 0, min, max);
    case shrink:
      return sized(side, size - DOCK_STEP, 0, 0, min, max);
    case "Home":
      return min;
    case "End":
      return max;
  }
  return undefined;
}
