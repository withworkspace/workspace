import { cx } from "../ui/cx";
import css from "./TeamRepoDot.module.css";

// TeamRepoDot is a repo's colour, as a small square before its name.
export function TeamRepoDot({ color, className }: { color?: string; className?: string }) {
  return <span className={cx(css.dot, className)} style={{ background: color }} />;
}
