import type { ReactNode } from "react";
import css from "./ArtifactTweakPresets.module.css";

// ArtifactTweakPresets is a page's presets, a button each.
export function ArtifactTweakPresets({ children }: { children: ReactNode }) {
  return (
    <div className={css.presets} role="group" aria-label="Presets">
      {children}
    </div>
  );
}

// ArtifactTweakPreset is a button that sets several tweaks at once: its
// label over a line or two about it. It's on while they're all as it
// gives them.
export function ArtifactTweakPreset({ label, description, on, onPress }: { label: string; description?: string; on: boolean; onPress: () => void }) {
  return (
    <button type="button" className={css.preset} aria-pressed={on} onClick={onPress}>
      <span className={css.label}>{label}</span>
      {description && <span className={css.description}>{description}</span>}
    </button>
  );
}
