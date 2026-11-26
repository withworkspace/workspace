import type { ReactNode } from "react";
import { cx } from "../../ui/cx";
import { toneClass } from "../../ui/tone";
import css from "./MissionVerifyBundles.module.css";

// MissionVerifyBundles is the mission's bundles in a line: a label (To
// approve, in the attention tone, or that there's nothing to), then a
// MissionVerifyBundle each, and the rest behind a MissionVerifyMenu.
export function MissionVerifyBundles({ label, attention, children }: { label: ReactNode; attention?: boolean; children: ReactNode }) {
  return (
    <div className={css.bundles} role="group" aria-label="Verification bundles">
      <span className={cx(css.label, attention && toneClass("attention"), attention && css.attention)}>{label}</span>
      {children}
    </div>
  );
}
