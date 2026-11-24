import type { ReactNode } from "react";
import css from "./FindSection.module.css";

// FindSection is a group of what Find found (its missions, its files),
// under what the group's called: title, where that's worth saying. label
// is what a screen reader calls it either way.
export function FindSection({ label, title, children }: { label: string; title?: string; children: ReactNode }) {
  return (
    <section className={css.section} aria-label={label}>
      {title && <h3 className={css.title}>{title}</h3>}
      {children}
    </section>
  );
}
