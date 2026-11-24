import type { ReactNode } from "react";
import { cx } from "../../ui/cx";
import css from "./FindNote.module.css";

// FindNote is a line under a group's rows, said quietly: how the search
// went, what to do next. capped is one that says the search stopped
// short, said where it's seen; foot, one under the whole list, over a
// rule.
export function FindNote({ capped, foot, children }: { capped?: boolean; foot?: boolean; children: ReactNode }) {
  return <p className={cx(css.note, capped && css.capped, foot && css.foot)}>{children}</p>;
}
