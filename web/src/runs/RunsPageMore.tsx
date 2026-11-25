import type { ReactNode } from "react";
import css from "./RunsPageMore.module.css";

// RunsPageMore is what's under a row of the Runs page while it's open: its
// facts (a RunsPageFacts), the runs of a task (a RunsPagePast), and what
// it printed, which takes runsPageLogClass.
export function RunsPageMore({ children }: { children: ReactNode }) {
  return <div className={css.more}>{children}</div>;
}

// runsPageLogClass is the look of what a run printed, under its row: a
// RunOutput's or a Printed's.
export const runsPageLogClass = css.log;
