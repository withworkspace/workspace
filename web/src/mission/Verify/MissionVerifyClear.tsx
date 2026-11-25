import type { ReactNode } from "react";
import { Button } from "../../ui/Button";
import { cx } from "../../ui/cx";
import { Empty } from "../../ui/Empty";
import { Keys } from "../../ui/Keys";
import { MoreLink } from "../../ui/MoreLink";
import css from "./MissionVerifyClear.module.css";

type MissionVerifyClearProps = {
  // The bundle that's clear.
  id: string;
  // What still stands as it was, in a sentence.
  children: ReactNode;
  // What approving it does.
  after: ReactNode;
  busy?: boolean;
  onApprove: () => void;
  // Back to its checks.
  onAgain: () => void;
  // In a tab that fills what's under the mission's bar: it fills its pane.
  fills?: boolean;
};

// MissionVerifyClear is the check's pane once the last check left for you
// is approved: that you're clear, what still stands as it was, and the
// bundle's approval at a size you can't miss.
export function MissionVerifyClear({ id, children, after, busy, onApprove, onAgain, fills }: MissionVerifyClearProps) {
  return (
    <article className={cx(css.clear, fills && css.fills)} aria-label="Nothing's left for you">
      <Empty
        big
        done
        icon="check"
        title="You're all clear"
        action={
          <Button variant="primary" className={css.big} disabled={busy} onClick={onApprove} title="Approve it, with the checks as you've approved them" keys="mod+⏎">
            Approve {id} <Keys k="mod+⏎" />
          </Button>
        }
        foot={
          <>
            <div className={css.after}>{after}</div>
            <MoreLink onClick={onAgain}>Look again</MoreLink>
          </>
        }
      >
        {children}
      </Empty>
    </article>
  );
}
