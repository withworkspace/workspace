import type { ReactNode } from "react";
import css from "./FindActs.module.css";

// FindActs is what can be done to the thing in Find's pane, under it and
// a rule: a row of buttons (FindAct, a pin), wrapped where they don't
// fit.
export function FindActs({ children }: { children: ReactNode }) {
  return (
    <div className={css.acts} aria-label="Do to it">
      {children}
    </div>
  );
}
