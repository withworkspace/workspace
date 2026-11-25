import type { ReactNode } from "react";
import css from "./AppView.module.css";

// AppView is a workspace app's page: its bar over its card, filling the
// room the page has.
export function AppView({ children }: { children: ReactNode }) {
  return <div className={css.view}>{children}</div>;
}
