import { ArtifactTweaksButton } from "./ArtifactTweaksButton";
import { changed, type State, type Told } from "./tweaks";
import { TweaksBody } from "./TweaksBody";
import { TweaksHead } from "./TweaksHead";
import { hasTweaks } from "./useTweaks";

// TweaksButton is a frame's tweaks behind a button, where there isn't the
// room for a tray: a side of a comparison. Nothing, for a page with none.
export function TweaksButton({ told, set, label }: { told: Told | undefined; set: (some: State) => void; label: string }) {
  if (!hasTweaks(told)) return null;
  return (
    <ArtifactTweaksButton label={label} changed={changed(told).length}>
      <TweaksHead told={told} />
      <TweaksBody told={told} set={set} />
    </ArtifactTweaksButton>
  );
}
