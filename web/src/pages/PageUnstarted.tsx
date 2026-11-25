import type { ReactNode } from "react";
import css from "./PageUnstarted.module.css";

// PageUnstarted says a cell's agent hasn't started, or couldn't, and
// offers to try again (children).
export function PageUnstarted({ words, children }: { words: ReactNode; children: ReactNode }) {
  return (
    <div className={css.unstarted}>
      <span className="dim">{words}</span>
      {children}
    </div>
  );
}
