import type { ReactNode, Ref } from "react";
import css from "./MissionBriefMain.module.css";

// MissionBriefMain is the Brief tab's middle pane: the documents (a
// MissionBriefDocs) and, with the planner folded away, where it's asked
// at their foot (a MissionBriefAskFoot).
export function MissionBriefMain({ ref, children }: { ref?: Ref<HTMLDivElement>; children: ReactNode }) {
  return (
    <div className={css.main} ref={ref}>
      {children}
    </div>
  );
}
