import type { ReactNode } from "react";
import { cx } from "../../ui/cx";
import css from "./MissionArtifactPage.module.css";

// MissionArtifactPage is the page of one of a mission's documents: at a
// reading width, or wider while it's commented on (annotating: the
// threads take a margin beside it).
export function MissionArtifactPage({ annotating, children }: { annotating?: boolean; children: ReactNode }) {
  return <div className={cx("page", css.page, annotating && css.annotating)}>{children}</div>;
}
