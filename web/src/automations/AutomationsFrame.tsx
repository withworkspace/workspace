import type { ReactNode } from "react";
import css from "./AutomationsFrame.module.css";

// AutomationsFrame is the Automations pages' frame: a page's width and
// room, as every page has; its parts a gap apart.
export function AutomationsFrame({ children }: { children: ReactNode }) {
  return <div className={css.page}>{children}</div>;
}

// AutomationsLede is what's said under a page's head, quieter.
export function AutomationsLede({ children }: { children: ReactNode }) {
  return <p className={css.lede}>{children}</p>;
}

// AutomationsSection is a part of a page, under its GroupHead.
export function AutomationsSection({ children }: { children: ReactNode }) {
  return <section className={css.section}>{children}</section>;
}
