import { reviewKeys } from "./keys";
import { useSaid } from "../keymap/keymap";
import type { ReactNode } from "react";
import { cx } from "../ui/cx";
import { Icon, type IconName } from "../ui/Icon";
import { keyProps } from "../ui/keyHint";
import css from "./ReviewViewRow.module.css";

// ReviewViewRow is one of a review's views down the side: its icon and
// name, how it stands (each pass's mark), how much is in it, a dot while
// there's something new in it, and tools of its own (the Changes view's).
// Its key (1 for the first) shows it.
export function ReviewViewRow({
  icon,
  label,
  n,
  here,
  stands,
  count,
  news,
  tools,
  onPick,
}: {
  icon: IconName;
  label: string;
  n: number;
  here: boolean;
  stands?: ReactNode;
  count?: ReactNode;
  news?: string; // what's new in it since you looked
  tools?: ReactNode;
  onPick: () => void;
}) {
  const said = useSaid(viewCommands[n - 1]);
  return (
    <li className={cx(css.row, here && css.here)} {...keyProps(said, `${label}${news ? `: ${news}` : ""}`, onPick)} aria-current={here ? "page" : undefined}>
      <span className={css.mark}>
        <Icon name={icon} size={11} />
      </span>
      <span className={css.name}>{label}</span>
      {stands && <span className={css.stands}>{stands}</span>}
      {count ? <span className={css.count}>{count}</span> : null}
      {news && <span className={css.news} aria-label={news} />}
      {tools && (
        <span className={css.tools} onClick={(e) => e.stopPropagation()}>
          {tools}
        </span>
      )}
    </li>
  );
}

// The command that shows each view, by where it is in the list.
const viewCommands = [reviewKeys.view1, reviewKeys.view2, reviewKeys.view3, reviewKeys.view4, reviewKeys.view5];
