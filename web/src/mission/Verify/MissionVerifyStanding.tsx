import type { ReactNode } from "react";
import { cx } from "../../ui/cx";
import { toneClass } from "../../ui/tone";
import css from "./MissionVerifyStanding.module.css";

const tones = { attention: css.attention, done: css.done };

// MissionVerifyStanding is where a bundle stands, in a few quiet words:
// in the attention tone when it waits on you (or is stale, or was sent
// back), in done's once you've approved it.
export function MissionVerifyStanding({ tone, children }: { tone?: "attention" | "done"; children: ReactNode }) {
  return <span className={cx(css.standing, tone && toneClass(tone), tone && tones[tone])}>{children}</span>;
}
