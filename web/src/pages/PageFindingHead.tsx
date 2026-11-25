import type { ReactNode } from "react";
import css from "./PageFindingHead.module.css";

// PageFindingHead is a finding's kind and the words it's about.
export function PageFindingHead({ children }: { children: ReactNode }) {
  return <div className={css.head}>{children}</div>;
}
