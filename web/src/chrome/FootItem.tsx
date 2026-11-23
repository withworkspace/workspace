import type { ReactNode } from "react";
import { Link } from "react-router";
import { cx } from "../ui/cx";
import css from "./FootItem.module.css";

type FootItemProps = {
  // What it says when pointed at, and what it's called when it's only an
  // icon.
  title?: string;
  label?: string;
  // What it opens is open (the sidebar's toggle). Leave it unsaid for a
  // part that isn't a toggle.
  on?: boolean;
  // It may be cut short when the foot hasn't the room.
  shrink?: boolean;
  // Somewhere to go, in place of something to press.
  to?: string;
  onPress?: () => void;
  className?: string;
  children: ReactNode;
};

// FootItem is one part of the foot: a few words or an icon, pressed for
// what it opens, or a link to a place.
export function FootItem({
  title,
  label,
  on,
  shrink,
  to,
  onPress,
  className,
  children,
}: FootItemProps) {
  const cls = cx(css.item, on && css.on, shrink && css.shrink, className);
  if (to !== undefined) {
    return (
      <Link
        to={to}
        title={title}
        aria-label={label}
        aria-current={on ? "page" : undefined}
        className={cls}
      >
        {children}
      </Link>
    );
  }
  return (
    <button
      type="button"
      data-ui=""
      title={title}
      aria-label={label}
      aria-pressed={on}
      className={cls}
      onClick={onPress}
    >
      {children}
    </button>
  );
}
