import type { Ref } from "react";
import css from "./SettingsTint.module.css";

type SettingsTintProps = {
  label: string;
  // Where it's at, 0 to 100: set, or being dragged.
  value: number;
  // The slider's own input, for its change (which comes when it's let go).
  slider?: Ref<HTMLInputElement>;
  // The slider moved: a tint to wear, not yet kept.
  onPreview: (n: number) => void;
};

// SettingsTint is how far the accent tints the greys: a slider from none
// to the most, with where it's at beside it, and under it the surfaces it
// tints as they are now.
export function SettingsTint({ label, value, slider, onPreview }: SettingsTintProps) {
  return (
    <div className={css.tint}>
      <div className={css.range}>
        <span className={css.end}>Grey</span>
        <input ref={slider} type="range" min={0} max={100} step={5} value={value} aria-label={label} onChange={(e) => onPreview(Number(e.target.value))} />
        <span className={css.end}>Tinted</span>
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
