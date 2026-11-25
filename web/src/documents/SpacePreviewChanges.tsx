import type { ReactNode } from "react";
import css from "./SpacePreviewChanges.module.css";

// SpacePreviewChanges is the changes to the document picked lately, under
// its text: a DocumentsChanges.
export function SpacePreviewChanges({ children }: { children: ReactNode }) {
  return (
    <section className={css.changes} aria-label="Changes to it lately">
      <h3>Changes to it lately</h3>
      {children}
    </section>
  );
}
