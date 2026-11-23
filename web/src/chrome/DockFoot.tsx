import type { ReactNode } from "react";
import css from "./DockFoot.module.css";

// DockFoot is what stays at the bottom of a dock's tab while its body
// scrolls: a conversation's reply box, what you'd tell crew.
export function DockFoot({ children }: { children: ReactNode }) {
  return <div className={css.foot}>{children}</div>;
}
