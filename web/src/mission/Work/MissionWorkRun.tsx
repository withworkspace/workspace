import type { ReactNode } from "react";
import css from "./MissionWorkRun.module.css";

// MissionWorkRun is one of a capsule's runs, in a line: its mark (a
// RunMark), its name and where it answers or what it came to (children),
// quiet lines under those (under), and what's done to it at its end
// (action: Stop).
export function MissionWorkRun({ mark, children, under, action }: { mark: ReactNode; children: ReactNode; under?: ReactNode; action?: ReactNode }) {
  return (
    <li className={css.run}>
      {mark}
      <span className={css.main}>
        <span className={css.top}>{children}</span>
        {under}
      </span>
      {action}
    </li>
  );
}
