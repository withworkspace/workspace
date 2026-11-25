import type { ReactNode } from "react";
import css from "./SettingsSide.module.css";

type SettingsSideProps = {
  title: ReactNode;
  // Find a setting: a SettingsFind.
  find: ReactNode;
  // The sections: a SettingsNav.
  children: ReactNode;
};

// SettingsSide is the column beside the sidebar: the page's name, Find,
// and the list of sections. It runs the page's height under a hairline,
// and what's in it stays put as the section scrolls. The shell finds it by
// data-settings-side, to centre the glow on the section's column.
export function SettingsSide({ title, find, children }: SettingsSideProps) {
  return (
    <aside className={css.side} aria-label="Settings" data-settings-side="">
      <div className={css.in}>
        <div className={css.title}>{title}</div>
        {find}
        {children}
      </div>
    </aside>
  );
}
