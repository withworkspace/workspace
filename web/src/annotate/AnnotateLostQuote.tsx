import type { ReactNode } from "react";
import css from "./AnnotateLostQuote.module.css";

// AnnotateLostQuote is what a thread was on, struck, in a card whose text
// isn't in this version of the document; its title says where it was.
export function AnnotateLostQuote({ title, children }: { title?: string; children: ReactNode }) {
  return (
    <blockquote className={css.quote} title={title}>
      {children}
    </blockquote>
  );
}
