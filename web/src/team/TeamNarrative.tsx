import type { ReactNode } from "react";
import { cx } from "../ui/cx";
import { Spinner } from "../ui/Spinner";
import css from "./TeamNarrative.module.css";
import { textClass } from "../ui/Text";

// TeamNarrative is a line over the Team page's rows of what's going on
// with them: what's new since you looked (TeamNarrativeNew), what's being
// read (TeamNarrativeReading), and what can be asked for; or that only a
// topic's changes are showing. dim: it's news in passing, not a filter.
export function TeamNarrative({ dim, children }: { dim?: boolean; children: ReactNode }) {
  return <p className={cx(css.narrative, dim && textClass("dim"))}>{children}</p>;
}

// TeamNarrativeNew is how many changes have something new since you looked.
export function TeamNarrativeNew({ children }: { children: ReactNode }) {
  return <span className={css.new}>{children}</span>;
}

// TeamNarrativeReading is changes being read by an agent now.
export function TeamNarrativeReading({ children }: { children: ReactNode }) {
  return (
    <span className={css.reading} title="An agent is reading each change and writing what it does">
      <Spinner size={12} /> {children}
    </span>
  );
}
