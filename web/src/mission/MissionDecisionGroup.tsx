import type { ReactNode } from "react";
import { Count } from "../ui/Count";
import { cx } from "../ui/cx";
import css from "./MissionDecisionGroup.module.css";

// MissionDecisionGroup is some of a mission's decisions under a heading:
// whose they are (yours, the work's), with how many and a line saying
// what that means for them, or (step) the step they were made at, a
// smaller heading in a group.
export function MissionDecisionGroup({
  title,
  count,
  note,
  step,
  end,
  children,
}: {
  title: ReactNode;
  count?: number;
  note?: ReactNode;
  step?: boolean;
  // At the heading's end: what's done to the group (a decision added).
  end?: ReactNode;
  children: ReactNode;
}) {
  const Head = step ? "h4" : "h3";
  return (
    <section className={cx(css.group, step && css.step)}>
      <div className={css.head}>
        <Head className={css.title}>
          {title}
          {count !== undefined && <Count>{count}</Count>}
        </Head>
        {note && <span className={css.note}>{note}</span>}
        {end && <span className={css.end}>{end}</span>}
      </div>
      <div className={css.list}>{children}</div>
    </section>
  );
}
