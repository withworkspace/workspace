import type { ReactNode } from "react";
import { cx } from "../../ui/cx";
import { type Tone, toneClass } from "../../ui/tone";
import css from "./MissionVerifyJudged.module.css";

// MissionVerifyJudged is what ws's judge said of a check, in the tone of
// what it came to: who says what, in bold, then why.
export function MissionVerifyJudged({ tone, children }: { tone: Tone; children: ReactNode }) {
  return <p className={cx(css.judged, toneClass(tone))}>{children}</p>;
}
