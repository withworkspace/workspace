import { ArtifactTweakText } from "./ArtifactTweakText";
import type { ControlProps } from "./controls";
import { takes } from "./tweaks";
import { useShown } from "./useShown";

// TweakWords is a tweak that's a word or a line.
export function TweakWords({ id, t, value, set }: ControlProps<string>) {
  const [text, setText] = useShown(value);
  return (
    <ArtifactTweakText
      id={id}
      value={text}
      onChange={(v) => {
        setText(v);
        if (takes(t, v)) set(v);
      }}
    />
  );
}
