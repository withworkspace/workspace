import type { ReactNode } from "react";
import css from "./SettingsRows.module.css";

// SettingsRows is a block's rows, a SettingsRow each, a hairline between
// them.
export function SettingsRows({ children }: { children: ReactNode }) {
  return <div className={css.rows}>{children}</div>;
}
