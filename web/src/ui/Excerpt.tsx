import { cx } from "./cx";
import css from "./Excerpt.module.css";

// Excerpt is a few lines of a file, numbered from the line they start
// at: the lines a comment's on, where they aren't showing beside it.
export function Excerpt({ lines, from = 1, title, className }: { lines: string[]; from?: number; title?: string; className?: string }) {
  return (
    <pre className={cx(css.excerpt, className)} title={title}>
      {lines.map((l, i) => (
        <div key={i}>
          <span className={css.ln}>{from + i}</span>
          {l || " "}
        </div>
      ))}
    </pre>
  );
}
