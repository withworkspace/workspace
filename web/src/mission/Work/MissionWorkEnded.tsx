import type { ReactNode } from "react";
import css from "./MissionWorkEnded.module.css";

// MissionWorkEnded is the capsules that ended alone (handed on, or
// dropped), under the mission's own in the Work tab's list.
export function MissionWorkEnded({ children }: { children: ReactNode }) {
  return (
    <div className={css.ended}>
      <h3>Ended alone</h3>
      {children}
    </div>
  );
}
