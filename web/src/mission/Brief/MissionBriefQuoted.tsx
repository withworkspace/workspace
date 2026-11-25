import type { ReactNode } from "react";
import css from "./MissionBriefQuoted.module.css";

// MissionBriefQuoted is the passage a turn of yours was about, over what
// you said of it: a few lines of it, set apart.
export function MissionBriefQuoted({ children }: { children: ReactNode }) {
  return <span className={css.quoted}>{children}</span>;
}
