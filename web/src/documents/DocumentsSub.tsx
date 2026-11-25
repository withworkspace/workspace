import type { ReactNode } from "react";
import { cx } from "../ui/cx";
import css from "./DocumentsSub.module.css";

// DocumentsSub is what a heading says after its count, quietly: across
// how many spaces, how many pages filling.
export function DocumentsSub({ children }: { children: ReactNode }) {
  return <span className={cx("dim", css.sub)}>{children}</span>;
}
