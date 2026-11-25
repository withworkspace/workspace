import type { ReactNode } from "react";
import css from "./MissionBriefTalk.module.css";

// MissionBriefTalk is the Brief tab's middle pane while nothing's drafted:
// the planner's conversation in the documents' place, or where it's begun
// (a MissionBriefStart).
export function MissionBriefTalk({ children }: { children: ReactNode }) {
  return <div className={css.talk}>{children}</div>;
}
