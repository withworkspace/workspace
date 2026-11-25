import { ArtifactTweakNumber } from "./ArtifactTweakNumber";
import { ArtifactTweakRange } from "./ArtifactTweakRange";
import { type ControlProps, stepOf } from "./controls";
import { said, takes } from "./tweaks";
import { useShown } from "./useShown";

// TweakNumber is a slider, for a number with both ends, with what it's at
// beside it; else a box to type one in.
export function TweakNumber({ id, t, value, set }: ControlProps<number>) {
  const [text, setText] = useShown(String(value));
  const put = (s: string) => {
    setText(s);
    const n = Number(s);
    if (s.trim() !== "" && takes(t, n)) set(n);
  };
  if (t.min !== undefined && t.max !== undefined) {
    return <ArtifactTweakRange id={id} min={t.min} max={t.max} step={stepOf(t, t.min, t.max)} value={text} said={said(t, Number(text))} onChange={put} />;
  }
  return <ArtifactTweakNumber id={id} value={text} unit={t.unit} invalid={!takes(t, Number(text)) || text.trim() === ""} onChange={put} />;
}
