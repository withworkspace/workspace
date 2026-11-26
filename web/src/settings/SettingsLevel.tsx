import type { Ref } from "react";
import css from "./SettingsLevel.module.css";

type SettingsLevelProps = {
  label: string;
  // What its two ends are: none, and the most.
  ends: [string, string];
  // Where it's at, 0 to 100: set, or being dragged.
  value: number;
  // The slider's own input, for its change (which comes when it's let go).
  slider?: Ref<HTMLInputElement>;
  // The slider moved: a level to wear, not yet kept.
  onPreview: (n: number) => void;
};

// SettingsLevel is how far a look goes (the accent's tint, the dark
// theme's depth): a slider from none to the most, with where it's at
// beside it, and under it the surfaces it changes as they are now.
export function SettingsLevel({ label, ends, value, slider, onPreview }: SettingsLevelProps) {
  return (
    <div className={css.level}>
      <div className={css.range}>
        <span className={css.end}>{ends[0]}</span>
        <input ref={slider} type="range" min={0} max={100} step={5} value={value} aria-label={label} onChange={(e) => onPreview(Number(e.target.value))} />
        <span className={css.end}>{ends[1]}</span>
        <output className={css.said}>{value}%</output>
      </div>
      <span className={css.shown} aria-hidden="true" title="The page, a panel, a card and a line, as they're drawn now">
        <span className={css.page} />
        <span className={css.panel} />
        <span className={css.raised} />
        <span className={css.line} />
      </span>
    </div>
  );
}
