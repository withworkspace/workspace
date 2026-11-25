import type { ReactNode } from "react";
import css from "./ReviewViews.module.css";

// ReviewViews is a list of a review's views, down the side: those read
// beside the changes, or the Changes view itself. A second follows the
// first under a rule.
export function ReviewViews({ children }: { children: ReactNode }) {
  return <ul className={css.list}>{children}</ul>;
}
