import css from "./ArtifactTweakColour.module.css";

// ArtifactTweakColour is a tweak that's a colour: how it's written, and
// the browser's colour box, at the end of its label's line.
export function ArtifactTweakColour({ id, label, value, onChange }: { id: string; label: string; value: string; onChange: (value: string) => void }) {
  return (
    <span className={css.colour}>
      <span className="mono dim">{value}</span>
      <input id={id} type="color" value={value} aria-label={label} onChange={(e) => onChange(e.target.value)} />
    </span>
  );
}
