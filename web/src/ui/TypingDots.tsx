import css from "./TypingDots.module.css";
import { cx } from "./cx";

// TypingDots are three dots that pulse in turn: an agent's at it, with
// nothing yet to show. They take the colour of the words they're beside.
export function TypingDots({ className }: { className?: string }) {
  return (
    <span className={cx(css.dots, className)} aria-hidden>
      <span />
      <span />
      <span />
    </span>
  );
}
