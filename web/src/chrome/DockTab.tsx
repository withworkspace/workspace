import type { ReactNode } from "react";
import { cx } from "../ui/cx";
import { Icon } from "../ui/Icon";
import css from "./DockTab.module.css";

type DockTabProps = {
  // What it's of: a conversation's title, "Details", "Results".
  name: string;
  // Whether it's the one showing.
  on: boolean;
  onPick: () => void;
  // Before its name: an icon for its kind, or how it stands (a Status).
  lead?: ReactNode;
  // How many it holds, where that's worth saying.
  count?: number;
  // Closing it, for a tab that's a thing you opened. A tab without is the
  // dock's own, and stays.
  onClose?: () => void;
  // It isn't kept yet, and why, in a few words after its name: a peek,
  // which the next thing peeked at takes the place of, or a conversation
  // that hasn't begun. Its name is slanted, as an editor's preview tab is.
  passing?: string;
};

// DockTab is one of the things open in a dock. Beside, a tab is a thing
// (a conversation, crew at work, a document), not a tool; below, it's a
// view of what's running.
export function DockTab({
  name,
  on,
  onPick,
  lead,
  count,
  onClose,
  passing,
}: DockTabProps) {
  return (
    <span className={cx(css.tab, on && css.on, !onClose && css.own, passing && css.passing)}>
      <button
        type="button"
        data-ui=""
        role="tab"
        aria-selected={on}
        tabIndex={on ? 0 : -1}
        title={passing ? `${name}: ${passing}` : name}
        className={css.pick}
        onClick={onPick}
      >
        {lead}
        <span className={css.name}>{name}</span>
        {count !== undefined && <span className={css.count}>{count}</span>}
      </button>
      {onClose && (
        <button
          type="button"
          data-ui=""
          tabIndex={on ? 0 : -1}
          aria-label={`Close ${name}`}
          title="Close"
          className={css.close}
          onClick={onClose}
        >
          <Icon name="x" size={10} />
        </button>
      )}
    </span>
  );
}
