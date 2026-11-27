import type { ComponentProps, ReactNode } from "react";
import { cx } from "../ui/cx";
import { Icon, type IconName } from "../ui/Icon";
import css from "./MissionPlanStep.module.css";
import { SrOnly } from "../ui/SrOnly";

type MissionPlanStepProps = Omit<ComponentProps<"li">, "title" | "children"> & {
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
  // What's under the rest: what its checks came to (a
  // MissionPlanCheckList), or, of a row, the step said in full
  // (MissionPlanNow).
  under?: ReactNode;
  // row: a row to point at, in a list of them (MissionPlan rows): its
  // name on one line, cut short, and its kind at the row's end.
  row?: boolean;
  // Of a row: what kind of step it is, as an icon that says so.
  kind?: { icon: IconName; says: string };
  // Of a row: a Work step's level (MissionPlanLevel), beside its kind.
  level?: ReactNode;
  // It's yours to pass: its kind is in the accent.
  yours?: boolean;
  // It was skipped: a dash for its mark, its name struck through.
  skipped?: boolean;
  // Of a row: it's opened, so what's under it says its kind and the rest.
  opened?: boolean;
  // Pressing it does something: the pointer says so.
  pressable?: boolean;
};

const states = { accepted: css.accepted, current: css.current, todo: css.todo };

// MissionPlanStep is one stage of a mission's plan: its mark on the line
// (a tick when it's done, a dot when it's the one the mission's on), its
// number and name, and what's to be known of it. On Summary it's a step
// with all of that under it; in a mission's details it's a row.
export function MissionPlanStep({ n, title, state, hint, children, why, under, row, kind, level, yours, skipped, opened, pressable, className, ...rest }: MissionPlanStepProps) {
  return (
    <li
      className={cx(css.step, states[state], row && css.row, yours && css.yours, skipped && css.skipped, opened && css.opened, pressable && css.pressable, className)}
      title={hint}
      data-plan-step={n}
      {...rest}
    >
      <span className={css.mark} aria-hidden>
        {skipped ? <Icon name="minus" size={10} /> : state === "accepted" ? <Icon name="check" size={10} /> : null}
      </span>
      <span className={css.body}>
        <span className={css.title}>
          <span className={css.n}>{n}</span> {title}
          <SrOnly>{skipped ? " (skipped)" : state === "accepted" ? " (done)" : state === "current" ? " (now)" : ""}</SrOnly>
        </span>
        {children && <span className={css.notes}>{children}</span>}
        {why && <span className={css.why}>{why}</span>}
        {under && <div className={css.under}>{under}</div>}
      </span>
      {(kind || level) && !opened && (
        <span className={css.end}>
          {level}
          {kind && (
            <span className={css.kind} title={kind.says}>
              <Icon name={kind.icon} size={12} />
            </span>
          )}
        </span>
      )}
    </li>
  );
}
