import type { ReactNode } from "react";
import css from "./MissionBriefOutlineList.module.css";

// MissionBriefOutlineList is a document's sections in the brief's outline:
// a MissionBriefOutlineItem each.
export function MissionBriefOutlineList({ children }: { children: ReactNode }) {
  return <ul className={css.list}>{children}</ul>;
}
