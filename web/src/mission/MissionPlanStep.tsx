import type { ReactNode } from "react";
import { cx } from "../ui/cx";
import { Icon } from "../ui/Icon";
import css from "./MissionPlanStep.module.css";

type MissionPlanStepProps = {
  n: number;
  title: ReactNode;
  // How it stands: done, the one the mission's on, or to come.
  state: "accepted" | "current" | "todo";
  // What it's to do, said when it's pointed at.
  hint?: string;
  // What's to be known of it, under its name: MissionPlanNotes, crew's
  // badges.
  children?: ReactNode;
  // Why it stands as it does, in a line.
  why?: ReactNode;
  // What its checks came to, under the rest (MissionPlanChecks).
  under?: ReactNode;
};

const states = { accepted: css.accepted, current: css.current, todo: css.todo };
const said = { accepted: " (done)", current: " (now)", todo: "" };

// MissionPlanStep is one stage of a mission's plan: its mark on the line
// (a tick when it's done, a dot when it's the one the mission's on), its
// number and name, and what's to be known of it.
export function MissionPlanStep({ n, title, state, hint, children, why, under }: MissionPlanStepProps) {
  return (
    <li className={cx(css.step, states[state])} title={hint}>
      <span className={css.mark} aria-hidden>
        {state === "accepted" ? <Icon name="check" size={10} /> : null}
      </span>
      <span className={css.body}>
        <span className={css.title}>
          <span className={css.n}>{n}</span> {title}
          <span className="sr-only">{said[state]}</span>
        </span>
        {children && <span className={css.notes}>{children}</span>}
        {why && <span className={css.why}>{why}</span>}
        {under}
      </span>
    </li>
  );
}
