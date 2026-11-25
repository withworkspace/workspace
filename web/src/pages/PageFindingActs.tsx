import type { ReactNode } from "react";
import css from "./PageFindingActs.module.css";

// PageFindingActs are what's done with a finding: use its words, done,
// find out, dismiss; or, settled, open it again.
export function PageFindingActs({ children }: { children: ReactNode }) {
  return <div className={css.acts}>{children}</div>;
}
