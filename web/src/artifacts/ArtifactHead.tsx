import type { ReactNode } from "react";
import css from "./ArtifactHead.module.css";

// ArtifactHead is the head over a version's frame: its title's line, what
// the version is and says, and its tools, one under another.
export function ArtifactHead({ children }: { children: ReactNode }) {
  return <header className={css.head}>{children}</header>;
}
