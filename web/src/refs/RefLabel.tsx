import type { ReactNode } from "react";
import { cx } from "../ui/cx";
import { type Tone, toneClass } from "../ui/tone";
import css from "./RefLabel.module.css";

// RefLabel is what's said after a name in what's read: where a mission
// stands, a page's name. It isn't the text's own words: a reader quoting
// it leaves it out. It has its look in markdown.
export function RefLabel({ tone, children }: { tone?: Tone; children: ReactNode }) {
  return (
    <span className={cx(css.label, toneClass(tone))} data-skip="">
      {children}
    </span>
  );
}
