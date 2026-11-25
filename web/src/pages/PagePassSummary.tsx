import type { ReactNode } from "react";
import { cx } from "../ui/cx";
import css from "./PagePassSummary.module.css";

// PagePassSummary is what a pass said of the page as a whole.
export function PagePassSummary({ children }: { children: ReactNode }) {
  return <div className={cx(css.summary, "prose")}>{children}</div>;
}
