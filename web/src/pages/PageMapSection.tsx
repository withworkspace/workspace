import type { ReactNode } from "react";
import css from "./PageMapSection.module.css";

// PageMapSection is a part of the map under its heading: a list item for
// each thing (a PageMapItem in each).
export function PageMapSection({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className={css.section}>
      <h3>{title}</h3>
      <ul>{children}</ul>
    </section>
  );
}
