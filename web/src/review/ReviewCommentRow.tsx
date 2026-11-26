import { cx } from "../ui/cx";
import { Icon } from "../ui/Icon";
import { capital } from "../ui/strings";
import type { CommentEntry } from "./comments";
import { statusWords } from "./words";
import css from "./ReviewCommentRow.module.css";

// How each state reads: its colour.
const stateClass: Record<CommentEntry["state"], string> = { draft: css.draft, open: css.open, addressed: css.addressed, resolved: css.resolved, wontfix: css.wontfix };

// ReviewCommentRow is one comment in a review's list of them: its kind,
// how it stands and its lines, two lines of what it says, and the latest
// reply. Pressed, it's gone to; on, it's the one last gone to.
export function ReviewCommentRow({ c, on, onPick }: { c: CommentEntry; on: boolean; onPick: () => void }) {
  return (
    <button type="button" data-ui="" className={cx(css.row, on && css.on)} aria-current={on || undefined} onClick={onPick}>
      <span className={css.head}>
        <span className={`kind k-${c.kind || "none"}`}>{c.kind ? capital(c.kind) : "Comment"}</span>
        <span className={cx(css.state, stateClass[c.state])}>{c.state === "draft" ? "Draft" : (statusWords[c.state] ?? c.state)}</span>
        {c.lines && (
          <span className={css.lines}>
            {c.removed && "removed "}
            {c.lines}
          </span>
        )}
      </span>
      <span className={css.body}>{c.body}</span>
      {c.last && (
        <span className={css.reply}>
          <Icon name="reply" size={11} className={css.replyIcon} />
          <strong>{c.last.by}</strong> {c.last.body}
          {c.replies > 1 && <span className={css.more}>+{c.replies - 1}</span>}
        </span>
      )}
    </button>
  );
}
