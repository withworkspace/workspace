import type { ReactNode } from "react";
import css from "./ReviewOrphans.module.css";

// ReviewOrphans are comments on files the diff doesn't have (re-reviewing,
// files that haven't changed since), after its files: to answer and
// resolve. Each comes under its file's path (ReviewOrphanPath).
export function ReviewOrphans({ label, children }: { label: string; children: ReactNode }) {
  return (
    <section className={css.file}>
      <div className={css.head}>
        <span className="dim">{label}</span>
      </div>
      <div className={css.lines}>{children}</div>
    </section>
  );
}

