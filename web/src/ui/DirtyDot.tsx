import { cx } from "./cx";
import css from "./DirtyDot.module.css";

// DirtyDot says something has changes that aren't committed: a file in a
// tree, a capsule in a list. Its title says so, to the pointer.
export function DirtyDot({ title = "Uncommitted changes", className }: { title?: string; className?: string }) {
  return <span className={cx(css.dot, className)} title={title} />;
}
