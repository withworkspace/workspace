import css from "./ArtifactTweakColour.module.css";
import { Text } from "../ui/Text";

// ArtifactTweakColour is a tweak that's a colour: how it's written, and
// the browser's colour box, at the end of its label's line.
export function ArtifactTweakColour({ id, label, value, onChange }: { id: string; label: string; value: string; onChange: (value: string) => void }) {
  return (
    <span className={css.colour}>
      <Text mono dim>{value}</Text>
      <input id={id} type="color" value={value} aria-label={label} onChange={(e) => onChange(e.target.value)} />
    </span>
  );
}
