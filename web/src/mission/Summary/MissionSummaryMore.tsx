import type { ReactNode } from "react";
import { MoreLink } from "../../ui/MoreLink";
import css from "./MissionSummaryMore.module.css";

// MissionSummaryMore leads from what a row of a mission's Summary shows to
// the rest of it, in another tab (the brief, the whole log).
export function MissionSummaryMore({ onClick, children }: { onClick: () => void; children: ReactNode }) {
  return (
    <MoreLink className={css.more} onClick={onClick}>
      {children}
    </MoreLink>
  );
}
