import type { ReactNode } from "react";
import { Count } from "../../ui/Count";
import { cx } from "../../ui/cx";
import css from "./MissionBriefOutlineItem.module.css";

type MissionBriefOutlineItemProps = {
  // The section's heading.
  children: ReactNode;
  // How many changes are to read under it.
  count?: number;
  // It's the one being read.
  here?: boolean;
  onGo: () => void;
};

// MissionBriefOutlineItem is a section of the brief or the plan in the
// outline: it goes to the section, or to the first change to read in it,
// with how many there are at its end. The one being read has a tick in
// the margin.
export function MissionBriefOutlineItem({ children, count = 0, here, onGo }: MissionBriefOutlineItemProps) {
  return (
    <li>
      <button type="button" className={cx(css.item, here && css.here)} aria-current={here || undefined} onClick={onGo}>
        <span>{children}</span>
        {count > 0 && (
          <Count tone="attention" title={`${count} to review`}>
            {count}
          </Count>
        )}
      </button>
    </li>
  );
}
