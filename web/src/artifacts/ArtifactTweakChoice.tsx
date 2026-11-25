import { Segment, Segmented } from "../ui/Segmented";
import css from "./ArtifactTweakChoice.module.css";
import type { Choice } from "./tweaks";

// ArtifactTweakChoice is a tweak that's one of a few, as a row of
// buttons: a Segmented, no wider than the tray.
export function ArtifactTweakChoice({ id, label, value, options, onChange }: { id: string; label: string; value: string; options: Choice[]; onChange: (value: string) => void }) {
  return (
    <Segmented id={id} className={css.choice} value={value} onChange={onChange} aria-label={label}>
      {options.map((o) => (
        <Segment key={o.value} value={o.value} title={o.description || undefined}>
          {o.label}
        </Segment>
      ))}
    </Segmented>
  );
}
