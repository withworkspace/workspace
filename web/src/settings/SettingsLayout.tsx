import type { ReactNode } from "react";
import { cx } from "../ui/cx";
import css from "./SettingsLayout.module.css";

type SettingsLayoutProps = {
  // The list of sections: a SettingsSide.
  side: ReactNode;
  // The section you're in, or what Find found.
  children: ReactNode;
};

// SettingsLayout is the Settings page's two columns: the list of its
// sections docked beside the sidebar, and the section you're in centred in
// what's left, to a width. Narrow, the list runs along the top.
export function SettingsLayout({ side, children }: SettingsLayoutProps) {
  return (
    <div className={cx("page", css.page)}>
      {side}
      <div className={css.body}>{children}</div>
    </div>
  );
}
