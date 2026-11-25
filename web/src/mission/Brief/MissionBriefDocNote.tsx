import type { ReactNode } from "react";
import { cx } from "../../ui/cx";
import css from "./MissionBriefDocNote.module.css";

// MissionBriefDocNote is a quiet line over a document in the Brief tab:
// a change that couldn't be shown in place, the form ws holds a plan to.
export function MissionBriefDocNote({ children }: { children: ReactNode }) {
  return <p className={cx("dim", css.note)}>{children}</p>;
}
