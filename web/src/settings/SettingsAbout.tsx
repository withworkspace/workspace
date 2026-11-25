import type { ReactNode } from "react";
import css from "./SettingsAbout.module.css";

// SettingsAbout is what a block of the Settings page is about: a quiet
// paragraph under its heading.
export function SettingsAbout({ children }: { children: ReactNode }) {
  return <p className={css.about}>{children}</p>;
}
