import type { ReactNode } from "react";
import { Button } from "../ui/Button";
import { cx } from "../ui/cx";
import css from "./SettingsSource.module.css";

type SettingsSourceProps = {
  // yours is a dot in the accent; the team's, and this browser's, a ring.
  yours?: boolean;
  // Whose, in words, and what it's over.
  children: ReactNode;
  title?: string;
  // Reset takes yours away: there when it's given.
  onReset?: () => void;
  resetTitle?: string;
};

// SettingsSource says where a setting's value comes from, when it isn't
// ws's default: the team's file, yours (with the way back), or this
// browser.
export function SettingsSource({ yours, children, title, onReset, resetTitle }: SettingsSourceProps) {
  return (
    <div className={css.source} title={title}>
      <span className={cx(css.dot, yours ? css.yours : css.ring)} />
      {children}
      {onReset && (
        <Button variant="link" className={css.action} onClick={onReset} title={resetTitle}>
          Reset
        </Button>
      )}
    </div>
  );
}
