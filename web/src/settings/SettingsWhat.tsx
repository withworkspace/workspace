import type { ReactNode } from "react";
import { cx } from "../ui/cx";
import { SettingsKey } from "./SettingsKey";
import css from "./SettingsWhat.module.css";

type SettingsWhatProps = {
  label: ReactNode;
  // Its key in the file, or a role's table ([crew]).
  tomlKey?: ReactNode;
  // The section it's from, said before its name.
  crumb?: string;
  about?: ReactNode;
  // More under what it says: why a role can't run.
  children?: ReactNode;
  // A class of what holds it, to place it.
  className?: string;
};

// SettingsWhat is what a setting is: its name, its key in the file, and
// what it says.
export function SettingsWhat({ label, tomlKey, crumb, about, children, className }: SettingsWhatProps) {
  return (
    <div className={cx(css.what, className)}>
      <div className={css.label}>
        {crumb && <span className={css.crumb}>{crumb} ·</span>}
        <span>{label}</span>
        {tomlKey && <SettingsKey>{tomlKey}</SettingsKey>}
      </div>
      {about && <div className={css.about}>{about}</div>}
      {children}
    </div>
  );
}
