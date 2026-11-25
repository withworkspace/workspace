import type { ReactNode } from "react";
import css from "./ArtifactView.module.css";

// ArtifactView is the page a version runs on: its head (an ArtifactHead),
// and under it what fills the rest of the window.
export function ArtifactView({ children }: { children: ReactNode }) {
  return <div className={css.view}>{children}</div>;
}
