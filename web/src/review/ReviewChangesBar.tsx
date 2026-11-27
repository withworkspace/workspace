import { Keys } from "../ui/Keys";
import { reviewKeys } from "./keys";
import { Icon } from "../ui/Icon";
import { Segment, Segmented } from "../ui/Segmented";
import css from "./ReviewChangesBar.module.css";

type ReviewChangesBarProps = {
  // What the diff is of, where there's been work since your last review:
  // only what's changed since (since), or all of it. i switches.
  // sinceName is what that review's called ("r2", "your review on GitHub").
  scope?: { since: boolean; sinceName: string; onChange: (since: boolean) => void };
  additions: number;
  deletions: number;
  // How it's laid out: one column, or side by side (u), on a window with
  // room for two.
  split?: { on: boolean; onChange: (split: boolean) => void };
};

// ReviewChangesBar is the Changes view's own bar, under the review's: what
// the diff is of, its size, and how it's laid out. It stays under the
// review's bar as the diff scrolls, and the files' heads stay under it.
export function ReviewChangesBar({ scope, additions, deletions, split }: ReviewChangesBarProps) {
  return (
    <div className={css.bar} data-review-changes-bar="">
      {scope && (
        <Segmented size="small" aria-label="What the changes are of" value={scope.since ? "since" : "all"} onChange={(v) => scope.onChange(v === "since")}>
          <Segment value="since" title={`Only what's changed since ${scope.sinceName}`} keys={reviewKeys.since}>
            Since your review
          </Segment>
          <Segment value="all" title="All the changes" keys={reviewKeys.since}>
            All changes <Keys k={reviewKeys.since} className={css.key} />
          </Segment>
        </Segmented>
      )}
      <span className={css.totals}>
        <span className={css.plus}>+{additions}</span> <span className={css.minus}>−{deletions}</span>
      </span>
      <span className={css.spacer} />
      {split && (
        <Segmented size="small" className={css.layout} aria-label="Diff layout" value={split.on ? "split" : "unified"} onChange={(v) => split.onChange(v === "split")}>
          <Segment value="unified" title="Unified" keys={reviewKeys.split}>
            <Icon name="unified" size={13} /> <span className={css.words}>Unified</span>
          </Segment>
          <Segment value="split" title="Side by side" keys={reviewKeys.split}>
            <Icon name="split" size={13} /> <span className={css.words}>Split</span>
          </Segment>
        </Segmented>
      )}
    </div>
  );
}
