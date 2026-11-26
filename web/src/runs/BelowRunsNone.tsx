import type { ReactNode } from "react";
import { Empty } from "../ui/Empty";
import css from "./BelowRunsNone.module.css";

// BelowRunsNone is Below's Runs with nothing running: why (title, and
// what to do), and what's pressed for it (action).
export function BelowRunsNone({ title, action, children }: { title: string; action?: ReactNode; children?: ReactNode }) {
  return (
    <Empty icon="play" title={title} action={action} className={css.none}>
      {children}
    </Empty>
  );
}
