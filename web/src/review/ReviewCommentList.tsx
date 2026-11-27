import { Keys } from "../ui/Keys";
import { reviewKeys } from "./keys";
import { Fragment } from "react";
import { Segment, Segmented } from "../ui/Segmented";
import { type CommentEntry, type CommentFilter, inFilter } from "./comments";
import { ReviewCommentFile } from "./ReviewCommentFile";
import { ReviewCommentRow } from "./ReviewCommentRow";
import css from "./ReviewCommentList.module.css";

const filters: [CommentFilter, string][] = [
  ["all", "All"],
  ["open", "Open"],
  ["draft", "Drafts"],
  ["resolved", "Resolved"],
];

// ReviewCommentList is a review's comments to read and go to: which to
// show (all, open, your drafts, resolved: each with how many), then each
// file's under its name, in reading order. at is the one last gone to.
export function ReviewCommentList({ list, filter, onFilter, at, onPick }: { list: CommentEntry[]; filter: CommentFilter; onFilter: (f: CommentFilter) => void; at?: string; onPick: (key: string) => void }) {
  const shown = list.filter(inFilter(filter));
  const files = new Map<string, CommentEntry[]>();
  for (const c of shown) files.set(c.path, [...(files.get(c.path) ?? []), c]);
  return (
    <div className={css.list}>
      <div className={css.filter}>
        <Segmented size="small" fill aria-label="Which comments" value={filter} onChange={onFilter}>
          {filters.map(([f, label]) => (
            <Segment key={f} value={f}>
              {label} <span className={css.n}>{list.filter(inFilter(f)).length}</span>
            </Segment>
          ))}
        </Segmented>
      </div>
      {[...files].map(([path, cs]) => (
        <Fragment key={path}>
          <ReviewCommentFile path={path} count={cs.length} />
          {cs.map((c) => (
            <ReviewCommentRow key={c.key} c={c} on={c.key === at} onPick={() => onPick(c.key)} />
          ))}
        </Fragment>
      ))}
      {!shown.length && <p className={css.none}>{list.length ? `No ${filters.find(([f]) => f === filter)![1].toLowerCase()} comments.` : "No comments yet: c on a line makes one."}</p>}
      <p className={css.keys}>
        <Keys k={reviewKeys.nextComment} /> next · <Keys k={reviewKeys.prevComment} /> previous
      </p>
    </div>
  );
}
