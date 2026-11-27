import type { ReactNode } from "react";
import type { Verdict } from "../api";
import { cx } from "../ui/cx";
import { Icon } from "../ui/Icon";
import { hintOnClick } from "../ui/keyHint";
import { Keys } from "../ui/Keys";
import type { Command } from "../keymap/commands";
import { saidOf, useKeymap } from "../keymap/keymap";
import { verdictKeys } from "./keys";
import css from "./ReviewVerdicts.module.css";

const verdicts: [Verdict, Command, string, "check-circle" | "alert" | "message"][] = [
  ["approve", verdictKeys.approve, "Approve", "check-circle"],
  ["changes", verdictKeys.changes, "Request changes", "alert"],
  ["comment", verdictKeys.comment, "Comment", "message"],
];

const verdictClass: Record<Verdict, string> = { approve: css.approve, changes: css.changes, comment: css.comment };

// ReviewVerdicts are the verdicts a review's submitted with, one to pick:
// Approve (a), Request changes (c) or Comment (m), unless you've set other
// keys (keys.ts, verdictKeys). keyed says their keys;
// unavailable is why one can't be picked, where it can't. What's said
// after them (after) sits in their row.
export function ReviewVerdicts({
  label,
  value,
  keyed,
  unavailable,
  after,
  onPick,
}: {
  label: string;
  value?: Verdict;
  keyed: boolean;
  unavailable?: (v: Verdict) => string | undefined;
  after?: ReactNode;
  onPick: (v: Verdict) => void;
}) {
  const map = useKeymap();
  return (
    <div className={css.verdicts} role="radiogroup" aria-label={label}>
      {verdicts.map(([v, command, words, icon]) => {
        const why = unavailable?.(v);
        const key = saidOf(command, map);
        return (
          <button key={v} role="radio" aria-checked={value === v} disabled={!!why} title={why} className={cx(css.verdict, verdictClass[v], value === v && css.on)} onClick={hintOnClick(key, () => onPick(v))}>
            <Icon name={icon} size={14} /> {words} {keyed && key && <Keys k={key} />}
          </button>
        );
      })}
      {after}
    </div>
  );
}
