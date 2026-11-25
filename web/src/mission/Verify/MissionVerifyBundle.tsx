import type { ReactNode } from "react";
import { Count } from "../../ui/Count";
import { cx } from "../../ui/cx";
import css from "./MissionVerifyBundle.module.css";

type MissionVerifyBundleProps = {
  id: string;
  // What it's of: its step, or its capsule.
  title: ReactNode;
  // Where it stands: a MissionVerifyStanding.
  standing: ReactNode;
  // How many of its checks are left for you.
  left?: number;
  // It's the one that's open.
  selected?: boolean;
  // It's open without being one to approve: its edge is dashed.
  other?: boolean;
  onPick: () => void;
};

// MissionVerifyBundle is one bundle in the row of them: its name, what
// it's of, where it stands, and how many of its checks are left for you.
// Pressed, it opens.
export function MissionVerifyBundle({ id, title, standing, left = 0, selected, other, onPick }: MissionVerifyBundleProps) {
  return (
    <button type="button" className={cx(css.bundle, selected && css.selected, other && css.other)} aria-pressed={selected} onClick={onPick}>
      <span className="ref">{id}</span>
      <span className={css.title}>{title}</span>
      {standing}
      {left > 0 && (
        <Count tone="attention" title={`${left} of its checks ${left === 1 ? "is" : "are"} left for you`}>
          {left}
        </Count>
      )}
    </button>
  );
}
