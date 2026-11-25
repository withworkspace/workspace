import { Chip } from "../ui/Chip";
import { Icon } from "../ui/Icon";
import css from "./ReviewScope.module.css";

// ReviewScope says what a review's diff is of: since your last review
// (since, with what that was), how many commits ahead of what, and the
// lines added and taken away.
export function ReviewScope({
  since,
  commits,
  base,
  additions,
  deletions,
}: {
  since?: { name: string; title: string };
  commits: number;
  base?: string; // what it's ahead of, when it's all of it
  additions: number;
  deletions: number;
}) {
  return (
    <span className={css.scope}>
      {since ? (
        <Chip tone="attention" title={since.title}>
          <Icon name="rotate" size={11} /> Since {since.name}
        </Chip>
      ) : null}
      <span className={css.commits}>
        {commits} {commits === 1 ? "commit" : "commits"}
        {base && ` ahead of ${base}`}
      </span>
      <span className={css.totals}>
        <span className={css.plus}>+{additions}</span> <span className={css.minus}>−{deletions}</span>
      </span>
    </span>
  );
}
