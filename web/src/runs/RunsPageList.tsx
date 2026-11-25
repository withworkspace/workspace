import type { ReactNode } from "react";
import { Rows } from "../ui/Rows";
import css from "./RunsPageList.module.css";

// RunsPageList is a group of the Runs page's rows: what's running, the
// results, the silos, what's ended. Each is a RunsPageRow.
export function RunsPageList({ children }: { children: ReactNode }) {
  return <Rows className={css.list}>{children}</Rows>;
}
