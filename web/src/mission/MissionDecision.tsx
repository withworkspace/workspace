import type { ReactNode } from "react";
import type { Decision } from "../api";
import { cx } from "../ui/cx";
import { Inline } from "../ui/inline";
import { Copyable } from "../ui/Copyable";
import { Time } from "../ui/time";
import { whoDecided } from "./decisions";
import css from "./MissionDecision.module.css";

// MissionDecision is one of a mission's decisions: what was decided, why,
// then its name (which copies), who made it, at which step, where, and
// when. One of yours has a rule beside it, as it stands over what the
// plan asks. One that no longer stands is struck, and says what became of
// it in place of why. What can be done to it (actions) is at the end of
// its first line, and what's open on it (children: replacing it,
// withdrawing it) under it.
export function MissionDecision({
  d,
  who,
  step,
  yours,
  marked,
  dense,
  actions,
  children,
}: {
  d: Decision;
  // Who made it, where a name isn't enough (crew's badge).
  who?: ReactNode;
  // The step it was made at, where the list it's in doesn't say.
  step?: number;
  yours?: boolean;
  // The one the page's address names.
  marked?: boolean;
  // In a summary: its words only as long as two lines, and no why.
  dense?: boolean;
  actions?: ReactNode;
  children?: ReactNode;
}) {
  const gone = d.state !== "current";
  return (
    <article className={cx(css.decision, yours && css.yours, marked && css.marked, gone && css.gone, dense && css.dense)} data-decision={d.id} aria-label={`Decision ${d.id}`}>
      <div className={css.line}>
        <div className={css.what}>
          <Inline text={d.what} lines={dense ? 2 : undefined} />
        </div>
        {actions && <div className={css.actions}>{actions}</div>}
      </div>
      {gone ? (
        <div className={css.why}>{d.state === "superseded" ? <Inline text={`Replaced by ${d.supersededBy}`} /> : <Inline text={`Withdrawn: ${d.withdrawnWhy}`} />}</div>
      ) : (
        d.why &&
        !dense && (
          <div className={css.why}>
            <Inline text={d.why} />
          </div>
        )
      )}
      <div className={css.meta}>
        <Copyable text={d.id} className="ref" />
        <span>{who ?? whoDecided(d.by)}</span>
        {!!step && <span>at step {step}</span>}
        {d.from && (
          <span>
            from <Inline text={d.from} />
          </span>
        )}
        <Time iso={d.at} as="relative" />
      </div>
      {children}
    </article>
  );
}
