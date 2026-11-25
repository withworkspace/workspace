import type { ReactNode } from "react";
import css from "./ArtifactTools.module.css";

// ArtifactTools is the row of tools at the foot of an artifact's head:
// its versions and its width, then at the row's end an ArtifactToolsEnd.
export function ArtifactTools({ children }: { children: ReactNode }) {
  return <div className={css.tools}>{children}</div>;
}

// ArtifactToolsEnd is what's pressed, at the row's end. A link among them
// (Compare, Open alone) is drawn as the small buttons beside it are.
export function ArtifactToolsEnd({ children }: { children: ReactNode }) {
  return <span className={css.end}>{children}</span>;
}
