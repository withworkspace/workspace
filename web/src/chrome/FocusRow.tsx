import type { ReactNode } from "react";
import { cx } from "../ui/cx";
import { Icon } from "../ui/Icon";
import css from "./FocusRow.module.css";
import { SidebarRow } from "./SidebarRow";

type FocusRowProps = Pick<
  React.HTMLAttributes<HTMLAnchorElement>,
  "onDragStart" | "onDragOver" | "onDragEnd" | "onContextMenu"
> & {
  to: string;
  // How the pinned thing stands (a Status), or its kind's icon.
  lead: ReactNode;
  title: string;
  says?: string;
  // The page you're on is its own.
  on?: boolean;
  onUnpin: () => void;
  // It's the row being dragged to another place in the list.
  dragging?: boolean;
  // What's being dragged would go before it, or after it.
  drop?: "before" | "after";
};

// FocusRow is a pin: how the thing stands, as an icon, and what it's
// called. Its ✕, which unpins it, is over the row's end while it's
// pointed at, so showing it moves nothing. It's dragged to put the pins
// in order: where it'd go is a line.
export function FocusRow({
  to,
  lead,
  title,
  says,
  on,
  onUnpin,
  dragging,
  drop,
  ...its
}: FocusRowProps) {
  return (
    <SidebarRow
      to={to}
      lead={lead}
      title={title}
      says={says}
      on={on}
      draggable
      className={cx(
        css.pin,
        dragging && css.dragging,
        drop === "before" && css.before,
        drop === "after" && css.after,
      )}
      end={
        <button
          type="button"
          data-ui=""
          title="Unpin"
          aria-label={`Unpin ${title}`}
          className={css.unpin}
          onClick={(e) => {
            e.preventDefault();
            e.stopPropagation();
            onUnpin();
          }}
        >
          <Icon name="x" size={12} />
        </button>
      }
      {...its}
    />
  );
}
