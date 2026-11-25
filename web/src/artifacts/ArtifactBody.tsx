import type { ReactNode } from "react";
import { cx } from "../ui/cx";
import css from "./ArtifactBody.module.css";

// ArtifactBody is what's under an artifact's head: the stage its frame
// is on and, beside it, the tray of its tweaks (under it, on a narrow
// window).
export function ArtifactBody({ children }: { children: ReactNode }) {
  return <div className={css.body}>{children}</div>;
}

// ArtifactStage is the room a version's frame fills, which scrolls where
// the frame's wider than it: or what's said where there's no frame.
// compared is one of two side by side, with a rule between them.
export function ArtifactStage({ compared, children }: { compared?: boolean; children: ReactNode }) {
  return <div className={cx(css.stage, compared && css.compared)}>{children}</div>;
}
