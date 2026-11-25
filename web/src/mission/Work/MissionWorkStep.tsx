import type { ReactNode } from "react";
import { cx } from "../../ui/cx";
import { Icon } from "../../ui/Icon";
import css from "./MissionWorkStep.module.css";
import { SrOnly } from "../../ui/SrOnly";

const states = { accepted: css.accepted, current: css.current, todo: css.todo };

// MissionWorkStep is a step that ran in the capsule that's open, in a
// line: its mark (a tick when it's done, a dot when it's the one the
// mission's on), its number and name, then who's on it and the bundles
// that show it (children). hint says more of it when it's pointed at.
export function MissionWorkStep({ n, title, state, hint, children }: { n: number; title: ReactNode; state: "accepted" | "current" | "todo"; hint?: string; children?: ReactNode }) {
  return (
    <li className={cx(css.step, states[state])} title={hint}>
      <span className={css.mark} aria-hidden>
        {state === "accepted" && <Icon name="check" size={10} />}
      </span>
      <span className={css.n}>{n}</span>
      <span className={css.line}>
        <span className={css.title}>{title}</span>
        {children}
        <SrOnly>{state === "accepted" ? " (done)" : state === "current" ? " (now)" : ""}</SrOnly>
      </span>
    </li>
  );
}
