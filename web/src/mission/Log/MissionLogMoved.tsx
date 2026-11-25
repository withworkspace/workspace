import type { ReactNode } from "react";
import { cx } from "../../ui/cx";
import css from "./MissionLogMoved.module.css";

// MissionLogMoved says, under the log, where its crew are: on Work, with
// the capsule each worked in, and which are here because they worked in
// none.
export function MissionLogMoved({ children }: { children: ReactNode }) {
  return <p className={cx(css.moved, "dim")}>{children}</p>;
}
