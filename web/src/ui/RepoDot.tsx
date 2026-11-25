import { cx } from "./cx";
import css from "./RepoDot.module.css";

// RepoDot is a repo's colour, as ws labels it, beside its name.
export function RepoDot({ color, className }: { color?: string; className?: string }) {
  return <span className={cx(css.dot, className)} style={{ background: color }} />;
}
