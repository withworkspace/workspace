import type { ReactNode } from "react";
import { Chip } from "../../ui/Chip";
import { Count } from "../../ui/Count";
import { cx } from "../../ui/cx";
import css from "./MissionBriefOutlineSection.module.css";

type MissionBriefOutlineSectionProps = {
  title: ReactNode;
  // What it is, where that's drawn apart: the changes to read and the step
  // the mission's at are cards; what was requested is cut short.
  kind?: "changes" | "now" | "requested";
  // How many changes are to read (the changes' own).
  count?: number;
  // Which of them is in hand, at the heading's end (the changes' own).
  at?: ReactNode;
  // It's new with this draft (the plan's).
  fresh?: boolean;
  children: ReactNode;
};

const kinds = { changes: css.changes, now: css.now, requested: css.requested };

// MissionBriefOutlineSection is one section of the brief's outline: a
// small heading over what's listed under it.
export function MissionBriefOutlineSection({ title, kind, count, at, fresh, children }: MissionBriefOutlineSectionProps) {
  return (
    <section className={cx(css.section, kind && kinds[kind])}>
      <h3>
        {title}
        {count !== undefined && <Count tone="attention" className={css.tag}>{count}</Count>}
        {fresh && <Chip tone="quiet" className={css.tag}>new</Chip>}
        {kind === "changes" && <span className={css.spacer} />}
        {at && <span className={css.at}>{at}</span>}
      </h3>
      {children}
    </section>
  );
}
