import type { ReactNode } from "react";
import css from "./MissionSideSection.module.css";

// MissionSideSection is one part of a mission's sidebar (its plan, its
// outputs, its capsules, its crew): a small head with a count beside it,
// over what's in it.
export function MissionSideSection({ title, count, children }: { title: string; count?: string | number; children: ReactNode }) {
  return (
    <section className={css.section}>
      <h3 className={css.head}>
        {title}
        {count !== undefined && <span className={css.count}>{count}</span>}
      </h3>
      {children}
    </section>
  );
}
