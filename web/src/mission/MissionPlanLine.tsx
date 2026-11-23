import type { ReactNode } from "react";
import { cx } from "../ui/cx";
import { Icon, type IconName } from "../ui/Icon";
import css from "./MissionPlanLine.module.css";

// MissionPlanLine is one thing said of the step the plan is on
// (MissionPlanNow): an icon and words in a row, quietly; in amber where
// something's amiss (warn), in the accent where it's yours. words: a
// sentence, which reads on as one and isn't laid out in a row.
export function MissionPlanLine({ icon, warn, yours, words, children }: { icon?: IconName; warn?: boolean; yours?: boolean; words?: boolean; children: ReactNode }) {
  return (
    <div className={cx(css.line, words && css.words, warn && css.warn, yours && css.yours)}>
      {icon && <Icon name={icon} size={12} />}
      {children}
    </div>
  );
}
