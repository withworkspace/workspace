import type { ReactNode } from "react";
import css from "./MissionBriefBar.module.css";

// MissionBriefBar is the one bar over the Brief tab's panes: where the
// brief and plan stand (a MissionBriefWhat), the editor's toolbar (a
// MissionBriefToolbar) and what there is to confirm (MissionBriefActs).
// On a wide window they're the panes' columns.
export function MissionBriefBar({ children }: { children: ReactNode }) {
  return (
    <div className={css.bar} role="region" aria-label="What's to confirm">
      {children}
    </div>
  );
}
