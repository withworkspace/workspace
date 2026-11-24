import { cx } from "../ui/cx";
import { Icon, type IconName } from "../ui/Icon";
import css from "./PullRequestChecks.module.css";

export type PullRequestChecksState = "success" | "failure" | "pending";

const says: Record<PullRequestChecksState, { icon: IconName; words: string; look: string }> = {
  success: { icon: "check-circle", words: "Checks pass", look: css.success },
  failure: { icon: "x-circle", words: "Checks failing", look: css.failure },
  pending: { icon: "clock", words: "Checks running", look: css.pending },
};

// PullRequestChecks is what a pull request's checks came to, in words:
// passing, failing, or still running.
export function PullRequestChecks({ checks }: { checks: PullRequestChecksState }) {
  const s = says[checks];
  return (
    <span className={cx(css.checks, s.look)}>
      <Icon name={s.icon} size={13} />
      {s.words}
    </span>
  );
}
