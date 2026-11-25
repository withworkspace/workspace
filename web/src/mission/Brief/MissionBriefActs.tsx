import type { ReactNode } from "react";
import css from "./MissionBriefActs.module.css";

// MissionBriefActs is what there is to do, at the end of the tab's bar:
// the changes to step through, your edits to discard, a proposal to
// decline, and the one thing that confirms, accepts or saves.
export function MissionBriefActs({ children }: { children: ReactNode }) {
  return <div className={css.act}>{children}</div>;
}
