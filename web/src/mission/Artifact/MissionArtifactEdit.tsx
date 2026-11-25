import type { ReactNode } from "react";
import css from "./MissionArtifactEdit.module.css";

// MissionArtifactEdit is one of a mission's documents being edited: what
// saving it does and what's happened to it since (Callouts), over the
// editor, over its bar (MissionArtifactEditBar), a gap between each.
export function MissionArtifactEdit({ children }: { children: ReactNode }) {
  return <div className={css.edit}>{children}</div>;
}
