import type { ReactNode } from "react";
import { cx } from "../ui/cx";
import { Icon, type IconName } from "../ui/Icon";
import css from "./MissionPlanNote.module.css";

// MissionPlanNote is one thing to be known of a step of the plan, under
// its name: who's on it when it isn't crew's, its clock, its checks,
// where it runs. yours is one that's yours to pass, in the accent; late,
// one that's run out or come up short, in amber. Pointed at, it says more.
export function MissionPlanNote({ icon, yours, late, title, children }: { icon: IconName; yours?: boolean; late?: boolean; title?: string; children: ReactNode }) {
  return (
    <span className={cx(css.note, yours && css.yours, late && css.late)} title={title}>
      <Icon name={icon} size={11} /> {children}
    </span>
  );
}
