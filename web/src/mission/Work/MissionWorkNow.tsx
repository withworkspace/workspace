import type { ReactNode } from "react";
import { cx } from "../../ui/cx";
import { type Tone, toneClass } from "../../ui/tone";
import css from "./MissionWorkNow.module.css";

// MissionWorkNow is the step the mission's on, when it's in the capsule
// that's open: whose it is, in a line, and what moves it on (actions). In
// attention it's yours; in progress, someone's at it.
export function MissionWorkNow({ tone, children, actions }: { tone: Tone; children: ReactNode; actions?: ReactNode }) {
  return (
    <div className={cx(css.now, toneClass(tone))}>
      <p>{children}</p>
      {actions}
    </div>
  );
}
