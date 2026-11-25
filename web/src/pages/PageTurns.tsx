import type { ReactNode } from "react";
import { cx } from "../ui/cx";
import css from "./PageTurns.module.css";

// PageTurns are what's been said in a cell, or about a finding: its
// versions, what you asked, its answers. flush is for under a finding,
// with no room of a panel's around them.
export function PageTurns({ flush, children }: { flush?: boolean; children: ReactNode }) {
  return <ol className={cx(css.turns, flush && css.flush)}>{children}</ol>;
}
