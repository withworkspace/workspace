import type { ReactNode } from "react";
import { cx } from "../ui/cx";
import css from "./PageFindingQuote.module.css";

// PageFindingQuote is the page's words a finding is about: pressed, to go
// to them (onClick); lost, when they're not on the page any more.
export function PageFindingQuote({ lost, onClick, children }: { lost?: boolean; onClick?: () => void; children: ReactNode }) {
  if (onClick && !lost)
    return (
      <button className={css.quote} onClick={onClick} title="Go to text">
        {children}
      </button>
    );
  return (
    <span className={cx(css.quote, lost && css.lost)} title={lost ? "Text no longer on the page" : undefined}>
      {children}
    </span>
  );
}
