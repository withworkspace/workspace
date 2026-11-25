import type { ReactNode } from "react";
import css from "./MissionVerifyTally.module.css";

// MissionVerifyTally is what a bundle's checks came to, a chip for each
// verdict (its children), and, after them, the bundle it took the place
// of (replaces).
export function MissionVerifyTally({ children, replaces }: { children: ReactNode; replaces?: ReactNode }) {
  return (
    <div className={css.tally}>
      {children}
      {replaces && <span className={css.replaces}>{replaces}</span>}
    </div>
  );
}
