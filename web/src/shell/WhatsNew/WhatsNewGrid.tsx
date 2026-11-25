import type { ReactNode } from "react";
import css from "./WhatsNewGrid.module.css";

// WhatsNewGrid is What's new after the first: its entries as cards, as
// many across as there's room for.
export function WhatsNewGrid({ children }: { children: ReactNode }) {
  return <div className={css.grid}>{children}</div>;
}
