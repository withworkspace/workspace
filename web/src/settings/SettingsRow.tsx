import type { ReactNode } from "react";
import { cx } from "../ui/cx";
import { SettingsControl } from "./SettingsControl";
import css from "./SettingsRow.module.css";
import { SettingsWhat } from "./SettingsWhat";

type SettingsRowProps = {
  // Its name, its key in the file, and what it says.
  label: ReactNode;
  tomlKey?: ReactNode;
  about?: ReactNode;
  // The section it's from, where it's out of its block (Find's results).
  crumb?: string;
  // wide puts the control under the words, for one that wants the room.
  wide?: boolean;
  // Its control.
  children: ReactNode;
  // Where its value comes from, under the control: a SettingsSource.
  source?: ReactNode;
};

// SettingsRow is a row of the Settings page: what a setting is and says,
// then its control and, under that, whose its value is.
export function SettingsRow({ label, tomlKey, about, crumb, wide, children, source }: SettingsRowProps) {
  return (
    <div className={cx(css.row, wide && css.wide)}>
      <SettingsWhat label={label} tomlKey={tomlKey} crumb={crumb} about={about} />
      <SettingsControl align={wide ? "start" : undefined}>
        {children}
        {source}
      </SettingsControl>
    </div>
  );
}
