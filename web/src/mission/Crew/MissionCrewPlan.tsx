import type { ReactNode } from "react";
import { cx } from "../../ui/cx";
import { Icon } from "../../ui/Icon";
import css from "./MissionCrewPlan.module.css";

type MissionCrewPlanProps = {
  // How many of its items are done, of how many.
  done: number;
  of: number;
  // The item it's on, said while the plan's folded.
  now?: string;
  // Folded to its line (it's drawn open).
  folded?: boolean;
  // Its items (Todos).
  children: ReactNode;
};

// MissionCrewPlan is crew's latest plan, pinned on top of its
// conversation: how far through it is, and (opened) each item.
export function MissionCrewPlan({ done, of, now, folded, children }: MissionCrewPlanProps) {
  return (
    <details className={css.plan} open={!folded}>
      <summary>
        <Icon name="chevron-right" size={12} className={css.chevron} />
        <span className={css.title}>Plan</span>
        <span className={css.progress} aria-hidden>
          <span style={{ width: `${(100 * done) / of}%` }} />
        </span>
        <span className={cx("dim", css.count)}>
          {done} of {of}
        </span>
        {now && <span className={css.now}>{now}</span>}
      </summary>
      {children}
    </details>
  );
}
