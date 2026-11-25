import type { ReactNode } from "react";
import css from "./GuideBox.module.css";

// GuideBox holds a review's guide, to a reading measure.
export function GuideBox({ children }: { children: ReactNode }) {
  return <div className={css.guide}>{children}</div>;
}
