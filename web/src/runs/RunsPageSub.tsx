import type { ReactNode } from "react";
import { RowSub } from "../ui/Rows";
import css from "./RunsPageSub.module.css";

// RunsPageSub is a quiet line under a row's name on the Runs page: its
// facts side by side (a span each), the links among them underlined.
export function RunsPageSub({ children }: { children: ReactNode }) {
  return <RowSub className={css.sub}>{children}</RowSub>;
}
