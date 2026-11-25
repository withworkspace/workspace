import type { ReactNode } from "react";
import { cx } from "../../ui/cx";
import { Time } from "../../ui/Time";
import css from "./MissionLogEvents.module.css";

// MissionLogEvents is a stretch of a mission's log: a list of
// MissionLogEvents, and where it spans days, a MissionLogDay over each
// day's.
export function MissionLogEvents({ children }: { children: ReactNode }) {
  return <ol className={css.events}>{children}</ol>;
}

// MissionLogDay is a day's name over its events, which stays under the
// mission's bar as they scroll by.
export function MissionLogDay({ children }: { children: ReactNode }) {
  return (
    <li className={css.day} aria-hidden>
      {children}
    </li>
  );
}

// MissionLogEvent is one thing that happened: who (actor, then their name
// in words where it isn't a crew's badge), what, and when on the right.
// What ws did itself is quiet.
export function MissionLogEvent({
  actor,
  who,
  title,
  quiet,
  iso,
  as,
  children,
}: {
  actor: ReactNode;
  who?: string;
  // The words replaced in what it says, on hover.
  title?: string;
  quiet?: boolean;
  iso: string;
  as: "clock" | "stamp";
  children: ReactNode;
}) {
  return (
    <li className={cx(css.event, quiet && css.quiet)}>
      {actor}
      <span className={css.text} title={title}>
        {who && <span className={css.who}>{who} </span>}
        {children}
      </span>
      <Time iso={iso} as={as} className={css.time} />
    </li>
  );
}
