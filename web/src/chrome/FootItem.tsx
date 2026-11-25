import type { MouseEvent, ReactNode } from "react";
import { Link } from "react-router";
import { cx } from "../ui/cx";
import { keyProps } from "../ui/keyHint";
import css from "./FootItem.module.css";

type FootItemProps = {
  // What it says when pointed at, and what it's called when it's only an
  // icon.
  title?: string;
  // The key that does what pressing it does, said after its title and
  // hinted at (ui/keyHint.ts); more is said after that.
  keys?: string;
  more?: string;
  label?: string;
  // What it opens is open (the sidebar's toggle). Leave it unsaid for a
  // part that isn't a toggle.
  on?: boolean;
  // It may be cut short when the foot hasn't the room.
  shrink?: boolean;
  // Somewhere to go, in place of something to press.
  to?: string;
  onPress?: () => void;
  // Its own menu, where it has more to it than pressing it does.
  onContextMenu?: (e: MouseEvent<Element>) => void;
  className?: string;
  children: ReactNode;
};

// FootItem is one part of the foot: a few words or an icon, pressed for
// what it opens, or a link to a place.
export function FootItem({
  title,
  keys,
  more,
  label,
  on,
  shrink,
  to,
  onPress,
  onContextMenu,
  className,
  children,
}: FootItemProps) {
  const cls = cx(css.item, on && css.on, shrink && css.shrink, className);
  const keyed = keyProps(keys, title, onPress && (() => onPress()));
  const says = [keyed.title, more].filter(Boolean).join(". ") || undefined;
  if (to !== undefined) {
    return (
      <Link
        to={to}
        title={says}
        onClick={keyed.onClick}
        aria-label={label}
        aria-current={on ? "page" : undefined}
        className={cls}
        onContextMenu={onContextMenu}
      >
        {children}
      </Link>
    );
  }
  return (
    <button
      type="button"
      data-ui=""
      title={says}
      aria-label={label}
      aria-pressed={on}
      className={cls}
      onClick={keyed.onClick}
      onContextMenu={onContextMenu}
    >
      {children}
    </button>
  );
}
