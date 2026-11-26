import type { ReactNode } from "react";
import { Button } from "../ui/Button";
import { Icon } from "../ui/Icon";
import { ArtifactTweaksTray } from "./ArtifactTweaksTray";
import type { State, Told } from "./tweaks";
import { TweaksBody } from "./TweaksBody";
import { TweaksHead } from "./TweaksHead";

// TweaksTray is a frame's tweaks, beside it: the artifact's page. What's
// under them (the way to compare it with how it opens) is its children.
// keys is the key that hides it, where the page binds one.
export function TweaksTray({ told, set, onClose, keys, children }: { told: Told; set: (some: State) => void; onClose: () => void; keys?: string; children?: ReactNode }) {
  return (
    <ArtifactTweaksTray foot={children}>
      <TweaksHead told={told}>
        <Button variant="ghost" size="small" square onClick={onClose} aria-label="Hide tweaks" title="Hide tweaks" keys={keys}>
          <Icon name="x" size={13} />
        </Button>
      </TweaksHead>
      <TweaksBody told={told} set={set} />
    </ArtifactTweaksTray>
  );
}
