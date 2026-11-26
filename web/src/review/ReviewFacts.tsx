import type { ReactNode } from "react";
import css from "./ReviewFacts.module.css";

// ReviewFacts are what's said of a review in a card, a fact a row
// (ReviewFact): what it's called on the left, what it is on the right.
export function ReviewFacts({ children }: { children: ReactNode }) {
  return <dl className={css.facts}>{children}</dl>;
}
