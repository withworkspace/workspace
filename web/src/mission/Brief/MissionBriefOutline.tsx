import type { ReactNode } from "react";
import css from "./MissionBriefOutline.module.css";

// MissionBriefOutline is what the brief and plan are made of, down the
// Brief tab's left on a wide window: MissionBriefOutlineSections, each
// leading to its place in the documents.
export function MissionBriefOutline({ children }: { children: ReactNode }) {
  return (
    <aside className={css.outline} aria-label="What the brief and plan are made of">
      {children}
    </aside>
  );
}
