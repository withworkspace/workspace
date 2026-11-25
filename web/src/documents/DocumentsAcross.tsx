import type { ReactNode } from "react";
import css from "./DocumentsAcross.module.css";

// DocumentsAcross is what's happening across every space, over them on
// Documents: what changed lately and what's in flight, side by side where
// there's room for both.
export function DocumentsAcross({ children }: { children: ReactNode }) {
  return <div className={css.across}>{children}</div>;
}
