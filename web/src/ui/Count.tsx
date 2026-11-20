import css from "./Count.module.css";
import { cx } from "./cx";
import { toneClass } from "./tone";

type CountProps = { tone?: "attention" } & React.ComponentProps<"span">;

// Count is how many, in a small pill beside what it counts: a heading's
// rows, a tab's things. In attention it's of things that need you.
export function Count({ tone, className, ...rest }: CountProps) {
  return <span className={cx(css.count, tone === "attention" && css.attention, toneClass(tone), className)} {...rest} />;
}
