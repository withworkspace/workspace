import { ArtifactTweakList } from "./ArtifactTweak";
import { ArtifactTweakPreset, ArtifactTweakPresets } from "./ArtifactTweakPresets";
import { ArtifactTweakProblems } from "./ArtifactTweakProblems";
import { ArtifactTweaksBody } from "./ArtifactTweaksBody";
import { presetOn, type State, type Told } from "./tweaks";
import { TweakRow } from "./TweakRow";

// TweaksBody is the presets, a button each, and the tweaks, a control
// each. A preset is on while every tweak it lists is as it gives it.
export function TweaksBody({ told, set }: { told: Told; set: (some: State) => void }) {
  return (
    <ArtifactTweaksBody>
      {told.presets.length > 0 && (
        <ArtifactTweakPresets>
          {told.presets.map((p, i) => (
            <ArtifactTweakPreset key={i} label={p.label} description={p.description} on={presetOn(p, told.state)} onPress={() => set(p.state)} />
          ))}
        </ArtifactTweakPresets>
      )}
      <ArtifactTweakList>
        {told.tweaks.map((t) => (
          <TweakRow key={t.name} t={t} value={told.state[t.name]} opens={told.opening[t.name]} set={(v) => set({ [t.name]: v })} />
        ))}
      </ArtifactTweakList>
      {told.problems.length > 0 && <ArtifactTweakProblems problems={told.problems} />}
    </ArtifactTweaksBody>
  );
}
