import type { ReactNode } from "react";
import css from "./TeamGroup.module.css";

// TeamGroup is one group of the Team page's rows: a day's, or a person's,
// under its GroupHead.
export function TeamGroup({ label, children }: { label: string; children: ReactNode }) {
  return (
    <section className={css.group} aria-label={label}>
      {children}
    </section>
  );
}
