import type { ReactNode } from "react";
import css from "./MissionVerifyPanes.module.css";

// MissionVerifyPanes is the open bundle in two columns: its head and its
// checks (side), and the check that's open, in a card whose edge stays
// put while the check scrolls in it. With no check open there's no card.
export function MissionVerifyPanes({ side, children }: { side: ReactNode; children?: ReactNode }) {
  return (
    <div className={css.panes}>
      <div className={css.side}>{side}</div>
      <div className={css.pane}>{children}</div>
    </div>
  );
}
