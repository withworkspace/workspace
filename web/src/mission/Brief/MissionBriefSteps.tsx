import type { MouseEvent, ReactNode } from "react";
import css from "./MissionBriefSteps.module.css";

// MissionBriefSteps holds the plan's steps in the brief's outline, as the
// mission's details list them: a row each, the one it's at too. A row
// pressed goes to its step (onClick hears it: the list is the pointer's).
export function MissionBriefSteps({ onClick, children }: { onClick: (e: MouseEvent) => void; children: ReactNode }) {
  return (
    <div className={css.steps} onClick={onClick}>
      {children}
    </div>
  );
}
