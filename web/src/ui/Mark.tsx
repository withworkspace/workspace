import css from "./Mark.module.css";
import { cx } from "./cx";

type MarkProps = {
  // What's new, said to someone pointing at it or reading the page aloud.
  title?: string;
  // Over the corner of an icon: ringed in what's behind it, so it reads as
  // on the icon, not beside it.
  ringed?: boolean;
  // Something failing, not only new.
  tone?: "danger";
  className?: string;
};

// Mark is a dot on something that has news you haven't seen: a place in
// the sidebar, a row, the bell. It's the accent, and never a number: amber
// and a count are for what needs you.
export function Mark({ title, ringed, tone, className }: MarkProps) {
  return <span className={cx(css.mark, ringed && css.ringed, tone === "danger" && css.danger, className)} title={title} role={title ? "img" : undefined} aria-label={title} aria-hidden={title ? undefined : true} />;
}
