import type { ReactNode } from "react";
import { cx } from "../ui/cx";
import css from "./DocumentsSpaceCards.module.css";

// DocumentsSpaceCards is the spaces, each a DocumentsSpaceCard: as many
// to a row as fit, or (few: one or two) stretched to the row.
export function DocumentsSpaceCards({ few, children }: { few?: boolean; children: ReactNode }) {
  return <div className={cx(css.cards, few && css.few)}>{children}</div>;
}
