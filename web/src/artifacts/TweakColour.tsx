import { ArtifactTweakColour } from "./ArtifactTweakColour";
import { type ControlProps, six } from "./controls";
import { useShown } from "./useShown";

// TweakColour is a tweak that's a colour.
export function TweakColour({ id, t, value, set }: ControlProps<string>) {
  const [shown, setShown] = useShown(six(value));
  return (
    <ArtifactTweakColour
      id={id}
      label={t.label}
      value={shown}
      onChange={(v) => {
        setShown(v);
        set(v);
      }}
    />
  );
}
