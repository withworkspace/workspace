import type { ReactNode } from "react";
import { cx } from "../../ui/cx";
import { DirtyDot } from "../../ui/DirtyDot";
import { keyProps } from "../../ui/keyHint";
import css from "./MissionSideCapsule.module.css";

// MissionSideCapsule is a capsule in a mission's details: its name, with a
// dot for changes not committed, and where it stands under (its badges).
// Pressed, it's reviewed; burned (gone), it can't be. id is what the
// page's keys find it by; selected, their cursor is on it. keys is the
// key that reviews it too (v), said after its title.
export function MissionSideCapsule({
  id,
  name,
  dirty,
  badges,
  selected,
  gone,
  title,
  keys,
  onReview,
}: {
  id: string;
  name: ReactNode;
  dirty?: boolean;
  badges?: ReactNode;
  selected?: boolean;
  gone?: boolean;
  title: string;
  keys?: string;
  onReview: () => void;
}) {
  return (
    <button id={id} className={cx(css.capsule, selected && css.selected)} {...keyProps(keys, title, onReview)} disabled={gone}>
      <span className={css.name}>
        {dirty && <DirtyDot title="Uncommitted changes" />}
        <span className={cx("mono", css.path)}>{name}</span>
      </span>
      {badges}
    </button>
  );
}
