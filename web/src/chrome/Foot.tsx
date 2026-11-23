import type { ReactNode } from "react";
import { cx } from "../ui/cx";
import css from "./Foot.module.css";

type FootProps = {
  // From its start: what's going on now. It gives way when there isn't
  // the room, each part shortening as it's made to (FootItem's shrink).
  start: ReactNode;
  // At its end: what keeps its room, however short the foot is of it.
  end?: ReactNode;
  className?: string;
};

// Foot is the line along the bottom of the window, the whole width of
// it: what's going on now, in parts (FootItem, FootCrew), each a way to
// the rest of it. It's a window on to the work, for whoever's curious,
// and the one part of the chrome that's always there.
export function Foot({ start, end, className }: FootProps) {
  return (
    <footer aria-label="What's going on" className={cx(css.foot, className)}>
      <div className={css.start}>{start}</div>
      {end && <div className={css.end}>{end}</div>}
    </footer>
  );
}
