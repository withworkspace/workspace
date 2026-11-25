import type { ReactNode } from "react";
import css from "./MissionArtifactText.module.css";
import { Prose } from "../../ui/Prose";

// MissionArtifactText is one of a mission's documents, read: its markdown
// (children), larger, at a reading measure (prose).
export function MissionArtifactText({ children }: { children: ReactNode }) {
  return <Prose as="article" className={css.text}>{children}</Prose>;
}
