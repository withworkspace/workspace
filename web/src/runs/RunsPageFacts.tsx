import type { ReactNode } from "react";
import css from "./RunsPageFacts.module.css";

// RunsPageFacts is how a run was started, under its row: a <dt> and a <dd>
// each (its command, folder, commit, when and by whom).
export function RunsPageFacts({ children }: { children: ReactNode }) {
  return <dl className={css.facts}>{children}</dl>;
}
