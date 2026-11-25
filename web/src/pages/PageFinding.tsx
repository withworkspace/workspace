import type { ReactNode } from "react";
import { cx } from "../ui/cx";
import css from "./PageFinding.module.css";

// PageFinding is one thing a pass found: a card in its list, quieter once
// it's settled, or (in) in a PagePanel opened from its words.
export function PageFinding({ look = "card", children }: { look?: "card" | "settled" | "in"; children: ReactNode }) {
  if (look === "in") return <div className={css.in}>{children}</div>;
  return <li className={cx(css.card, look === "settled" && css.settled)}>{children}</li>;
}
