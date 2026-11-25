import type { ReactNode } from "react";
import { cx } from "../../ui/cx";
import { DirtyDot } from "../../ui/DirtyDot";
import { Icon } from "../../ui/Icon";
import css from "./FilesSwitchMenu.module.css";

// FilesSwitchMenu is what a repo's worktree switches to: its ground and
// each of its capsules (FilesSwitchItems), or what's said with no
// capsules (none).
export function FilesSwitchMenu({ label, none, children }: { label: string; none?: ReactNode; children: ReactNode }) {
  return (
    <ul className={css.menu} role="listbox" aria-label={label}>
      {children}
      {none && <li className={cx("dim", css.none)}>{none}</li>}
    </ul>
  );
}

type FilesSwitchItemProps = {
  name: string;
  ground?: boolean;
  // What it is: its branch, or the mission it's for.
  sub: string;
  ahead?: number;
  behind?: number;
  dirty?: boolean;
  // It's the one this is.
  current?: boolean;
  // The keys are on it.
  on?: boolean;
  onHover: () => void;
  onPick: () => void;
};

// FilesSwitchItem is one of a repo's worktrees to switch to: what it is,
// how far it is from its upstream, and whether it has changes.
export function FilesSwitchItem({ name, ground, sub, ahead, behind, dirty, current, on, onHover, onPick }: FilesSwitchItemProps) {
  return (
    <li role="option" aria-selected={current} className={cx(on && css.on)} onMouseMove={onHover} onClick={onPick}>
      <Icon name={ground ? "layers" : "box"} size={14} className="dim" />
      <span className={css.item}>
        <span className={css.name}>{ground ? "ground" : name}</span>
        <span className={css.sub}>{sub}</span>
      </span>
      {!!ahead && (
        <span className={css.ab} title={`${ahead} ahead of its upstream`}>
          ↑{ahead}
        </span>
      )}
      {!!behind && (
        <span className={css.ab} title={`${behind} behind its upstream`}>
          ↓{behind}
        </span>
      )}
      {dirty && <DirtyDot title="Changes not committed" />}
      {current && <Icon name="check" size={13} className={css.check} />}
    </li>
  );
}
