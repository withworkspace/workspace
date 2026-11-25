import type { ReactNode, Ref } from "react";
import css from "./PageDoc.module.css";

// PageDoc holds the page's words: what's opened over them (a cell's panel,
// a finding, the box that asks for a change) is placed in it.
export function PageDoc({ ref, children }: { ref?: Ref<HTMLDivElement>; children: ReactNode }) {
  return (
    <div className={css.doc} ref={ref}>
      {children}
    </div>
  );
}
