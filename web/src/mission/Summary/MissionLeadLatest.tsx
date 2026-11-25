import type { ReactNode } from "react";
import css from "./MissionLeadLatest.module.css";

// MissionLeadLatest is what a mission's crew last said, at the foot of its
// lead: a MissionLeadSaid for each who's said something and isn't at
// work.
export function MissionLeadLatest({ children }: { children: ReactNode }) {
  return <dl className={css.latest}>{children}</dl>;
}

// MissionLeadSaid is one crew's last word: who, beside what.
export function MissionLeadSaid({ who, children }: { who: ReactNode; children: ReactNode }) {
  return (
    <div className={css.said}>
      <dt>{who}</dt>
      <dd>{children}</dd>
    </div>
  );
}
