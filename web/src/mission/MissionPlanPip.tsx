import type { ComponentProps } from "react";
import { cx } from "../ui/cx";
import css from "./MissionPlanPip.module.css";

type MissionPlanPipProps = ComponentProps<"i"> & {
  state: "accepted" | "current" | "todo";
  // It passed with a check not met.
  warn?: boolean;
  // It's to come, and yours.
  yours?: boolean;
  // It waits on something outside.
  wait?: boolean;
};

// MissionPlanPip is a step of the plan as a dot, in a fold of them: done,
// passed with a check unmet, or to come (a ring; in the accent when it's
// yours; dotted when it waits on something outside). It can be pointed
// at, for its card.
export function MissionPlanPip({ state, warn, yours, wait, className, ...rest }: MissionPlanPipProps) {
  return <i className={cx(css.pip, state === "accepted" && css.accepted, warn && css.warn, yours && css.yours, wait && css.wait, className)} {...rest} />;
}
