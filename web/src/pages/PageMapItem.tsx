import type { CSSProperties, ReactNode } from "react";
import { Link } from "react-router";
import { cx } from "../ui/cx";
import css from "./PageMapItem.module.css";

const looks = { filled: css.filled, filling: css.filling, starting: css.starting, stale: css.stale, failed: css.failed, moved: css.moved, open: css.open };
export type MapItemLook = keyof typeof looks;

// PageMapItem is a thing on the map: pressed, it goes to it (onClick), or
// it's a link (to), or neither. Its look says where a cell or pass stands
// (filled, filling, stale…), a file that's moved since, or something open
// (a box before it).
export function PageMapItem({ look, to, onClick, title, style, children }: { look?: MapItemLook; to?: string; onClick?: () => void; title?: string; style?: CSSProperties; children: ReactNode }) {
  const className = cx(css.item, look && looks[look]);
  if (to)
    return (
      <Link to={to} className={className} title={title} style={style}>
        {children}
      </Link>
    );
  if (onClick)
    return (
      <button className={className} title={title} onClick={onClick} style={style}>
        {children}
      </button>
    );
  return (
    <span className={className} title={title} style={style}>
      {children}
    </span>
  );
}
