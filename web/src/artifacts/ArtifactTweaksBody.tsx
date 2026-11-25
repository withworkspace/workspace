import type { ReactNode } from "react";
import css from "./ArtifactTweaksBody.module.css";

// ArtifactTweaksBody is what's under the tweaks' head, which scrolls: the
// presets, the tweaks, and what's wrong with how the page declares them.
export function ArtifactTweaksBody({ children }: { children: ReactNode }) {
  return <div className={css.body}>{children}</div>;
}
