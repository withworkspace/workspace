import type { ReactNode } from "react";
import { cx } from "./cx";
import css from "./TreeMark.module.css";

export type TreeTone = "done" | "attention" | "danger" | "accent" | "quiet";

// TreeMark is a letter or a small icon in a tone, on a row of a Tree: how
// a file's changed (A in done, M in attention, D in danger), before its
// icon as the row's mark, or among its treats at the end. Its title says
// it in words, to the pointer and to a screen reader.
export function TreeMark({ tone = "quiet", title, children }: { tone?: TreeTone; title: string; children: ReactNode }) {
  const tones: Record<TreeTone, string | undefined> = { done: css.done, attention: css.attention, danger: css.danger, accent: css.accent, quiet: css.quiet };
  return (
    <span className={cx(css.mark, tones[tone])} title={title} role="img" aria-label={title}>
      {children}
    </span>
  );
}
