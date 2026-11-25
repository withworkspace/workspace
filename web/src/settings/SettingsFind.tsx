import type { Ref } from "react";
import { Icon } from "../ui/Icon";
import { Input } from "../ui/Input";
import { Keys } from "../ui/Keys";
import css from "./SettingsFind.module.css";

type SettingsFindProps = {
  ref?: Ref<HTMLInputElement>;
  value: string;
  onChange: (text: string) => void;
  // Escape: what's typed is taken away, and the field left.
  onClear: () => void;
};

// SettingsFind is Find a setting: the sidebar's Find, as a field.
export function SettingsFind({ ref, value, onChange, onClear }: SettingsFindProps) {
  return (
    <label className={css.find}>
      <Icon name="search" size={13} />
      <Input
        ref={ref}
        type="text"
        placeholder="Find a setting…"
        value={value}
        spellCheck={false}
        autoComplete="off"
        aria-label="Find a setting"
        onChange={(e) => onChange(e.target.value)}
        onKeyDown={(e) => {
          if (e.key !== "Escape") return;
          onClear();
          e.currentTarget.blur();
        }}
      />
      <Keys k="/" />
    </label>
  );
}
