import type { ReactNode, Ref } from "react";
import { cx } from "../../ui/cx";
import { DirtyDot } from "../../ui/DirtyDot";
import { Icon } from "../../ui/Icon";
import { RepoDot } from "../../ui/RepoDot";
import css from "./FilesSwitch.module.css";

type FilesSwitchProps = {
  repo: string; // as ws labels it
  color?: string;
  // Which of its worktrees: the ground, or a capsule's name.
  name: string;
  ground?: boolean;
  dirty?: boolean;
  open: boolean;
  onToggle: () => void;
  // What's listed to switch to, while it's open (a FilesSwitchMenu).
  menu?: ReactNode;
  ref?: Ref<HTMLDivElement>;
};

// FilesSwitch is the title of a repo's worktree on the Files page: the
// repo and which of its worktrees this is, the ground or a capsule, and
// (open) the others, to switch to.
export function FilesSwitch({ repo, color, name, ground, dirty, open, onToggle, menu, ref }: FilesSwitchProps) {
  return (
    <div className={css.switch} ref={ref}>
      <button className={css.title} aria-haspopup="listbox" aria-expanded={open} onClick={onToggle} title={`${ground ? "The default branch, as it's checked out" : "A capsule"}. Switch to the ground or another capsule (w)`}>
        <RepoDot color={color} />
        <span className={cx(css.repo, css.cut)}>{repo}</span>
        <span className={css.sep}>/</span>
        <span className={cx(css.name, css.cut)}>{ground ? "ground" : name}</span>
        {dirty && !ground && <DirtyDot title="Changes not committed" />}
        <Icon name="chevron-down" size={13} className={css.chevron} />
      </button>
      {open && menu}
    </div>
  );
}
