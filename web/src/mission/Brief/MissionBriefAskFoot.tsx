import type { ReactNode } from "react";
import { cx } from "../../ui/cx";
import css from "./MissionBriefAskFoot.module.css";

type MissionBriefAskFootProps = {
  // The documents are side by side: the field's a little wider.
  side?: boolean;
  // The planner's field.
  children: ReactNode;
};

// MissionBriefAskFoot is the planner's field at the documents' foot, when
// its conversation is folded away: over their last lines, which fade under
// it, as the ask bar is on a document. It says it's where the planner's
// asked (data-brief-ask), for the tab's keys.
export function MissionBriefAskFoot({ side, children }: MissionBriefAskFootProps) {
  return (
    <div className={cx(css.foot, side && css.side)} data-brief-ask>
      {children}
    </div>
  );
}
