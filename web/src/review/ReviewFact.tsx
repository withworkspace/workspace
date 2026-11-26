import type { ReactNode } from "react";
import css from "./ReviewFact.module.css";

// ReviewFact is one row of ReviewFacts: what it's called, and what it is.
export function ReviewFact({ term, children }: { term: string; children: ReactNode }) {
  return (
    <>
      <dt className={css.term}>{term}</dt>
      <dd className={css.is}>{children}</dd>
    </>
  );
}
