import type { ReactNode } from "react";
import { cx } from "../ui/cx";
import css from "./DocumentsSpaceCardKinds.module.css";

// DocumentsSpaceCardKinds is a space's card's list: a
// DocumentsSpaceCardKind for each of its kinds, or its folders; while
// its documents are being read, what it's said of its folders (reading).
export function DocumentsSpaceCardKinds({ reading, children }: { reading?: string; children?: ReactNode }) {
  return (
    <ul className={css.kinds}>
      {children}
      {reading !== undefined && <li className={cx("dim", css.reading)}>{reading}</li>}
    </ul>
  );
}
