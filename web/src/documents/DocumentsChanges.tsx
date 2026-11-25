import type { ReactNode } from "react";
import { cx } from "../ui/cx";
import css from "./DocumentsChanges.module.css";

// DocumentsChanges is changes to documents, each a DocumentsChangeLine;
// more says how many earlier ones aren't shown.
export function DocumentsChanges({ more, children }: { more?: ReactNode; children: ReactNode }) {
  return (
    <ul className={css.changes}>
      {children}
      {more && <li className={cx(css.more, "dim")}>{more}</li>}
    </ul>
  );
}
