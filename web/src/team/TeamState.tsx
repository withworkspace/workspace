import type { ReactNode } from "react";
import { cx } from "../ui/cx";
import { type Tone, toneClass } from "../ui/tone";
import css from "./TeamState.module.css";

// TeamState is where a change stands now, in words, at the end of its
// line.
export function TeamState({ tone, children }: { tone: Tone; children: ReactNode }) {
  return <span className={cx(css.state, toneClass(tone))}>{children}</span>;
}
