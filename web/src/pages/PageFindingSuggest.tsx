import type { ReactNode } from "react";
import { cx } from "../ui/cx";
import css from "./PageFindingSuggest.module.css";

// PageFindingSuggest is the words a finding proposes in place of those it's
// about.
export function PageFindingSuggest({ children }: { children: ReactNode }) {
  return (
    <div className={css.suggest}>
      <span className={cx("dim", css.label)}>Should read</span>
      {children}
    </div>
  );
}
