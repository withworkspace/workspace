import type { ReactNode } from "react";
import { cx } from "../ui/cx";
import { Icon } from "../ui/Icon";
import { Time } from "../ui/Time";
import css from "./DocumentsActivityFold.module.css";
import { textClass } from "../ui/Text";

type DocumentsActivityFoldProps = {
  inFlight: number; // capsules with changes to its documents
  week: number; // changes this week
  latest?: { title: string; by: string; at: string }; // the latest change
  open: boolean;
  setOpen: (open: boolean) => void;
  children: ReactNode; // In flight and Changed lately, each a DocumentsActivitySection (flat)
};

// DocumentsActivityFold is what's happening in a space, folded to one
// line (how many in flight, how many changes this week, the latest) and
// open for what's in flight and what changed, side by side.
export function DocumentsActivityFold({ inFlight, week, latest, open, setOpen, children }: DocumentsActivityFoldProps) {
  return (
    <details className={css.fold} open={open} onToggle={(e) => setOpen(e.currentTarget.open)}>
      <summary>
        <Icon name="clock" size={14} className={textClass("dim")} />
        <span>
          {inFlight > 0 && (
            <>
              <strong>{inFlight}</strong> in flight
            </>
          )}
          {inFlight > 0 && week > 0 && " · "}
          {week > 0 && (
            <>
              <strong>{week}</strong> {week === 1 ? "change" : "changes"} this week
            </>
          )}
          {!inFlight && !week && "Nothing this week"}
        </span>
        {latest && (
          <span className={css.latest}>
            · latest: {latest.title}, {latest.by.split(" ")[0]} <Time iso={latest.at} as="age" /> ago
          </span>
        )}
        <Icon name="chevron-down" size={14} className={cx(textClass("dim"), css.caret)} />
      </summary>
      <div className={css.body}>{children}</div>
    </details>
  );
}
