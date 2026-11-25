import type { ReactNode } from "react";
import { cx } from "../../ui/cx";
import { Icon, type IconName } from "../../ui/Icon";
import { toneClass } from "../../ui/tone";
import css from "./MissionBriefWhat.module.css";

type MissionBriefWhatProps = {
  icon: IconName;
  // In attention: there's something of it to read, or to save.
  tone?: "attention";
  title?: string;
  // What it says: a draft, an edit, as written.
  children: ReactNode;
};

// MissionBriefWhat is where the brief and plan stand, at the start of the
// tab's bar: an icon and a few words.
export function MissionBriefWhat({ icon, tone, title, children }: MissionBriefWhatProps) {
  return (
    <div className={cx(css.what, toneClass(tone), tone === "attention" && css.attention)} title={title}>
      <Icon name={icon} size={13} />
      <span className={css.text}>{children}</span>
    </div>
  );
}
