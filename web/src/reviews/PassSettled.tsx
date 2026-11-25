import type { ReactNode } from "react";
import css from "./PassSettled.module.css";

// PassSettled is what a pass found that's settled, folded under how many
// were taken and dismissed.
export function PassSettled({ summary, children }: { summary: string; children: ReactNode }) {
  return (
    <details className={css.settled}>
      <summary className="dim">{summary}</summary>
      {children}
    </details>
  );
}
