import type { ReactNode } from "react";
import css from "./ReaderCites.module.css";

// ReaderCites is what's listed below a document's text (children: what
// cites it and what it cites, or the conversations about it), apart from
// it.
export function ReaderCites({ label, children }: { label: string; children: ReactNode }) {
  return (
    <aside className={css.cites} aria-label={label}>
      {children}
    </aside>
  );
}
