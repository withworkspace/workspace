import type { ReactNode } from "react";
import css from "./AnnotateLost.module.css";
import { Text } from "../ui/Text";

// AnnotateLost holds the threads whose text isn't in this version of the
// document (changed, or removed), over it: what's said of them, then
// their cards.
export function AnnotateLost({ said, children }: { said: ReactNode; children: ReactNode }) {
  return (
    <section className={css.lost}>
      <Text as="div" dim>{said}</Text>
      {children}
    </section>
  );
}
