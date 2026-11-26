import type { ReactNode } from "react";
import { Time } from "../ui/Time";
import css from "./ReviewSaid.module.css";

// ReviewSaid is one comment in a review comment's thread: who said it and
// when, on a line with what's to be known of it (marks: a status it left,
// the commit it was said at), and what they said under it. The first is
// the comment itself; each after it, a reply.
export function ReviewSaid({ who, title, at, marks, children }: { who: ReactNode; title?: string; at?: string; marks?: ReactNode; children?: ReactNode }) {
  return (
    <div className={css.said}>
      <div className={css.head}>
        <strong className={css.who} title={title}>
          {who}
        </strong>
        {at && <Time iso={at} as="relative" className={css.when} />}
        {marks}
      </div>
      {children != null && children !== "" && <div className={css.body}>{children}</div>}
    </div>
  );
}

// saidAt is a time to say, or none for one that was never kept (a comment
// from before they were: the zero time).
export const saidAt = (iso?: string) => (iso && !iso.startsWith("0001-") ? iso : undefined);
