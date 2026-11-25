import type { ReactNode } from "react";
import { Link } from "react-router";
import { cx } from "../ui/cx";
import css from "./MissionCrewRow.module.css";

// MissionCrewRow is one of a mission's crew in a line: its badge, where it
// stands, and what it is (its harness and model) and where (its capsule),
// a link to its terminal while it's live, and to its transcript once it's
// ended; what it last said of itself under, and in a fuller list (Log)
// when it started and ended and what it was sent to do. In the details'
// narrow column (side), what it is takes a line of its own.
export function MissionCrewRow({
  id,
  badge,
  status,
  what,
  where,
  to,
  ended,
  note,
  times,
  dispatch,
  side,
}: {
  id: string;
  badge: ReactNode;
  status: ReactNode;
  what: ReactNode;
  // Its capsule.
  where?: string;
  // Its page: its terminal, or its transcript.
  to: string;
  ended?: boolean;
  note?: ReactNode;
  // When it started, and ended.
  times?: ReactNode;
  // What it was sent to do.
  dispatch?: ReactNode;
  side?: boolean;
}) {
  const body = (
    <>
      {badge}
      {status}
      <span className={css.what}>
        {what}
        {where && <span className="dim"> in {where}</span>}
      </span>
    </>
  );
  return (
    <div className={cx(css.row, side && css.side)}>
      <div className={css.line}>
        {ended ? body : (
          <Link to={to} className={css.link} title={`Open ${id}'s terminal`}>
            {body}
          </Link>
        )}
        {ended && (
          <Link to={to} className={css.transcript} title={`Read ${id}'s transcript`}>
            Transcript
          </Link>
        )}
      </div>
      {note && <div className={css.note}>{note}</div>}
      {times && <div className={css.times}>{times}</div>}
      {dispatch && (
        <div className={css.dispatch}>
          <span className="dim">Dispatched to </span>
          {dispatch}
        </div>
      )}
    </div>
  );
}
