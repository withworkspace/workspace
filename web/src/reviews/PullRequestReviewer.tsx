import { cx } from "../ui/cx";
import { Icon } from "../ui/Icon";
import css from "./PullRequestReviewer.module.css";

// PullRequestReviewer is where another reviewer stands: approved, asked
// for changes, or had a review dismissed; quiet when it was of earlier
// commits.
export function PullRequestReviewer({ login, kind, stale }: { login: string; kind: string; stale?: boolean }) {
  return (
    <span
      className={cx(css.reviewer, kind === "approved" && css.approved, kind === "changes" && css.changes, stale && css.stale)}
      title={`${login} ${kind === "approved" ? "approved" : kind === "changes" ? "asked for changes" : "had a review dismissed"}${stale ? ", on earlier commits" : ""}`}
    >
      <Icon name={kind === "approved" ? "check-circle" : kind === "changes" ? "alert" : "message"} size={12} className={css.icon} /> {login}
    </span>
  );
}
