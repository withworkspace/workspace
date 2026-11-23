import css from "./Skeleton.module.css";
import { cx } from "./cx";

// How wide each row is, of the room: uneven, as lines of words are.
const widths = [92, 76, 84, 60, 88, 70];

// Skeleton is the shape of what's coming: rows of grey bars, with a
// sheen that passes over them.
export function Skeleton({ rows = 4, className }: { rows?: number; className?: string }) {
  return (
    <div className={cx(css.skeleton, className)} aria-hidden>
      {Array.from({ length: rows }, (_, i) => (
        <div key={i} className={css.row} style={{ width: `${widths[i % widths.length]}%` }} />
      ))}
    </div>
  );
}
