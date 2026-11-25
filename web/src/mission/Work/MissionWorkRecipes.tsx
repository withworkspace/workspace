import type { ReactNode } from "react";
import css from "./MissionWorkRecipes.module.css";

// MissionWorkRecipes is a capsule's recipes that aren't running, each a
// button that runs it.
export function MissionWorkRecipes({ children }: { children: ReactNode }) {
  return <div className={css.recipes}>{children}</div>;
}
