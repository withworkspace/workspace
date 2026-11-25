import type { ReactNode } from "react";
import css from "./ArtifactTweaksTray.module.css";

// ArtifactTweaksTray is the tray beside a version's frame: its head and
// its tweaks (its children), over a foot. On a narrow window it's under
// the frame.
export function ArtifactTweaksTray({ foot, children }: { foot?: ReactNode; children: ReactNode }) {
  return (
    <aside className={css.tray} aria-label="Tweaks">
      {children}
      {foot && <div className={css.foot}>{foot}</div>}
    </aside>
  );
}
