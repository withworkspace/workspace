import type { ReactNode } from "react";
import { Link } from "react-router";
import css from "./BelowRunsHead.module.css";

type BelowRunsHeadProps = {
  // Whose services they are: the place here (its name), or everywhere's.
  of?: string;
  // How many are running.
  count: number;
  // Starting another, where there's one to start (BelowRunsStart).
  start?: ReactNode;
};

// BelowRunsHead is the line over Below's Runs: whose services are listed
// and how many run, a way to start another of the place's, and the way
// to all of them, on the Runs page.
export function BelowRunsHead({ of, count, start }: BelowRunsHeadProps) {
  return (
    <div className={css.head}>
      <span className={css.said}>
        {count === 0 ? "Nothing running" : `${count} running`}
        {of ? (
          <>
            {" in "}
            <b className={css.of}>{of}</b>
          </>
        ) : (
          " in the workspace"
        )}
      </span>
      {start}
      <Link to="/runs" className={css.all}>
        All runs
      </Link>
    </div>
  );
}
