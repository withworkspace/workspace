import type { ReactNode } from "react";
import { cx } from "../../ui/cx";
import css from "./MissionLeadRequest.module.css";

// MissionLeadRequest is what was asked for, under a mission's lead while
// it's to be briefed.
export function MissionLeadRequest({ children }: { children: ReactNode }) {
  return <div className={cx("dim", css.request)}>{children}</div>;
}
