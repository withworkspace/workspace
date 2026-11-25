import type { ReactNode } from "react";
import css from "./DocumentsGrid.module.css";

// DocumentsGrid is Documents' spaces, each a card, with Local (the pages
// that aren't in a space) beside them.
export function DocumentsGrid({ spaces, local }: { spaces: ReactNode; local: ReactNode }) {
  return (
    <div className={css.grid}>
      <section aria-label="Spaces">{spaces}</section>
      <section aria-label="Local">{local}</section>
    </div>
  );
}
