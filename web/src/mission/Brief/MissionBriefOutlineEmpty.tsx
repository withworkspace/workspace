import type { ReactNode } from "react";
import css from "./MissionBriefOutlineEmpty.module.css";

// MissionBriefOutlineEmpty is what a section of the brief's outline says
// while there's nothing to list in it.
export function MissionBriefOutlineEmpty({ children }: { children: ReactNode }) {
  return <p className={css.empty}>{children}</p>;
}
