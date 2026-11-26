import type { PR } from "../api";
import { PullRequestCheckMark } from "./PullRequestCheckMark";
import css from "./PullRequestMetaChecks.module.css";

// PullRequestMetaChecks are a pull request's checks in its meta line: the
// mark, and what it means.
export function PullRequestMetaChecks({ checks }: { checks: NonNullable<PR["checks"]> }) {
  return (
    <span className={css.checks}>
      <PullRequestCheckMark checks={checks} /> {checks === "success" ? "Checks pass" : checks === "failure" ? "Checks failing" : "Checks running"}
    </span>
  );
}
