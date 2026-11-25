import type { ReactNode } from "react";
import css from "./RefFoot.module.css";

// RefFoot is the line under what a ref's card says: how to go there, and
// pinning it, which is told to keep to its end (PinButton's end). With nothing in it, it isn't there.
export function RefFoot({ children }: { children: ReactNode }) {
  return <span className={css.foot}>{children}</span>;
}
