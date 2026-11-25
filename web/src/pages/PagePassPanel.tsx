import type { ReactNode } from "react";
import css from "./PagePassPanel.module.css";

// PagePassPanel is a pass over the page, on its tab: beside the page, and
// never in it.
export function PagePassPanel({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className={css.panel} role="tabpanel" aria-label={label}>
      {children}
    </div>
  );
}
