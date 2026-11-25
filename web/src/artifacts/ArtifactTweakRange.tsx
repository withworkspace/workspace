import css from "./ArtifactTweakRange.module.css";

type ArtifactTweakRangeProps = {
  id: string;
  min: number;
  max: number;
  step: number;
  // Where it's at, as the slider takes it, and as it's said beside it.
  value: string;
  said: string;
  onChange: (value: string) => void;
};

// ArtifactTweakRange is a number between two ends: a slider, with what
// it's at beside it. It's the browser's own range, in the accent: the
// library has no slider.
export function ArtifactTweakRange({ id, min, max, step, value, said, onChange }: ArtifactTweakRangeProps) {
  return (
    <div className={css.range}>
      <input id={id} type="range" min={min} max={max} step={step} value={value} onChange={(e) => onChange(e.target.value)} />
      <output htmlFor={id}>{said}</output>
    </div>
  );
}
