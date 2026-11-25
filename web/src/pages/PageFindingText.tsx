import type { ReactNode } from "react";
import css from "./PageFindingText.module.css";

// PageFindingText is what a finding says of its words.
export function PageFindingText({ children }: { children: ReactNode }) {
  return <div className={css.text}>{children}</div>;
}
