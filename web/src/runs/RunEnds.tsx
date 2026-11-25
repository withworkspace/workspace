import css from "./RunEnds.module.css";

// RunEnds is a quiet line under a run's name: when it ends, what it
// needs, the commit a task ran at.
export function RunEnds({ children }: { children: React.ReactNode }) {
  return <span className={css.ends}>{children}</span>;
}
