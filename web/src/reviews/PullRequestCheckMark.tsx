import type { PR } from "../api";
import { cx } from "../ui/cx";
import { Icon, type IconName } from "../ui/Icon";
import css from "./PullRequestCheckMark.module.css";
import { SrOnly } from "../ui/SrOnly";

const says: Record<NonNullable<PR["checks"]>, { icon: IconName; words: string; look: string }> = {
  success: { icon: "check", words: "Checks pass", look: css.success },
  failure: { icon: "x", words: "Checks are failing", look: css.failure },
  pending: { icon: "clock", words: "Checks are running", look: css.pending },
};

// PullRequestCheckMark is a pull request's checks as a mark: passing,
// failing, running. With none, it keeps the mark's room.
export function PullRequestCheckMark({ checks }: { checks?: PR["checks"] }) {
  if (!checks) return <span className={css.mark} />;
  const s = says[checks];
  return (
    <span className={cx(css.mark, s.look)} title={s.words}>
      <Icon name={s.icon} size={12} />
      <SrOnly>{s.words}</SrOnly>
    </span>
  );
}
