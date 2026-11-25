import type { ReactNode } from "react";
import { Icon } from "../../ui/Icon";
import css from "./MissionWorkSub.module.css";

// MissionWorkSub is a quiet word over some of a group's rows: In hand, Up
// next, Done. Given onToggle it's a button that folds its rows away, and
// rest is what it says of them in a line while they're folded.
export function MissionWorkSub({ children, open, onToggle, rest }: { children: ReactNode; open?: boolean; onToggle?: () => void; rest?: ReactNode }) {
  if (!onToggle) return <div className={css.sub}>{children}</div>;
  return (
    <button className={css.sub} aria-expanded={open} onClick={onToggle}>
      <Icon name="chevron-right" size={11} />
      {children}
      {!open && rest && <span className={css.rest}>· {rest}</span>}
    </button>
  );
}
