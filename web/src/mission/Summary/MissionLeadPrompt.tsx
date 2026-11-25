import type { ReactNode } from "react";
import css from "./MissionLeadPrompt.module.css";

// MissionLeadPrompt is what crew are stopped on, answered from the
// mission's lead: what they ask, and the way to their conversation.
export function MissionLeadPrompt({ children }: { children: ReactNode }) {
  return <div className={css.prompt}>{children}</div>;
}
