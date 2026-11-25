import type { ReactNode } from "react";
import css from "./MissionWork.module.css";

// MissionWork is a mission's Work tab laid out: what's been made, a row
// each (list), and the one that's open beside them, in a card whose edge
// stays put while what's in it scrolls. On a wide window it fills what's
// under the mission's bar; narrower, the card goes under the list and the
// page scrolls. With nothing open there's no card.
export function MissionWork({ list, children }: { list: ReactNode; children?: ReactNode }) {
  return (
    <div className={css.work}>
      <div className={css.list}>{list}</div>
      <section className={css.pane} aria-live="polite">
        {children}
      </section>
    </div>
  );
}
