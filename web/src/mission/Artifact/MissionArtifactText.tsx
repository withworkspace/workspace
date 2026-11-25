import type { ReactNode } from "react";
import { cx } from "../../ui/cx";
import css from "./MissionArtifactText.module.css";

// MissionArtifactText is one of a mission's documents, read: its markdown
// (children), larger, at a reading measure (prose).
export function MissionArtifactText({ children }: { children: ReactNode }) {
  return <article className={cx(css.text, "prose")}>{children}</article>;
}
