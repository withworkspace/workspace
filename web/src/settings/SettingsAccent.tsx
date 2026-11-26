import type { CSSProperties, Ref } from "react";
import { cx } from "../ui/cx";
import { Input } from "../ui/Input";
import css from "./SettingsAccent.module.css";
import { SettingsNote } from "./SettingsNote";
import { textClass } from "../ui/Text";

type SettingsAccentProps = {
  label: string;
  // ws's own, and the others to pick from, each with its name.
  ours: string;
  accents: [hex: string, name: string][];
  // The one the page wears now: picked, or being picked.
  picked: string;
  // What the hex field says: what's being typed, or the one picked.
  hex: string;
  dark: boolean;
  // How this theme draws the pick, where it lightens or darkens it so it
  // reads on the page: which way, and as what.
  pulled?: { way: string; as: string };
  // The colour picker's own input, for its change (which comes when it's
  // let go).
  picker?: Ref<HTMLInputElement>;
  onOurs: () => void;
  onPick: (hex: string) => void;
  // The picker moved: a colour to wear, not yet kept.
  onPreview: (hex: string) => void;
  onHexFocus: () => void;
  onHexChange: (text: string) => void;
  onHexBlur: (text: string) => void;
  // Escape in the hex field: it's left as it was.
  onHexCancel: () => void;
};

const swatch = (hex: string) => ({ "--swatch": hex }) as CSSProperties;

// SettingsAccent is the accent's control: ws's own and a few others to
// pick from, a colour picker for any other, and the hex it's at. Under
// them is the accent as this theme shows it: in text, as a tint, and on
// the button.
export function SettingsAccent({ label, ours, accents, picked, hex, dark, pulled, picker, onOurs, onPick, onPreview, onHexFocus, onHexChange, onHexBlur, onHexCancel }: SettingsAccentProps) {
  const other = picked !== ours && !accents.some(([a]) => a === picked);
  return (
    <div className={css.accent}>
      <div className={css.swatches} role="group" aria-label={`${label} presets`}>
        <button type="button" className={css.swatch} style={swatch(ours)} aria-pressed={picked === ours} title="Workspace Violet" aria-label="Workspace Violet" onClick={onOurs} />
        {accents.map(([a, name]) => (
          <button key={a} type="button" className={css.swatch} style={swatch(a)} aria-pressed={picked === a} title={name} aria-label={name} onClick={() => picked !== a && onPick(a)} />
        ))}
        <label className={cx(css.swatch, css.any, other && css.on)} style={swatch(picked)} title="Custom colour">
          <input ref={picker} type="color" value={picked} aria-label={`${label}: custom colour`} onChange={(e) => onPreview(e.target.value)} />
        </label>
      </div>
      <div className={css.line}>
        <span className={css.shown} title={`Preview in ${dark ? "dark" : "light"} theme`}>
          <span className={css.shownText}>Text</span>
          <span className={css.shownTint}>Selected</span>
          <span className={css.shownButton}>Button</span>
        </span>
        <Input
          type="text"
          className={cx(textClass("mono"), css.hex)}
          value={hex}
          spellCheck={false}
          aria-label={`${label}, as hex`}
          onFocus={onHexFocus}
          onChange={(e) => onHexChange(e.target.value)}
          onBlur={(e) => onHexBlur(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") e.currentTarget.blur();
            if (e.key === "Escape") {
              onHexCancel();
              e.currentTarget.blur();
            }
          }}
        />
      </div>
      {pulled && (
        <SettingsNote>
          Shown {pulled.way} in the {dark ? "dark" : "light"} theme, as <code>{pulled.as}</code>, for
          contrast.
        </SettingsNote>
      )}
    </div>
  );
}
