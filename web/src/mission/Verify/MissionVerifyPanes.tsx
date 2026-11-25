import type { ReactNode } from "react";
import { cx } from "../../ui/cx";
import css from "./MissionVerifyPanes.module.css";

// MissionVerifyPanes is the open bundle in two columns: its head and its
// checks (side), and the check that's open, in a card whose edge stays
// put while the check scrolls in it. With no check open there's no card.
// fills is the tab filling what's under the mission's bar, as the mission
// page's does: only then do the two scroll by themselves.
export function MissionVerifyPanes({ side, children, fills }: { side: ReactNode; children?: ReactNode; fills?: boolean }) {
  return (
    <div className={cx(css.panes, fills && css.fills)}>
      <div className={css.side}>{side}</div>
      <div className={css.pane}>{children}</div>
    </div>
  );
}
