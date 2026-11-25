import type { ReactNode } from "react";
import css from "./SettingsKey.module.css";

// SettingsKey is a setting's key as the file has it (crew.model), quiet
// beside its name.
export function SettingsKey({ children }: { children: ReactNode }) {
  return <code className={css.key}>{children}</code>;
}
