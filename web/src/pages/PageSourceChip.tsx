import type { ReactNode } from "react";
import { Link } from "react-router";
import { Chip, chipClass } from "../ui/Chip";
import css from "./PageSourceChip.module.css";

// PageSourceChip is a file a cell was written from: to read in Files (to),
// in the attention's colour when it's changed or gone since.
export function PageSourceChip({ to, moved, title, children }: { to?: string; moved?: boolean; title: string; children: ReactNode }) {
  return to ? (
    <Link to={to} className={chipClass(moved ? "attention" : undefined, css.chip)} title={title}>
      {children}
    </Link>
  ) : (
    <Chip tone={moved ? "attention" : undefined} className={css.chip} title={title}>
      {children}
    </Chip>
  );
}
