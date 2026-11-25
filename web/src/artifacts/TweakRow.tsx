import { useId } from "react";
import { ArtifactTweak } from "./ArtifactTweak";
import { said, type Tweak, type Value } from "./tweaks";
import { TweakControl } from "./TweakControl";

// TweakRow is one tweak: its label, what it's for, and its control. A
// switch and a colour sit at the end of the label's line, the rest under
// it. One that isn't as the page opens it is marked, and says how it
// opens.
export function TweakRow({ t, value, opens, set }: { t: Tweak; value: Value; opens: Value; set: (v: Value) => void }) {
  const id = useId();
  const moved = value !== opens;
  const chosen = t.kind === "choice" ? t.options?.find((o) => o.value === value)?.description : "";
  return (
    <ArtifactTweak id={id} kind={t.kind} label={t.label} description={t.description} opensAs={moved ? said(t, opens) : undefined} inline={t.kind === "toggle" || t.kind === "color"} chosen={chosen}>
      <TweakControl id={id} t={t} value={value} set={set} />
    </ArtifactTweak>
  );
}
