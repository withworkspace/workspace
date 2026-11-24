import { cx } from "./cx";
import css from "./TreeDot.module.css";
import type { TreeTone } from "./TreeMark";

// TreeDot is a dot in a tone among a row's treats: something's in this
// folder (a change, a comment), or this file isn't committed. Its title
// says what, to the pointer and to a screen reader.
export function TreeDot({ tone = "attention", title }: { tone?: TreeTone; title: string }) {
  const tones: Record<TreeTone, string | undefined> = { done: css.done, attention: css.attention, danger: css.danger, accent: css.accent, quiet: css.quiet };
  return <span className={cx(css.dot, tones[tone])} title={title} role="img" aria-label={title} />;
}
