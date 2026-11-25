import { ArtifactTweakChoice } from "./ArtifactTweakChoice";
import { ArtifactTweakSelect } from "./ArtifactTweakSelect";
import { type ControlProps, short } from "./controls";

// TweakChoose is a tweak that's one of a few: a row of buttons where
// they're few and short, else a dropdown.
export function TweakChoose({ id, t, value, set }: ControlProps<string>) {
  const options = t.options ?? [];
  if (short(t)) return <ArtifactTweakChoice id={id} label={t.label} value={value} options={options} onChange={set} />;
  return <ArtifactTweakSelect id={id} label={t.label} value={value} options={options} onChange={set} />;
}
