import type { ReactNode } from "react";
import { MoreLink } from "../../ui/MoreLink";
import css from "./MissionSummaryMore.module.css";

// MissionSummaryMore leads from what a row of a mission's Summary shows to
// the rest of it, in another tab (the brief, the whole log).
// keys is the key that goes there too (the tab's number).
export function MissionSummaryMore({ onClick, children, keys }: { onClick: () => void; children: ReactNode; keys?: string }) {
  return (
    <MoreLink className={css.more} onClick={onClick} keys={keys}>
      {children}
    </MoreLink>
  );
}
