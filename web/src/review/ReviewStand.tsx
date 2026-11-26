import type { ReactNode } from "react";
import { HoverCard, HoverCardHead } from "../ui/HoverCard";
import { Status, type StatusInfo } from "../ui/Status";
import css from "./ReviewStand.module.css";

// ReviewStand is where a review stands, as its icon at the start of its
// bar: pointed at (or focused), a card says it in words, with what else
// is said of it (children: ReviewFacts, its links).
export function ReviewStand({ s, children }: { s: StatusInfo; children?: ReactNode }) {
  return (
    <HoverCard className={css.stand} trigger={<Status s={s} iconOnly size={15} title="" />}>
      <span className={css.card}>
        <HoverCardHead>
          <Status s={s} />
        </HoverCardHead>
        {children}
      </span>
    </HoverCard>
  );
}
