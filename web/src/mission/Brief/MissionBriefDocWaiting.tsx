import type { ReactNode } from "react";
import { cx } from "../../ui/cx";
import css from "./MissionBriefDocWaiting.module.css";

// MissionBriefDocWaiting is what's said in a document's place while
// there's nothing in it: no plan yet, and who writes one.
export function MissionBriefDocWaiting({ children }: { children: ReactNode }) {
  return <div className={cx(css.waiting, "dim")}>{children}</div>;
}
