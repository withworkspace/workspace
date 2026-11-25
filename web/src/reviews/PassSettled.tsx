import type { ReactNode } from "react";
import css from "./PassSettled.module.css";
import { textClass } from "../ui/Text";

// PassSettled is what a pass found that's settled, folded under how many
// were taken and dismissed.
export function PassSettled({ summary, children }: { summary: string; children: ReactNode }) {
  return (
    <details className={css.settled}>
      <summary className={textClass("dim")}>{summary}</summary>
      {children}
    </details>
  );
}
