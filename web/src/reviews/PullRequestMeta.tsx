import type { ReactNode } from "react";
import { chipClass } from "../ui/Chip";
import type { Tone } from "../ui/tone";
import css from "./PullRequestMeta.module.css";

// pullRequestMetaChipClass is a link in a pull request's meta line drawn
// as a chip (its mission, its capsule, a conversation about it).
export const pullRequestMetaChipClass = (tone: Tone) => chipClass(tone, css.chip);

// PullRequestMeta is where a pull request stands, under its review's
// head: a line of what's said of it (children), quiet, its names in the
// page's colour.
export function PullRequestMeta({ children }: { children: ReactNode }) {
  return <div className={css.meta}>{children}</div>;
}

// pullRequestMetaCodeClass is a branch's name in the line (as Text mono):
// in the words' colour, not quieter.
export const pullRequestMetaCodeClass: string = css.code;
