import type { ReactNode } from "react";
import { cx } from "../ui/cx";
import { Inline } from "../ui/Inline";
import css from "./PassFinding.module.css";

// PassFinding is one thing a pass found, in the Passes view: its kind,
// where it is (go, a link to it), what's said of it and what's done with
// it (end) in its head; the lines it's on, where they aren't in the diff
// (lines); then what it found. One that blocks has a red rule; one that's
// settled a quiet one, and its words are quiet too.
export function PassFinding({ kind, where, said, end, lines, text, blocks, settled }: { kind: ReactNode; where: ReactNode; said?: ReactNode; end: ReactNode; lines?: ReactNode; text: string; blocks?: boolean; settled?: boolean }) {
  return (
    <li className={cx(css.finding, blocks && css.blocks, settled && css.settled)}>
      <div className={css.head}>
        {kind}
        {where}
        {said}
        <span className={css.spacer} />
        {end}
      </div>
      {lines}
      <div className={cx(css.text, settled && "dim")}>
        <Inline text={text} />
      </div>
    </li>
  );
}

