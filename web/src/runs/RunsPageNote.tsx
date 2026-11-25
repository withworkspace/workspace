import type { ReactNode } from "react";
import css from "./RunsPageNote.module.css";

// RunsPageNote is a quiet line under a group's head on the Runs page: what
// the group is, and how long what's in it is kept.
export function RunsPageNote({ children }: { children: ReactNode }) {
  return <p className={css.note}>{children}</p>;
}
