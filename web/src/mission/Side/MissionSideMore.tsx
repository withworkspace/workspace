import type { ReactNode } from "react";
import { MoreLink } from "../../ui/MoreLink";
import css from "./MissionSideMore.module.css";

// MissionSideMore leads from what a part of a mission's details shows to
// the rest of it, in another tab ("3 more in Work").
export function MissionSideMore({ onClick, children }: { onClick: () => void; children: ReactNode }) {
  return (
    <MoreLink className={css.more} onClick={onClick}>
      {children}
    </MoreLink>
  );
}
