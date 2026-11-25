import type { ReactNode } from "react";
import css from "./SettingsNav.module.css";

// SettingsNav is the list of the Settings page's sections, a
// SettingsNavLink each, under SettingsNavHeadings. Narrow, it's a strip
// along the top, without its headings.
export function SettingsNav({ children }: { children: ReactNode }) {
  return (
    <nav className={css.nav} aria-label="Sections">
      {children}
    </nav>
  );
}

// SettingsNavHeading is what the sections under it are about.
export function SettingsNavHeading({ children }: { children: ReactNode }) {
  return <h2 className={css.heading}>{children}</h2>;
}
