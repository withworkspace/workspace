import type { ReactNode } from "react";
import css from "./TeamSub.module.css";

// TeamSub is a quiet heading inside a person's group: Reviewing.
export function TeamSub({ children }: { children: ReactNode }) {
  return <h3 className={css.sub}>{children}</h3>;
}
