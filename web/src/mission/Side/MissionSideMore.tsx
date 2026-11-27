import type { KeyLike } from "../../keymap/keymap";
import type { ReactNode } from "react";
import { MoreLink } from "../../ui/MoreLink";
import css from "./MissionSideMore.module.css";

// MissionSideMore leads from what a part of a mission's details shows to
// the rest of it, in another tab ("3 more in Work").
// keys is the key that goes there too (the tab's number).
export function MissionSideMore({ onClick, children, keys }: { onClick: () => void; children: ReactNode; keys?: KeyLike }) {
  return (
    <MoreLink className={css.more} onClick={onClick} keys={keys}>
      {children}
    </MoreLink>
  );
}
