import type { ReactNode } from "react";
import { Icon } from "../../ui/Icon";
import css from "./MissionWorkFileGroup.module.css";
import { MissionWorkLines } from "./MissionWorkLines";

type MissionWorkFileGroupProps = {
  // The folder, as it's said: demo/, or "top level".
  dir: string;
  // How many files of it changed, and by how many lines.
  n: number;
  additions: number;
  deletions: number;
  open: boolean;
  onToggle: () => void;
  // Its files, while it's open: a MissionWorkFile each.
  children?: ReactNode;
};

// MissionWorkFileGroup is a top-level folder of a capsule's changed
// files: its head, which folds it, over its files.
export function MissionWorkFileGroup({ dir, n, additions, deletions, open, onToggle, children }: MissionWorkFileGroupProps) {
  return (
    <div>
      <button className={css.head} aria-expanded={open} onClick={onToggle}>
        <Icon name="chevron-right" size={11} />
        <span className="mono">{dir}</span>
        <span className={css.n}>{n}</span>
        <MissionWorkLines additions={additions} deletions={deletions} tally />
      </button>
      {open && children}
    </div>
  );
}
