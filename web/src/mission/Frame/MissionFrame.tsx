import type { ReactNode } from "react";
import { cx } from "../../ui/cx";
import css from "./MissionFrame.module.css";

// MissionFrame is a mission's page: its timeline and its bar (the PageBar,
// given missionFrameBarClass), then the tab under them. A tab that fills
// the page (Brief, Work, Verify) has it to the window's edges, each of its
// panes scrolling by itself; the rest are read in a column. It says how
// tall the bar is (--mission-bar-h), for what sticks under it.
export function MissionFrame({ fills, children }: { fills?: boolean; children: ReactNode }) {
  return <div className={cx(css.frame, fills && css.fills)}>{children}</div>;
}

// missionFrameBarClass is the mission's PageBar's, in its frame.
export const missionFrameBarClass = css.bar;

// MissionFrameBody is the page under the bar, but for Brief's: the tab's
// column (MissionFrameMain), with the details beside it (side), and as
// wide as the window where the tab is somewhere to work (wide).
export function MissionFrameBody({ side, wide, children }: { side?: boolean; wide?: boolean; children: ReactNode }) {
  return <div className={cx("page", css.body, side && css.withSide, wide && css.wide)}>{children}</div>;
}

// MissionFrameMain is the tab's column: the page's scroller.
export function MissionFrameMain({ children }: { children: ReactNode }) {
  return (
    <div className={css.main} data-page-scroller="">
      {children}
    </div>
  );
}

// MissionFramePanel is what a tab shows, as the tabs' panel (id is what
// the Tabs say they control). brief: it's the Brief tab's, which has the
// page under the bar to itself.
export function MissionFramePanel({ id, label, brief, children }: { id: string; label: string; brief?: boolean; children: ReactNode }) {
  return (
    <div role="tabpanel" id={id} aria-label={label} className={cx(css.panel, brief && css.briefPanel)}>
      {children}
    </div>
  );
}
