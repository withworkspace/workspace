import type { ReactNode } from "react";
import { cx } from "../ui/cx";
import css from "./CodeNavList.module.css";

// CodeNavList is a list to pick from in code navigation: each row its
// own (children), the one picked lit. Rows read down (what it is over
// where), or across (inline). Maybes are quieter.
export function CodeNavList({ rows, inline, maybes }: { rows: { key: string; picked: boolean; onEnter: () => void; onClick: () => void; children: ReactNode }[]; inline?: boolean; maybes?: boolean }) {
  return (
    <ul className={cx(css.list, inline && css.inline, maybes && css.maybes)}>
      {rows.map((r) => (
        <li key={r.key} className={r.picked ? css.picked : undefined} data-picked={r.picked || undefined} onMouseEnter={r.onEnter} onClick={r.onClick}>
          {r.children}
        </li>
      ))}
    </ul>
  );
}

// codeNavListWhereClass is where a row's name is (as Text mono): a size
// smaller.
export const codeNavListWhereClass: string = css.where;
