import type { ReactNode } from "react";
import { cx } from "../../ui/cx";
import { Icon } from "../../ui/Icon";
import { toneClass } from "../../ui/tone";
import css from "./MissionVerifyGroup.module.css";

type MissionVerifyGroupProps = {
  title: ReactNode;
  // How many checks are in it.
  n: number;
  // Its checks are left for you: its head is in the attention tone.
  attention?: boolean;
  // It folds (the checks you've approved): its head is a button.
  fold?: { open: boolean; onToggle: () => void };
  // Its checks: a MissionVerifyRow each.
  children?: ReactNode;
};

// MissionVerifyGroup is some of a bundle's checks under a head: those
// left for you, those you've approved (folded: they're done with, and a
// click away), those settled without you.
export function MissionVerifyGroup({ title, n, attention, fold, children }: MissionVerifyGroupProps) {
  const says = (
    <>
      {title}
      <span className={css.n}>{n}</span>
    </>
  );
  return (
    <div className={cx(css.group, fold && css.folds)}>
      <h3 className={attention ? cx(toneClass("attention"), css.attention) : undefined}>
        {fold ? (
          <button type="button" className={css.toggle} aria-expanded={fold.open} onClick={fold.onToggle}>
            <Icon name={fold.open ? "chevron-down" : "chevron-right"} size={12} />
            {says}
          </button>
        ) : (
          says
        )}
      </h3>
      {children}
    </div>
  );
}
