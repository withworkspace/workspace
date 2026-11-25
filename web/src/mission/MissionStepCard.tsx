import { Fragment, type ReactNode } from "react";
import { cx } from "../ui/cx";
import { Icon, type IconName } from "../ui/Icon";
import css from "./MissionStepCard.module.css";
import { Text } from "../ui/Text";

type MissionStepCardProps = {
  // What kind of step it is, and which it is of how many ("step 3 of 7").
  kind?: { icon: IconName; says: string };
  which: string;
  // How it stands, in words, and as what (for their colour).
  state: "accepted" | "current" | "todo";
  stands: ReactNode;
  title: ReactNode;
  // What it's to do.
  text?: ReactNode;
  // What's known of it: a label, and what it is.
  rows?: [string, ReactNode][];
  // Its checks: what they came to, in a line, then each (a
  // MissionPlanCheckList).
  checksSay?: ReactNode;
  checks?: ReactNode;
};

// MissionStepCard is what a hover card says of a step of a mission's
// plan: what there isn't room for in its row. Its kind and place, how it
// stands, its name and text, what's known of it, and its checks.
export function MissionStepCard({ kind, which, state, stands, title, text, rows = [], checksSay, checks }: MissionStepCardProps) {
  return (
    <>
      <div className={css.top}>
        {kind && <Icon name={kind.icon} size={13} />}
        <span>
          {kind ? `${kind.says} · ` : ""}
          {which}
        </span>
        <span className={cx(css.stands, state === "accepted" && css.accepted, state === "current" && css.current)}>{stands}</span>
      </div>
      <div>
        <div className={css.title}>{title}</div>
        {text && <div className={css.text}>{text}</div>}
      </div>
      {rows.length > 0 && (
        <dl className={css.rows}>
          {rows.map(([label, value]) => (
            <Fragment key={label}>
              <dt>{label}</dt>
              <dd>{value}</dd>
            </Fragment>
          ))}
        </dl>
      )}
      {checks && (
        <div className={css.checks}>
          <Text as="div" dim>{checksSay}</Text>
          {checks}
        </div>
      )}
    </>
  );
}

// missionStepCardClass is the card's own width and spacing, for the
// FloatingCard that holds it.
export function missionStepCardClass(): string {
  return css.card;
}
