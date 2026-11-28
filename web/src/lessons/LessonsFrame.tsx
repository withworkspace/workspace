import type { ReactNode } from "react";
import css from "./LessonsFrame.module.css";

// LessonsFrame is the Lessons page's frame: a page's width and room, as
// every page has; its parts a gap apart.
export function LessonsFrame({ children }: { children: ReactNode }) {
  return <div className={css.page}>{children}</div>;
}

// LessonsLede is what's said under the page's head, quieter.
export function LessonsLede({ children }: { children: ReactNode }) {
  return <p className={css.lede}>{children}</p>;
}

// LessonsSection is a group of lessons, under its GroupHead.
export function LessonsSection({ children }: { children: ReactNode }) {
  return <section className={css.section}>{children}</section>;
}

// LessonsFoot is how reading handoffs stands, at the page's foot.
export function LessonsFoot({ children }: { children: ReactNode }) {
  return <p className={css.foot}>{children}</p>;
}
