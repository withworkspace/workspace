import type { ReactNode } from "react";
import { Button } from "../../ui/Button";
import { cx } from "../../ui/cx";
import { Icon } from "../../ui/Icon";
import css from "./ArrangePanel.module.css";

type ArrangePanelProps = {
  // The sidebar's places, and those in the workspace's menu: an
  // ArrangeList each.
  sidebar: ReactNode;
  menu: ReactNode;
  // Taking your arrangement away, where you have one.
  onReset?: () => void;
  onCancel: () => void;
  onDone: () => void;
};

// ArrangePanel is the sidebar while it's arranged: the sidebar's list,
// the menu's under it, and Done under them.
export function ArrangePanel({ sidebar, menu, onReset, onCancel, onDone }: ArrangePanelProps) {
  return (
    // data-field: keys typed in it are its own, not the page's (keys.ts).
    <div className={css.panel} data-field>
      <div className={css.head}>Arrange the sidebar</div>
      {sidebar}
      <div className={cx(css.head, css.menuHead)}>
        <Icon name="chevron-down" size={12} />
        In the workspace's menu
      </div>
      {menu}
      <div className={css.actions}>
        {onReset && (
          <Button variant="ghost" size="small" onClick={onReset} title="Take your arrangement away: the team's, or as ws has it">
            Reset
          </Button>
        )}
        <span className={css.spacer} />
        <Button variant="ghost" size="small" onClick={onCancel}>
          Cancel
        </Button>
        <Button variant="primary" size="small" onClick={onDone}>
          Done
        </Button>
      </div>
    </div>
  );
}
