import type { ReactNode, Ref } from "react";
import { cx } from "../ui/cx";
import { Row, RowMain } from "../ui/Rows";
import { Time } from "../ui/Time";
import { RunName } from "./RunName";
import css from "./RunsPageRow.module.css";

// RunsPageRow is one of the Runs page's rows (a run, or a silo): its mark
// (a RunMark), its name (nameTitle on hover) and what's beside it (top),
// the quieter lines under (sub: RunsPageSubs, then a RunEnds), how long
// since it started (started), what's done to it at its end (actions), and,
// while it's open, what's under it (more: a RunsPageMore). An ended run
// has its name quieter.
export function RunsPageRow({
  rowRef,
  mark,
  name,
  nameTitle,
  top,
  sub,
  started,
  actions,
  open,
  ended,
  more,
}: {
  rowRef?: Ref<HTMLLIElement>;
  mark: ReactNode;
  name: string;
  nameTitle?: string;
  top?: ReactNode;
  sub?: ReactNode;
  started?: string;
  actions: ReactNode;
  open?: boolean;
  ended?: boolean;
  more?: ReactNode;
}) {
  return (
    <Row ref={rowRef} className={cx(css.row, ended && css.ended, open && css.open)}>
      <div className={css.line}>
        {mark}
        <RowMain>
          <span className={css.top}>
            <RunName className={css.name} title={nameTitle}>
              {name}
            </RunName>
            {top}
          </span>
          {sub}
        </RowMain>
        <span className={css.when}>{started && <Time iso={started} as="age" className={css.age} />}</span>
        <span className={css.actions}>{actions}</span>
      </div>
      {open && more}
    </Row>
  );
}
