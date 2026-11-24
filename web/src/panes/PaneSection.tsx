import type { ReactNode } from "react";
import { Count } from "../ui/Count";
import css from "./PaneSection.module.css";

type PaneSectionProps = {
  // What it holds, in a word or two: Plan, Crew, Capsules.
  title: string;
  // How many, where that's worth saying.
  count?: ReactNode;
  children: ReactNode;
};

// PaneSection is one part of what a pane says of its thing, under a quiet
// label: a mission's plan, its crew, a pull request's stack. A pane is
// read top to bottom, so its sections are only set apart, not boxed.
export function PaneSection({ title, count, children }: PaneSectionProps) {
  return (
    <div className={css.section} role="group" aria-label={title}>
      <h4 className={css.title}>
        {title}
        {count !== undefined && <Count>{count}</Count>}
      </h4>
      {children}
    </div>
  );
}
