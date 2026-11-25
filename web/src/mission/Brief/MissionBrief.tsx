import type { ReactNode } from "react";
import { cx } from "../../ui/cx";
import css from "./MissionBrief.module.css";

type MissionBriefProps = {
  // What's over the panes: where it stands, the toolbar, what's to confirm.
  bar?: ReactNode;
  // The planner's conversation is a third pane, beside the documents.
  planner?: boolean;
  // The panes: the outline, the documents (or the conversation, in their
  // place), then the planner.
  children: ReactNode;
};

// MissionBrief is a mission's Brief tab: one bar over its panes. On a wide
// window it has the page under the mission's bar to itself, each pane
// scrolling by itself; on a narrow one it's the documents, then the
// conversation, down the page.
export function MissionBrief({ bar, planner, children }: MissionBriefProps) {
  return (
    <div className={cx(css.tab, planner && css.planner)}>
      {bar}
      <div className={css.panes}>{children}</div>
    </div>
  );
}
