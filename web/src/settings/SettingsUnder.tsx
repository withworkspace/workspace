import type { ReactNode } from "react";
import css from "./SettingsUnder.module.css";

type SettingsUnderProps = {
  // What the blocks after it are, together (Plugins).
  title: ReactNode;
  // What they're about, under it.
  about?: ReactNode;
};

// SettingsUnder heads a run of blocks of a section that are one of a
// kind: Notifications' blocks, one per plugin, under Plugins. Each block
// keeps its own heading under it.
export function SettingsUnder({ title, about }: SettingsUnderProps) {
  return (
    <div className={css.under}>
      <h2 className={css.title}>{title}</h2>
      {about && <p className={css.about}>{about}</p>}
    </div>
  );
}
