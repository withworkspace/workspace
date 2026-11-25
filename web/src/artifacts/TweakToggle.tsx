import { ArtifactTweakSwitch } from "./ArtifactTweakSwitch";
import type { ControlProps } from "./controls";
import { useShown } from "./useShown";

// TweakToggle is a tweak that's on or off.
export function TweakToggle({ id, t, value, set }: ControlProps<boolean>) {
  const [on, setOn] = useShown(value);
  return (
    <ArtifactTweakSwitch
      id={id}
      label={t.label}
      on={on}
      onChange={(v) => {
        setOn(v);
        set(v);
      }}
    />
  );
}
