import type { ReactNode } from "react";
import { ArtifactTweaksHead } from "./ArtifactTweaksHead";
import { changed, stateWords, type Told } from "./tweaks";

// TweaksHead is the head over a frame's tweaks: how many aren't as the
// page opens them, and what each is set to, to copy.
export function TweaksHead({ told, children }: { told: Told; children?: ReactNode }) {
  return (
    <ArtifactTweaksHead changed={changed(told).length} copy={stateWords(told)}>
      {children}
    </ArtifactTweaksHead>
  );
}
