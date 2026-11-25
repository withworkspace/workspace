import type { ReactNode } from "react";
import css from "./AppCard.module.css";

// AppCard is the card an app is drawn in, under its bar: the app framed
// (an iframe), or an AppWaiting while there's nothing to frame.
export function AppCard({ children }: { children: ReactNode }) {
  return <div className={css.card}>{children}</div>;
}
