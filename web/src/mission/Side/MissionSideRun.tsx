import type { ReactNode } from "react";
import { Link } from "react-router";
import { Time } from "../../ui/Time";
import css from "./MissionSideRun.module.css";

// MissionSideRun is one of the orchestrator's runs on a mission, in its
// details: how it went, what it was run for, and how long ago.
export function MissionSideRun({ to, status, what, iso }: { to: string; status: ReactNode; what: string; iso: string }) {
  return (
    <Link to={to} className={css.run}>
      {status}
      <span className={css.what}>on {what}</span>
      <Time iso={iso} as="age" className={css.age} />
    </Link>
  );
}
