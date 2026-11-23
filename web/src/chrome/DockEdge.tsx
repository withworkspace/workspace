import { useRef } from "react";
import { cx } from "../ui/cx";
import css from "./DockEdge.module.css";
import { type DockSide, keyed, sized } from "./dockSize";

type DockEdgeProps = {
  side: DockSide;
  // The dock's size now, and the least and most it may be.
  size: number;
  min: number;
  max: number;
  onSize: (size: number) => void;
  // The size a double-click puts it back to.
  rest?: number;
  // What it resizes, for a screen reader: "Beside's width".
  label: string;
};

// DockEdge is a dock's edge towards the page: dragged, it resizes the
// dock, and the arrow keys do the same a step at a time. A double-click
// puts the dock back to the size it started at.
export function DockEdge({
  side,
  size,
  min,
  max,
  onSize,
  rest,
  label,
}: DockEdgeProps) {
  const from = useRef<{ x: number; y: number; size: number } | undefined>(
    undefined,
  );
  return (
    <div
      role="separator"
      tabIndex={0}
      aria-label={label}
      aria-orientation={side === "beside" ? "vertical" : "horizontal"}
      aria-valuenow={size}
      aria-valuemin={min}
      aria-valuemax={max}
      title={
        rest === undefined
          ? "Drag to resize"
          : "Drag to resize; double-click to put it back"
      }
      className={cx(css.edge, side === "beside" ? css.beside : css.below)}
      onPointerDown={(e) => {
        e.preventDefault();
        e.currentTarget.setPointerCapture(e.pointerId);
        from.current = { x: e.clientX, y: e.clientY, size };
      }}
      onPointerMove={(e) => {
        const f = from.current;
        if (f)
          onSize(
            sized(side, f.size, e.clientX - f.x, e.clientY - f.y, min, max),
          );
      }}
      onPointerUp={() => (from.current = undefined)}
      onPointerCancel={() => (from.current = undefined)}
      onDoubleClick={() => rest !== undefined && onSize(rest)}
      onKeyDown={(e) => {
        const to = keyed(side, e.key, size, min, max);
        if (to === undefined) return;
        e.preventDefault();
        onSize(to);
      }}
    />
  );
}
