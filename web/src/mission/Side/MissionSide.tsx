import { missionKeys } from "../keys";
import { useSaid } from "../../keymap/keymap";
import type { ReactNode } from "react";
import { cx } from "../../ui/cx";
import { Keys } from "../../ui/Keys";
import css from "./MissionSide.module.css";

// MissionSide is a mission's details: a column beside its Summary, or
// (over) a card over another tab that slides in from the right when open,
// there all along so it can slide out again.
export function MissionSide({ over, open, children }: { over?: boolean; open?: boolean; children: ReactNode }) {
  const detailsKey = useSaid(missionKeys.details);
  return (
    <aside className={cx(css.side, over && css.over, over && open && css.open)} aria-label="Details" inert={over && !open}>
      <div className={css.head}>
        <span>Details</span>
        {detailsKey && <Keys k={detailsKey} />}
      </div>
      {children}
    </aside>
  );
}
