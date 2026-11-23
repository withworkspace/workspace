import type { CSSProperties, ReactNode } from "react";
import { cx } from "../ui/cx";
import css from "./Dock.module.css";
import type { DockSide } from "./dockSize";

type DockProps = {
  side: DockSide;
  // What it is, for a screen reader: "Beside", "Below".
  label: string;
  // Its width beside the page, or its height below it.
  size: number;
  // Over the page, not next to it: a little see-through, with what's
  // under it blurred, and a deeper shadow. Where it sits is its place's
  // to say (className).
  over?: boolean;
  // A sheet, on a narrow window: it fills the place it's given, whatever
  // its size, and has no edge to drag.
  sheet?: boolean;
  // Its edge, to drag (DockEdge).
  edge?: ReactNode;
  className?: string;
  children: ReactNode;
};

// Dock is a card docked to the page: beside it or below it. It's the
// sidebar's card on another side: the same ground, line, corners and
// shadow. What's in it is a DockHead and what its tab shows (DockBody,
// DockFoot). It's a region by its role, not a <section>: styles.css
// spaces every <section> for the pages, which a part's styles can't undo.
export function Dock({
  side,
  label,
  size,
  over,
  sheet,
  edge,
  className,
  children,
}: DockProps) {
  return (
    <div
      role="region"
      aria-label={label}
      className={cx(
        css.dock,
        side === "beside" ? css.beside : css.below,
        over && css.over,
        sheet && css.sheet,
        className,
      )}
      style={{ "--dock-size": `${size}px` } as CSSProperties}
    >
      {!sheet && edge}
      {children}
    </div>
  );
}
