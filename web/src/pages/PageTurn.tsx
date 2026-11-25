import type { ReactNode } from "react";
import { cx } from "../ui/cx";
import css from "./PageTurn.module.css";

// PageTurn is one thing said in a cell: what you asked, a version its
// agent wrote, an answer (beside the page, not in it), its agent at work,
// or what went wrong.
export function PageTurn({ kind, children }: { kind?: "failed" | "answer" | "version" | "filling"; children: ReactNode }) {
  return <li className={cx(css.turn, kind === "failed" && css.failed, kind === "answer" && css.answer)}>{children}</li>;
}
